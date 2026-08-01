package media

import (
	"bytes"
	"errors"
	"testing"

	call_state "github.com/evolution-foundation/evolution-go/pkg/call/voip/call"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
)

type fakePacketSource struct {
	callKey []byte
}

func (f *fakePacketSource) RelayData(string, string) (*core.RelayData, bool) {
	return nil, false
}

func (f *fakePacketSource) State(string, string) (*call_state.Info, bool) {
	return nil, false
}

func (f *fakePacketSource) SRTPKeying(_, _, selfDeviceJID, peerDeviceJID string) (core.SRTPKeyingMaterial, core.SRTPKeyingMaterial, error) {
	send, err := DerivePerJIDSRTPKey(f.callKey, selfDeviceJID)
	if err != nil {
		return core.SRTPKeyingMaterial{}, core.SRTPKeyingMaterial{}, err
	}
	receive, err := DerivePerJIDSRTPKey(f.callKey, peerDeviceJID)
	if err != nil {
		send.Wipe()
		return core.SRTPKeyingMaterial{}, core.SRTPKeyingMaterial{}, err
	}
	return send, receive, nil
}

func TestPacketRegistryProtectsAndUnprotectsOpus(t *testing.T) {
	callKey := bytes.Repeat([]byte{0x5a}, 32)
	source := &fakePacketSource{callKey: callKey}
	registry := NewPacketRegistry(source)
	const (
		instanceID = "instance-1"
		callID     = "call-1"
		selfDevice = "5511000000000:1@lid"
		peerDevice = "5511999999999:2@lid"
		selfSSRC   = uint32(0x11223344)
		peerSSRC   = uint32(0x55667788)
	)
	if err := registry.PrepareWithDevices(instanceID, callID, selfDevice, peerDevice, selfSSRC, peerSSRC); err != nil {
		t.Fatal(err)
	}
	defer registry.Close(instanceID)

	peerSend, err := DerivePerJIDSRTPKey(callKey, peerDevice)
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

	incomingPayload := []byte("peer opus frame")
	incomingFrame, err := peerSession.Protect(peerRTP.CreatePacket(incomingPayload, true))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := registry.Unprotect(instanceID, callID, incomingFrame)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded.Payload, incomingPayload) || decoded.Header.SSRC != peerSSRC {
		t.Fatalf("unexpected decoded packet: ssrc=%d payload=%q", decoded.Header.SSRC, decoded.Payload)
	}
	decoded.Wipe()

	outgoingPayload := []byte("local opus frame")
	outgoingFrame, err := registry.ProtectOpus(instanceID, callID, outgoingPayload, 960, true)
	if err != nil {
		t.Fatal(err)
	}
	peerDecoded, err := peerSession.Unprotect(outgoingFrame)
	if err != nil {
		t.Fatal(err)
	}
	defer peerDecoded.Wipe()
	if !bytes.Equal(peerDecoded.Payload, outgoingPayload) || peerDecoded.Header.SSRC != selfSSRC {
		t.Fatalf("unexpected peer packet: ssrc=%d payload=%q", peerDecoded.Header.SSRC, peerDecoded.Payload)
	}
}

func TestPacketRegistryHandleInvokesCallbackAndRejectsNonRTP(t *testing.T) {
	callKey := bytes.Repeat([]byte{0x6b}, 32)
	registry := NewPacketRegistry(&fakePacketSource{callKey: callKey})
	const (
		instanceID = "instance-2"
		callID     = "call-2"
		selfDevice = "self:1@lid"
		peerDevice = "peer:2@lid"
		selfSSRC   = uint32(101)
		peerSSRC   = uint32(202)
	)
	if err := registry.PrepareWithDevices(instanceID, callID, selfDevice, peerDevice, selfSSRC, peerSSRC); err != nil {
		t.Fatal(err)
	}
	defer registry.Close(instanceID)

	peerSend, _ := DerivePerJIDSRTPKey(callKey, peerDevice)
	peerReceive, _ := DerivePerJIDSRTPKey(callKey, selfDevice)
	defer peerSend.Wipe()
	defer peerReceive.Wipe()
	peerSession, err := NewSRTPSession(peerSend, peerReceive, 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer peerSession.Close()
	peerRTP, _ := NewWhatsAppOpusRTPSession(peerSSRC)
	frame, err := peerSession.Protect(peerRTP.CreatePacket([]byte{1, 2, 3}, false))
	if err != nil {
		t.Fatal(err)
	}

	called := false
	registry.SetOnRTP(func(gotInstance, gotCall string, packet *RTPPacket) {
		called = true
		if gotInstance != instanceID || gotCall != callID || !bytes.Equal(packet.Payload, []byte{1, 2, 3}) {
			t.Fatalf("unexpected callback data: %s %s %v", gotInstance, gotCall, packet.Payload)
		}
	})
	if err = registry.Handle(instanceID, callID, frame); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("RTP callback was not invoked")
	}
	if err = registry.Handle(instanceID, callID, []byte{0x00, 0x01}); !errors.Is(err, ErrNonRTPFrame) {
		t.Fatalf("expected non-RTP error, got %v", err)
	}
}

func TestPacketRegistryRejectsUnexpectedPeerSSRC(t *testing.T) {
	callKey := bytes.Repeat([]byte{0x7c}, 32)
	registry := NewPacketRegistry(&fakePacketSource{callKey: callKey})
	if err := registry.PrepareWithDevices("instance", "call", "self@lid", "peer@lid", 11, 22); err != nil {
		t.Fatal(err)
	}
	defer registry.Close("instance")

	peerSend, _ := DerivePerJIDSRTPKey(callKey, "peer@lid")
	peerReceive, _ := DerivePerJIDSRTPKey(callKey, "self@lid")
	defer peerSend.Wipe()
	defer peerReceive.Wipe()
	peerSession, _ := NewSRTPSession(peerSend, peerReceive, 4, 4)
	defer peerSession.Close()
	wrongRTP, _ := NewWhatsAppOpusRTPSession(99)
	frame, err := peerSession.Protect(wrongRTP.CreatePacket([]byte{1}, false))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = registry.Unprotect("instance", "call", frame); err == nil {
		t.Fatal("expected unexpected SSRC error")
	}
}
