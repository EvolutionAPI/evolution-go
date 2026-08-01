// Package incoming keeps private material required to accept WhatsApp calls.
// Call keys and device metadata are intentionally separated from the public
// runtime snapshots and are never serialized.
package incoming

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/signaling"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/wa"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

const prepareTimeout = 30 * time.Second

type callSecret struct {
	callKey []byte
	peer    types.JID
	creator types.JID
	video   bool
}

type session struct {
	mu        sync.RWMutex
	client    *whatsmeow.Client
	handlerID uint32
	secrets   map[string]*callSecret
}

func newSession(client *whatsmeow.Client) *session {
	s := &session{
		client:  client,
		secrets: make(map[string]*callSecret),
	}
	if client != nil {
		s.handlerID = client.AddEventHandler(s.handleEvent)
	}
	return s
}

func (s *session) handleEvent(rawEvent interface{}) {
	switch event := rawEvent.(type) {
	case *events.CallOffer:
		// Decrypting the Signal payload and sending preaccept must not block the
		// main Evolution event dispatcher.
		go s.prepareOffer(event)
	case *events.CallReject:
		s.remove(event.CallID)
	case *events.CallTerminate:
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

	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()
	if client == nil {
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
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), prepareTimeout)
	defer cancel()

	socket := wa.NewSocket(client)
	callKey, err := signaling.DecryptCallKeyInNode(ctx, socket, event.Data, peer)
	if err != nil || len(callKey) != 32 {
		return
	}

	secret := &callSecret{
		callKey: append([]byte(nil), callKey...),
		peer:    peer,
		creator: creator,
		video:   signaling.NodeContainsVideo(event.Data),
	}
	s.store(event.CallID, secret)

	// WhatsApp expects preaccept before the user explicitly accepts the call.
	// A send failure does not expose or discard the key; the accept endpoint will
	// still return the concrete signaling error if the session is unhealthy.
	_ = socket.SendNode(ctx, signaling.BuildPreacceptStanza(peer, event.CallID, creator))
}

func (s *session) accept(ctx context.Context, callID string) error {
	secret, ok := s.copySecret(callID)
	if !ok {
		return fmt.Errorf("incoming call %s is not ready to accept", callID)
	}
	defer zeroBytes(secret.callKey)

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
		secret.callKey,
		secret.peer,
		secret.creator,
		secret.video,
	)
	if err != nil {
		return fmt.Errorf("build call accept: %w", err)
	}
	if err := socket.SendNode(ctx, node); err != nil {
		return fmt.Errorf("send call accept: %w", err)
	}
	return nil
}

func (s *session) store(callID string, secret *callSecret) {
	if callID == "" || secret == nil {
		return
	}
	s.mu.Lock()
	if previous := s.secrets[callID]; previous != nil {
		zeroBytes(previous.callKey)
	}
	s.secrets[callID] = secret
	s.mu.Unlock()
}

func (s *session) copySecret(callID string) (*callSecret, bool) {
	s.mu.RLock()
	secret, ok := s.secrets[callID]
	if !ok || secret == nil {
		s.mu.RUnlock()
		return nil, false
	}
	copyValue := &callSecret{
		callKey: append([]byte(nil), secret.callKey...),
		peer:    secret.peer,
		creator: secret.creator,
		video:   secret.video,
	}
	s.mu.RUnlock()
	return copyValue, true
}

func (s *session) remove(callID string) {
	s.mu.Lock()
	if secret := s.secrets[callID]; secret != nil {
		zeroBytes(secret.callKey)
	}
	delete(s.secrets, callID)
	s.mu.Unlock()
}

func (s *session) clear() {
	s.mu.Lock()
	for callID, secret := range s.secrets {
		if secret != nil {
			zeroBytes(secret.callKey)
		}
		delete(s.secrets, callID)
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

func zeroBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

// Registry stores one private incoming-call session per Evolution instance.
type Registry struct {
	mu       sync.RWMutex
	sessions map[string]*session
}

func NewRegistry() *Registry {
	return &Registry{sessions: make(map[string]*session)}
}

// Attach installs an isolated call handler on the same authenticated client used
// by messaging. Reattaching the same pointer is idempotent.
func (r *Registry) Attach(instanceID string, client *whatsmeow.Client) {
	if instanceID == "" || client == nil {
		return
	}

	r.mu.RLock()
	current := r.sessions[instanceID]
	if current != nil && current.client == client {
		r.mu.RUnlock()
		return
	}
	r.mu.RUnlock()

	candidate := newSession(client)

	r.mu.Lock()
	previous := r.sessions[instanceID]
	r.sessions[instanceID] = candidate
	r.mu.Unlock()

	if previous != nil {
		previous.close()
	}
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
