package media

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/signaling"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/wa"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

const peerCallKeyDecryptTimeout = 5 * time.Second

type storedPeerCallKey struct {
	key  []byte
	peer types.JID
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
	if registry == nil || client == nil || event == nil || event.CallID == "" || event.Data == nil || event.From.IsEmpty() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), peerCallKeyDecryptTimeout)
	defer cancel()
	key, err := signaling.DecryptCallKeyInNode(ctx, wa.NewSocket(client), event.Data, event.From)
	if err != nil || len(key) != 32 {
		return
	}
	defer zeroBytes(key)

	peerCallKeyObservers.Lock()
	observer := peerCallKeyObservers.registries[registry][instanceID]
	if observer == nil || observer.client != client {
		peerCallKeyObservers.Unlock()
		return
	}
	if previous, exists := observer.keys[event.CallID]; exists {
		zeroBytes(previous.key)
	}
	observer.keys[event.CallID] = storedPeerCallKey{
		key:  append([]byte(nil), key...),
		peer: event.From,
	}
	peerCallKeyObservers.Unlock()

	// A relay may have pre-connected on CallPreAccept. Rebuild only the packet
	// session, preserving the peer key that was just stored.
	removePacketSessionOnly(registry, instanceID, event.CallID)
	if err = registry.Prepare(instanceID, event.CallID); err != nil {
		slog.Debug("defer peer call-key SRTP refresh", "instance", instanceID, "call_id", event.CallID, "err", err)
		return
	}
	slog.Info("WhatsApp peer call key applied", "instance", instanceID, "call_id", event.CallID, "peer", event.From.String())
}

func removePacketSessionOnly(registry *PacketRegistry, instanceID, callID string) {
	if registry == nil || instanceID == "" || callID == "" {
		return
	}
	registry.mu.Lock()
	calls := registry.sessions[instanceID]
	session := calls[callID]
	delete(calls, callID)
	if len(calls) == 0 {
		delete(registry.sessions, instanceID)
	}
	registry.mu.Unlock()
	if session != nil {
		session.close()
	}
}

func peerCallKey(registry *PacketRegistry, instanceID, callID string) ([]byte, types.JID, bool) {
	peerCallKeyObservers.Lock()
	defer peerCallKeyObservers.Unlock()
	observer := peerCallKeyObservers.registries[registry][instanceID]
	if observer == nil {
		return nil, types.JID{}, false
	}
	stored, ok := observer.keys[callID]
	if !ok || len(stored.key) != 32 || stored.peer.IsEmpty() {
		return nil, types.JID{}, false
	}
	return append([]byte(nil), stored.key...), stored.peer, true
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

	peerKey, acceptedPeer, hasPeerKey := peerCallKey(registry, instanceID, callID)
	defer zeroBytes(peerKey)
	peerKeyJIDs := []string(nil)
	if hasPeerKey {
		peerKeyJIDs = uniqueJIDStrings(acceptedPeer.String(), ensureDeviceJIDString(acceptedPeer.String()))
		peerKeyJIDs = uniqueJIDStrings(append(peerKeyJIDs, receiveJIDs...)...)
	}

	keyings := make([]packetSRTPCandidateKeying, 0, len(peerKeyJIDs)+len(receiveJIDs))
	seenMaterial := make(map[string]struct{})
	appendCandidate := func(receiveJID string, send, receive core.SRTPKeyingMaterial) {
		identity := fmt.Sprintf("%x:%x", receive.MasterKey, receive.MasterSalt)
		if _, exists := seenMaterial[identity]; exists {
			send.Wipe()
			receive.Wipe()
			return
		}
		seenMaterial[identity] = struct{}{}
		keyings = append(keyings, packetSRTPCandidateKeying{receiveJID: receiveJID, send: send, receive: receive})
	}

	if hasPeerKey {
		for _, receiveJID := range peerKeyJIDs {
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

func uniqueJIDStrings(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	output := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		output = append(output, value)
	}
	return output
}

func wipePacketCandidateKeyings(keyings []packetSRTPCandidateKeying) {
	for index := range keyings {
		keyings[index].send.Wipe()
		keyings[index].receive.Wipe()
	}
}
