package incoming

import (
	"bytes"
	"testing"

	call_media "github.com/evolution-foundation/evolution-go/pkg/call/voip/media"
	"go.mau.fi/whatsmeow/types"
)

func TestSessionDerivesSRTPWithoutExposingCallKey(t *testing.T) {
	callKey := bytes.Repeat([]byte{0x42}, 32)
	session := newSession(nil)
	peer := types.NewJID("5511999999999", types.HiddenUserServer)
	creator := types.NewJID("5511000000000", types.HiddenUserServer)
	session.storeOutgoing("call-1", callKey, peer, creator, false, nil)
	defer session.clear()

	// The relay may expose a synthetic hosted.lid participant. For an outgoing
	// call the receive key must still use the accepted peer account/device.
	send, receive, err := session.deriveSRTPKeying("call-1", "self:1@lid", "5511999999999:99@hosted.lid")
	if err != nil {
		t.Fatal(err)
	}
	defer send.Wipe()
	defer receive.Wipe()

	wantSend, err := call_media.DerivePerJIDSRTPKey(callKey, "self:1@lid")
	if err != nil {
		t.Fatal(err)
	}
	defer wantSend.Wipe()
	wantReceive, err := call_media.DerivePerJIDSRTPKey(callKey, "5511999999999:0@lid")
	if err != nil {
		t.Fatal(err)
	}
	defer wantReceive.Wipe()

	if !bytes.Equal(send.MasterKey, wantSend.MasterKey) || !bytes.Equal(send.MasterSalt, wantSend.MasterSalt) {
		t.Fatal("send SRTP keying mismatch")
	}
	if !bytes.Equal(receive.MasterKey, wantReceive.MasterKey) || !bytes.Equal(receive.MasterSalt, wantReceive.MasterSalt) {
		t.Fatal("receive SRTP keying mismatch")
	}

	zeroBytes(send.MasterKey)
	zeroBytes(send.MasterSalt)
	again, againReceive, err := session.deriveSRTPKeying("call-1", "self:1@lid", "5511999999999:99@hosted.lid")
	if err != nil {
		t.Fatal(err)
	}
	defer again.Wipe()
	defer againReceive.Wipe()
	if bytes.Equal(again.MasterKey, make([]byte, len(again.MasterKey))) {
		t.Fatal("wiping returned material modified the stored call key")
	}
}

func TestReceiveDeviceJIDKeepsIncomingRelayParticipant(t *testing.T) {
	material := &callMaterial{}
	got := receiveDeviceJID(material, "5511888888888:7@lid")
	if got != "5511888888888:7@lid" {
		t.Fatalf("receiveDeviceJID() = %q, want relay participant", got)
	}
}

func TestEnsureSRTPDeviceJID(t *testing.T) {
	if got := ensureSRTPDeviceJID("5511999999999@lid"); got != "5511999999999:0@lid" {
		t.Fatalf("ensureSRTPDeviceJID() = %q", got)
	}
	if got := ensureSRTPDeviceJID("5511999999999:3@lid"); got != "5511999999999:3@lid" {
		t.Fatalf("device JID changed: %q", got)
	}
}

func TestSessionRejectsMissingSRTPMaterial(t *testing.T) {
	session := newSession(nil)
	if _, _, err := session.deriveSRTPKeying("missing", "self@lid", "peer@lid"); err == nil {
		t.Fatal("expected missing call-key error")
	}
	if _, _, err := session.deriveSRTPKeying("", "self@lid", "peer@lid"); err == nil {
		t.Fatal("expected empty call ID error")
	}
}
