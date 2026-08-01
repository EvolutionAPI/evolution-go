package transport

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestVarintEncoding(t *testing.T) {
	cases := []struct {
		input uint64
		want  []byte
	}{
		{0, []byte{0x00}},
		{127, []byte{0x7f}},
		{128, []byte{0x80, 0x01}},
		{300, []byte{0xac, 0x02}},
	}
	for _, testCase := range cases {
		if got := encodeVarint(testCase.input); !bytes.Equal(got, testCase.want) {
			t.Fatalf("encodeVarint(%d)=%x, want %x", testCase.input, got, testCase.want)
		}
	}
}

func TestSenderSubscriptions(t *testing.T) {
	inner := []byte{0x18, 0x10, 0x28, 0x00, 0x30, 0x00}
	want := append([]byte{0x0a, byte(len(inner))}, inner...)
	if got := BuildSenderSubscriptions(0x10); !bytes.Equal(got, want) {
		t.Fatalf("sender subscriptions mismatch: got=%x want=%x", got, want)
	}
}

func TestSSRCSubscriptionListOmitsZeroValues(t *testing.T) {
	withoutZero := BuildSSRCSubscriptionList([]uint32{100}, []uint32{200}, 1, 2)
	withZero := BuildSSRCSubscriptionList([]uint32{0, 100}, []uint32{200, 0}, 1, 2)
	if !bytes.Equal(withoutZero, withZero) {
		t.Fatalf("zero SSRC changed payload: without=%x with=%x", withoutZero, withZero)
	}
}

func TestSTUNBindingFingerprint(t *testing.T) {
	subscriptions := BuildSenderSubscriptions(0x12345678)
	message, err := BuildBindingRequestWithSubscriptions(nil, nil, subscriptions, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint32(message[4:8]) != stunMagicCookie {
		t.Fatal("missing STUN magic cookie")
	}
	info := ParseSTUNResponse(message)
	if info == nil || info.Method != "binding" || info.Class != "request" {
		t.Fatalf("unexpected parsed binding request: %#v", info)
	}
	last := info.Attributes[len(info.Attributes)-1]
	if last.TypeName != "FINGERPRINT" {
		t.Fatalf("expected fingerprint last, got %s", last.TypeName)
	}
	fingerprintStart := len(message) - 8
	want := crc32Checksum(message[:fingerprintStart]) ^ stunFingerprintXOR
	got := binary.BigEndian.Uint32(message[len(message)-4:])
	if got != want {
		t.Fatalf("fingerprint mismatch: got=%08x want=%08x", got, want)
	}
}

func TestAllocateRequestIncludesRelayAddress(t *testing.T) {
	message, err := BuildAllocateForRelay([]byte{1}, []byte{2}, []byte("secret"), "127.0.0.1", 3480)
	if err != nil {
		t.Fatal(err)
	}
	info := ParseSTUNResponse(message)
	if info == nil || info.Method != "allocate" {
		t.Fatalf("unexpected allocation request: %#v", info)
	}
	var found bool
	for _, attribute := range info.Attributes {
		if attribute.TypeName == "XOR-RELAYED-ADDRESS" {
			found = true
		}
	}
	if !found {
		t.Fatal("allocation request did not contain relay address")
	}
}

func TestAllocateRequestRejectsInvalidAddress(t *testing.T) {
	if _, err := BuildAllocateForRelay(nil, nil, nil, "not-an-ip", 3480); err == nil {
		t.Fatal("expected invalid IP error")
	}
	if _, err := BuildAllocateForRelay(nil, nil, nil, "127.0.0.1", 70000); err == nil {
		t.Fatal("expected invalid port error")
	}
}

func TestPacketClassification(t *testing.T) {
	ping, err := BuildWhatsAppPing()
	if err != nil {
		t.Fatal(err)
	}
	if !IsSTUNPacket(ping) || IsRTPPacket(ping) {
		t.Fatalf("unexpected ping classification: %s", ClassifyPacket(ping))
	}
	if classification := ClassifyPacket(ping); !strings.Contains(classification, "wa-ping") {
		t.Fatalf("unexpected ping description: %s", classification)
	}

	rtp := []byte{0x80, 120, 0x01, 0x02}
	if !IsRTPPacket(rtp) || IsSTUNPacket(rtp) {
		t.Fatalf("unexpected RTP classification: %s", ClassifyPacket(rtp))
	}
	if classification := ClassifyPacket(rtp); !strings.Contains(classification, "seq=258") {
		t.Fatalf("unexpected RTP description: %s", classification)
	}
}

func TestParseSTUNRejectsTruncatedMessage(t *testing.T) {
	message, err := BuildBindingRequestWithSubscriptions(nil, nil, nil, false, false)
	if err != nil {
		t.Fatal(err)
	}
	message = message[:len(message)-1]
	if ParseSTUNResponse(message) != nil {
		t.Fatal("expected truncated STUN message to be rejected")
	}
}

func crc32Checksum(value []byte) uint32 {
	return crc32.ChecksumIEEE(value)
}
