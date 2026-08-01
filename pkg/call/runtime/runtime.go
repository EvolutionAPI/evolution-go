package call_runtime

import (
	"sort"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
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
	mu         sync.RWMutex
	instanceID string
	client     *whatsmeow.Client
	calls      map[string]Call
}

func New(instanceID string, client *whatsmeow.Client) *Runtime {
	return &Runtime{
		instanceID: instanceID,
		client:     client,
		calls:      make(map[string]Call),
	}
}

func (r *Runtime) InstanceID() string {
	return r.instanceID
}

// AttachClient replaces the client after an Evolution instance reconnects.
// Active media resources must be torn down by the future AstraCalls adapter
// before this method is called.
func (r *Runtime) AttachClient(client *whatsmeow.Client) {
	r.mu.Lock()
	r.client = client
	r.mu.Unlock()
}

func (r *Runtime) Client() *whatsmeow.Client {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.client
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
	delete(r.runtimes, instanceID)
	r.mu.Unlock()
}
