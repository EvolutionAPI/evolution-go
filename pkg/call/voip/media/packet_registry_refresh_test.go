package media

import (
	"bytes"
	"errors"
	"testing"

	"go.mau.fi/whatsmeow/types"
)

func TestDropSessionPreservesPeerCallKeyUntilFinalRemove(t *testing.T) {
	registry := NewPacketRegistry(&fakePacketSource{callKey: bytes.Repeat([]byte{0x41}, 32)})
	const (
		instanceID = "refresh-instance"
		callID     = "refresh-call"
	)
	if err := registry.PrepareWithDevices(instanceID, callID, "self:1@lid", "peer:2@lid", 101, 202); err != nil {
		t.Fatal(err)
	}

	peer := types.NewJID("peer", types.HiddenUserServer)
	peerCallKeyObservers.Lock()
	peerCallKeyObservers.registries[registry] = map[string]*peerCallKeyObserver{
		instanceID: {
			keys: map[string]storedPeerCallKey{
				callID: {key: bytes.Repeat([]byte{0x52}, 32), peers: []types.JID{peer}},
			},
		},
	}
	peerCallKeyObservers.Unlock()
	defer detachPeerCallKeyObserver(registry, instanceID)

	registry.dropSession(instanceID, callID)
	if _, err := registry.packetSession(instanceID, callID, false); !errors.Is(err, ErrPacketSessionNotReady) {
		t.Fatalf("packet session remained after refresh drop: %v", err)
	}
	key, peers, ok := peerCallKey(registry, instanceID, callID)
	if !ok || len(key) != 32 || len(peers) != 1 || peers[0].String() != peer.String() {
		t.Fatalf("peer key was not preserved: ok=%v key=%d peers=%#v", ok, len(key), peers)
	}
	zeroBytes(key)

	registry.Remove(instanceID, callID)
	if _, _, ok = peerCallKey(registry, instanceID, callID); ok {
		t.Fatal("final Remove did not wipe the peer call key")
	}
}
