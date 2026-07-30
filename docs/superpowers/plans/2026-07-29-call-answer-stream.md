# Call Answer + Media Stream Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `POST /call/answer`, `POST /call/hangup`, and a dedicated WebSocket bridge (`GET /call/stream/:callId`) to Evolution Go so an accepted WhatsApp call's audio/video can be piped to any external consumer (AI voice pipeline, recorder, human console).

**Architecture:** `github.com/purpshell/meowcaller` wraps each instance's `*whatsmeow.Client` and is constructed the moment that client is created (before `.Connect()`), exactly like `pkg/whatsmeow/service` already tracks a `map[string]*whatsmeow.Client`. Incoming `*meowcaller.Call` handles are captured via `OnIncomingCall` into a small shared registry (`pkg/call/registry`), keyed by call ID and instance ID. `pkg/call/service` answers/hangs up calls by looking them up in that registry. A new `pkg/call/stream` package bridges a `*meowcaller.Call`'s audio/video sinks and source to a per-call WebSocket connection, using a Twilio-Media-Streams-style JSON envelope.

**Tech Stack:** Go 1.25, gin, gorilla/websocket (already a dependency via `pkg/events/websocket`), `go.mau.fi/whatsmeow`, `github.com/purpshell/meowcaller` (new, MIT-licensed).

## Global Constraints

- `meowcaller.NewClient(wa)` MUST be called before `wa.Connect()` — this is a hard requirement from the library's own doc comment, not a style choice.
- No AI/STT/LLM/TTS logic in this repo — this PR ships transport/plumbing only (per approved design spec `docs/superpowers/specs/2026-07-29-call-answer-stream-design.md`).
- Video is inbound-only (record/forward the peer's video); outbound synthetic video is out of scope.
- Follow the existing Handler → Service → Repository convention and package-alias-as-name-with-underscore style already used throughout (`call_service`, `call_handler`, etc.) — see `docs/wiki/desenvolvimento/contributing.md`.
- Audio wire format: 16 kHz mono, 960-sample (60 ms) frames, `float32` inside the process (`meowcaller.SampleRate`, `meowcaller.FrameSamples`), PCM16LE + base64 on the wire.
- Run `make fmt` and `make lint` before every commit that touches Go files (repo convention).

---

### Task 1: Add the `meowcaller` dependency and verify the build

**Files:**
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Produces: `github.com/purpshell/meowcaller` importable at `v0.0.0-...` (pseudo-version, no tagged releases yet), and `go.mau.fi/whatsmeow` bumped to at least `v0.0.0-20260722203353-e9a033b24933` (meowcaller's floor; Go's minimal version selection will pick the higher of the two requirements automatically).

- [ ] **Step 1: Add the dependency**

```bash
cd /home/rarosh/projetos/evolution-go
go get github.com/purpshell/meowcaller@latest
```

- [ ] **Step 2: Verify the whatsmeow bump didn't break anything**

**Do NOT run `go mod tidy` in this task.** No `.go` file imports `meowcaller` yet — that
starts in Task 3 — and `go mod tidy` removes `require` entries for modules nothing in
the module actually imports. Run it now and it will silently delete the line `go get`
just added, and `go build ./...` will still pass (nothing references the module, so its
absence doesn't break compilation) — meaning a clean build here is not proof the
dependency is actually present. Confirm it explicitly instead:

```bash
grep meowcaller go.mod
```

Expected: one line, `github.com/purpshell/meowcaller v0.0.0-...`. If it's not there, the
`go get` in Step 1 didn't take — re-run it and check again before moving on.

```bash
go build ./...
```

Expected: exits 0. If it fails, the failures will be in `pkg/whatsmeow/service/whatsmeow.go` from whatsmeow API drift between the pinned versions (2026-06-30 → 2026-07-22) — read the compiler errors, they'll name the exact symbols that moved; fix those call sites before continuing (do not downgrade whatsmeow back down, meowcaller requires the newer floor).

(Task 8 Step 6-7, once `meowcaller` is actually imported by real code, is the right time to run `go mod tidy` — it will no longer have anything unused to strip.)

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "feat: add meowcaller dependency for WhatsApp call media"
```

---

### Task 2: Call registry (shared between whatsmeow_service and call_service)

**Files:**
- Create: `pkg/call/registry/call_registry.go`
- Test: `pkg/call/registry/call_registry_test.go`

**Interfaces:**
- Produces:
  - `type CallRegistry struct` (opaque, all fields unexported)
  - `func NewCallRegistry() *CallRegistry`
  - `func (r *CallRegistry) Store(instanceID string, call *meowcaller.Call)`
  - `func (r *CallRegistry) Get(instanceID, callID string) (*meowcaller.Call, bool)` — returns `false` if the call isn't registered *or* belongs to a different instance (this is the security boundary that keeps one instance's apikey from reaching into another instance's call)
  - `func (r *CallRegistry) Delete(callID string)`

This is its own leaf package (not inside `pkg/call/service`) specifically to avoid an import cycle: `pkg/call/service` already imports `pkg/whatsmeow/service`, and `pkg/whatsmeow/service` needs to write into this registry too — if the registry lived inside `pkg/call/service`, that would make `pkg/whatsmeow/service` import `pkg/call/service` while `pkg/call/service` imports `pkg/whatsmeow/service`, which Go rejects.

- [ ] **Step 1: Write the failing test**

```go
// pkg/call/registry/call_registry_test.go
package call_registry

import (
	"testing"

	"github.com/purpshell/meowcaller"
)

func TestStoreAndGet(t *testing.T) {
	r := NewCallRegistry()
	call := &meowcaller.Call{}

	r.Store("instance-a", call)

	got, ok := r.Get("instance-a", callIDOf(call))
	if !ok {
		t.Fatal("expected call to be found")
	}
	if got != call {
		t.Fatal("expected the same call pointer back")
	}
}

func TestGetWrongInstanceFails(t *testing.T) {
	r := NewCallRegistry()
	call := &meowcaller.Call{}
	r.Store("instance-a", call)

	_, ok := r.Get("instance-b", callIDOf(call))
	if ok {
		t.Fatal("expected lookup from a different instance to fail")
	}
}

func TestDeleteRemovesEntry(t *testing.T) {
	r := NewCallRegistry()
	call := &meowcaller.Call{}
	r.Store("instance-a", call)
	r.Delete(callIDOf(call))

	_, ok := r.Get("instance-a", callIDOf(call))
	if ok {
		t.Fatal("expected entry to be gone after Delete")
	}
}

// callIDOf mirrors what CallRegistry.Store keys entries by: meowcaller.Call.ID().
// A zero-value *meowcaller.Call has an empty string ID, which is a perfectly valid
// (if degenerate) key for exercising Store/Get/Delete without a live call.
func callIDOf(call *meowcaller.Call) string {
	return call.ID()
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/call/registry/... -v`
Expected: FAIL — `call_registry.go` doesn't exist yet, compile error `undefined: NewCallRegistry`.

- [ ] **Step 3: Write the implementation**

```go
// pkg/call/registry/call_registry.go
package call_registry

import (
	"sync"

	"github.com/purpshell/meowcaller"
)

// entry pairs a live call with the instance it belongs to, so Get can refuse to hand
// back a call to a caller authenticated as a different instance.
type entry struct {
	instanceID string
	call       *meowcaller.Call
}

// CallRegistry tracks in-progress meowcaller calls by call ID, scoped by instance.
// It is written from the whatsmeow event-handling goroutine (on incoming call) and
// read/written from HTTP handler goroutines (answer/hangup/stream), so all access is
// mutex-guarded.
type CallRegistry struct {
	mu      sync.RWMutex
	entries map[string]entry
}

// NewCallRegistry returns an empty registry.
func NewCallRegistry() *CallRegistry {
	return &CallRegistry{entries: make(map[string]entry)}
}

// Store records call under its own ID (meowcaller.Call.ID()), tagged with instanceID.
func (r *CallRegistry) Store(instanceID string, call *meowcaller.Call) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[call.ID()] = entry{instanceID: instanceID, call: call}
}

// Get returns the call for callID, but only if it was stored under instanceID.
func (r *CallRegistry) Get(instanceID, callID string) (*meowcaller.Call, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[callID]
	if !ok || e.instanceID != instanceID {
		return nil, false
	}
	return e.call, true
}

// Delete removes callID regardless of which instance it belongs to.
func (r *CallRegistry) Delete(callID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, callID)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/call/registry/... -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add pkg/call/registry
git commit -m "feat: add call registry for tracking answerable meowcaller calls"
```

---

### Task 3: Wire meowcaller into `pkg/whatsmeow/service`

**Files:**
- Modify: `pkg/whatsmeow/service/whatsmeow.go`

**Interfaces:**
- Consumes: `call_registry.CallRegistry` from Task 2 (`Store`, `Delete`)
- Produces: `whatsmeowService.meowcallerPointer map[string]*meowcaller.Client` (internal, not part of the `WhatsmeowService` interface — nothing outside this file needs to reach into it), and `NewWhatsmeowService` gains one new parameter.

No automated test for this task: it's wiring inside a 2800-line file that drives a live WhatsApp connection, already untested elsewhere in the repo for the same reason (`ClientData`/`MyClient` flow has zero existing test coverage). Verified by `go build` here and by the manual end-to-end test in Task 9.

- [ ] **Step 1: Add the import**

In `pkg/whatsmeow/service/whatsmeow.go`, add to the import block (alongside the other `github.com/evolution-foundation/evolution-go/...` and third-party imports):

```go
	call_registry "github.com/evolution-foundation/evolution-go/pkg/call/registry"
	"github.com/purpshell/meowcaller"
```

- [ ] **Step 2: Add the two new struct fields**

Find the `whatsmeowService` struct (currently starts `type whatsmeowService struct {` around line 78). Add two fields, right after `clientPointer      map[string]*whatsmeow.Client`:

```go
	clientPointer      map[string]*whatsmeow.Client
	meowcallerPointer  map[string]*meowcaller.Client
	callRegistry       *call_registry.CallRegistry
```

- [ ] **Step 3: Add the new constructor parameter**

Find `func NewWhatsmeowService(` (around line 2793). Add `callRegistry *call_registry.CallRegistry` as a new parameter right before `loggerWrapper *logger_wrapper.LoggerManager`:

```go
func NewWhatsmeowService(
	instanceRepository instance_repository.InstanceRepository,
	authDB *sql.DB,
	messageRepository message_repository.MessageRepository,
	labelRepository label_repository.LabelRepository,
	config *config.Config,
	killChannel map[string](chan bool),
	clientPointer map[string]*whatsmeow.Client,
	rabbitmqProducer producer_interfaces.Producer,
	webhookProducer producer_interfaces.Producer,
	websocketProducer producer_interfaces.Producer,
	sqliteDB *sql.DB,
	exPath string,
	mediaStorage storage_interfaces.MediaStorage,
	natsProducer producer_interfaces.Producer,
	callRegistry *call_registry.CallRegistry,
	loggerWrapper *logger_wrapper.LoggerManager,
) WhatsmeowService {
```

And in the returned `&whatsmeowService{...}` literal in the same function, add two lines (right after `clientPointer:      clientPointer,`):

```go
		clientPointer:      clientPointer,
		meowcallerPointer:  make(map[string]*meowcaller.Client),
		callRegistry:       callRegistry,
```

- [ ] **Step 4: Construct the meowcaller client at the exact point whatsmeow's client is created**

In `func (w whatsmeowService) StartClient(cd *ClientData)`, find this existing line (around line 413-415):

```go
	client := whatsmeow.NewClient(deviceStore, clientLog)

	w.clientPointer[cd.Instance.Id] = client
```

Change it to:

```go
	client := whatsmeow.NewClient(deviceStore, clientLog)

	w.clientPointer[cd.Instance.Id] = client

	// meowcaller.NewClient must run before client.Connect() — it installs the raw
	// <call> stanza adapter, and doing so after Connect() is a documented race.
	meowcallerClient := meowcaller.NewClient(client)
	w.meowcallerPointer[cd.Instance.Id] = meowcallerClient
	instanceID := cd.Instance.Id
	meowcallerClient.OnIncomingCall(func(call *meowcaller.Call) {
		w.loggerWrapper.GetLogger(instanceID).LogInfo("[%s] meowcaller captured incoming call %s from %s", instanceID, call.ID(), call.Peer().String())
		w.callRegistry.Store(instanceID, call)
	})
```

(`instanceID` is captured into a local variable before the closure so the closure doesn't capture the loop-mutated `cd` — `cd` itself isn't reused in a loop here, but this keeps the closure's captured value explicit and independent of whatever happens to `cd` later in this same function.)

- [ ] **Step 5: Clean up the registry when whatsmeow itself reports the call ended**

Find the existing `case *events.CallTerminate:` block (around line 1939):

```go
	case *events.CallTerminate:
		doWebhook = true
		postMap["event"] = "CallTerminate"
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Got call terminate %+v", mycli.userID, evt)
```

Add one line so a call that's never answered (or whose stream socket never connects) doesn't leak in the registry forever:

```go
	case *events.CallTerminate:
		doWebhook = true
		postMap["event"] = "CallTerminate"
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Got call terminate %+v", mycli.userID, evt)
		mycli.service.(*whatsmeowService).callRegistry.Delete(evt.CallID)
```

Note the `mycli.service.(*whatsmeowService)` type assertion: `mycli.service` is typed as the `WhatsmeowService` interface (see the `MyClient` struct's `service WhatsmeowService` field), which doesn't expose `callRegistry` — and it shouldn't, nothing outside this file needs to touch the registry directly. The type assertion is safe here because `mycli.service` is always set to `&w` (a `*whatsmeowService`) at construction (see the existing `service: &w,` in the `&MyClient{...}` literal a few dozen lines above `StartClient`'s call-event switch) and this code lives in the same package.

- [ ] **Step 6: Build just this package**

```bash
go build ./pkg/whatsmeow/service/...
```

Expected: exits 0. (Don't run `go build ./...` yet — `cmd/evolution-go/main.go` still calls the old-arity `NewWhatsmeowService` and won't compile until Task 8 fixes that call site.)

- [ ] **Step 7: Commit**

```bash
git add pkg/whatsmeow/service/whatsmeow.go
git commit -m "feat: capture incoming meowcaller calls into the call registry"
```

---

### Task 4: `AnswerCall` / `HangupCall` / `GetActiveCall` in `pkg/call/service`

**Files:**
- Modify: `pkg/call/service/call_service.go`

**Interfaces:**
- Consumes: `call_registry.CallRegistry.Get`/`Delete` (Task 2); `whatsmeow_service.NewWhatsmeowService`'s new `callRegistry` param (Task 3, for the constructor wiring done in Task 8)
- Produces:
  - `type AnswerCallStruct struct { CallCreator types.JID; CallID string }` (JSON: `callCreator`, `callId`)
  - `type HangupCallStruct struct { CallID string }` (JSON: `callId`)
  - `CallService.AnswerCall(data *AnswerCallStruct, instance *instance_model.Instance) (*meowcaller.Call, error)`
  - `CallService.HangupCall(data *HangupCallStruct, instance *instance_model.Instance) error`
  - `CallService.GetActiveCall(instanceId, callId string) (*meowcaller.Call, error)` — used by `pkg/call/stream` (Task 7)

No automated test here: `*meowcaller.Call`'s exported methods (`Answer`, `Hangup`, `AcceptVideo`) all dereference an unexported, unset-from-outside-the-package `eng *engine` field — calling them on a zero-value `&meowcaller.Call{}` (the only kind of `Call` this package can construct in a test) panics with a nil pointer dereference. There is no seam to inject a fake without forking meowcaller. This is exactly the kind of external, un-mockable dependency the design spec's Testing section already called out — verified instead by the manual real-call test in Task 9.

- [ ] **Step 1: Add imports**

In `pkg/call/service/call_service.go`, add to the import block:

```go
	call_registry "github.com/evolution-foundation/evolution-go/pkg/call/registry"
	"github.com/purpshell/meowcaller"
```

- [ ] **Step 2: Add the new struct types**

Right after the existing `RejectCallStruct` definition:

```go
type RejectCallStruct struct {
	CallCreator types.JID `json:"callCreator"`
	CallID      string    `json:"callId"`
}

type AnswerCallStruct struct {
	CallCreator types.JID `json:"callCreator"`
	CallID      string    `json:"callId"`
}

type HangupCallStruct struct {
	CallID string `json:"callId"`
}
```

- [ ] **Step 3: Add `callRegistry` to the service struct and constructor**

```go
type callService struct {
	clientPointer    map[string]*whatsmeow.Client
	whatsmeowService whatsmeow_service.WhatsmeowService
	callRegistry     *call_registry.CallRegistry
	loggerWrapper    *logger_wrapper.LoggerManager
}
```

```go
func NewCallService(
	clientPointer map[string]*whatsmeow.Client,
	whatsmeowService whatsmeow_service.WhatsmeowService,
	callRegistry *call_registry.CallRegistry,
	loggerWrapper *logger_wrapper.LoggerManager,
) CallService {
	return &callService{
		clientPointer:    clientPointer,
		whatsmeowService: whatsmeowService,
		callRegistry:     callRegistry,
		loggerWrapper:    loggerWrapper,
	}
}
```

- [ ] **Step 4: Extend the `CallService` interface**

```go
type CallService interface {
	RejectCall(data *RejectCallStruct, instance *instance_model.Instance) error
	AnswerCall(data *AnswerCallStruct, instance *instance_model.Instance) (*meowcaller.Call, error)
	HangupCall(data *HangupCallStruct, instance *instance_model.Instance) error
	GetActiveCall(instanceId, callId string) (*meowcaller.Call, error)
}
```

- [ ] **Step 5: Implement the three methods**

Add after the existing `RejectCall` method:

```go
func (c *callService) AnswerCall(data *AnswerCallStruct, instance *instance_model.Instance) (*meowcaller.Call, error) {
	call, ok := c.callRegistry.Get(instance.Id, data.CallID)
	if !ok {
		return nil, errors.New("no pending call with that id")
	}

	if err := call.Answer(); err != nil {
		logger.LogError("[%s] error answering call: %v", instance.Id, err)
		return nil, err
	}

	if call.IsVideo() {
		if err := call.AcceptVideo(); err != nil {
			c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] answered call but failed to accept video: %v", instance.Id, err)
		}
	}

	return call, nil
}

func (c *callService) HangupCall(data *HangupCallStruct, instance *instance_model.Instance) error {
	call, ok := c.callRegistry.Get(instance.Id, data.CallID)
	if !ok {
		return errors.New("no active call with that id")
	}

	err := call.Hangup()
	c.callRegistry.Delete(data.CallID)
	if err != nil {
		logger.LogError("[%s] error hanging up call: %v", instance.Id, err)
		return err
	}
	return nil
}

func (c *callService) GetActiveCall(instanceId, callId string) (*meowcaller.Call, error) {
	call, ok := c.callRegistry.Get(instanceId, callId)
	if !ok {
		return nil, errors.New("no active call with that id")
	}
	return call, nil
}
```

- [ ] **Step 6: Build just this package**

```bash
go build ./pkg/call/service/...
```

Expected: exits 0. (Don't run `go build ./...` yet — `cmd/evolution-go/main.go` still calls the old 3-argument `NewCallService` and won't compile until Task 8 fixes that call site; building only this package sidesteps that known, temporary gap.)

- [ ] **Step 7: Commit**

```bash
git add pkg/call/service/call_service.go
git commit -m "feat: add AnswerCall, HangupCall, GetActiveCall to call service"
```

---

### Task 5: `AnswerCall` / `HangupCall` handlers in `pkg/call/handler`

**Files:**
- Modify: `pkg/call/handler/call_handler.go`

**Interfaces:**
- Consumes: `call_service.CallService.AnswerCall`, `.HangupCall` (Task 4)
- Produces: `CallHandler.AnswerCall(ctx *gin.Context)`, `CallHandler.HangupCall(ctx *gin.Context)`

- [ ] **Step 1: Extend the `CallHandler` interface**

```go
type CallHandler interface {
	RejectCall(ctx *gin.Context)
	AnswerCall(ctx *gin.Context)
	HangupCall(ctx *gin.Context)
}
```

- [ ] **Step 2: Implement `AnswerCall`**

Add after the existing `RejectCall` method, following its exact shape:

```go
// Answer call
// @Summary Answer call
// @Description Answer an incoming call and (if it's a video call) accept its video
// @Tags Call
// @Accept json
// @Produce json
// @Param message body call_service.AnswerCallStruct true "Call data"
// @Success 200 {object} gin.H "success"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /call/answer [post]
func (g *callHandler) AnswerCall(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *call_service.AnswerCallStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = g.callService.AnswerCall(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}
```

- [ ] **Step 3: Implement `HangupCall`**

```go
// Hangup call
// @Summary Hangup call
// @Description Hangup an active call
// @Tags Call
// @Accept json
// @Produce json
// @Param message body call_service.HangupCallStruct true "Call data"
// @Success 200 {object} gin.H "success"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /call/hangup [post]
func (g *callHandler) HangupCall(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *call_service.HangupCallStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = g.callService.HangupCall(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}
```

- [ ] **Step 4: Build just this package**

```bash
go build ./pkg/call/handler/...
```

Expected: exits 0.

- [ ] **Step 5: Commit**

```bash
git add pkg/call/handler/call_handler.go
git commit -m "feat: add answer and hangup call HTTP handlers"
```

---

### Task 6: Register the new routes

**Files:**
- Modify: `pkg/routes/routes.go`

**Interfaces:**
- Consumes: `call_handler.CallHandler.AnswerCall`, `.HangupCall` (Task 5)

- [ ] **Step 1: Add the two routes to the existing `/call` group**

Find (around line 192-198):

```go
	routes = eng.Group("/call")
	{
		routes.Use(r.authMiddleware.Auth)
		{
			routes.POST("/reject", r.jidValidationMiddleware.ValidateNumberField(), r.callHandler.RejectCall)
		}
	}
```

Change to:

```go
	routes = eng.Group("/call")
	{
		routes.Use(r.authMiddleware.Auth)
		{
			routes.POST("/reject", r.jidValidationMiddleware.ValidateNumberField(), r.callHandler.RejectCall)
			routes.POST("/answer", r.jidValidationMiddleware.ValidateNumberField(), r.callHandler.AnswerCall)
			routes.POST("/hangup", r.callHandler.HangupCall)
		}
	}
```

(`/answer`'s body has a `callCreator` field, same shape as `/reject`, so it gets the same `ValidateNumberField()` call for consistency with its sibling route — note this middleware validates a field literally named `number`, which neither `/reject` nor `/answer` actually send, so in practice it's a no-op passthrough for both; that's pre-existing behavior in this file, not something this task changes. `/hangup`'s body has no JID-shaped field, so it gets no JID middleware, matching how routes without JID fields elsewhere in this file skip it too.)

`GET /call/stream/:callId` is intentionally **not** added here — it needs query-string apikey auth (WebSocket clients can't always set custom headers), which doesn't fit `authMiddleware.Auth`'s header-only check. Task 7 registers it directly on the `*gin.Engine`, the same way `pkg/passkey/handler` already registers its own public routes outside this file (see `passkey_handler.RegisterRoutes(r, whatsmeowService)` in `cmd/evolution-go/main.go`).

- [ ] **Step 2: Build just this package**

```bash
go build ./pkg/routes/...
```

Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
git add pkg/routes/routes.go
git commit -m "feat: register call answer and hangup routes"
```

---

### Task 7: `pkg/call/stream` — the WebSocket media bridge

**Files:**
- Create: `pkg/call/stream/codec.go`
- Test: `pkg/call/stream/codec_test.go`
- Create: `pkg/call/stream/bridge.go`
- Create: `pkg/call/stream/handler.go`

**Interfaces:**
- Consumes: `call_service.CallService.GetActiveCall` (Task 4); `instance_service.InstanceService.GetInstanceByToken` (existing); `meowcaller.SampleRate`, `meowcaller.AudioSink`, `meowcaller.AudioSource`, `meowcaller.VideoSink`, `meowcaller.Call.{Receive,ReceiveVideo,Play,OnEnd,Hangup,IsVideo,ID}` (external library)
- Produces: `call_stream.RegisterRoutes(r *gin.Engine, callService call_service.CallService, instanceService instance_service.InstanceService)`

#### Part A: pure PCM16LE ⇄ float32 conversion (TDD, no I/O)

- [ ] **Step 1: Write the failing test**

```go
// pkg/call/stream/codec_test.go
package call_stream

import "testing"

func TestPCM16RoundTrip(t *testing.T) {
	frame := []float32{0, 0.5, -0.5, 1, -1, 0.25}

	pcm := pcm16FromFloat32(frame)
	if len(pcm) != len(frame)*2 {
		t.Fatalf("expected %d bytes, got %d", len(frame)*2, len(pcm))
	}

	back := float32FromPCM16(pcm)
	if len(back) != len(frame) {
		t.Fatalf("expected %d samples back, got %d", len(frame), len(back))
	}

	for i, want := range frame {
		got := back[i]
		diff := got - want
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.001 {
			t.Errorf("sample %d: want %v, got %v (diff %v)", i, want, got, diff)
		}
	}
}

func TestPCM16ClampsOutOfRange(t *testing.T) {
	frame := []float32{2.0, -2.0} // out of [-1, 1] range
	pcm := pcm16FromFloat32(frame)
	back := float32FromPCM16(pcm)

	if back[0] < 0.99 {
		t.Errorf("expected clamp to max positive, got %v", back[0])
	}
	if back[1] > -0.99 {
		t.Errorf("expected clamp to max negative, got %v", back[1])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/call/stream/... -v`
Expected: FAIL — `pcm16FromFloat32`/`float32FromPCM16` undefined.

- [ ] **Step 3: Write the implementation**

```go
// pkg/call/stream/codec.go
package call_stream

import "encoding/binary"

// pcm16FromFloat32 converts one mono PCM frame (as meowcaller delivers it: float32
// samples in [-1, 1]) into little-endian 16-bit PCM bytes, clamping out-of-range
// samples the same way meowcaller's own WAVRecorder does.
func pcm16FromFloat32(frame []float32) []byte {
	out := make([]byte, len(frame)*2)
	for i, s := range frame {
		v := s * 32768.0
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		binary.LittleEndian.PutUint16(out[2*i:], uint16(int16(v)))
	}
	return out
}

// float32FromPCM16 converts little-endian 16-bit PCM bytes (as sent by a stream
// consumer) back into mono float32 samples in [-1, 1] for meowcaller.Call.Play.
func float32FromPCM16(b []byte) []float32 {
	n := len(b) / 2
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		v := int16(binary.LittleEndian.Uint16(b[2*i:]))
		out[i] = float32(v) / 32768.0
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/call/stream/... -v`
Expected: PASS (2 tests)

- [ ] **Step 5: Commit**

```bash
git add pkg/call/stream/codec.go pkg/call/stream/codec_test.go
git commit -m "feat: add PCM16LE/float32 conversion for call media stream"
```

#### Part B: the bridge (implements AudioSink + VideoSink + AudioSource)

- [ ] **Step 1: Write the bridge**

```go
// pkg/call/stream/bridge.go
package call_stream

import (
	"encoding/base64"
	"io"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/purpshell/meowcaller"
)

// wsMessage is the JSON envelope carried over the stream socket, modeled on Twilio
// Media Streams so existing AI-voice integrations need minimal adapting.
type wsMessage struct {
	Event      string `json:"event"`
	CallID     string `json:"callId,omitempty"`
	SampleRate int    `json:"sampleRate,omitempty"`
	Video      bool   `json:"video,omitempty"`
	Track      string `json:"track,omitempty"`
	Payload    string `json:"payload,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// bridge adapts one WebSocket connection to meowcaller's AudioSink (Call.Receive),
// VideoSink (Call.ReceiveVideo), and AudioSource (Call.Play) interfaces, so a single
// object plugs a call's media straight into the socket in both directions.
type bridge struct {
	conn      *websocket.Conn
	writeMu   sync.Mutex
	incoming  chan []float32
	closed    chan struct{}
	closeOnce sync.Once
}

func newBridge(conn *websocket.Conn) *bridge {
	return &bridge{
		conn: conn,
		// ~3 seconds of buffering at one 60ms frame per slot before frames start
		// getting dropped — enough slack for scheduling jitter without unbounded growth.
		incoming: make(chan []float32, 50),
		closed:   make(chan struct{}),
	}
}

func (b *bridge) writeJSON(msg wsMessage) error {
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	return b.conn.WriteJSON(msg)
}

// writeStart sends the initial handshake message once the socket is up.
func (b *bridge) writeStart(callID string, video bool) {
	_ = b.writeJSON(wsMessage{
		Event:      "start",
		CallID:     callID,
		SampleRate: meowcaller.SampleRate,
		Video:      video,
	})
}

// WriteFrame implements meowcaller.AudioSink: one decoded mono frame from the peer.
func (b *bridge) WriteFrame(frame []float32) error {
	payload := base64.StdEncoding.EncodeToString(pcm16FromFloat32(frame))
	return b.writeJSON(wsMessage{Event: "media", Track: "inbound", Payload: payload})
}

// WriteVideo implements meowcaller.VideoSink: one Annex-B H.264 access unit.
func (b *bridge) WriteVideo(accessUnit []byte) error {
	payload := base64.StdEncoding.EncodeToString(accessUnit)
	return b.writeJSON(wsMessage{Event: "video", Track: "inbound", Payload: payload})
}

// ReadFrame implements meowcaller.AudioSource: frames the consumer sent back over the
// socket, decoded by readLoop and handed here on demand.
func (b *bridge) ReadFrame() ([]float32, error) {
	select {
	case frame, ok := <-b.incoming:
		if !ok {
			return nil, io.EOF
		}
		return frame, nil
	case <-b.closed:
		return nil, io.EOF
	}
}

// Close satisfies AudioSink/VideoSink/AudioSource's shared Close() error method. Safe
// to call more than once (from the call's OnEnd callback and from readLoop exiting).
func (b *bridge) Close() error {
	b.closeOnce.Do(func() {
		_ = b.writeJSON(wsMessage{Event: "stop", Reason: "hangup"})
		close(b.closed)
		_ = b.conn.Close()
	})
	return nil
}

// readLoop blocks reading consumer-sent media messages until the connection closes
// (by either side) or send Close(). Every non-media message is ignored rather than
// erroring, so the wire format can grow new event types without breaking old clients.
func (b *bridge) readLoop() {
	for {
		var msg wsMessage
		if err := b.conn.ReadJSON(&msg); err != nil {
			b.Close()
			return
		}
		if msg.Event != "media" || msg.Track != "outbound" {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(msg.Payload)
		if err != nil {
			continue
		}
		frame := float32FromPCM16(raw)
		select {
		case b.incoming <- frame:
		case <-b.closed:
			return
		default:
			// Consumer is sending audio faster than the call can play it out; drop
			// the frame rather than block the socket read loop.
		}
	}
}
```

- [ ] **Step 2: Build**

```bash
go build ./pkg/call/stream/...
```

Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
git add pkg/call/stream/bridge.go
git commit -m "feat: add WebSocket bridge implementing meowcaller media interfaces"
```

#### Part C: the HTTP/WS handler

- [ ] **Step 1: Write the handler**

```go
// pkg/call/stream/handler.go
package call_stream

import (
	"net/http"

	call_service "github.com/evolution-foundation/evolution-go/pkg/call/service"
	instance_service "github.com/evolution-foundation/evolution-go/pkg/instance/service"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// RegisterRoutes mounts GET /call/stream/:callId directly on the engine, bypassing the
// header-based authMiddleware chain used by pkg/routes: WebSocket clients (especially
// browser-based ones) can't always set a custom apikey header on the upgrade request,
// so auth here is a query parameter instead, resolved the same way authMiddleware.Auth
// resolves it. This mirrors how pkg/passkey/handler.RegisterRoutes already registers
// its own routes directly on *gin.Engine for the same "doesn't fit the standard
// middleware chain" reason.
func RegisterRoutes(r *gin.Engine, callService call_service.CallService, instanceService instance_service.InstanceService) {
	r.GET("/call/stream/:callId", serveStream(callService, instanceService))
}

func serveStream(callService call_service.CallService, instanceService instance_service.InstanceService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		instance, err := instanceService.GetInstanceByToken(ctx.Query("apikey"))
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authorized"})
			return
		}

		callID := ctx.Param("callId")
		call, err := callService.GetActiveCall(instance.Id, callID)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
		if err != nil {
			return
		}

		b := newBridge(conn)
		call.OnEnd(func(reason string) { b.Close() })
		call.Receive(b)
		call.ReceiveVideo(b)
		call.Play(b)

		b.writeStart(callID, call.IsVideo())
		b.readLoop() // blocks until the socket closes, from either end

		_ = call.Hangup()
	}
}
```

- [ ] **Step 2: Build just this package**

```bash
go build ./pkg/call/stream/...
```

Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
git add pkg/call/stream/handler.go
git commit -m "feat: add call stream HTTP/WebSocket handler"
```

---

### Task 8: Wire everything together in `main.go`

**Files:**
- Modify: `cmd/evolution-go/main.go`

**Interfaces:**
- Consumes: `call_registry.NewCallRegistry` (Task 2), `whatsmeow_service.NewWhatsmeowService`'s new param (Task 3), `call_service.NewCallService`'s new param (Task 4), `call_stream.RegisterRoutes` (Task 7)

- [ ] **Step 1: Add imports**

```go
	call_registry "github.com/evolution-foundation/evolution-go/pkg/call/registry"
	call_stream "github.com/evolution-foundation/evolution-go/pkg/call/stream"
```

- [ ] **Step 2: Create the shared registry alongside `clientPointer`**

Find (around line 87):

```go
	killChannel := make(map[string](chan bool))
	clientPointer := make(map[string]*whatsmeow.Client)
```

Change to:

```go
	killChannel := make(map[string](chan bool))
	clientPointer := make(map[string]*whatsmeow.Client)
	callRegistry := call_registry.NewCallRegistry()
```

- [ ] **Step 3: Pass it into `NewWhatsmeowService`**

Find the `whatsmeowService := whatsmeow_service.NewWhatsmeowService(...)` call (around line 165) and add `callRegistry,` as the new second-to-last argument (matching the parameter position added in Task 3 Step 3):

```go
	whatsmeowService := whatsmeow_service.NewWhatsmeowService(
		instanceRepository,
		authDB,
		message_repository.NewMessageRepository(db),
		labelRepository,
		config,
		killChannel,
		clientPointer,
		rabbitmqProducer,
		webhookProducer,
		websocketProducer,
		sqliteDB,
		exPath,
		mediaStorage,
		natsProducer,
		callRegistry,
		loggerWrapper,
	)
```

- [ ] **Step 4: Pass it into `NewCallService`**

Find (around line 195):

```go
	callService := call_service.NewCallService(clientPointer, whatsmeowService, loggerWrapper)
```

Change to:

```go
	callService := call_service.NewCallService(clientPointer, whatsmeowService, callRegistry, loggerWrapper)
```

- [ ] **Step 5: Register the stream route**

Find the existing:

```go
	passkey_handler.RegisterRoutes(r, whatsmeowService)
```

Add right after it:

```go
	call_stream.RegisterRoutes(r, callService, instanceService)
```

(`instanceService` is already constructed a few lines above this point in the same function.)

- [ ] **Step 6: Tidy modules now that `meowcaller` is actually imported, then build**

This is the first point in the plan where real code imports `meowcaller` (Tasks 3, 4,
and 7 all added imports of it), so `go mod tidy` is now safe to run — it has something
to keep. Task 1 deliberately skipped this step for exactly that reason.

```bash
go mod tidy
grep meowcaller go.mod
```

Expected: the `grep` still shows the `github.com/purpshell/meowcaller` line (confirming
`tidy` kept it now that it's actually used).

```bash
go build ./...
```

Expected: exits 0 — this is the point where every wiring gap from Tasks 4-7 gets closed.

- [ ] **Step 7: Run the full test suite**

```bash
go test ./...
```

Expected: PASS across the repo (including the pre-existing 4 test files and the new tests from Tasks 2 and 7).

- [ ] **Step 8: Format and lint**

```bash
make fmt
make lint
```

Expected: both exit 0. If `make fmt` rewrites anything, re-run `go build ./...` and `go test ./...` once more before committing.

- [ ] **Step 9: Commit**

```bash
git add cmd/evolution-go/main.go
git commit -m "feat: wire call registry and stream route into main"
```

---

### Task 9: Manual end-to-end validation and PR prep

**Files:**
- Modify: `docs/superpowers/plans/2026-07-29-call-answer-stream.md` (check off as validated)

This is the step that actually proves the feature works — everything up to here only proves it compiles and the pure-logic pieces are correct. Do not open the PR before this passes.

- [ ] **Step 1: Run the server locally against a real test instance**

Follow the repo's existing local-run instructions (`README.md` / `docker/`) to start Evolution Go pointed at a WhatsApp test number you control, and confirm the instance shows `connected: true`.

- [ ] **Step 2: Trigger an incoming call**

From a second phone, place a WhatsApp voice call to the test number. Confirm (via logs) the line added in Task 3 Step 4 fires:

```
[<instanceId>] meowcaller captured incoming call <callId> from <peerJid>
```

- [ ] **Step 3: Answer it via the API**

```bash
curl -X POST http://localhost:<port>/call/answer \
  -H "apikey: <instance token>" \
  -H "Content-Type: application/json" \
  -d '{"callCreator": "<peer JID from the log line above>", "callId": "<call id from the log line above>"}'
```

Expected: `{"message":"success"}`, and the call actually connects on the calling phone (no more "Connecting…").

- [ ] **Step 4: Connect a throwaway WS client and confirm audio**

Use any small script (Python `websockets`, `wscat`, etc.) to connect to `ws://localhost:<port>/call/stream/<callId>?apikey=<instance token>`, write every `{"event":"media","track":"inbound",...}` message's decoded, base64-decoded, PCM16LE payload to a raw file, then convert that raw file to WAV (16-bit, mono, 16000 Hz) with any audio tool. Speak into the calling phone during the call. Confirm the resulting WAV is intelligible speech.

- [ ] **Step 5: Confirm hangup behavior both ways**

- Call `POST /call/hangup` with the same `callId` — confirm the call actually ends on the calling phone, and the WS connection receives `{"event":"stop","reason":"hangup"}` and closes.
- On a second test call, hang up from the *calling phone* instead — confirm the WS connection closes on its own (via `Call.OnEnd` → `bridge.Close()`) without needing `/call/hangup`.

- [ ] **Step 6: Update Swagger**

```bash
make swagger
git add docs/
git commit -m "docs: regenerate swagger for call answer/hangup endpoints"
```

- [ ] **Step 7: Push the branch and open the PR**

```bash
git push -u origin feature/call-answer-stream
gh pr create \
  --repo evolution-foundation/evolution-go \
  --title "feat: answer WhatsApp calls and stream their media over WebSocket" \
  --body "$(cat <<'EOF'
## Summary

- Adds `POST /call/answer` and `POST /call/hangup`, alongside the existing `POST /call/reject`.
- Adds `GET /call/stream/:callId`, a dedicated per-call WebSocket that carries the
  call's audio (and inbound video) as base64 PCM16LE/H.264 frames, in a JSON envelope
  modeled on Twilio Media Streams. Sending `{"event":"media","track":"outbound",...}`
  back over the same socket plays audio into the call (e.g. TTS output).
- This PR ships transport/plumbing only — no AI/STT/LLM/TTS logic lives here. What's on
  the other end of the WebSocket (an AI voice pipeline, a recorder, a human console) is
  entirely up to the consumer.

## Why

`go.mau.fi/whatsmeow` (this project's WhatsApp protocol library) has never implemented
accepting a call or handling its media — only rejecting. Building that from scratch is a
multi-year reverse-engineering effort (see `tulir/whatsmeow#555`).
[`purpshell/meowcaller`](https://github.com/purpshell/meowcaller) (MIT-licensed, pure Go,
actively maintained) already did that work as an independent interoperability research
project — see its [DISCLAIMER.md](https://github.com/purpshell/meowcaller/blob/main/DISCLAIMER.md)
for its scope and ground rules. This PR wires it into Evolution Go's existing per-instance
client lifecycle.

## How it's wired

`meowcaller.NewClient` is constructed the moment each instance's `*whatsmeow.Client` is
created, before `.Connect()` (required by the library). Incoming calls are captured via
`OnIncomingCall` into a small new `pkg/call/registry` package, scoped by instance so one
instance's apikey can't reach another instance's call. `pkg/call/service` answers/hangs
up by looking calls up in that registry. `pkg/call/stream` bridges a call's audio/video
sinks and source to the WebSocket.

## Test plan

- [x] Unit tests for the call registry (`pkg/call/registry`) and PCM16LE/float32
      conversion (`pkg/call/stream`)
- [x] `go build ./...` and `go test ./...` pass
- [x] `make fmt` / `make lint` clean
- [x] Manually verified against a real WhatsApp call: answered via the API, recorded
      intelligible audio from the stream socket, confirmed hangup both via the API and
      via the remote end hanging up first
EOF
)"
```

- [ ] **Step 8: Check off validation in this plan**

Edit this file's Task 9 checkboxes to `[x]` once every step above has actually been run and passed, and commit that update too.

---

## Self-Review Notes

- **Spec coverage:** every section of `docs/superpowers/specs/2026-07-29-call-answer-stream-design.md` maps to a task — architecture/constraint (Task 3), components (Tasks 2, 4, 5, 6, 7), wire format (Task 7 Part B), auth (Task 7 Part C), lifecycle/error handling (Tasks 3 Step 5, 4 Step 5, 7 Part B `Close`/`OnEnd`), testing (Tasks 2, 7 Part A automated; Task 9 manual), PR plan (Task 9 Steps 6-7).
- **Type consistency checked:** `AnswerCallStruct`/`HangupCallStruct` (Task 4) match what Task 5's handlers bind into; `CallService` interface additions (Task 4 Step 4) match the concrete methods implemented in the same task and the calls made from Task 7's handler; `call_registry.CallRegistry`'s `Store(instanceID string, call *meowcaller.Call)`/`Get(instanceID, callID string)`/`Delete(callID string)` signatures are identical everywhere they're called (Tasks 3, 4).
- **No placeholders:** every step has complete, real code — the only two forward references (`NewCallService`'s call site in `main.go`, fixed in Task 8; the intermediate `go build` failures in Tasks 4-7) are explicitly called out as expected and temporary, with the exact fixing task named.
