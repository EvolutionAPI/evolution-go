package media

import (
	"bytes"
	"testing"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
)

func TestPacketRegistryFallsBackToAuthenticatedReceiveJID(t *testing.T) {
	callKey := bytes.Repeat([]byte{0x31}, 32)
	registry := NewPacketRegistry(&fakePacketSource{callKey: callKey})
	const (
		instanceID = "instance-candidates"
		callID     = "call-candidates"
		selfDevice = "self:3@lid"
		wrongPeer  = "peer:99@hosted.lid"
		actualPeer = "peer:0@lid"
		selfSSRC   = uint32(0x10101010)
		peerSSRC   = uint32(0x20202020)
	)

	if err := registry.PrepareWithDeviceCandidates(
		instanceID,
		callID,
		selfDevice,
		[]string{wrongPeer, actualPeer},
		selfSSRC,
		peerSSRC,
	); err != nil {
		t.Fatal(err)
	}
	defer registry.Close(instanceID)

	peerSend, err := DerivePerJIDSRTPKey(callKey, actualPeer)
	if err != nil {
		t.Fatal(err)
	}
	defer peerSend.Wipe()
	peerReceive, err := DerivePerJIDSRTPKey(callKey, selfDevice)
	if err != nil {
		t.Fatal(err)
	}
	defer peerReceive.Wipe()
	peerSession, err := NewSRTPSession(peerSend, peerReceive, core.SRTPRecvAuthTagLen, core.SRTPSendAuthTagLen)
	if err != nil {
		t.Fatal(err)
	}
	defer peerSession.Close()

	peerRTP, err := NewWhatsAppOpusRTPSession(peerSSRC)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := peerSession.Protect(peerRTP.CreatePacket([]byte("authenticated peer audio"), true))
	if err != nil {
		t.Fatal(err)
	}

	packet, err := registry.Unprotect(instanceID, callID, frame)
	if err != nil {
		t.Fatalf("expected fallback receive key to authenticate packet: %v", err)
	}
	defer packet.Wipe()
	if !bytes.Equal(packet.Payload, []byte("authenticated peer audio")) {
		t.Fatalf("unexpected payload: %q", packet.Payload)
	}

	session, err := registry.packetSession(instanceID, callID, false)
	if err != nil {
		t.Fatal(err)
	}
	session.mu.RLock()
	selected := session.srtpCandidates[session.activeCandidate].receiveJID
	observed := session.receiveObserved
	session.mu.RUnlock()
	if !observed || selected != actualPeer {
		t.Fatalf("unexpected selected receive JID: observed=%v selected=%s", observed, selected)
	}
}
