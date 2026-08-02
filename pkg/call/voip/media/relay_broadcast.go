package media

import "fmt"

// Broadcast sends one already-framed packet to every open relay connection for
// the selected call. The caller retains ownership of the supplied buffer.
func (r *RelayRegistry) Broadcast(instanceID, callID string, data []byte) error {
	if r == nil {
		return fmt.Errorf("relay registry is nil")
	}
	r.mu.RLock()
	session := r.sessions[instanceID]
	r.mu.RUnlock()
	if session == nil {
		return fmt.Errorf("relay runtime is not attached for instance %s", instanceID)
	}

	session.mu.Lock()
	relay := session.transports[callID]
	session.mu.Unlock()
	if relay == nil {
		return fmt.Errorf("call %s has no relay transport", callID)
	}
	return relay.Broadcast(data)
}

func (r *RelayRegistry) HasConnection(instanceID, callID string) bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	session := r.sessions[instanceID]
	r.mu.RUnlock()
	if session == nil {
		return false
	}
	session.mu.Lock()
	relay := session.transports[callID]
	session.mu.Unlock()
	return relay != nil && relay.HasConnection()
}
