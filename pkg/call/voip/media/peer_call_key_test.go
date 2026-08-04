package media

import (
	"bytes"
	"testing"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func TestCallKeyPeerCandidatesPreservesOrderAndDeduplicates(t *testing.T) {
	from := types.NewJID("5511999999999", types.HiddenUserServer)
	creator := types.NewJID("5511888888888", types.HiddenUserServer)
	alt := types.NewJID("5511777777777", types.DefaultUserServer)
	event := &events.CallAccept{}
	event.From = from
	event.CallCreator = creator
	event.CallCreatorAlt = alt

	got := callKeyPeerCandidates(event)
	if len(got) != 3 {
		t.Fatalf("unexpected candidate count: %d (%#v)", len(got), got)
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

func TestStaleClientCannotDeleteReplacementPeerKeys(t *testing.T) {
	registry := NewPacketRegistry(&fakePacketSource{callKey: bytes.Repeat([]byte{0x31}, 32)})
	const (
		instanceID = "replacement-instance"
		callID     = "replacement-call"
	)
	staleClient := &whatsmeow.Client{}
	activeClient := &whatsmeow.Client{}
	peer := types.NewJID("5511888888888", types.HiddenUserServer)

	peerCallKeyObservers.Lock()
	peerCallKeyObservers.registries[registry] = map[string]*peerCallKeyObserver{
		instanceID: {
			client: activeClient,
			keys: map[string]storedPeerCallKey{
				callID: {
					key:   bytes.Repeat([]byte{0x52}, 32),
					peers: []types.JID{peer},
				},
			},
		},
	}
	peerCallKeyObservers.Unlock()
	defer detachPeerCallKeyObserver(registry, instanceID)

	removePeerCallKeyForClient(registry, instanceID, staleClient, callID)
	clearPeerCallKeysForClient(registry, instanceID, staleClient)
	key, peers, ok := peerCallKey(registry, instanceID, callID)
	if !ok || len(key) != 32 || len(peers) != 1 || peers[0].String() != peer.String() {
		t.Fatalf("stale client removed replacement key: ok=%v key=%d peers=%#v", ok, len(key), peers)
	}
	zeroBytes(key)

	removePeerCallKeyForClient(registry, instanceID, activeClient, callID)
	if _, _, ok = peerCallKey(registry, instanceID, callID); ok {
		t.Fatal("active client failed to remove its own peer key")
	}
}
