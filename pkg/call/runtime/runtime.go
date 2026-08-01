package call_runtime

import (
	"sort"
	"strings"
	"sync"
	"time"

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

// Call contains transport-independent call state. The AstraCalls adapter will
// update these records while the existing Evolution event producers publish them.
type Call struct {
	ID        string    `json:"id"`
	Peer      string    `json:"peer"`
	Direction Direction `json:"direction"`
	State     State     `json:"state"`
	Video     bool      `json:"video"`
	EndReason string    `json:"endReason,omitempty"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Snapshot is a safe, serializable view of one instance runtime.
type Snapshot struct {
	InstanceID string `json:"instanceId"`
	Connected  bool   `json:"connected"`
	Calls      []Call `json:"calls"`
}

// Runtime owns the VoIP state associated with exactly one Evolution instance.
// It deliberately reuses the instance's existing whatsmeow client so messaging
// and calls share a single authenticated WhatsApp session.
type Runtime struct {
	mu             sync.RWMutex
	instanceID     string
	client         *whatsmeow.Client
	eventHandlerID uint32
	calls          map[string]Call
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

	// A reconnect can race with event-handler registration. Keep the handler only
	// when this client is still the currently attached one.
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

// Close detaches the runtime event handler. Media teardown will be added by the
// AstraCalls driver when its transport and WebRTC resources are ported.
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
}

// UpsertCall creates or updates a call while preserving its creation time.
func (r *Runtime) UpsertCall(call Call) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	if current, ok := r.calls[call.ID]; ok {
		if call.CreatedAt.IsZero() {
			call.CreatedAt = current.CreatedAt
		}
	} else if call.CreatedAt.IsZero() {
		call.CreatedAt = now
	}

	if call.UpdatedAt.IsZero() {
		call.UpdatedAt = now
	}
	r.calls[call.ID] = call
}

// Transition applies a partial lifecycle update without erasing metadata that
// was captured by an earlier call event.
func (r *Runtime) Transition(callID, peer string, direction Direction, state State, video *bool, endReason string) {
	if callID == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	call, exists := r.calls[callID]
	if !exists {
		call = Call{
			ID:        callID,
			CreatedAt: now,
		}
	}
	if peer != "" {
		call.Peer = peer
	}
	if direction != "" && call.Direction == "" {
		call.Direction = direction
	}
	if state != "" {
		call.State = state
	}
	if video != nil {
		call.Video = *video
	}
	if endReason != "" {
		call.EndReason = endReason
	}
	call.UpdatedAt = now
	r.calls[callID] = call
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
}

func (r *Runtime) Snapshot() Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	calls := make([]Call, 0, len(r.calls))
	for _, call := range r.calls {
		calls = append(calls, call)
	}
	sort.Slice(calls, func(i, j int) bool {
		return calls[i].CreatedAt.Before(calls[j].CreatedAt)
	})

	connected := r.client != nil && r.client.IsConnected()
	return Snapshot{
		InstanceID: r.instanceID,
		Connected:  connected,
		Calls:      calls,
	}
}

func (r *Runtime) handleEvent(rawEvent interface{}) {
	switch event := rawEvent.(type) {
	case *events.CallOffer:
		video := callNodeContainsVideo(event.Data)
		r.Transition(
			event.CallID,
			callPeer(event.CallCreator, event.From),
			DirectionIncoming,
			StateRinging,
			&video,
			"",
		)
	case *events.CallOfferNotice:
		video := strings.EqualFold(event.Media, "video") || callNodeContainsVideo(event.Data)
		r.Transition(
			event.CallID,
			callPeer(event.CallCreator, event.From),
			DirectionIncoming,
			StateRinging,
			&video,
			"",
		)
	case *events.CallPreAccept:
		r.Transition(
			event.CallID,
			callPeer(event.CallCreator, event.From),
			DirectionOutgoing,
			StateConnecting,
			nil,
			"",
		)
	case *events.CallAccept:
		r.Transition(
			event.CallID,
			callPeer(event.CallCreator, event.From),
			DirectionOutgoing,
			StateActive,
			nil,
			"",
		)
	case *events.CallTransport:
		r.Transition(
			event.CallID,
			callPeer(event.CallCreator, event.From),
			"",
			StateConnecting,
			nil,
			"",
		)
	case *events.CallReject:
		r.Transition(
			event.CallID,
			callPeer(event.CallCreator, event.From),
			DirectionOutgoing,
			StateEnded,
			nil,
			"rejected",
		)
	case *events.CallTerminate:
		r.Transition(
			event.CallID,
			callPeer(event.CallCreator, event.From),
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

func (r *Runtime) failOpenCalls(reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	for callID, call := range r.calls {
		if call.State == StateEnded || call.State == StateFailed {
			continue
		}
		call.State = StateFailed
		call.Error = reason
		call.UpdatedAt = now
		r.calls[callID] = call
	}
}

func callPeer(callCreator, from types.JID) string {
	if !callCreator.IsEmpty() {
		return callCreator.String()
	}
	if !from.IsEmpty() {
		return from.String()
	}
	return ""
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
	r.mu.Lock()
	defer r.mu.Unlock()

	if runtime, ok := r.runtimes[instanceID]; ok {
		runtime.AttachClient(client)
		return runtime
	}

	runtime := New(instanceID, client)
	r.runtimes[instanceID] = runtime
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
