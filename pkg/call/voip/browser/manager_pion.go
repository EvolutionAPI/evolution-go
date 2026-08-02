//go:build voip_pion

package browser

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

const (
	maxSessionsPerCall = 4
	maxOfferBytes       = 256 * 1024
	maxBufferedAmount   = 512 * 1024
	mediaQueueDepth     = 8
)

type pionManager struct {
	mu       sync.RWMutex
	feeder   PCMFeeder
	sessions map[string]map[string]map[string]*pionSession
}

type pionSession struct {
	manager    *pionManager
	instanceID string
	callID     string
	id         string
	createdAt  time.Time
	pc         *webrtc.PeerConnection

	mu            sync.RWMutex
	channel       *webrtc.DataChannel
	state         SessionState
	inputFrames   uint64
	outputFrames  uint64
	droppedFrames uint64

	incoming chan []float32
	outgoing chan []byte
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

func NewManager(feeder PCMFeeder) Manager {
	return &pionManager{
		feeder:   feeder,
		sessions: make(map[string]map[string]map[string]*pionSession),
	}
}

func (m *pionManager) Create(ctx context.Context, instanceID, callID string, request CreateRequest) (CreateResponse, error) {
	if m == nil || instanceID == "" || callID == "" {
		return CreateResponse{}, ErrInvalidOffer
	}
	offerType := strings.ToLower(strings.TrimSpace(request.Offer.Type))
	if offerType != "offer" || request.Offer.SDP == "" || len(request.Offer.SDP) > maxOfferBytes {
		return CreateResponse{}, ErrInvalidOffer
	}
	if ctx == nil {
		ctx = context.Background()
	}

	m.mu.Lock()
	calls := m.sessions[instanceID]
	if calls == nil {
		calls = make(map[string]map[string]*pionSession)
		m.sessions[instanceID] = calls
	}
	callSessions := calls[callID]
	if callSessions == nil {
		callSessions = make(map[string]*pionSession)
		calls[callID] = callSessions
	}
	if len(callSessions) >= maxSessionsPerCall {
		m.mu.Unlock()
		return CreateResponse{}, ErrSessionLimit
	}
	sessionID := uuid.NewString()
	m.mu.Unlock()

	pc, err := newBrowserPeerConnection()
	if err != nil {
		return CreateResponse{}, fmt.Errorf("create browser peer connection: %w", err)
	}
	session := &pionSession{
		manager:    m,
		instanceID: instanceID,
		callID:     callID,
		id:         sessionID,
		createdAt:  time.Now().UTC(),
		pc:         pc,
		state:      SessionStateConnecting,
		incoming:   make(chan []float32, mediaQueueDepth),
		outgoing:   make(chan []byte, mediaQueueDepth),
		stopCh:     make(chan struct{}),
	}
	session.wg.Add(2)
	go session.inputLoop()
	go session.outputLoop()

	m.mu.Lock()
	calls = m.sessions[instanceID]
	if calls == nil {
		calls = make(map[string]map[string]*pionSession)
		m.sessions[instanceID] = calls
	}
	callSessions = calls[callID]
	if callSessions == nil {
		callSessions = make(map[string]*pionSession)
		calls[callID] = callSessions
	}
	if len(callSessions) >= maxSessionsPerCall {
		m.mu.Unlock()
		session.close()
		return CreateResponse{}, ErrSessionLimit
	}
	callSessions[sessionID] = session
	m.mu.Unlock()

	fail := func(cause error) (CreateResponse, error) {
		_ = m.CloseSession(instanceID, callID, sessionID)
		return CreateResponse{}, cause
	}

	pc.OnDataChannel(func(channel *webrtc.DataChannel) {
		session.attachDataChannel(channel)
	})
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		switch state {
		case webrtc.PeerConnectionStateFailed:
			session.setState(SessionStateFailed)
			go func() { _ = m.CloseSession(instanceID, callID, sessionID) }()
		case webrtc.PeerConnectionStateClosed:
			go func() { _ = m.CloseSession(instanceID, callID, sessionID) }()
		}
	})

	remote := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: request.Offer.SDP}
	if err = pc.SetRemoteDescription(remote); err != nil {
		return fail(fmt.Errorf("set browser remote description: %w", err))
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return fail(fmt.Errorf("create browser SDP answer: %w", err))
	}
	gatheringComplete := webrtc.GatheringCompletePromise(pc)
	if err = pc.SetLocalDescription(answer); err != nil {
		return fail(fmt.Errorf("set browser local description: %w", err))
	}
	select {
	case <-gatheringComplete:
	case <-ctx.Done():
		return fail(fmt.Errorf("gather browser ICE candidates: %w", ctx.Err()))
	}
	local := pc.LocalDescription()
	if local == nil || local.SDP == "" {
		return fail(fmt.Errorf("create browser SDP answer: empty local description"))
	}

	return CreateResponse{
		SessionID: sessionID,
		Answer:    SDPDescription{Type: "answer", SDP: local.SDP},
		Audio:     DefaultProtocolInfo(),
	}, nil
}

func (m *pionManager) Sessions(instanceID, callID string) ([]SessionInfo, error) {
	if m == nil {
		return nil, ErrSessionNotFound
	}
	sessions := m.snapshot(instanceID, callID)
	result := make([]SessionInfo, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, session.info())
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func (m *pionManager) CloseSession(instanceID, callID, sessionID string) error {
	if m == nil || sessionID == "" {
		return ErrSessionNotFound
	}
	m.mu.Lock()
	calls := m.sessions[instanceID]
	callSessions := calls[callID]
	session := callSessions[sessionID]
	if session != nil {
		delete(callSessions, sessionID)
		if len(callSessions) == 0 {
			delete(calls, callID)
		}
		if len(calls) == 0 {
			delete(m.sessions, instanceID)
		}
	}
	m.mu.Unlock()
	if session == nil {
		return ErrSessionNotFound
	}
	session.close()
	return nil
}

func (m *pionManager) CloseCall(instanceID, callID string) {
	for _, session := range m.takeCall(instanceID, callID) {
		session.close()
	}
}

func (m *pionManager) CloseInstance(instanceID string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	calls := m.sessions[instanceID]
	delete(m.sessions, instanceID)
	m.mu.Unlock()
	for _, callSessions := range calls {
		for _, session := range callSessions {
			session.close()
		}
	}
}

func (m *pionManager) HandlePCM(instanceID, callID string, pcm []float32) {
	if len(pcm) == 0 {
		return
	}
	frame, err := EncodePCMFrame(pcm)
	if err != nil {
		return
	}
	defer zeroFrame(frame)
	for _, session := range m.snapshot(instanceID, callID) {
		session.enqueueOutgoing(append([]byte(nil), frame...))
	}
}

func (m *pionManager) snapshot(instanceID, callID string) []*pionSession {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	callSessions := m.sessions[instanceID][callID]
	result := make([]*pionSession, 0, len(callSessions))
	for _, session := range callSessions {
		result = append(result, session)
	}
	m.mu.RUnlock()
	return result
}

func (m *pionManager) takeCall(instanceID, callID string) []*pionSession {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	calls := m.sessions[instanceID]
	callSessions := calls[callID]
	delete(calls, callID)
	if len(calls) == 0 {
		delete(m.sessions, instanceID)
	}
	result := make([]*pionSession, 0, len(callSessions))
	for _, session := range callSessions {
		result = append(result, session)
	}
	m.mu.Unlock()
	return result
}

func (s *pionSession) attachDataChannel(channel *webrtc.DataChannel) {
	if channel == nil || channel.Label() != DataChannelLabel {
		if channel != nil {
			_ = channel.Close()
		}
		s.incrementDropped()
		go func() { _ = s.manager.CloseSession(s.instanceID, s.callID, s.id) }()
		return
	}
	if protocol := channel.Protocol(); protocol != "" && protocol != DataChannelProtocol {
		_ = channel.Close()
		s.incrementDropped()
		go func() { _ = s.manager.CloseSession(s.instanceID, s.callID, s.id) }()
		return
	}

	s.mu.Lock()
	if s.channel != nil || s.state == SessionStateClosed || s.state == SessionStateFailed {
		s.mu.Unlock()
		_ = channel.Close()
		return
	}
	s.channel = channel
	s.mu.Unlock()

	channel.SetBufferedAmountLowThreshold(maxBufferedAmount / 2)
	channel.OnOpen(func() { s.setState(SessionStateOpen) })
	channel.OnClose(func() {
		go func() { _ = s.manager.CloseSession(s.instanceID, s.callID, s.id) }()
	})
	channel.OnMessage(func(message webrtc.DataChannelMessage) {
		if message.IsString {
			s.incrementDropped()
			return
		}
		pcm, err := DecodePCMFrame(message.Data)
		if err != nil {
			s.incrementDropped()
			return
		}
		s.enqueueIncoming(pcm)
	})
}

func (s *pionSession) enqueueIncoming(pcm []float32) {
	select {
	case <-s.stopCh:
		zeroPCM(pcm)
	case s.incoming <- pcm:
	default:
		zeroPCM(pcm)
		s.incrementDropped()
	}
}

func (s *pionSession) enqueueOutgoing(frame []byte) {
	if !s.isOpen() {
		zeroFrame(frame)
		s.incrementDropped()
		return
	}
	select {
	case <-s.stopCh:
		zeroFrame(frame)
	case s.outgoing <- frame:
	default:
		zeroFrame(frame)
		s.incrementDropped()
	}
}

func (s *pionSession) inputLoop() {
	defer s.wg.Done()
	for {
		select {
		case <-s.stopCh:
			return
		case pcm := <-s.incoming:
			if s.manager.feeder != nil {
				if err := s.manager.feeder(s.instanceID, s.callID, pcm); err != nil {
					s.incrementDropped()
				} else {
					s.mu.Lock()
					s.inputFrames++
					s.mu.Unlock()
				}
			} else {
				s.incrementDropped()
			}
			zeroPCM(pcm)
		}
	}
}

func (s *pionSession) outputLoop() {
	defer s.wg.Done()
	for {
		select {
		case <-s.stopCh:
			return
		case frame := <-s.outgoing:
			s.sendFrame(frame)
			zeroFrame(frame)
		}
	}
}

func (s *pionSession) sendFrame(frame []byte) {
	s.mu.RLock()
	channel := s.channel
	open := s.state == SessionStateOpen && channel != nil
	s.mu.RUnlock()
	if !open || channel.BufferedAmount() > maxBufferedAmount {
		s.incrementDropped()
		return
	}
	if err := channel.Send(frame); err != nil {
		s.incrementDropped()
		return
	}
	s.mu.Lock()
	s.outputFrames++
	s.mu.Unlock()
}

func (s *pionSession) setState(state SessionState) {
	s.mu.Lock()
	if s.state != SessionStateClosed {
		s.state = state
	}
	s.mu.Unlock()
}

func (s *pionSession) isOpen() bool {
	s.mu.RLock()
	open := s.state == SessionStateOpen && s.channel != nil
	s.mu.RUnlock()
	return open
}

func (s *pionSession) incrementDropped() {
	s.mu.Lock()
	s.droppedFrames++
	s.mu.Unlock()
}

func (s *pionSession) info() SessionInfo {
	s.mu.RLock()
	info := SessionInfo{
		SessionID:     s.id,
		CallID:        s.callID,
		State:         s.state,
		ChannelOpen:   s.state == SessionStateOpen && s.channel != nil,
		CreatedAt:     s.createdAt,
		InputFrames:   s.inputFrames,
		OutputFrames:  s.outputFrames,
		DroppedFrames: s.droppedFrames,
	}
	s.mu.RUnlock()
	return info
}

func (s *pionSession) close() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
		s.mu.Lock()
		s.state = SessionStateClosed
		channel := s.channel
		s.channel = nil
		pc := s.pc
		s.pc = nil
		s.mu.Unlock()
		if channel != nil {
			_ = channel.Close()
		}
		if pc != nil {
			_ = pc.Close()
		}
		s.wg.Wait()
		for {
			select {
			case pcm := <-s.incoming:
				zeroPCM(pcm)
			default:
				goto outgoing
			}
		}
	outgoing:
		for {
			select {
			case frame := <-s.outgoing:
				zeroFrame(frame)
			default:
				return
			}
		}
	})
}

var _ Manager = (*pionManager)(nil)
