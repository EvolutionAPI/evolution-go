package media

import "fmt"

func (s *relaySession) updatePeerSSRC(callID string, ssrc uint32) error {
	if s == nil || callID == "" || ssrc == 0 {
		return fmt.Errorf("invalid peer SSRC update")
	}
	s.mu.Lock()
	relay := s.transports[callID]
	s.mu.Unlock()
	if relay == nil {
		return fmt.Errorf("relay transport for call %s is not ready", callID)
	}
	relay.SetSubscriptionSSRC(ssrc)
	relay.ResendSubscriptions()
	return nil
}

// UpdatePeerSSRC replaces the negotiation-derived relay subscription with the
// SSRC authenticated from the first real remote RTP frame.
func (r *RelayRegistry) UpdatePeerSSRC(instanceID, callID string, ssrc uint32) error {
	if r == nil {
		return fmt.Errorf("relay registry is not ready")
	}
	r.mu.RLock()
	session := r.sessions[instanceID]
	r.mu.RUnlock()
	if session == nil {
		return fmt.Errorf("relay runtime is not attached for instance %s", instanceID)
	}
	return session.updatePeerSSRC(callID, ssrc)
}
