package call_runtime

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	call_signaling "github.com/evolution-foundation/evolution-go/pkg/call/voip/signaling"
	call_wa "github.com/evolution-foundation/evolution-go/pkg/call/voip/wa"
	"go.mau.fi/whatsmeow"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// State represents the lifecycle state of a WhatsApp call.
type State string

const (
	StateIdle       State = "idle"
	StateRinging    State = "ringing"
	StateConnecting State = "connecting"
	StateActive     State = "active"
	StateEnded      State = "ended"
	StateFailed     State = "failed"
)

// Direction identifies whether a call was created locally or received from a peer.
type Direction string

const (
	DirectionIncoming Direction = "incoming"
	DirectionOutgoing Direction = "outgoing"
)

// Preparation describes whether the private incoming-call signaling material is
// ready for a local answer. It intentionally contains no private call keys or
// relay data.
type Preparation string

const (
	PreparationPreparing Preparation = "preparing"
	PreparationReady     Preparation = "ready"
	PreparationFailed    Preparation = "failed"
)

// Call contains transport-independent call state.
type Call struct {
	ID          string      `json:"id"`
	Peer        string      `json:"peer"`
	Direction   Direction   `json:"direction"`
	State       State       `json:"state"`
	Video       bool        `json:"video"`
	Preparation Preparation `json:"preparation,omitempty"`
	EndReason   string      `json:"endReason,omitempty"`
	Error       string      `json:"error,omitempty"`
	AnsweredBy  string      `json:"answeredBy,omitempty"`
	AnsweredAt  *time.Time  `json:"answeredAt,omitempty"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
}

// Snapshot is a safe, serializable view of one instance runtime.
type Snapshot struct {
	InstanceID string `json:"instanceId"`
	Connected  bool   `json:"connected"`
	Calls      []Call `json:"calls"`
}

// Runtime owns the VoIP state associated with exactly one Evolution instance.
type Runtime struct {
	mu             sync.RWMutex
	instanceID     string
	client         *whatsmeow.Client
	eventHandlerID uint32
	calls          map[string]Call
	onChange       func(Call)
}

func New(instanceID string, client *whatsmeow.Client) *Runtime {
	runtime := &Runtime{
		instanceID: instanceID,
		calls:      make(map[string]Call),
	}
	runtime.AttachClient(client)
	return runtime
}

func (r *Runtime) InstanceID() string {
	return r.instanceID
}

// AttachClient replaces the client after an Evolution instance reconnects and
// registers an isolated call event handler on the same authenticated session.
func (r *Runtime) AttachClient(client *whatsmeow.Client) {
	r.mu.Lock()
	if r.client == client && (client == nil || r.eventHandlerID != 0) {
		r.mu.Unlock()
		return
	}

	previousClient := r.client
	previousHandlerID := r.eventHandlerID
	r.client = client
	r.eventHandlerID = 0
	r.mu.Unlock()

	if previousClient != nil && previousHandlerID != 0 {
		previousClient.RemoveEventHandler(previousHandlerID)
	}
	if client == nil {
		return
	}

	handlerID := client.AddEventHandler(r.handleEvent)
	r.mu.Lock()
	if r.client == client {
		r.eventHandlerID = handlerID
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()
	client.RemoveEventHandler(handlerID)
}

func (r *Runtime) Client() *whatsmeow.Client {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.client
}

func (r *Runtime) Close() {
	r.mu.Lock()
	client := r.client
	handlerID := r.eventHandlerID
	r.client = nil
	r.eventHandlerID = 0
	r.mu.Unlock()

	if client != nil && handlerID != 0 {
		client.RemoveEventHandler(handlerID)
	}
	r.closeWatchdogs()
}

// UpsertCall creates or updates a call while preserving its creation time.
func (r *Runtime) UpsertCall(call Call) {
	r.mu.Lock()
	now := time.Now().UTC()
	if current, ok := r.calls[call.ID]; ok {
		if call.CreatedAt.IsZero() {
			call.CreatedAt = current.CreatedAt
		}
		if call.Peer == "" {
			call.Peer = current.Peer
		}
		if call.AnsweredAt == nil && current.AnsweredAt != nil {
			answeredAt := *current.AnsweredAt
			call.AnsweredAt = &answeredAt
		}
		if call.AnsweredBy == "" {
			call.AnsweredBy = current.AnsweredBy
		}
	} else if call.CreatedAt.IsZero() {
		call.CreatedAt = now
	}

	if call.UpdatedAt.IsZero() {
		call.UpdatedAt = now
	}
	r.calls[call.ID] = call
	r.mu.Unlock()
	r.syncWatchdog(call)
	r.notifyChange(call)
}

// MarkAnswered records the authenticated Manager user that is answering an
// incoming call. It is called before the accept stanza is sent so a companion
// echo of that stanza is not mistaken for another device answering first.
func (r *Runtime) MarkAnswered(callID, answeredBy string) (Call, bool) {
	if r == nil || callID == "" {
		return Call{}, false
	}

	r.mu.Lock()
	call, exists := r.calls[callID]
	if !exists {
		r.mu.Unlock()
		return Call{}, false
	}
	now := time.Now().UTC()
	if call.AnsweredAt == nil {
		answeredAt := now
		call.AnsweredAt = &answeredAt
	}
	if strings.TrimSpace(answeredBy) != "" {
		call.AnsweredBy = strings.TrimSpace(answeredBy)
	}
	call.UpdatedAt = now
	r.calls[callID] = call
	r.mu.Unlock()
	r.notifyChange(call)
	return call, true
}

// ClearAnswerMetadata rolls back a failed local answer attempt while the call
// is still ringing. Once signaling has advanced, retaining the metadata is
// safer than reviving or rewriting a real remote outcome.
func (r *Runtime) ClearAnswerMetadata(callID string) (Call, bool) {
	if r == nil || callID == "" {
		return Call{}, false
	}

	r.mu.Lock()
	call, exists := r.calls[callID]
	if !exists {
		r.mu.Unlock()
		return Call{}, false
	}
	if call.State != StateRinging {
		r.mu.Unlock()
		return call, true
	}
	call.AnsweredAt = nil
	call.AnsweredBy = ""
	call.UpdatedAt = time.Now().UTC()
	r.calls[callID] = call
	r.mu.Unlock()
	r.notifyChange(call)
	return call, true
}

// MarkIncomingPrepared enables a local answer only after the private call key
// was stored and the preaccept stanza was successfully sent.
func (r *Runtime) MarkIncomingPrepared(callID string) (Call, bool) {
	if r == nil || callID == "" {
		return Call{}, false
	}

	r.mu.Lock()
	call, exists := r.calls[callID]
	if !exists || call.Direction != DirectionIncoming || call.State != StateRinging {
		r.mu.Unlock()
		return call, exists
	}
	call.Preparation = PreparationReady
	call.UpdatedAt = time.Now().UTC()
	r.calls[callID] = call
	r.mu.Unlock()
	r.notifyChange(call)
	return call, true
}

// MarkIncomingPreparationFailed keeps the incoming call visible but prevents
// the Manager from attempting an answer that cannot complete safely. Detailed
// diagnostics remain only in server logs.
func (r *Runtime) MarkIncomingPreparationFailed(callID string) (Call, bool) {
	if r == nil || callID == "" {
		return Call{}, false
	}

	r.mu.Lock()
	call, exists := r.calls[callID]
	if !exists || call.Direction != DirectionIncoming || call.State != StateRinging || call.Preparation == PreparationReady {
		r.mu.Unlock()
		return call, exists
	}
	call.Preparation = PreparationFailed
	call.UpdatedAt = time.Now().UTC()
	r.calls[callID] = call
	r.mu.Unlock()
	r.notifyChange(call)
	return call, true
}

// MarkAnsweredElsewhere turns an incoming ringing call into a terminal state
// when another linked WhatsApp device accepts it. This removes the answer
// controls immediately and makes the outcome explicit to the Manager.
func (r *Runtime) MarkAnsweredElsewhere(callID string) (Call, bool) {
	if r == nil || callID == "" {
		return Call{}, false
	}

	r.mu.Lock()
	call, exists := r.calls[callID]
	if !exists {
		r.mu.Unlock()
		return Call{}, false
	}
	if call.State == StateEnded || call.State == StateFailed {
		r.mu.Unlock()
		return call, true
	}
	now := time.Now().UTC()
	if call.AnsweredAt == nil {
		answeredAt := now
		call.AnsweredAt = &answeredAt
	}
	call.AnsweredBy = "Outro dispositivo"
	call.State = StateEnded
	call.EndReason = "answered_elsewhere"
	call.UpdatedAt = now
	r.calls[callID] = call
	r.mu.Unlock()
	r.cancelWatchdog(callID)
	r.notifyChange(call)
	return call, true
}

// Transition applies a partial lifecycle update without erasing metadata that
// was captured by an earlier call event.
func (r *Runtime) Transition(callID, peer string, direction Direction, state State, video *bool, endReason string) {
	if callID == "" {
		return
	}

	r.mu.Lock()
	now := time.Now().UTC()
	call, exists := r.calls[callID]
	if !exists {
		call = Call{
			ID:        callID,
			CreatedAt: now,
		}
		if direction == DirectionIncoming && state == StateRinging {
			call.Preparation = PreparationPreparing
		}
	}
	if shouldReplacePeer(call.Peer, peer) {
		call.Peer = peer
	}
	if direction != "" && call.Direction == "" {
		call.Direction = direction
	}
	// WhatsApp signaling can be delivered out of order. Once a call has a
	// terminal outcome, a delayed transport/preaccept stanza must not revive
	// it and make the Manager show actionable controls again.
	if state != "" && !(isTerminalState(call.State) && !isTerminalState(state)) {
		call.State = state
	}
	if video != nil {
		call.Video = *video
	}
	if endReason != "" && !preserveExternalOutcome(call.EndReason, endReason) {
		call.EndReason = endReason
	}
	call.UpdatedAt = now
	r.calls[callID] = call
	r.mu.Unlock()
	r.syncWatchdog(call)
	r.notifyChange(call)
}

func isTerminalState(state State) bool {
	return state == StateEnded || state == StateFailed
}

func preserveExternalOutcome(current, candidate string) bool {
	if current == candidate {
		return false
	}
	return current == "answered_elsewhere" || current == "rejected_elsewhere"
}

// SetOnChange registers a callback that receives every public call state
// change. Callbacks are invoked after the runtime lock has been released so
// observers can safely read a snapshot without blocking the WhatsApp event
// handler.
func (r *Runtime) SetOnChange(callback func(Call)) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.onChange = callback
	r.mu.Unlock()
}

func (r *Runtime) notifyChange(call Call) {
	if r == nil {
		return
	}
	r.mu.RLock()
	callback := r.onChange
	r.mu.RUnlock()
	if callback != nil {
		callback(call)
	}
}

func shouldReplacePeer(current, candidate string) bool {
	if candidate == "" {
		return false
	}
	if current == "" {
		return true
	}
	currentJID, currentErr := types.ParseJID(current)
	candidateJID, candidateErr := types.ParseJID(candidate)
	return currentErr == nil && candidateErr == nil &&
		currentJID.Server == types.HiddenUserServer && candidateJID.Server == types.DefaultUserServer
}

func (r *Runtime) Call(callID string) (Call, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	call, ok := r.calls[callID]
	return call, ok
}

func (r *Runtime) RemoveCall(callID string) {
	r.mu.Lock()
	delete(r.calls, callID)
	r.mu.Unlock()
	r.cancelWatchdog(callID)
}

func (r *Runtime) Snapshot() Snapshot {
	r.mu.RLock()
	calls := make([]Call, 0, len(r.calls))
	for _, call := range r.calls {
		calls = append(calls, call)
	}
	client := r.client
	instanceID := r.instanceID
	connected := client != nil && client.IsConnected()
	r.mu.RUnlock()

	for index := range calls {
		calls[index].Peer = resolveDisplayPeer(client, calls[index].Peer)
		if calls[index].AnsweredAt != nil {
			answeredAt := *calls[index].AnsweredAt
			calls[index].AnsweredAt = &answeredAt
		}
	}
	sort.Slice(calls, func(i, j int) bool {
		return calls[i].CreatedAt.Before(calls[j].CreatedAt)
	})

	return Snapshot{
		InstanceID: instanceID,
		Connected:  connected,
		Calls:      calls,
	}
}

func (r *Runtime) handleEvent(rawEvent interface{}) {
	switch event := rawEvent.(type) {
	case *events.CallOffer:
		if call_signaling.IsAlreadyEndedOffer(event.Data) {
			slog.Info("ignoring already-ended WhatsApp call offer",
				"instance", r.instanceID,
				"call_id", event.CallID,
			)
			return
		}
		video := callNodeContainsVideo(event.Data)
		r.Transition(
			event.CallID,
			r.eventPeer(event.CallCreator, event.From),
			DirectionIncoming,
			StateRinging,
			&video,
			"",
		)
	case *events.CallOfferNotice:
		// Whatsmeow documents offer notices as group-call signaling. The private
		// negotiation pipeline supports one-to-one calls only, so do not expose
		// an unusable group call as an actionable Manager incoming call.
		slog.Info("ignoring unsupported WhatsApp call offer notice",
			"instance", r.instanceID,
			"call_id", event.CallID,
			"type", event.Type,
		)
	case *events.CallPreAccept:
		call, exists := r.Call(event.CallID)
		// A preaccept is received from the remote device while a locally started
		// call is ringing. It must not advance an incoming call: doing so hides
		// the Manager's answer controls before the local user can answer.
		if !exists || call.Direction != DirectionOutgoing {
			slog.Debug("ignoring WhatsApp preaccept for non-outgoing call",
				"instance", r.instanceID,
				"call_id", event.CallID,
				"direction", call.Direction,
			)
			return
		}
		r.Transition(
			event.CallID,
			r.eventPeer(event.From, event.CallCreator),
			DirectionOutgoing,
			StateConnecting,
			nil,
			"",
		)
	case *events.CallAccept:
		if call, exists := r.Call(event.CallID); exists && call.Direction == DirectionIncoming && call.AnsweredBy == "" {
			r.MarkAnsweredElsewhere(event.CallID)
			return
		}
		r.Transition(
			event.CallID,
			r.eventPeer(event.From, event.CallCreator),
			DirectionOutgoing,
			StateConnecting,
			nil,
			"",
		)
	case *events.CallTransport:
		r.Transition(
			event.CallID,
			r.eventPeer(event.From, event.CallCreator),
			"",
			StateConnecting,
			nil,
			"",
		)
	case *events.CallReject:
		call, exists := r.Call(event.CallID)
		reason := "rejected"
		direction := DirectionOutgoing
		previousState := State("")
		if exists {
			direction = call.Direction
			previousState = call.State
			if call.Direction == DirectionIncoming {
				if call.State == StateRinging {
					// A reject stanza emitted by one of our linked devices means the
					// browser did not lose the call: it was explicitly dismissed
					// elsewhere. A peer-originated reject means WhatsApp ended the
					// call before the local user answered it.
					if r.eventFromOwnDevice(event.From) {
						reason = "rejected_elsewhere"
					} else {
						reason = "ended_before_answer"
					}
				} else {
					reason = "peer_ended"
				}
			} else if call.State != StateRinging {
				reason = "peer_ended"
			}
		}
		slog.Info("WhatsApp call reject received",
			"instance", r.instanceID,
			"call_id", event.CallID,
			"direction", direction,
			"previous_state", previousState,
			"outcome", reason,
		)
		r.Transition(
			event.CallID,
			r.eventPeer(event.CallCreator, event.From),
			direction,
			StateEnded,
			nil,
			reason,
		)
	case *events.CallTerminate:
		r.Transition(
			event.CallID,
			r.eventPeer(event.CallCreator, event.From),
			"",
			StateEnded,
			nil,
			event.Reason,
		)
	case *events.Disconnected:
		r.failOpenCalls("whatsapp client disconnected")
	case *events.LoggedOut:
		r.failOpenCalls("whatsapp client logged out")
	}
}

func (r *Runtime) eventFromOwnDevice(jid types.JID) bool {
	if r == nil || jid.IsEmpty() {
		return false
	}
	r.mu.RLock()
	client := r.client
	r.mu.RUnlock()
	return isOwnJID(client, jid)
}

func (r *Runtime) eventPeer(primary, secondary types.JID) string {
	r.mu.RLock()
	client := r.client
	r.mu.RUnlock()

	candidates := []types.JID{primary, secondary}
	for _, candidate := range candidates {
		if candidate.IsEmpty() || isOwnJID(client, candidate) {
			continue
		}
		resolved := resolveDisplayJID(client, candidate)
		if resolved.Server == types.DefaultUserServer {
			return resolved.String()
		}
	}
	for _, candidate := range candidates {
		if !candidate.IsEmpty() && !isOwnJID(client, candidate) {
			return candidate.ToNonAD().String()
		}
	}
	return ""
}

func resolveDisplayPeer(client *whatsmeow.Client, peer string) string {
	jid, err := types.ParseJID(peer)
	if err != nil || jid.IsEmpty() {
		return peer
	}
	return resolveDisplayJID(client, jid).String()
}

func resolveDisplayJID(client *whatsmeow.Client, jid types.JID) types.JID {
	jid = jid.ToNonAD()
	if client == nil || jid.Server != types.HiddenUserServer {
		return jid
	}
	return call_wa.NewSocket(client).ResolvePNForLID(context.Background(), jid).ToNonAD()
}

func isOwnJID(client *whatsmeow.Client, jid types.JID) bool {
	if client == nil || client.Store == nil || jid.IsEmpty() {
		return false
	}
	jid = jid.ToNonAD()
	if client.Store.ID != nil {
		ownPN := client.Store.ID.ToNonAD()
		if jid.User == ownPN.User && jid.Server == ownPN.Server {
			return true
		}
	}
	ownLID := client.Store.LID.ToNonAD()
	return !ownLID.IsEmpty() && jid.User == ownLID.User && jid.Server == ownLID.Server
}

func (r *Runtime) failOpenCalls(reason string) {
	r.mu.Lock()
	now := time.Now().UTC()
	failedCalls := make([]Call, 0)
	for callID, call := range r.calls {
		if call.State == StateEnded || call.State == StateFailed {
			continue
		}
		call.State = StateFailed
		call.Error = reason
		call.UpdatedAt = now
		r.calls[callID] = call
		failedCalls = append(failedCalls, call)
	}
	r.mu.Unlock()
	for _, call := range failedCalls {
		r.cancelWatchdog(call.ID)
		r.notifyChange(call)
	}
}

func callNodeContainsVideo(node *waBinary.Node) bool {
	if node == nil {
		return false
	}
	if strings.EqualFold(node.Tag, "video") {
		return true
	}
	for key, value := range node.Attrs {
		keyLower := strings.ToLower(key)
		valueString := strings.ToLower(strings.TrimSpace(valueToString(value)))
		if (keyLower == "media" || keyLower == "type") && valueString == "video" {
			return true
		}
	}

	switch content := node.Content.(type) {
	case []waBinary.Node:
		for index := range content {
			if callNodeContainsVideo(&content[index]) {
				return true
			}
		}
	case *waBinary.Node:
		return callNodeContainsVideo(content)
	}
	return false
}

func valueToString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return ""
	}
}

// Registry stores one Runtime per Evolution instance.
type Registry struct {
	mu       sync.RWMutex
	runtimes map[string]*Runtime
}

func NewRegistry() *Registry {
	return &Registry{runtimes: make(map[string]*Runtime)}
}

// Attach returns the existing runtime or creates it. On reconnect it updates the
// runtime to point at the newly-created whatsmeow client.
func (r *Registry) Attach(instanceID string, client *whatsmeow.Client) *Runtime {
	runtime := r.Ensure(instanceID)
	runtime.AttachClient(client)
	return runtime
}

// Ensure returns the public runtime without registering a WhatsApp event
// handler. Coordinators can use it to configure observers before attaching a
// client, so an early incoming offer cannot be missed.
func (r *Registry) Ensure(instanceID string) *Runtime {
	r.mu.Lock()
	if runtime, ok := r.runtimes[instanceID]; ok {
		r.mu.Unlock()
		return runtime
	}

	runtime := New(instanceID, nil)
	r.runtimes[instanceID] = runtime
	r.mu.Unlock()
	return runtime
}

func (r *Registry) Get(instanceID string) (*Runtime, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	runtime, ok := r.runtimes[instanceID]
	return runtime, ok
}

func (r *Registry) Remove(instanceID string) {
	r.mu.Lock()
	runtime, ok := r.runtimes[instanceID]
	if ok {
		delete(r.runtimes, instanceID)
	}
	r.mu.Unlock()

	if ok {
		runtime.Close()
	}
}
