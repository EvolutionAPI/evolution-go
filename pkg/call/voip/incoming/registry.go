// Package incoming keeps private material required by WhatsApp call negotiation.
// Call keys, relay tokens and device metadata are intentionally separated from
// public runtime snapshots and are never serialized.
package incoming

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	call_state "github.com/evolution-foundation/evolution-go/pkg/call/voip/call"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/signaling"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/wa"
	"go.mau.fi/whatsmeow"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

const prepareTimeout = 30 * time.Second

type callMaterial struct {
	callKey   []byte
	peer      types.JID
	creator   types.JID
	video     bool
	relayData *core.RelayData
	state     *call_state.Info
}

type session struct {
	mu              sync.RWMutex
	client          *whatsmeow.Client
	handlerID       uint32
	prepareIncoming bool
	materials       map[string]*callMaterial
}

func newSession(client *whatsmeow.Client, prepareIncoming ...bool) *session {
	enabled := true
	if len(prepareIncoming) > 0 {
		enabled = prepareIncoming[0]
	}
	s := &session{
		client:          client,
		prepareIncoming: enabled,
		materials:       make(map[string]*callMaterial),
	}
	if client != nil {
		s.handlerID = client.AddEventHandler(s.handleEvent)
	}
	return s
}

func (s *session) usesClient(client *whatsmeow.Client) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client == client && client != nil
}

func (s *session) setPrepareIncoming(enabled bool) {
	s.mu.Lock()
	s.prepareIncoming = enabled
	s.mu.Unlock()
}

func (s *session) handleEvent(rawEvent interface{}) {
	switch event := rawEvent.(type) {
	case *events.CallOffer:
		s.mu.RLock()
		prepareIncoming := s.prepareIncoming
		s.mu.RUnlock()
		if prepareIncoming {
			go s.prepareOffer(event)
		}
	case *events.CallAccept:
		_ = s.transition(event.CallID, call_state.Transition{Type: call_state.TransitionRemoteAccepted})
		s.captureRelays(event.CallID, event.Data)
	case *events.CallTransport:
		s.captureRelays(event.CallID, event.Data)
	case *events.CallReject:
		_ = s.transition(event.CallID, call_state.Transition{
			Type:   call_state.TransitionRemoteRejected,
			Reason: core.EndCallReasonDeclined,
		})
		s.remove(event.CallID)
	case *events.CallTerminate:
		reason := core.EndCallReason(event.Reason)
		if reason == "" {
			reason = core.EndCallReasonUnknown
		}
		_ = s.transition(event.CallID, call_state.Transition{Type: call_state.TransitionTerminated, Reason: reason})
		s.remove(event.CallID)
	case *events.Disconnected:
		s.clear()
	case *events.LoggedOut:
		s.clear()
	}
}

func (s *session) prepareOffer(event *events.CallOffer) {
	if event == nil || event.CallID == "" || event.Data == nil {
		return
	}
	if signaling.IsAlreadyEndedOffer(event.Data) {
		slog.Info("ignoring already-ended incoming WhatsApp call offer", "call_id", event.CallID, "from", event.From.String())
		return
	}

	s.mu.RLock()
	client := s.client
	prepareIncoming := s.prepareIncoming
	s.mu.RUnlock()
	if client == nil || !prepareIncoming {
		return
	}

	peer := event.From
	creator := event.CallCreator
	if creator.IsEmpty() {
		creator = peer
	}
	if peer.IsEmpty() {
		peer = creator
	}
	if peer.IsEmpty() || creator.IsEmpty() {
		slog.Warn("cannot prepare incoming WhatsApp call without peer metadata", "call_id", event.CallID)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), prepareTimeout)
	defer cancel()

	socket := wa.NewSocket(client)
	callKey, err := signaling.DecryptCallKeyInNode(ctx, socket, event.Data, peer)
	if err != nil {
		slog.Warn("failed to decrypt incoming WhatsApp call key",
			"call_id", event.CallID,
			"peer", peer.String(),
			"creator", creator.String(),
			"err", err,
		)
		return
	}
	if len(callKey) != 32 {
		slog.Warn("incoming WhatsApp call key has invalid length",
			"call_id", event.CallID,
			"peer", peer.String(),
			"creator", creator.String(),
			"key_bytes", len(callKey),
		)
		return
	}
	if !s.usesClient(client) {
		zeroBytes(callKey)
		return
	}

	video := signaling.NodeContainsVideo(event.Data)
	mediaType := core.CallMediaTypeAudio
	if video {
		mediaType = core.CallMediaTypeVideo
	}
	material := &callMaterial{
		callKey:   append([]byte(nil), callKey...),
		peer:      peer,
		creator:   creator,
		video:     video,
		relayData: relayDataFromNode(event.Data),
		state:     call_state.NewIncoming(event.CallID, peer.String(), creator.String(), mediaType),
	}
	zeroBytes(callKey)
	s.store(event.CallID, material)

	if err := socket.SendNode(ctx, signaling.BuildPreacceptStanza(peer, event.CallID, creator)); err != nil {
		slog.Warn("failed to send WhatsApp call preaccept",
			"call_id", event.CallID,
			"peer", peer.String(),
			"creator", creator.String(),
			"err", err,
		)
	}
}

func (s *session) storeOutgoing(callID string, callKey []byte, peer, creator types.JID, video bool, relayData *core.RelayData) {
	if callID == "" || len(callKey) == 0 || peer.IsEmpty() || creator.IsEmpty() {
		return
	}
	mediaType := core.CallMediaTypeAudio
	if video {
		mediaType = core.CallMediaTypeVideo
	}
	state := call_state.NewOutgoing(callID, peer.String(), creator.String(), mediaType)
	_ = state.Apply(call_state.Transition{Type: call_state.TransitionOfferSent})
	s.store(callID, &callMaterial{
		callKey:   append([]byte(nil), callKey...),
		peer:      peer,
		creator:   creator,
		video:     video,
		relayData: core.CloneRelayData(relayData),
		state:     state,
	})
}

func (s *session) captureRelays(callID string, node *waBinary.Node) {
	if callID == "" || node == nil {
		return
	}
	relayData := relayDataFromNode(node)
	if relayData == nil {
		return
	}

	s.mu.Lock()
	material := s.materials[callID]
	if material == nil {
		material = &callMaterial{}
		s.materials[callID] = material
	}
	core.ZeroRelayData(material.relayData)
	material.relayData = relayData
	s.mu.Unlock()
}

func (s *session) accept(ctx context.Context, callID string) error {
	material, ok := s.copyMaterial(callID)
	if !ok || len(material.callKey) == 0 {
		return fmt.Errorf("incoming call %s is not ready to accept", callID)
	}
	defer zeroMaterial(material)
	if material.state == nil || !material.state.CanAccept() {
		return fmt.Errorf("incoming call %s cannot be accepted in its current state", callID)
	}

	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()
	if client == nil {
		return fmt.Errorf("incoming call session is detached")
	}

	socket := wa.NewSocket(client)
	node, err := signaling.BuildAcceptStanza(
		ctx,
		socket,
		callID,
		material.callKey,
		material.peer,
		material.creator,
		material.video,
	)
	if err != nil {
		return fmt.Errorf("build call accept: %w", err)
	}
	if err := socket.SendNode(ctx, node); err != nil {
		return fmt.Errorf("send call accept: %w", err)
	}
	return s.transition(callID, call_state.Transition{Type: call_state.TransitionLocalAccepted})
}

func (s *session) terminate(ctx context.Context, callID string) error {
	material, ok := s.copyMaterial(callID)
	if !ok || material.peer.IsEmpty() || material.creator.IsEmpty() {
		return fmt.Errorf("call %s has no private signaling material", callID)
	}
	defer zeroMaterial(material)

	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()
	if client == nil {
		return fmt.Errorf("call session is detached")
	}

	node := signaling.BuildTerminateStanza(material.peer, callID, material.creator)
	if err := wa.NewSocket(client).SendNode(ctx, node); err != nil {
		return fmt.Errorf("send call terminate: %w", err)
	}
	_ = s.transition(callID, call_state.Transition{
		Type:   call_state.TransitionTerminated,
		Reason: core.EndCallReasonUserEnded,
	})
	s.remove(callID)
	return nil
}

func (s *session) transition(callID string, transition call_state.Transition) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	material := s.materials[callID]
	if material == nil || material.state == nil {
		return fmt.Errorf("call %s has no private state", callID)
	}
	return material.state.Apply(transition)
}

func (s *session) store(callID string, material *callMaterial) {
	if callID == "" || material == nil {
		return
	}
	s.mu.Lock()
	if previous := s.materials[callID]; previous != nil {
		if material.relayData == nil {
			material.relayData = core.CloneRelayData(previous.relayData)
		}
		if material.state == nil {
			material.state = previous.state.Clone()
		}
		zeroMaterial(previous)
	}
	s.materials[callID] = material
	s.mu.Unlock()
}

func (s *session) copyMaterial(callID string) (*callMaterial, bool) {
	s.mu.RLock()
	material, ok := s.materials[callID]
	if !ok || material == nil {
		s.mu.RUnlock()
		return nil, false
	}
	copyValue := &callMaterial{
		callKey:   append([]byte(nil), material.callKey...),
		peer:      material.peer,
		creator:   material.creator,
		video:     material.video,
		relayData: core.CloneRelayData(material.relayData),
		state:     material.state.Clone(),
	}
	s.mu.RUnlock()
	return copyValue, true
}

func (s *session) relayData(callID string) (*core.RelayData, bool) {
	s.mu.RLock()
	material := s.materials[callID]
	if material == nil || material.relayData == nil {
		s.mu.RUnlock()
		return nil, false
	}
	data := core.CloneRelayData(material.relayData)
	s.mu.RUnlock()
	return data, true
}

func (s *session) state(callID string) (*call_state.Info, bool) {
	s.mu.RLock()
	material := s.materials[callID]
	if material == nil || material.state == nil {
		s.mu.RUnlock()
		return nil, false
	}
	state := material.state.Clone()
	s.mu.RUnlock()
	return state, true
}

func (s *session) remove(callID string) {
	s.mu.Lock()
	if material := s.materials[callID]; material != nil {
		zeroMaterial(material)
	}
	delete(s.materials, callID)
	s.mu.Unlock()
}

func (s *session) clear() {
	s.mu.Lock()
	for callID, material := range s.materials {
		if material != nil {
			zeroMaterial(material)
		}
		delete(s.materials, callID)
	}
	s.mu.Unlock()
}

func (s *session) close() {
	s.mu.Lock()
	client := s.client
	handlerID := s.handlerID
	s.client = nil
	s.handlerID = 0
	s.mu.Unlock()

	if client != nil && handlerID != 0 {
		client.RemoveEventHandler(handlerID)
	}
	s.clear()
}

func relayDataFromNode(node *waBinary.Node) *core.RelayData {
	if node == nil {
		return nil
	}

	endpoints := signaling.ExtractRelayEndpoints(node)
	parsed := signaling.ParseRelayFromAck(node)
	if len(endpoints) == 0 {
		endpoints = parsed.Relays
	}
	if len(endpoints) == 0 && parsed.UUID == "" && len(parsed.HBHKey) == 0 &&
		len(parsed.ParticipantJIDs) == 0 && parsed.SelfPID == nil && parsed.PeerPID == nil {
		return nil
	}
	return &core.RelayData{
		Endpoints:       endpoints,
		ParticipantJIDs: parsed.ParticipantJIDs,
		UUID:            parsed.UUID,
		SelfPID:         parsed.SelfPID,
		PeerPID:         parsed.PeerPID,
		HBHKey:          parsed.HBHKey,
	}
}

func zeroMaterial(material *callMaterial) {
	if material == nil {
		return
	}
	zeroBytes(material.callKey)
	core.ZeroRelayData(material.relayData)
	material.callKey = nil
	material.peer = types.JID{}
	material.creator = types.JID{}
	material.relayData = nil
	material.state = nil
}

func zeroBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

// Registry stores one private call-negotiation session per Evolution instance.
type Registry struct {
	mu       sync.RWMutex
	sessions map[string]*session
}

func NewRegistry() *Registry {
	return &Registry{sessions: make(map[string]*session)}
}

func (r *Registry) Attach(instanceID string, client *whatsmeow.Client, prepareIncoming ...bool) {
	if instanceID == "" || client == nil {
		return
	}
	enabled := true
	if len(prepareIncoming) > 0 {
		enabled = prepareIncoming[0]
	}

	r.mu.RLock()
	current := r.sessions[instanceID]
	if current != nil && current.usesClient(client) {
		current.setPrepareIncoming(enabled)
		r.mu.RUnlock()
		return
	}
	r.mu.RUnlock()

	candidate := newSession(client, enabled)

	r.mu.Lock()
	previous := r.sessions[instanceID]
	r.sessions[instanceID] = candidate
	r.mu.Unlock()

	if previous != nil {
		previous.close()
	}
}

func (r *Registry) StoreOutgoing(instanceID, callID string, callKey []byte, peer, creator types.JID, video bool, relayData *core.RelayData) error {
	r.mu.RLock()
	s := r.sessions[instanceID]
	r.mu.RUnlock()
	if s == nil {
		return fmt.Errorf("call runtime is not attached for instance %s", instanceID)
	}
	s.storeOutgoing(callID, callKey, peer, creator, video, relayData)
	return nil
}

func (r *Registry) RelayData(instanceID, callID string) (*core.RelayData, bool) {
	r.mu.RLock()
	s := r.sessions[instanceID]
	r.mu.RUnlock()
	if s == nil {
		return nil, false
	}
	return s.relayData(callID)
}

func (r *Registry) State(instanceID, callID string) (*call_state.Info, bool) {
	r.mu.RLock()
	s := r.sessions[instanceID]
	r.mu.RUnlock()
	if s == nil {
		return nil, false
	}
	return s.state(callID)
}

func (r *Registry) Accept(ctx context.Context, instanceID, callID string) error {
	r.mu.RLock()
	s := r.sessions[instanceID]
	r.mu.RUnlock()
	if s == nil {
		return fmt.Errorf("incoming call runtime is not attached for instance %s", instanceID)
	}
	return s.accept(ctx, callID)
}

func (r *Registry) Terminate(ctx context.Context, instanceID, callID string) error {
	r.mu.RLock()
	s := r.sessions[instanceID]
	r.mu.RUnlock()
	if s == nil {
		return fmt.Errorf("call runtime is not attached for instance %s", instanceID)
	}
	return s.terminate(ctx, callID)
}

func (r *Registry) Remove(instanceID, callID string) {
	r.mu.RLock()
	s := r.sessions[instanceID]
	r.mu.RUnlock()
	if s != nil {
		s.remove(callID)
	}
}

func (r *Registry) Close(instanceID string) {
	r.mu.Lock()
	s := r.sessions[instanceID]
	delete(r.sessions, instanceID)
	r.mu.Unlock()
	if s != nil {
		s.close()
	}
}
