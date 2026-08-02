package incoming

import (
	"fmt"

	call_state "github.com/evolution-foundation/evolution-go/pkg/call/voip/call"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	waBinary "go.mau.fi/whatsmeow/binary"
)

// CaptureRelayNode merges relay metadata into private call material. Nothing is
// copied into the public runtime snapshot.
func (r *Registry) CaptureRelayNode(instanceID, callID string, node *waBinary.Node) {
	r.mu.RLock()
	session := r.sessions[instanceID]
	r.mu.RUnlock()
	if session != nil {
		session.captureRelays(callID, node)
	}
}

// EnsureRemoteAccepted idempotently advances an outgoing call to connecting.
func (r *Registry) EnsureRemoteAccepted(instanceID, callID string) error {
	r.mu.RLock()
	session := r.sessions[instanceID]
	r.mu.RUnlock()
	if session == nil {
		return fmt.Errorf("call runtime is not attached for instance %s", instanceID)
	}
	state, ok := session.state(callID)
	if !ok {
		return fmt.Errorf("call %s has no private state", callID)
	}
	switch state.StateData.State {
	case core.CallStateConnecting, core.CallStateActive, core.CallStateOnHold:
		return nil
	case core.CallStateRinging:
		return session.transition(callID, call_state.Transition{Type: call_state.TransitionRemoteAccepted})
	default:
		return fmt.Errorf("call %s cannot accept a remote answer in state %s", callID, state.StateData.State)
	}
}

// MarkMediaConnected idempotently advances a negotiated call to active.
func (r *Registry) MarkMediaConnected(instanceID, callID string) error {
	r.mu.RLock()
	session := r.sessions[instanceID]
	r.mu.RUnlock()
	if session == nil {
		return fmt.Errorf("call runtime is not attached for instance %s", instanceID)
	}
	state, ok := session.state(callID)
	if !ok {
		return fmt.Errorf("call %s has no private state", callID)
	}
	switch state.StateData.State {
	case core.CallStateActive, core.CallStateOnHold:
		return nil
	case core.CallStateConnecting:
		return session.transition(callID, call_state.Transition{Type: call_state.TransitionMediaConnected})
	default:
		return fmt.Errorf("call %s cannot connect media in state %s", callID, state.StateData.State)
	}
}
