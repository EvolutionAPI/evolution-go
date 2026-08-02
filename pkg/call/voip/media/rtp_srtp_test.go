package media

import (
	"bytes"
	"errors"
	"testing"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
)

func testKeying(t *testing.T, fill byte, jid string) core.SRTPKeyingMaterial {
	t.Helper()
	material, err := DerivePerJIDSRTPKey(bytes.Repeat([]byte{fill}, 32), jid)
	if err != nil {
		t.Fatal(err)
	}
	return material
}

func TestDerivePerJIDSRTPKey(t *testing.T) {
	callKey := bytes.Repeat([]byte{0xab}, 32)
	first, err := DerivePerJIDSRTPKey(callKey, "5511999999999:3@lid")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Wipe()
	second, err := DerivePerJIDSRTPKey(callKey, "5511999999999:3@lid")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Wipe()
	other, err := DerivePerJIDSRTPKey(callKey, "5511999999999:4@lid")
	if err != nil {
		t.Fatal(err)
	}
	defer other.Wipe()

	if len(first.MasterKey) != 16 || len(first.MasterSalt) != 14 {
		t.Fatalf("unexpected keying lengths: key=%d salt=%d", len(first.MasterKey), len(first.MasterSalt))
	}
	if !bytes.Equal(first.MasterKey, second.MasterKey) || !bytes.Equal(first.MasterSalt, second.MasterSalt) {
		t.Fatal("per-device derivation is not deterministic")
	}
	if bytes.Equal(first.MasterKey, other.MasterKey) && bytes.Equal(first.MasterSalt, other.MasterSalt) {
		t.Fatal("different device JIDs produced identical keying material")
	}
	if _, err = DerivePerJIDSRTPKey(callKey[:31], "device@lid"); err == nil {
		t.Fatal("expected invalid call-key length error")
	}
	if _, err = DerivePerJIDSRTPKey(callKey, ""); err == nil {
		t.Fatal("expected empty device JID error")
	}
}

func TestRTPPacketRoundTripWithExtensionAndPadding(t *testing.T) {
	header := NewRTPHeader(core.PayloadTypeWhatsAppOpus, 0x1234, 0xdeadbeef, 0xcafebabe)
	header.Marker = true
	header.Extension = true
	header.ExtensionProfile = 0xbede
	header.ExtensionData = []byte{1, 2, 3, 4, 5, 6, 7, 8}
	header.CSRC = []uint32{0x01020304, 0x05060708}
	header.Padding = true
	packet := &RTPPacket{Header: header, Payload: []byte{9, 8, 7, 6}, PaddingSize: 4}

	encoded, err := packet.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ParseRTPPacket(encoded)
	if err != nil {
		t.Fatal(err)
	}
	defer decoded.Wipe()
	if decoded.Header.SequenceNumber != header.SequenceNumber || decoded.Header.Timestamp != header.Timestamp || decoded.Header.SSRC != header.SSRC {
		t.Fatalf("header mismatch: %+v", decoded.Header)
	}
	if !bytes.Equal(decoded.Header.ExtensionData, header.ExtensionData) || !bytes.Equal(decoded.Payload, packet.Payload) {
		t.Fatal("RTP extension or payload mismatch")
	}
	if decoded.PaddingSize != 4 || len(decoded.Header.CSRC) != 2 {
		t.Fatalf("unexpected padding or CSRC count: padding=%d csrc=%d", decoded.PaddingSize, len(decoded.Header.CSRC))
	}
}

func TestRTPRejectsMalformedFrames(t *testing.T) {
	if _, err := ParseRTPPacket([]byte{0x80}); err == nil {
		t.Fatal("expected short RTP frame error")
	}
	header := NewRTPHeader(120, 1, 2, 3)
	header.Extension = true
	header.ExtensionData = []byte{1, 2, 3}
	if _, err := (&RTPPacket{Header: header, Payload: []byte{1}}).Marshal(); err == nil {
		t.Fatal("expected unaligned extension error")
	}
	header = NewRTPHeader(120, 1, 2, 3)
	header.Padding = true
	if _, err := (&RTPPacket{Header: header, Payload: []byte{1}}).Marshal(); err == nil {
		t.Fatal("expected missing padding-size error")
	}
}

func TestSRTPRoundTripAndAuthentication(t *testing.T) {
	self := testKeying(t, 0x11, "self:0@lid")
	peer := testKeying(t, 0x11, "peer:0@lid")
	defer self.Wipe()
	defer peer.Wipe()

	sender, err := NewSRTPSession(self, peer, core.SRTPSendAuthTagLen, core.SRTPRecvAuthTagLen)
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()
	receiver, err := NewSRTPSession(peer, self, core.SRTPRecvAuthTagLen, core.SRTPSendAuthTagLen)
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()

	rtp, err := NewWhatsAppOpusRTPSession(0xaabbccdd)
	if err != nil {
		t.Fatal(err)
	}
	payload := bytes.Repeat([]byte{0x42}, 40)
	packet := rtp.CreatePacket(payload, true)
	protected, err := sender.Protect(packet)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := receiver.Unprotect(protected)
	if err != nil {
		t.Fatal(err)
	}
	defer plain.Wipe()
	if !bytes.Equal(plain.Payload, payload) || plain.Header.SSRC != packet.Header.SSRC {
		t.Fatal("SRTP roundtrip mismatch")
	}

	tampered := append([]byte(nil), protected...)
	tampered[len(tampered)-1] ^= 0xff
	_, err = receiver.Unprotect(tampered)
	var srtpErr *SRTPError
	if !errors.As(err, &srtpErr) || srtpErr.Type != SRTPErrAuthFailed {
		t.Fatalf("expected authentication error, got %v", err)
	}
	_, err = receiver.Unprotect(protected)
	if !errors.As(err, &srtpErr) || srtpErr.Type != SRTPErrReplay {
		t.Fatalf("expected replay error, got %v", err)
	}
}

func TestSRTPAcceptsAuthenticatedOutOfOrderPackets(t *testing.T) {
	self := testKeying(t, 0x22, "self:0@lid")
	peer := testKeying(t, 0x22, "peer:0@lid")
	defer self.Wipe()
	defer peer.Wipe()
	sender, _ := NewSRTPSession(self, peer, 4, 4)
	receiver, _ := NewSRTPSession(peer, self, 4, 4)
	defer sender.Close()
	defer receiver.Close()

	first := &RTPPacket{Header: NewRTPHeader(120, 100, 1000, 55), Payload: []byte("first")}
	second := &RTPPacket{Header: NewRTPHeader(120, 101, 1960, 55), Payload: []byte("second")}
	protectedFirst, err := sender.Protect(first)
	if err != nil {
		t.Fatal(err)
	}
	protectedSecond, err := sender.Protect(second)
	if err != nil {
		t.Fatal(err)
	}
	decodedSecond, err := receiver.Unprotect(protectedSecond)
	if err != nil {
		t.Fatal(err)
	}
	decodedSecond.Wipe()
	decodedFirst, err := receiver.Unprotect(protectedFirst)
	if err != nil {
		t.Fatal(err)
	}
	defer decodedFirst.Wipe()
	if string(decodedFirst.Payload) != "first" {
		t.Fatalf("unexpected out-of-order payload: %q", decodedFirst.Payload)
	}
}

func TestSRTPSequenceRollover(t *testing.T) {
	self := testKeying(t, 0x33, "self:0@lid")
	peer := testKeying(t, 0x33, "peer:0@lid")
	defer self.Wipe()
	defer peer.Wipe()
	sender, _ := NewSRTPSession(self, peer, 4, 4)
	receiver, _ := NewSRTPSession(peer, self, 4, 4)
	defer sender.Close()
	defer receiver.Close()

	before := &RTPPacket{Header: NewRTPHeader(120, 0xffff, 1, 99), Payload: []byte{1}}
	after := &RTPPacket{Header: NewRTPHeader(120, 0, 2, 99), Payload: []byte{2}}
	protectedBefore, err := sender.Protect(before)
	if err != nil {
		t.Fatal(err)
	}
	protectedAfter, err := sender.Protect(after)
	if err != nil {
		t.Fatal(err)
	}
	decodedBefore, err := receiver.Unprotect(protectedBefore)
	if err != nil {
		t.Fatal(err)
	}
	decodedBefore.Wipe()
	decodedAfter, err := receiver.Unprotect(protectedAfter)
	if err != nil {
		t.Fatal(err)
	}
	defer decodedAfter.Wipe()
	if !bytes.Equal(decodedAfter.Payload, []byte{2}) {
		t.Fatalf("unexpected rollover payload: %v", decodedAfter.Payload)
	}
}

func TestSRTPRejectsSendIndexReuseAndClosedContext(t *testing.T) {
	self := testKeying(t, 0x44, "self:0@lid")
	peer := testKeying(t, 0x44, "peer:0@lid")
	defer self.Wipe()
	defer peer.Wipe()
	session, err := NewSRTPSession(self, peer, 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	packet := &RTPPacket{Header: NewRTPHeader(120, 7, 1, 1), Payload: []byte{1}}
	if _, err = session.Protect(packet); err != nil {
		t.Fatal(err)
	}
	if _, err = session.Protect(packet); err == nil {
		t.Fatal("expected duplicate send sequence to be rejected")
	}
	session.Close()
	if _, err = session.Protect(&RTPPacket{Header: NewRTPHeader(120, 8, 2, 1), Payload: []byte{2}}); err == nil {
		t.Fatal("expected closed session error")
	}
}
