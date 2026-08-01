package incoming

import (
	"fmt"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	call_media "github.com/evolution-foundation/evolution-go/pkg/call/voip/media"
)

func (s *session) deriveSRTPKeying(callID, selfDeviceJID, peerDeviceJID string) (core.SRTPKeyingMaterial, core.SRTPKeyingMaterial, error) {
	if callID == "" {
		return core.SRTPKeyingMaterial{}, core.SRTPKeyingMaterial{}, fmt.Errorf("call ID is empty")
	}
	if selfDeviceJID == "" || peerDeviceJID == "" {
		return core.SRTPKeyingMaterial{}, core.SRTPKeyingMaterial{}, fmt.Errorf("SRTP device JIDs are incomplete")
	}

	s.mu.RLock()
	material := s.materials[callID]
	if material == nil || len(material.callKey) == 0 {
		s.mu.RUnlock()
		return core.SRTPKeyingMaterial{}, core.SRTPKeyingMaterial{}, fmt.Errorf("call %s has no private encryption key", callID)
	}
	callKey := append([]byte(nil), material.callKey...)
	s.mu.RUnlock()
	defer zeroBytes(callKey)

	sendKeying, err := call_media.DerivePerJIDSRTPKey(callKey, selfDeviceJID)
	if err != nil {
		return core.SRTPKeyingMaterial{}, core.SRTPKeyingMaterial{}, fmt.Errorf("derive send SRTP keying: %w", err)
	}
	receiveKeying, err := call_media.DerivePerJIDSRTPKey(callKey, peerDeviceJID)
	if err != nil {
		sendKeying.Wipe()
		return core.SRTPKeyingMaterial{}, core.SRTPKeyingMaterial{}, fmt.Errorf("derive receive SRTP keying: %w", err)
	}
	return sendKeying, receiveKeying, nil
}

// SRTPKeying derives per-device keying material without exposing the private
// WhatsApp call key outside the negotiation registry. The caller owns both
// returned values and must wipe them after constructing the SRTP session.
func (r *Registry) SRTPKeying(instanceID, callID, selfDeviceJID, peerDeviceJID string) (core.SRTPKeyingMaterial, core.SRTPKeyingMaterial, error) {
	r.mu.RLock()
	s := r.sessions[instanceID]
	r.mu.RUnlock()
	if s == nil {
		return core.SRTPKeyingMaterial{}, core.SRTPKeyingMaterial{}, fmt.Errorf("call runtime is not attached for instance %s", instanceID)
	}
	return s.deriveSRTPKeying(callID, selfDeviceJID, peerDeviceJID)
}
