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
