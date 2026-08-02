package media

import (
	"bytes"
	"testing"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func TestCallKeyPeerCandidatesPreservesDeviceOrderAndDeduplicates(t *testing.T) {
	from := types.NewADJID("5511999999999", 7, 3)
	creator := types.NewJID("5511999999999", types.HiddenUserServer)
	alt := types.NewJID("5511999999999", types.DefaultUserServer)
	event := &events.CallAccept{}
	event.From = from
	event.CallCreator = creator
	event.CallCreatorAlt = alt

	got := callKeyPeerCandidates(event)
	if len(got) != 3 {
		t.Fatalf("unexpected candidate count: %d", len(got))
	}
	if got[0].String() != from.String() || got[1].String() != creator.String() || got[2].String() != alt.String() {
		t.Fatalf("unexpected candidate order: %#v", got)
	}

	duplicated := uniqueCallKeyPeers(from, from, creator, creator)
	if len(duplicated) != 2 {
		t.Fatalf("duplicate peers were not removed: %#v", duplicated)
	}
}

func TestPacketKeyingFingerprintIsStableAndSeparatesMaterials(t *testing.T) {
	first := core.SRTPKeyingMaterial{
		MasterKey:  bytes.Repeat([]byte{0x11}, 16),
		MasterSalt: bytes.Repeat([]byte{0x22}, 14),
	}
	same := core.SRTPKeyingMaterial{
		MasterKey:  append([]byte(nil), first.MasterKey...),
		MasterSalt: append([]byte(nil), first.MasterSalt...),
	}
	different := core.SRTPKeyingMaterial{
		MasterKey:  bytes.Repeat([]byte{0x11}, 16),
		MasterSalt: bytes.Repeat([]byte{0x23}, 14),
	}

	if packetKeyingFingerprint(first) != packetKeyingFingerprint(same) {
		t.Fatal("equal keying material produced different fingerprints")
	}
	if packetKeyingFingerprint(first) == packetKeyingFingerprint(different) {
		t.Fatal("different keying material produced the same fingerprint")
	}
}

func TestEqualCallKeyPeersIsOrderSensitive(t *testing.T) {
	first := types.NewJID("first", types.HiddenUserServer)
	second := types.NewJID("second", types.HiddenUserServer)
	if !equalCallKeyPeers([]types.JID{first, second}, []types.JID{first, second}) {
		t.Fatal("equal peer lists were not recognized")
	}
	if equalCallKeyPeers([]types.JID{first, second}, []types.JID{second, first}) {
		t.Fatal("different peer priority order was treated as equal")
	}
}
