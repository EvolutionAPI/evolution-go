package call_runtime

import (
	"sync"
	"time"
)

var (
	ringingWatchdogTimeout     = 90 * time.Second
	connectingWatchdogTimeout = 45 * time.Second
)

type runtimeWatchdogEntry struct {
	timer      *time.Timer
	generation uint64
	state      State
}

type runtimeWatchdogState struct {
	onTimeout  func(instanceID, callID string)
	entries    map[string]runtimeWatchdogEntry
	generation uint64
}

var runtimeWatchdogs = struct {
	sync.Mutex
	states map[*Runtime]*runtimeWatchdogState
}{states: make(map[*Runtime]*runtimeWatchdogState)}

// SetOnTimeout registers the cleanup callback invoked after a ringing or media
// negotiation timeout. The runtime updates the public call to StateFailed
// before invoking this callback.
func (r *Runtime) SetOnTimeout(callback func(instanceID, callID string)) {
	if r == nil {
		return
	}
	runtimeWatchdogs.Lock()
	state := runtimeWatchdogs.states[r]
	if state == nil {
		state = &runtimeWatchdogState{entries: make(map[string]runtimeWatchdogEntry)}
		runtimeWatchdogs.states[r] = state
	}
	state.onTimeout = callback
	runtimeWatchdogs.Unlock()
}

func (r *Runtime) syncWatchdog(call Call) {
	if r == nil || call.ID == "" {
		return
	}

	timeout, reason := watchdogTimeout(call.State)
	if timeout <= 0 {
		r.cancelWatchdog(call.ID)
		return
	}

	runtimeWatchdogs.Lock()
	state := runtimeWatchdogs.states[r]
	if state == nil {
		state = &runtimeWatchdogState{entries: make(map[string]runtimeWatchdogEntry)}
		runtimeWatchdogs.states[r] = state
	}
	if previous, ok := state.entries[call.ID]; ok {
		// Duplicate CallAccept/CallTransport/CallOffer events must not extend the
		// negotiation deadline indefinitely. Only a real state transition gets a
		// new timer.
		if previous.state == call.State {
			runtimeWatchdogs.Unlock()
			return
		}
		if previous.timer != nil {
			previous.timer.Stop()
		}
	}
	state.generation++
	generation := state.generation
	entry := runtimeWatchdogEntry{
		generation: generation,
		state:      call.State,
	}
	entry.timer = time.AfterFunc(timeout, func() {
		r.expireWatchdog(call.ID, generation, call.State, reason)
	})
	state.entries[call.ID] = entry
	runtimeWatchdogs.Unlock()
}

func watchdogTimeout(state State) (time.Duration, string) {
	switch state {
	case StateRinging:
		return ringingWatchdogTimeout, "call ringing timed out"
	case StateConnecting:
		return connectingWatchdogTimeout, "call media negotiation timed out"
	default:
		return 0, ""
	}
}

func (r *Runtime) expireWatchdog(callID string, generation uint64, expectedState State, reason string) {
	if r == nil || callID == "" {
		return
	}

	runtimeWatchdogs.Lock()
	watchdogState := runtimeWatchdogs.states[r]
	if watchdogState == nil {
		runtimeWatchdogs.Unlock()
		return
	}
	entry, ok := watchdogState.entries[callID]
	if !ok || entry.generation != generation {
		runtimeWatchdogs.Unlock()
		return
	}
	delete(watchdogState.entries, callID)
	callback := watchdogState.onTimeout
	runtimeWatchdogs.Unlock()

	r.mu.Lock()
	call, ok := r.calls[callID]
	if !ok || call.State != expectedState {
		r.mu.Unlock()
		return
	}
	call.State = StateFailed
	call.Error = reason
	call.EndReason = "timeout"
	call.UpdatedAt = time.Now().UTC()
	r.calls[callID] = call
	instanceID := r.instanceID
	r.mu.Unlock()

	if callback != nil {
		callback(instanceID, callID)
	}
}

func (r *Runtime) cancelWatchdog(callID string) {
	if r == nil || callID == "" {
		return
	}
	runtimeWatchdogs.Lock()
	state := runtimeWatchdogs.states[r]
	if state != nil {
		if entry, ok := state.entries[callID]; ok {
			if entry.timer != nil {
				entry.timer.Stop()
			}
			delete(state.entries, callID)
		}
	}
	runtimeWatchdogs.Unlock()
}

func (r *Runtime) closeWatchdogs() {
	if r == nil {
		return
	}
	runtimeWatchdogs.Lock()
	state := runtimeWatchdogs.states[r]
	delete(runtimeWatchdogs.states, r)
	if state != nil {
		for callID, entry := range state.entries {
			if entry.timer != nil {
				entry.timer.Stop()
			}
			delete(state.entries, callID)
		}
		state.onTimeout = nil
	}
	runtimeWatchdogs.Unlock()
}
