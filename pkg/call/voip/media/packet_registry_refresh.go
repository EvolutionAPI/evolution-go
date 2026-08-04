package media

// dropSession removes only the RTP/SRTP packet state for a call. Unlike
// Remove, it deliberately preserves a peer-provided call key so the session can
// be rebuilt immediately after a CallAccept carrying a new <enc> payload.
func (r *PacketRegistry) dropSession(instanceID, callID string) {
	if r == nil || instanceID == "" || callID == "" {
		return
	}

	r.mu.Lock()
	calls := r.sessions[instanceID]
	var session *packetSession
	if calls != nil {
		session = calls[callID]
		delete(calls, callID)
		if len(calls) == 0 {
			delete(r.sessions, instanceID)
		}
	}
	r.mu.Unlock()

	if session != nil {
		session.close()
	}
}
