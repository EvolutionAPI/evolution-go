package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/signaling"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/wa"
	"go.mau.fi/whatsmeow"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

const peerCallKeyDecryptTimeout = 5 * time.Second

type storedPeerCallKey struct {
	key   []byte
	peers []types.JID
}

type peerCallKeyObserver struct {
	client    *whatsmeow.Client
	handlerID uint32
	keys      map[string]storedPeerCallKey
}

var peerCallKeyObservers = struct {
	sync.Mutex
	registries map[*PacketRegistry]map[string]*peerCallKeyObserver
}{registries: make(map[*PacketRegistry]map[string]*peerCallKeyObserver)}

func attachPeerCallKeyObserver(registry *PacketRegistry, instanceID string, client *whatsmeow.Client) {
	if registry == nil || instanceID == "" || client == nil {
		return
	}

	peerCallKeyObservers.Lock()
	instances := peerCallKeyObservers.registries[registry]
	if instances == nil {
		instances = make(map[string]*peerCallKeyObserver)
		peerCallKeyObservers.registries[registry] = instances
	}
	current := instances[instanceID]
	if current != nil && current.client == client {
		peerCallKeyObservers.Unlock()
		return
	}
	peerCallKeyObservers.Unlock()
	if current != nil {
		detachPeerCallKeyObserver(registry, instanceID)
	}

	observer := &peerCallKeyObserver{client: client, keys: make(map[string]storedPeerCallKey)}
	observer.handlerID = client.AddEventHandler(func(rawEvent interface{}) {
		switch event := rawEvent.(type) {
		case *events.CallAccept:
			capturePeerCallKey(registry, instanceID, client, event)
		case *events.CallReject:
			removePeerCallKey(registry, instanceID, event.CallID)
		case *events.CallTerminate:
			removePeerCallKey(registry, instanceID, event.CallID)
		case *events.Disconnected:
			clearPeerCallKeys(registry, instanceID)
		case *events.LoggedOut:
			clearPeerCallKeys(registry, instanceID)
		}
	})

	peerCallKeyObservers.Lock()
	instances = peerCallKeyObservers.registries[registry]
	if instances == nil {
		instances = make(map[string]*peerCallKeyObserver)
		peerCallKeyObservers.registries[registry] = instances
	}
	if previous := instances[instanceID]; previous != nil && previous.client != client {
		peerCallKeyObservers.Unlock()
		client.RemoveEventHandler(observer.handlerID)
		wipePeerObserver(observer)
		return
	}
	instances[instanceID] = observer
	peerCallKeyObservers.Unlock()
}

func capturePeerCallKey(registry *PacketRegistry, instanceID string, client *whatsmeow.Client, event *events.CallAccept) {
	if registry == nil || client == nil || event == nil || event.CallID == "" || event.Data == nil {
		return
	}
	peerCandidates := callKeyPeerCandidates(event)
	if len(peerCandidates) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), peerCallKeyDecryptTimeout)
	defer cancel()
	key, decryptedPeer, err := decryptPeerCallKey(ctx, wa.NewSocket(client), event.Data, peerCandidates)
	if err != nil || len(key) != 32 {
		slog.Debug("WhatsApp peer call key not available",
			"instance", instanceID,
			"call_id", event.CallID,
			"candidates", len(peerCandidates),
			"err", err,
		)
		return
	}
	defer zeroBytes(key)
	peerCandidates = uniqueCallKeyPeers(append([]types.JID{decryptedPeer}, peerCandidates...)...)

	peerCallKeyObservers.Lock()
	observer := peerCallKeyObservers.registries[registry][instanceID]
	if observer == nil || observer.client != client {
		peerCallKeyObservers.Unlock()
		return
	}
	if previous, exists := observer.keys[event.CallID]; exists {
		if equalPeerCallKeys(previous.key, key) && equalCallKeyPeers(previous.peers, peerCandidates) {
			peerCallKeyObservers.Unlock()
			return
		}
		zeroBytes(previous.key)
	}
	observer.keys[event.CallID] = storedPeerCallKey{
		key:   append([]byte(nil), key...),
		peers: append([]types.JID(nil), peerCandidates...),
	}
	peerCallKeyObservers.Unlock()

	// A relay may have pre-connected on CallPreAccept. Rebuild any early packet
	// session now so the first post-accept RTP frame can use the peer-provided key.
	registry.dropSession(instanceID, event.CallID)
	if err = registry.Prepare(instanceID, event.CallID); err != nil {
		slog.Debug("defer peer call-key SRTP refresh", "instance", instanceID, "call_id", event.CallID, "err", err)
		return
	}
	slog.Info("WhatsApp peer call key applied",
		"instance", instanceID,
		"call_id", event.CallID,
		"peer", decryptedPeer.String(),
		"candidates", len(peerCandidates),
	)
}

func callKeyPeerCandidates(event *events.CallAccept) []types.JID {
	if event == nil {
		return nil
	}
	return uniqueCallKeyPeers(event.From, event.CallCreator, event.CallCreatorAlt)
}

func uniqueCallKeyPeers(values ...types.JID) []types.JID {
	seen := make(map[string]struct{}, len(values))
	output := make([]types.JID, 0, len(values))
	for _, value := range values {
		if value.IsEmpty() {
			continue
		}
		identity := value.String()
		if _, exists := seen[identity]; exists {
			continue
		}
		seen[identity] = struct{}{}
		output = append(output, value)
	}
	return output
}

func decryptPeerCallKey(ctx context.Context, socket core.VoipSocket, node *waBinary.Node, peers []types.JID) ([]byte, types.JID, error) {
	if socket == nil || node == nil || len(peers) == 0 {
		return nil, types.JID{}, fmt.Errorf("peer call-key inputs are incomplete")
	}
	attemptErrors := make([]error, 0, len(peers))
	for _, peer := range peers {
		key, err := signaling.DecryptCallKeyInNode(ctx, socket, node, peer)
		if err == nil {
			return key, peer, nil
		}
		attemptErrors = append(attemptErrors, fmt.Errorf("peer=%s: %w", peer.String(), err))
	}
	return nil, types.JID{}, fmt.Errorf("decrypt peer call key with %d candidates: %w", len(peers), errors.Join(attemptErrors...))
}

func peerCallKey(registry *PacketRegistry, instanceID, callID string) ([]byte, []types.JID, bool) {
	peerCallKeyObservers.Lock()
	defer peerCallKeyObservers.Unlock()
	observer := peerCallKeyObservers.registries[registry][instanceID]
	if observer == nil {
		return nil, nil, false
	}
	stored, ok := observer.keys[callID]
	if !ok || len(stored.key) != 32 || len(stored.peers) == 0 {
		return nil, nil, false
	}
	return append([]byte(nil), stored.key...), append([]types.JID(nil), stored.peers...), true
}

func removePeerCallKey(registry *PacketRegistry, instanceID, callID string) {
	if registry == nil || instanceID == "" || callID == "" {
		return
	}
	peerCallKeyObservers.Lock()
	observer := peerCallKeyObservers.registries[registry][instanceID]
	if observer != nil {
		if stored, ok := observer.keys[callID]; ok {
			zeroBytes(stored.key)
			delete(observer.keys, callID)
		}
	}
	peerCallKeyObservers.Unlock()
}

func clearPeerCallKeys(registry *PacketRegistry, instanceID string) {
	if registry == nil || instanceID == "" {
		return
	}
	peerCallKeyObservers.Lock()
	observer := peerCallKeyObservers.registries[registry][instanceID]
	if observer != nil {
		for callID, stored := range observer.keys {
			zeroBytes(stored.key)
			delete(observer.keys, callID)
		}
	}
	peerCallKeyObservers.Unlock()
}

func detachPeerCallKeyObserver(registry *PacketRegistry, instanceID string) {
	if registry == nil || instanceID == "" {
		return
	}
	peerCallKeyObservers.Lock()
	instances := peerCallKeyObservers.registries[registry]
	observer := instances[instanceID]
	delete(instances, instanceID)
	if len(instances) == 0 {
		delete(peerCallKeyObservers.registries, registry)
	}
	peerCallKeyObservers.Unlock()
	if observer != nil {
		if observer.client != nil && observer.handlerID != 0 {
			observer.client.RemoveEventHandler(observer.handlerID)
		}
		wipePeerObserver(observer)
	}
}

func wipePeerObserver(observer *peerCallKeyObserver) {
	if observer == nil {
		return
	}
	for callID, stored := range observer.keys {
		zeroBytes(stored.key)
		delete(observer.keys, callID)
	}
	observer.client = nil
	observer.handlerID = 0
}

func buildPacketSRTPCandidates(
	registry *PacketRegistry,
	instanceID, callID, selfDeviceJID string,
	receiveJIDs []string,
) ([]packetSRTPCandidateKeying, error) {
	if registry == nil || registry.source == nil {
		return nil, ErrPacketSessionNotReady
	}

	peerKey, acceptedPeers, hasPeerKey := peerCallKey(registry, instanceID, callID)
	defer zeroBytes(peerKey)
	candidateJIDs := append([]string(nil), receiveJIDs...)
	if hasPeerKey {
		peerJIDs := make([]string, 0, len(acceptedPeers)*2+len(receiveJIDs))
		for _, peer := range acceptedPeers {
			peerJIDs = append(peerJIDs, peer.String(), ensureDeviceJIDString(peer.String()))
		}
		peerJIDs = append(peerJIDs, candidateJIDs...)
		candidateJIDs = uniqueDeviceJIDs(peerJIDs...)
	}

	keyings := make([]packetSRTPCandidateKeying, 0, len(candidateJIDs)+len(receiveJIDs))
	seenMaterial := make(map[[sha256.Size]byte]struct{})
	appendCandidate := func(receiveJID string, send, receive core.SRTPKeyingMaterial) {
		identity := packetKeyingFingerprint(receive)
		if _, exists := seenMaterial[identity]; exists {
			send.Wipe()
			receive.Wipe()
			return
		}
		seenMaterial[identity] = struct{}{}
		keyings = append(keyings, packetSRTPCandidateKeying{receiveJID: receiveJID, send: send, receive: receive})
	}

	if hasPeerKey {
		for _, receiveJID := range candidateJIDs {
			send, originalReceive, err := registry.source.SRTPKeying(instanceID, callID, selfDeviceJID, receiveJID)
			if err != nil {
				wipePacketCandidateKeyings(keyings)
				return nil, fmt.Errorf("derive send keying for peer call key: %w", err)
			}
			originalReceive.Wipe()
			peerReceive, err := DerivePerJIDSRTPKey(peerKey, receiveJID)
			if err != nil {
				send.Wipe()
				wipePacketCandidateKeyings(keyings)
				return nil, fmt.Errorf("derive peer receive keying for %s: %w", receiveJID, err)
			}
			appendCandidate(receiveJID+" (peer-key)", send, peerReceive)
		}
	}

	for _, receiveJID := range receiveJIDs {
		send, receive, err := registry.source.SRTPKeying(instanceID, callID, selfDeviceJID, receiveJID)
		if err != nil {
			wipePacketCandidateKeyings(keyings)
			return nil, fmt.Errorf("derive SRTP candidate %s: %w", receiveJID, err)
		}
		appendCandidate(receiveJID, send, receive)
	}
	if len(keyings) == 0 {
		return nil, fmt.Errorf("call %s produced no unique SRTP key candidates", callID)
	}
	return keyings, nil
}

func packetKeyingFingerprint(keying core.SRTPKeyingMaterial) [sha256.Size]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte{byte(len(keying.MasterKey))})
	_, _ = hash.Write(keying.MasterKey)
	_, _ = hash.Write([]byte{byte(len(keying.MasterSalt))})
	_, _ = hash.Write(keying.MasterSalt)
	var output [sha256.Size]byte
	copy(output[:], hash.Sum(nil))
	return output
}

func wipePacketCandidateKeyings(keyings []packetSRTPCandidateKeying) {
	for index := range keyings {
		keyings[index].send.Wipe()
		keyings[index].receive.Wipe()
	}
}

func equalPeerCallKeys(left, right []byte) bool {
	return bytes.Equal(left, right)
}

func equalCallKeyPeers(left, right []types.JID) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].String() != right[index].String() {
			return false
		}
	}
	return true
}
