package signaling

import (
	"bytes"
	"encoding/base64"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
)

func TestParseRelayFromAck(t *testing.T) {
	token := []byte{0x01, 0x02, 0x03}
	authToken := []byte{0x04, 0x05, 0x06}
	hbhKey := bytes.Repeat([]byte{0x07}, 30)

	node := &waBinary.Node{
		Tag: "ack",
		Content: []waBinary.Node{
			{
				Tag: "user",
				Content: []waBinary.Node{
					{Tag: "device", Attrs: waBinary.Attrs{"jid": "self:1@s.whatsapp.net"}},
				},
			},
			{
				Tag: "relay",
				Attrs: waBinary.Attrs{
					"uuid":     "relay-uuid",
					"self_pid": "11",
					"peer_pid": "22",
				},
				Content: []waBinary.Node{
					{Tag: "participant", Attrs: waBinary.Attrs{"jid": "self:1@s.whatsapp.net"}},
					{Tag: "participant", Attrs: waBinary.Attrs{"jid": "peer:2@s.whatsapp.net"}},
					{Tag: "key", Content: []byte("relay-key")},
					{Tag: "hbh_key", Content: hbhKey},
					{Tag: "token", Attrs: waBinary.Attrs{"id": "token-1"}, Content: token},
					{Tag: "auth_token", Attrs: waBinary.Attrs{"id": "auth-1"}, Content: authToken},
					{
						Tag: "te2",
						Attrs: waBinary.Attrs{
							"token_id":     "token-1",
							"auth_token_id": "auth-1",
							"relay_id":      "1",
							"relay_name":    "slow",
							"protocol":      "1",
							"c2r_rtt":       "40",
						},
						Content: []byte{1, 2, 3, 4, 0x0d, 0x98},
					},
					{
						Tag: "te2",
						Attrs: waBinary.Attrs{
							"token_id":     "token-1",
							"auth_token_id": "auth-1",
							"relay_id":      "2",
							"relay_name":    "fast",
							"protocol":      "1",
							"c2r_rtt":       "10",
						},
						Content: []byte{5, 6, 7, 8, 0x0d, 0x99},
					},
					{Tag: "te2", Content: []byte{1, 2, 3}},
				},
			},
		},
	}

	parsed := ParseRelayFromAck(node)
	if parsed.UUID != "relay-uuid" {
		t.Fatalf("UUID = %q", parsed.UUID)
	}
	if parsed.SelfPID == nil || *parsed.SelfPID != 11 {
		t.Fatalf("SelfPID = %#v", parsed.SelfPID)
	}
	if parsed.PeerPID == nil || *parsed.PeerPID != 22 {
		t.Fatalf("PeerPID = %#v", parsed.PeerPID)
	}
	if len(parsed.ParticipantJIDs) != 2 {
		t.Fatalf("participants = %#v", parsed.ParticipantJIDs)
	}
	if len(parsed.Relays) != 2 {
		t.Fatalf("relays = %#v", parsed.Relays)
	}

	fast := parsed.Relays[0]
	if fast.IP != "5.6.7.8" || fast.Port != 3481 || fast.RelayName != "fast" {
		t.Fatalf("fast relay = %#v", fast)
	}
	if fast.C2RRtt == nil || *fast.C2RRtt != 10 {
		t.Fatalf("fast relay RTT = %#v", fast.C2RRtt)
	}
	if fast.Token != base64.StdEncoding.EncodeToString(token) {
		t.Fatalf("token = %q", fast.Token)
	}
	if fast.AuthToken != base64.StdEncoding.EncodeToString(authToken) {
		t.Fatalf("auth token = %q", fast.AuthToken)
	}
	if fast.Key != "relay-key" || fast.AuthTokenID != "auth-1" {
		t.Fatalf("relay credentials metadata = %#v", fast)
	}
	if !bytes.Equal(parsed.HBHKey, hbhKey) {
		t.Fatalf("HBH key mismatch")
	}

	// Parsed secrets must not alias protocol-node buffers.
	token[0] = 0xff
	authToken[0] = 0xff
	hbhKey[0] = 0xff
	if fast.RawToken[0] != 0x01 || fast.RawAuthToken[0] != 0x04 || parsed.HBHKey[0] != 0x07 {
		t.Fatal("parsed relay material aliases input buffers")
	}
}

func TestExtractRelayEndpoints(t *testing.T) {
	node := &waBinary.Node{
		Tag: "offer",
		Content: []waBinary.Node{
			{
				Tag: "relays",
				Content: []waBinary.Node{
					{Tag: "relay", Attrs: waBinary.Attrs{
						"ip": "10.0.0.2", "port": "4000", "token": "slow-token",
						"relay_key": "key-2", "relay_id": "2", "c2r_rtt": "30",
					}},
				},
			},
			{Tag: "relay", Attrs: waBinary.Attrs{
				"ip": "10.0.0.1", "token": "fast-token", "relay-key": "key-1",
				"relay-id": "1", "relay-name": "fast", "c2r-rtt": "5",
			}},
			{Tag: "relay", Attrs: waBinary.Attrs{"ip": "10.0.0.3"}},
		},
	}

	relays := ExtractRelayEndpoints(node)
	if len(relays) != 2 {
		t.Fatalf("relay count = %d", len(relays))
	}
	if relays[0].IP != "10.0.0.1" || relays[0].Port != 3480 || relays[0].RelayID != 1 {
		t.Fatalf("first relay = %#v", relays[0])
	}
	if relays[1].IP != "10.0.0.2" || relays[1].Port != 4000 || relays[1].Key != "key-2" {
		t.Fatalf("second relay = %#v", relays[1])
	}
}

func TestParseRelayFromAckDecodesBase64HBHKey(t *testing.T) {
	expected := bytes.Repeat([]byte{0x42}, 30)
	node := &waBinary.Node{Tag: "ack", Content: []waBinary.Node{{
		Tag: "relay",
		Content: []waBinary.Node{{Tag: "hbh_key", Content: []byte(base64.StdEncoding.EncodeToString(expected))}},
	}}}

	parsed := ParseRelayFromAck(node)
	if !bytes.Equal(parsed.HBHKey, expected) {
		t.Fatalf("decoded HBH key = %x", parsed.HBHKey)
	}
}
