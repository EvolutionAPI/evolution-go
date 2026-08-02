package media

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	call_state "github.com/evolution-foundation/evolution-go/pkg/call/voip/call"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	call_transport "github.com/evolution-foundation/evolution-go/pkg/call/voip/transport"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/wa"
	"go.mau.fi/whatsmeow"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// NegotiationSource exposes private call state without exposing keys or relay
// tokens to the public runtime or HTTP layer.
type NegotiationSource interface {
	RelayData(instanceID, callID string) (*core.RelayData, bool)
	State(instanceID, callID string) (*call_state.Info, bool)
	CaptureRelayNode(instanceID, callID string, node *waBinary.Node)
	EnsureRemoteAccepted(instanceID, callID string) error
	MarkMediaConnected(instanceID, callID string) error
}

type RelayFactory func(log *slog.Logger) call_transport.RelayTransport

type relaySession struct {
	mu          sync.Mutex
	instanceID  string
	client      *whatsmeow.Client
	handlerID   uint32
	source      NegotiationSource
	factory     RelayFactory
	log         *slog.Logger
	transports  map[string]call_transport.RelayTransport
	configuring map[string]bool
	ownJID      func() types.JID
	onConnected func(instanceID, callID string)
	onPacket    func(instanceID, callID string, packet []byte)
	onRemoved   func(instanceID, callID string)
	onCleanup   func(instanceID string)
}

func newRelaySession(instanceID string, client *whatsmeow.Client, source NegotiationSource, factory RelayFactory, log *slog.Logger) *relaySession {
	if log == nil {
		log = slog.Default()
	}
	if factory == nil {
		factory = call_transport.NewRelayTransport
	}
	session := &relaySession{
		instanceID:  instanceID,
		client:      client,
		source:      source,
		factory:     factory,
		log:         log,
		transports:  make(map[string]call_transport.RelayTransport),
		configuring: make(map[string]bool),
	}
	session.ownJID = func() types.JID {
		if client == nil {
			return types.JID{}
		}
		socket := wa.NewSocket(client)
		jid := socket.OwnLID()
		if jid.IsEmpty() {
			jid = socket.OwnPN()
		}
		return jid
	}
	if client != nil {
		session.handlerID = client.AddEventHandler(session.handleEvent)
	}
	return session
}

func (s *relaySession) usesClient(client *whatsmeow.Client) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return client != nil && s.client == client
}

func (s *relaySession) handleEvent(rawEvent interface{}) {
	switch event := rawEvent.(type) {
	case *events.CallAccept:
		_ = s.source.EnsureRemoteAccepted(s.instanceID, event.CallID)
		s.source.CaptureRelayNode(s.instanceID, event.CallID, event.Data)
		go s.startLogged(event.CallID)
	case *events.CallTransport:
		s.source.CaptureRelayNode(s.instanceID, event.CallID, event.Data)
		go s.startLogged(event.CallID)
	case *events.CallReject:
		s.remove(event.CallID)
	case *events.CallTerminate:
		s.remove(event.CallID)
	case *events.Disconnected:
		s.cleanup()
	case *events.LoggedOut:
		s.cleanup()
	}
}

func (s *relaySession) startLogged(callID string) {
	if err := s.start(callID); err != nil {
		s.log.Warn("WhatsApp relay setup failed", "instance", s.instanceID, "call_id", callID, "err", err)
	}
}

func (s *relaySession) start(callID string) error {
	if callID == "" || s.source == nil {
		return nil
	}
	state, ok := s.source.State(s.instanceID, callID)
	if !ok || state == nil || state.StateData.State != core.CallStateConnecting {
		return nil
	}
	relayData, ok := s.source.RelayData(s.instanceID, callID)
	if !ok || relayData == nil {
		return nil
	}
	defer core.ZeroRelayData(relayData)
	configs := call_transport.BuildRelayConfigs(relayData.Endpoints)
	if len(configs) == 0 {
		return nil
	}
	defer call_transport.ZeroRelayConfigs(configs)

	s.mu.Lock()
	if s.configuring[callID] {
		s.mu.Unlock()
		return nil
	}
	s.configuring[callID] = true
	relay := s.transports[callID]
	if relay == nil {
		relay = s.factory(s.log)
		s.transports[callID] = relay
	}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.configuring, callID)
		s.mu.Unlock()
	}()

	ownJID := types.JID{}
	if s.ownJID != nil {
		ownJID = s.ownJID()
	}
	peerJID, err := types.ParseJID(state.PeerJID)
	if err != nil || peerJID.IsEmpty() || ownJID.IsEmpty() {
		return fmt.Errorf("resolve SSRC participants for call %s", callID)
	}
	selfDevice, peerDevice := selectDeviceJIDs(relayData.ParticipantJIDs, ownJID, peerJID)
	selfSSRC, err := GenerateSecureSSRC(callID, selfDevice, 0)
	if err != nil {
		return err
	}
	peerSSRC, err := GenerateSecureSSRC(callID, peerDevice, 0)
	if err != nil {
		return err
	}

	relay.SetSSRC(selfSSRC)
	relay.SetSubscriptionSSRC(peerSSRC)
	relay.SetOnConnected(func(_ string, _ int) {
		if err := s.source.MarkMediaConnected(s.instanceID, callID); err != nil {
			s.log.Debug("ignore stale relay connection callback", "instance", s.instanceID, "call_id", callID, "err", err)
			return
		}
		s.mu.Lock()
		callback := s.onConnected
		s.mu.Unlock()
		if callback != nil {
			callback(s.instanceID, callID)
		}
	})
	relay.SetOnReceive(func(packet []byte) {
		s.mu.Lock()
		callback := s.onPacket
		s.mu.Unlock()
		if callback != nil {
			callback(s.instanceID, callID, append([]byte(nil), packet...))
		}
	})

	if err := relay.ConfigureRelays(configs); err != nil {
		if errors.Is(err, call_transport.ErrSCTPUnavailable) {
			s.remove(callID)
			return nil
		}
		return fmt.Errorf("configure relays for call %s: %w", callID, err)
	}
	return nil
}

func selectDeviceJIDs(participants []string, ownJID, peerJID types.JID) (string, string) {
	selfDevice := ownJID.String()
	peerDevice := peerJID.String()
	for _, participant := range participants {
		jid, err := types.ParseJID(participant)
		if err != nil || jid.IsEmpty() {
			continue
		}
		if jid.User == ownJID.User {
			selfDevice = jid.String()
		}
		if jid.User == peerJID.User {
			peerDevice = jid.String()
		}
	}
	return selfDevice, peerDevice
}

func (s *relaySession) remove(callID string) {
	s.mu.Lock()
	relay := s.transports[callID]
	delete(s.transports, callID)
	delete(s.configuring, callID)
	callback := s.onRemoved
	s.mu.Unlock()
	if relay != nil {
		relay.Cleanup()
	}
	if callback != nil && callID != "" {
		callback(s.instanceID, callID)
	}
}

func (s *relaySession) cleanup() {
	s.mu.Lock()
	transports := make([]call_transport.RelayTransport, 0, len(s.transports))
	for callID, relay := range s.transports {
		transports = append(transports, relay)
		delete(s.transports, callID)
	}
	s.configuring = make(map[string]bool)
	callback := s.onCleanup
	s.mu.Unlock()
	for _, relay := range transports {
		relay.Cleanup()
	}
	if callback != nil {
		callback(s.instanceID)
	}
}

func (s *relaySession) close() {
	s.mu.Lock()
	client := s.client
	handlerID := s.handlerID
	s.client = nil
	s.handlerID = 0
	s.mu.Unlock()
	if client != nil && handlerID != 0 {
		client.RemoveEventHandler(handlerID)
	}
	s.cleanup()
}

// RelayRegistry owns one relay-event session per Evolution instance.
type RelayRegistry struct {
	mu          sync.RWMutex
	source      NegotiationSource
	factory     RelayFactory
	log         *slog.Logger
	sessions    map[string]*relaySession
	onConnected func(instanceID, callID string)
	onPacket    func(instanceID, callID string, packet []byte)
	onRemoved   func(instanceID, callID string)
	onCleanup   func(instanceID string)
}

func NewRelayRegistry(source NegotiationSource, factory RelayFactory, log *slog.Logger) *RelayRegistry {
	if log == nil {
		log = slog.Default()
	}
	return &RelayRegistry{
		source:   source,
		factory:  factory,
		log:      log,
		sessions: make(map[string]*relaySession),
	}
}

func (r *RelayRegistry) SetOnConnected(callback func(instanceID, callID string)) {
	r.mu.Lock()
	r.onConnected = callback
	for _, session := range r.sessions {
		session.onConnected = callback
	}
	r.mu.Unlock()
}

func (r *RelayRegistry) SetOnPacket(callback func(instanceID, callID string, packet []byte)) {
	r.mu.Lock()
	r.onPacket = callback
	for _, session := range r.sessions {
		session.onPacket = callback
	}
	r.mu.Unlock()
}

func (r *RelayRegistry) SetOnRemoved(callback func(instanceID, callID string)) {
	r.mu.Lock()
	r.onRemoved = callback
	for _, session := range r.sessions {
		session.onRemoved = callback
	}
	r.mu.Unlock()
}

func (r *RelayRegistry) SetOnCleanup(callback func(instanceID string)) {
	r.mu.Lock()
	r.onCleanup = callback
	for _, session := range r.sessions {
		session.onCleanup = callback
	}
	r.mu.Unlock()
}

func (r *RelayRegistry) Attach(instanceID string, client *whatsmeow.Client) {
	if instanceID == "" || client == nil {
		return
	}
	r.mu.RLock()
	current := r.sessions[instanceID]
	if current != nil && current.usesClient(client) {
		r.mu.RUnlock()
		return
	}
	r.mu.RUnlock()

	candidate := newRelaySession(instanceID, client, r.source, r.factory, r.log)
	r.mu.Lock()
	candidate.onConnected = r.onConnected
	candidate.onPacket = r.onPacket
	candidate.onRemoved = r.onRemoved
	candidate.onCleanup = r.onCleanup
	previous := r.sessions[instanceID]
	r.sessions[instanceID] = candidate
	r.mu.Unlock()
	if previous != nil {
		previous.close()
	}
}

func (r *RelayRegistry) Start(instanceID, callID string) error {
	r.mu.RLock()
	session := r.sessions[instanceID]
	r.mu.RUnlock()
	if session == nil {
		return fmt.Errorf("relay runtime is not attached for instance %s", instanceID)
	}
	err := session.start(callID)
	if err != nil {
		r.log.Warn("WhatsApp relay setup failed", "instance", instanceID, "call_id", callID, "err", err)
	}
	return err
}

func (r *RelayRegistry) Remove(instanceID, callID string) {
	r.mu.RLock()
	session := r.sessions[instanceID]
	r.mu.RUnlock()
	if session != nil {
		session.remove(callID)
	}
}

func (r *RelayRegistry) Close(instanceID string) {
	r.mu.Lock()
	session := r.sessions[instanceID]
	delete(r.sessions, instanceID)
	r.mu.Unlock()
	if session != nil {
		session.close()
	}
}
