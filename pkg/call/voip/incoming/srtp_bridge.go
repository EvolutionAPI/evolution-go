package incoming

import (
	"fmt"
	"strings"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	call_media "github.com/evolution-foundation/evolution-go/pkg/call/voip/media"
)

func ensureSRTPDeviceJID(value string) string {
	at := strings.IndexByte(value, '@')
	if at <= 0 {
		return value
	}
	if strings.Contains(value[:at], ":") {
		return value
	}
	return value[:at] + ":0" + value[at:]
}

// receiveDeviceJID keeps relay routing and SRTP key derivation separate.
// WhatsApp relay participants may contain synthetic hosted.lid devices (for
// example :99@hosted.lid) that are valid for SSRC subscription, but outgoing
// receive keys are derived from the account/device that accepted the call.
func receiveDeviceJID(material *callMaterial, relayPeerDeviceJID string) string {
	if material != nil && material.state != nil && material.state.Direction == core.CallDirectionOutgoing && !material.peer.IsEmpty() {
		return ensureSRTPDeviceJID(material.peer.String())
	}
	return ensureSRTPDeviceJID(relayPeerDeviceJID)
}

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
	receiveJID := receiveDeviceJID(material, peerDeviceJID)
	s.mu.RUnlock()
	defer zeroBytes(callKey)

	sendKeying, err := call_media.DerivePerJIDSRTPKey(callKey, selfDeviceJID)
	if err != nil {
		return core.SRTPKeyingMaterial{}, core.SRTPKeyingMaterial{}, fmt.Errorf("derive send SRTP keying: %w", err)
	}
	receiveKeying, err := call_media.DerivePerJIDSRTPKey(callKey, receiveJID)
	if err != nil {
		sendKeying.Wipe()
		return core.SRTPKeyingMaterial{}, core.SRTPKeyingMaterial{}, fmt.Errorf("derive receive SRTP keying for %s: %w", receiveJID, err)
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
