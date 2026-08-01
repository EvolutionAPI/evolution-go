#!/usr/bin/env python3
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def write(path: str, content: str) -> None:
    target = ROOT / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(content, encoding="utf-8")


def replace_once(path: str, old: str, new: str, label: str) -> None:
    target = ROOT / path
    text = target.read_text(encoding="utf-8")
    count = text.count(old)
    if count != 1:
        raise RuntimeError(f"{label}: expected one match in {path}, found {count}")
    target.write_text(text.replace(old, new, 1), encoding="utf-8")


write("pkg/call/voip/browser/types.go", r'''package browser

import (
    "context"
    "errors"
    "time"
)

const (
    DataChannelLabel    = "evolution-call-pcm"
    DataChannelProtocol = "evcall.pcm.v1"
    PCMSampleRate       = 16000
    PCMChannels         = 1
    PCMFrameSamples     = 960
)

var (
    ErrWebRTCDisabled    = errors.New("browser WebRTC bridge requires the voip_pion build tag")
    ErrInvalidOffer      = errors.New("invalid WebRTC SDP offer")
    ErrSessionNotFound   = errors.New("browser WebRTC session not found")
    ErrSessionLimit      = errors.New("browser WebRTC session limit reached")
    ErrInvalidPCMMessage = errors.New("invalid browser PCM message")
)

type SDPDescription struct {
    Type string `json:"type" binding:"required"`
    SDP  string `json:"sdp" binding:"required"`
}

type CreateRequest struct {
    Offer SDPDescription `json:"offer" binding:"required"`
}

type ProtocolInfo struct {
    DataChannel string `json:"dataChannel"`
    Protocol    string `json:"protocol"`
    Format      string `json:"format"`
    SampleRate  int    `json:"sampleRate"`
    Channels    int    `json:"channels"`
    FrameSamples int   `json:"frameSamples"`
}

type CreateResponse struct {
    SessionID string         `json:"sessionId"`
    Answer    SDPDescription `json:"answer"`
    Audio     ProtocolInfo   `json:"audio"`
}

type SessionState string

const (
    SessionStateConnecting SessionState = "connecting"
    SessionStateOpen       SessionState = "open"
    SessionStateClosed     SessionState = "closed"
    SessionStateFailed     SessionState = "failed"
)

type SessionInfo struct {
    SessionID     string       `json:"sessionId"`
    CallID        string       `json:"callId"`
    State         SessionState `json:"state"`
    ChannelOpen   bool         `json:"channelOpen"`
    CreatedAt     time.Time    `json:"createdAt"`
    InputFrames   uint64       `json:"inputFrames"`
    OutputFrames  uint64       `json:"outputFrames"`
    DroppedFrames uint64       `json:"droppedFrames"`
}

type PCMFeeder func(instanceID, callID string, pcm []float32) error

type Manager interface {
    Create(ctx context.Context, instanceID, callID string, request CreateRequest) (CreateResponse, error)
    Sessions(instanceID, callID string) ([]SessionInfo, error)
    CloseSession(instanceID, callID, sessionID string) error
    CloseCall(instanceID, callID string)
    CloseInstance(instanceID string)
    HandlePCM(instanceID, callID string, pcm []float32)
}

func DefaultProtocolInfo() ProtocolInfo {
    return ProtocolInfo{
        DataChannel: DataChannelLabel,
        Protocol: DataChannelProtocol,
        Format: "f32le",
        SampleRate: PCMSampleRate,
        Channels: PCMChannels,
        FrameSamples: PCMFrameSamples,
    }
}
''')

write("pkg/call/voip/browser/frame.go", r'''package browser

import (
    "encoding/binary"
    "fmt"
    "math"
)

const (
    pcmHeaderSize = 16
    pcmVersion    = 1
    pcmKind       = 1
    maxPCMSamples = PCMFrameSamples * 4
)

var pcmMagic = [4]byte{'E', 'V', 'P', 'C'}

func EncodePCMFrame(pcm []float32) ([]byte, error) {
    if len(pcm) == 0 || len(pcm) > maxPCMSamples {
        return nil, fmt.Errorf("%w: sample count %d", ErrInvalidPCMMessage, len(pcm))
    }
    output := make([]byte, pcmHeaderSize+len(pcm)*4)
    copy(output[:4], pcmMagic[:])
    output[4] = pcmVersion
    output[5] = pcmKind
    binary.LittleEndian.PutUint16(output[6:8], 0)
    binary.LittleEndian.PutUint32(output[8:12], PCMSampleRate)
    binary.LittleEndian.PutUint32(output[12:16], uint32(len(pcm)))
    offset := pcmHeaderSize
    for _, sample := range pcm {
        binary.LittleEndian.PutUint32(output[offset:offset+4], math.Float32bits(sample))
        offset += 4
    }
    return output, nil
}

func DecodePCMFrame(frame []byte) ([]float32, error) {
    if len(frame) < pcmHeaderSize {
        return nil, fmt.Errorf("%w: frame has %d bytes", ErrInvalidPCMMessage, len(frame))
    }
    if string(frame[:4]) != string(pcmMagic[:]) || frame[4] != pcmVersion || frame[5] != pcmKind {
        return nil, fmt.Errorf("%w: unsupported framing", ErrInvalidPCMMessage)
    }
    if binary.LittleEndian.Uint16(frame[6:8]) != 0 {
        return nil, fmt.Errorf("%w: unsupported flags", ErrInvalidPCMMessage)
    }
    if binary.LittleEndian.Uint32(frame[8:12]) != PCMSampleRate {
        return nil, fmt.Errorf("%w: sample rate must be %d", ErrInvalidPCMMessage, PCMSampleRate)
    }
    sampleCount := int(binary.LittleEndian.Uint32(frame[12:16]))
    if sampleCount <= 0 || sampleCount > maxPCMSamples {
        return nil, fmt.Errorf("%w: sample count %d", ErrInvalidPCMMessage, sampleCount)
    }
    expected := pcmHeaderSize + sampleCount*4
    if len(frame) != expected {
        return nil, fmt.Errorf("%w: frame has %d bytes, want %d", ErrInvalidPCMMessage, len(frame), expected)
    }
    pcm := make([]float32, sampleCount)
    offset := pcmHeaderSize
    for index := range pcm {
        pcm[index] = math.Float32frombits(binary.LittleEndian.Uint32(frame[offset : offset+4]))
        offset += 4
    }
    return pcm, nil
}

func zeroPCM(values []float32) {
    for index := range values {
        values[index] = 0
    }
}

func zeroFrame(value []byte) {
    for index := range value {
        value[index] = 0
    }
}
''')

write("pkg/call/voip/browser/manager_default.go", r'''//go:build !voip_pion

package browser

import "context"

type disabledManager struct{}

func NewManager(PCMFeeder) Manager {
    return &disabledManager{}
}

func (*disabledManager) Create(context.Context, string, string, CreateRequest) (CreateResponse, error) {
    return CreateResponse{}, ErrWebRTCDisabled
}

func (*disabledManager) Sessions(string, string) ([]SessionInfo, error) {
    return nil, ErrWebRTCDisabled
}

func (*disabledManager) CloseSession(string, string, string) error {
    return ErrWebRTCDisabled
}

func (*disabledManager) CloseCall(string, string) {}
func (*disabledManager) CloseInstance(string)   {}
func (*disabledManager) HandlePCM(string, string, []float32) {}
''')

write("pkg/call/voip/browser/manager_pion.go", r'''//go:build voip_pion

package browser

import (
    "context"
    "errors"
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
        feeder: feeder,
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

    pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
    if err != nil {
        return CreateResponse{}, fmt.Errorf("create browser peer connection: %w", err)
    }
    session := &pionSession{
        manager: m,
        instanceID: instanceID,
        callID: callID,
        id: sessionID,
        createdAt: time.Now().UTC(),
        pc: pc,
        state: SessionStateConnecting,
        incoming: make(chan []float32, mediaQueueDepth),
        outgoing: make(chan []byte, mediaQueueDepth),
        stopCh: make(chan struct{}),
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
        Answer: SDPDescription{Type: "answer", SDP: local.SDP},
        Audio: DefaultProtocolInfo(),
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
        return
    }
    if protocol := channel.Protocol(); protocol != "" && protocol != DataChannelProtocol {
        _ = channel.Close()
        s.incrementDropped()
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
        SessionID: s.id,
        CallID: s.callID,
        State: s.state,
        ChannelOpen: s.state == SessionStateOpen && s.channel != nil,
        CreatedAt: s.createdAt,
        InputFrames: s.inputFrames,
        OutputFrames: s.outputFrames,
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
var _ = errors.Is
''')

write("pkg/call/voip/browser/frame_test.go", r'''package browser

import (
    "errors"
    "math"
    "testing"
)

func TestPCMFrameRoundTrip(t *testing.T) {
    input := []float32{-1, -0.25, 0, 0.5, 1}
    frame, err := EncodePCMFrame(input)
    if err != nil {
        t.Fatal(err)
    }
    output, err := DecodePCMFrame(frame)
    if err != nil {
        t.Fatal(err)
    }
    if len(output) != len(input) {
        t.Fatalf("decoded %d samples, want %d", len(output), len(input))
    }
    for index := range input {
        if math.Float32bits(output[index]) != math.Float32bits(input[index]) {
            t.Fatalf("sample %d=%v, want %v", index, output[index], input[index])
        }
    }
}

func TestPCMFrameRejectsMalformedInput(t *testing.T) {
    frame, err := EncodePCMFrame(make([]float32, PCMFrameSamples))
    if err != nil {
        t.Fatal(err)
    }
    cases := [][]byte{
        nil,
        frame[:10],
        append([]byte(nil), frame[:len(frame)-1]...),
        append([]byte("BAD!"), frame[4:]...),
    }
    for _, value := range cases {
        if _, err = DecodePCMFrame(value); !errors.Is(err, ErrInvalidPCMMessage) {
            t.Fatalf("expected invalid PCM error, got %v", err)
        }
    }
}

func TestPCMFrameLimitsSamples(t *testing.T) {
    if _, err := EncodePCMFrame(nil); !errors.Is(err, ErrInvalidPCMMessage) {
        t.Fatalf("expected empty frame error, got %v", err)
    }
    if _, err := EncodePCMFrame(make([]float32, maxPCMSamples+1)); !errors.Is(err, ErrInvalidPCMMessage) {
        t.Fatalf("expected oversized frame error, got %v", err)
    }
}
''')

write("pkg/call/voip/browser/manager_default_test.go", r'''//go:build !voip_pion

package browser

import (
    "context"
    "errors"
    "testing"
)

func TestDefaultManagerIsDisabled(t *testing.T) {
    manager := NewManager(nil)
    _, err := manager.Create(context.Background(), "instance", "call", CreateRequest{})
    if !errors.Is(err, ErrWebRTCDisabled) {
        t.Fatalf("expected disabled error, got %v", err)
    }
}
''')

write("pkg/call/voip/browser/manager_pion_test.go", r'''//go:build voip_pion

package browser

import (
    "context"
    "testing"
    "time"

    "github.com/pion/webrtc/v4"
)

func TestPionManagerBridgesPCMOverDataChannel(t *testing.T) {
    fed := make(chan []float32, 2)
    manager := NewManager(func(_, _ string, pcm []float32) error {
        fed <- append([]float32(nil), pcm...)
        return nil
    })
    defer manager.CloseInstance("instance")

    client, err := webrtc.NewPeerConnection(webrtc.Configuration{})
    if err != nil {
        t.Fatal(err)
    }
    defer client.Close()
    protocol := DataChannelProtocol
    channel, err := client.CreateDataChannel(DataChannelLabel, &webrtc.DataChannelInit{Protocol: &protocol})
    if err != nil {
        t.Fatal(err)
    }
    opened := make(chan struct{})
    received := make(chan []float32, 2)
    channel.OnOpen(func() { close(opened) })
    channel.OnMessage(func(message webrtc.DataChannelMessage) {
        pcm, decodeErr := DecodePCMFrame(message.Data)
        if decodeErr != nil {
            t.Errorf("decode server PCM: %v", decodeErr)
            return
        }
        received <- pcm
    })

    offer, err := client.CreateOffer(nil)
    if err != nil {
        t.Fatal(err)
    }
    gather := webrtc.GatheringCompletePromise(client)
    if err = client.SetLocalDescription(offer); err != nil {
        t.Fatal(err)
    }
    select {
    case <-gather:
    case <-time.After(10 * time.Second):
        t.Fatal("client ICE gathering timed out")
    }

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    response, err := manager.Create(ctx, "instance", "call", CreateRequest{Offer: SDPDescription{
        Type: "offer",
        SDP: client.LocalDescription().SDP,
    }})
    if err != nil {
        t.Fatal(err)
    }
    if err = client.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: response.Answer.SDP}); err != nil {
        t.Fatal(err)
    }
    select {
    case <-opened:
    case <-time.After(10 * time.Second):
        t.Fatal("PCM data channel did not open")
    }

    browserPCM := make([]float32, PCMFrameSamples)
    browserPCM[0] = 0.25
    frame, err := EncodePCMFrame(browserPCM)
    if err != nil {
        t.Fatal(err)
    }
    if err = channel.Send(frame); err != nil {
        t.Fatal(err)
    }
    select {
    case got := <-fed:
        if len(got) != PCMFrameSamples || got[0] != 0.25 {
            t.Fatalf("unexpected fed PCM: len=%d first=%v", len(got), got[0])
        }
    case <-time.After(10 * time.Second):
        t.Fatal("server did not receive browser PCM")
    }

    serverPCM := make([]float32, PCMFrameSamples)
    serverPCM[0] = -0.5
    manager.HandlePCM("instance", "call", serverPCM)
    select {
    case got := <-received:
        if len(got) != PCMFrameSamples || got[0] != -0.5 {
            t.Fatalf("unexpected browser PCM: len=%d first=%v", len(got), got[0])
        }
    case <-time.After(10 * time.Second):
        t.Fatal("browser did not receive server PCM")
    }

    sessions, err := manager.Sessions("instance", "call")
    if err != nil || len(sessions) != 1 || !sessions[0].ChannelOpen {
        t.Fatalf("unexpected sessions: %+v err=%v", sessions, err)
    }
    if err = manager.CloseSession("instance", "call", response.SessionID); err != nil {
        t.Fatal(err)
    }
    sessions, err = manager.Sessions("instance", "call")
    if err != nil || len(sessions) != 0 {
        t.Fatalf("session was not removed: %+v err=%v", sessions, err)
    }
}
''')

# Coordinator: PCM fanout and media cleanup hooks.
replace_once(
    "pkg/call/lifecycle/coordinator.go",
    '''\taudio    *call_media.AudioRegistry
\tonRTP    func(instanceID, callID string, packet *call_media.RTPPacket)

\tincomingEnabled map[string]bool
''',
    '''\taudio    *call_media.AudioRegistry
\tonRTP    func(instanceID, callID string, packet *call_media.RTPPacket)
\tonPCM    func(instanceID, callID string, pcm []float32)
\tbrowserPCM func(instanceID, callID string, pcm []float32)
\tonCallMediaCleanup func(instanceID, callID string)
\tonInstanceMediaCleanup func(instanceID string)

\tincomingEnabled map[string]bool
''',
    "coordinator callbacks",
)
replace_once(
    "pkg/call/lifecycle/coordinator.go",
    '''\tcoordinator.audio = call_media.NewAudioRegistry(func(instanceID, callID string, payload []byte, durationSamples uint32, marker bool) error {
\t\treturn coordinator.SendOpus(instanceID, callID, payload, durationSamples, marker)
\t}, nil)
\tcoordinator.relays = call_media.NewRelayRegistry(incoming, nil, nil)
''',
    '''\tcoordinator.audio = call_media.NewAudioRegistry(func(instanceID, callID string, payload []byte, durationSamples uint32, marker bool) error {
\t\treturn coordinator.SendOpus(instanceID, callID, payload, durationSamples, marker)
\t}, nil)
\tcoordinator.audio.SetOnPCM(coordinator.dispatchPCM)
\tcoordinator.relays = call_media.NewRelayRegistry(incoming, nil, nil)
\tcoordinator.relays.SetOnRemoved(func(instanceID, callID string) {
\t\tcoordinator.audio.Remove(instanceID, callID)
\t\tcoordinator.packets.Remove(instanceID, callID)
\t\tcoordinator.notifyCallMediaCleanup(instanceID, callID)
\t})
\tcoordinator.relays.SetOnCleanup(func(instanceID string) {
\t\tcoordinator.audio.Close(instanceID)
\t\tcoordinator.packets.Close(instanceID)
\t\tcoordinator.notifyInstanceMediaCleanup(instanceID)
\t})
''',
    "coordinator relay cleanup",
)
replace_once(
    "pkg/call/lifecycle/coordinator.go",
    '''\tc.relays.Close(instanceID)
\tc.audio.Close(instanceID)
\tc.packets.Close(instanceID)
''',
    '''\tc.relays.Close(instanceID)
\tc.audio.Close(instanceID)
\tc.packets.Close(instanceID)
\tc.notifyInstanceMediaCleanup(instanceID)
''',
    "coordinator detach cleanup",
)
replace_once(
    "pkg/call/lifecycle/coordinator.go",
    '''\tc.relays.Remove(instanceID, callID)
\tc.audio.Remove(instanceID, callID)
\tc.packets.Remove(instanceID, callID)
\treturn nil
''',
    '''\tc.relays.Remove(instanceID, callID)
\tc.audio.Remove(instanceID, callID)
\tc.packets.Remove(instanceID, callID)
\tc.notifyCallMediaCleanup(instanceID, callID)
\treturn nil
''',
    "coordinator terminate cleanup",
)
replace_once(
    "pkg/call/lifecycle/coordinator.go",
    '''// SetOnPCM registers the internal decoded-audio sink used by a future WebRTC,
// native playback or test bridge. The callback receives an owned PCM copy.
func (c *Coordinator) SetOnPCM(callback func(instanceID, callID string, pcm []float32)) {
\tif c != nil {
\t\tc.audio.SetOnPCM(callback)
\t}
}
''',
    '''// SetOnPCM registers an optional external decoded-audio observer. Browser
// media keeps a separate internal sink so neither callback replaces the other.
func (c *Coordinator) SetOnPCM(callback func(instanceID, callID string, pcm []float32)) {
\tif c == nil {
\t\treturn
\t}
\tc.mu.Lock()
\tc.onPCM = callback
\tc.mu.Unlock()
}

func (c *Coordinator) SetBrowserPCM(callback func(instanceID, callID string, pcm []float32)) {
\tif c == nil {
\t\treturn
\t}
\tc.mu.Lock()
\tc.browserPCM = callback
\tc.mu.Unlock()
}

func (c *Coordinator) SetMediaCleanupHooks(onCall func(instanceID, callID string), onInstance func(instanceID string)) {
\tif c == nil {
\t\treturn
\t}
\tc.mu.Lock()
\tc.onCallMediaCleanup = onCall
\tc.onInstanceMediaCleanup = onInstance
\tc.mu.Unlock()
}

func (c *Coordinator) dispatchPCM(instanceID, callID string, pcm []float32) {
\tc.mu.RLock()
\tbrowserCallback := c.browserPCM
\texternalCallback := c.onPCM
\tc.mu.RUnlock()
\tif browserCallback != nil {
\t\tbrowserCallback(instanceID, callID, append([]float32(nil), pcm...))
\t}
\tif externalCallback != nil {
\t\texternalCallback(instanceID, callID, append([]float32(nil), pcm...))
\t}
}

func (c *Coordinator) notifyCallMediaCleanup(instanceID, callID string) {
\tc.mu.RLock()
\tcallback := c.onCallMediaCleanup
\tc.mu.RUnlock()
\tif callback != nil {
\t\tcallback(instanceID, callID)
\t}
}

func (c *Coordinator) notifyInstanceMediaCleanup(instanceID string) {
\tc.mu.RLock()
\tcallback := c.onInstanceMediaCleanup
\tc.mu.RUnlock()
\tif callback != nil {
\t\tcallback(instanceID)
\t}
}
''',
    "coordinator PCM callbacks",
)
replace_once(
    "pkg/call/lifecycle/coordinator.go",
    '''\tc.relays.Remove(instanceID, callID)
\tc.audio.Remove(instanceID, callID)
\tc.packets.Remove(instanceID, callID)
\tc.incoming.Remove(instanceID, callID)
''',
    '''\tc.relays.Remove(instanceID, callID)
\tc.audio.Remove(instanceID, callID)
\tc.packets.Remove(instanceID, callID)
\tc.incoming.Remove(instanceID, callID)
\tc.notifyCallMediaCleanup(instanceID, callID)
''',
    "coordinator private cleanup",
)

# Relay lifecycle cleanup notifications.
replace_once(
    "pkg/call/voip/media/relay_registry.go",
    '''\tonConnected func(instanceID, callID string)
\tonPacket    func(instanceID, callID string, packet []byte)
}''',
    '''\tonConnected func(instanceID, callID string)
\tonPacket    func(instanceID, callID string, packet []byte)
\tonRemoved   func(instanceID, callID string)
\tonCleanup   func(instanceID string)
}''',
    "relay session callbacks",
)
replace_once(
    "pkg/call/voip/media/relay_registry.go",
    '''func (s *relaySession) remove(callID string) {
\ts.mu.Lock()
\trelay := s.transports[callID]
\tdelete(s.transports, callID)
\tdelete(s.configuring, callID)
\ts.mu.Unlock()
\tif relay != nil {
\t\trelay.Cleanup()
\t}
}''',
    '''func (s *relaySession) remove(callID string) {
\ts.mu.Lock()
\trelay := s.transports[callID]
\tdelete(s.transports, callID)
\tdelete(s.configuring, callID)
\tcallback := s.onRemoved
\ts.mu.Unlock()
\tif relay != nil {
\t\trelay.Cleanup()
\t}
\tif callback != nil && callID != "" {
\t\tcallback(s.instanceID, callID)
\t}
}''',
    "relay remove callback",
)
replace_once(
    "pkg/call/voip/media/relay_registry.go",
    '''\ts.configuring = make(map[string]bool)
\ts.mu.Unlock()
\tfor _, relay := range transports {
\t\trelay.Cleanup()
\t}
}''',
    '''\ts.configuring = make(map[string]bool)
\tcallback := s.onCleanup
\ts.mu.Unlock()
\tfor _, relay := range transports {
\t\trelay.Cleanup()
\t}
\tif callback != nil {
\t\tcallback(s.instanceID)
\t}
}''',
    "relay cleanup callback",
)
replace_once(
    "pkg/call/voip/media/relay_registry.go",
    '''\tonConnected func(instanceID, callID string)
\tonPacket    func(instanceID, callID string, packet []byte)
}''',
    '''\tonConnected func(instanceID, callID string)
\tonPacket    func(instanceID, callID string, packet []byte)
\tonRemoved   func(instanceID, callID string)
\tonCleanup   func(instanceID string)
}''',
    "relay registry callbacks",
)
replace_once(
    "pkg/call/voip/media/relay_registry.go",
    '''func (r *RelayRegistry) SetOnPacket(callback func(instanceID, callID string, packet []byte)) {
\tr.mu.Lock()
\tr.onPacket = callback
\tfor _, session := range r.sessions {
\t\tsession.onPacket = callback
\t}
\tr.mu.Unlock()
}
''',
    '''func (r *RelayRegistry) SetOnPacket(callback func(instanceID, callID string, packet []byte)) {
\tr.mu.Lock()
\tr.onPacket = callback
\tfor _, session := range r.sessions {
\t\tsession.onPacket = callback
\t}
\tr.mu.Unlock()
}

func (r *RelayRegistry) SetOnRemoved(callback func(instanceID, callID string)) {
\tr.mu.Lock()
\tr.onRemoved = callback
\tfor _, session := range r.sessions {
\t\tsession.onRemoved = callback
\t}
\tr.mu.Unlock()
}

func (r *RelayRegistry) SetOnCleanup(callback func(instanceID string)) {
\tr.mu.Lock()
\tr.onCleanup = callback
\tfor _, session := range r.sessions {
\t\tsession.onCleanup = callback
\t}
\tr.mu.Unlock()
}
''',
    "relay setters",
)
replace_once(
    "pkg/call/voip/media/relay_registry.go",
    '''\tcandidate.onConnected = r.onConnected
\tcandidate.onPacket = r.onPacket
''',
    '''\tcandidate.onConnected = r.onConnected
\tcandidate.onPacket = r.onPacket
\tcandidate.onRemoved = r.onRemoved
\tcandidate.onCleanup = r.onCleanup
''',
    "relay attach callbacks",
)

# Call service browser API.
replace_once(
    "pkg/call/service/call_service.go",
    '''\tcall_driver "github.com/evolution-foundation/evolution-go/pkg/call/voip/driver"
''',
    '''\tcall_browser "github.com/evolution-foundation/evolution-go/pkg/call/voip/browser"
\tcall_driver "github.com/evolution-foundation/evolution-go/pkg/call/voip/driver"
''',
    "service browser import",
)
replace_once(
    "pkg/call/service/call_service.go",
    '''const signalingTimeout = 30 * time.Second
''',
    '''const signalingTimeout = 30 * time.Second

var ErrCallNotActive = errors.New("call media is not active")
''',
    "service active error",
)
replace_once(
    "pkg/call/service/call_service.go",
    '''\tRejectCall(data *RejectCallStruct, instance *instance_model.Instance) error
\tRuntimeStatus(instance *instance_model.Instance) (call_runtime.Snapshot, error)
}''',
    '''\tRejectCall(data *RejectCallStruct, instance *instance_model.Instance) error
\tRuntimeStatus(instance *instance_model.Instance) (call_runtime.Snapshot, error)
\tCreateWebRTC(ctx context.Context, callID string, request call_browser.CreateRequest, instance *instance_model.Instance) (call_browser.CreateResponse, error)
\tWebRTCSessions(callID string, instance *instance_model.Instance) ([]call_browser.SessionInfo, error)
\tCloseWebRTC(callID, sessionID string, instance *instance_model.Instance) error
}''',
    "service interface",
)
replace_once(
    "pkg/call/service/call_service.go",
    '''\tloggerWrapper    *logger_wrapper.LoggerManager
\tcoordinator      *call_lifecycle.Coordinator
}''',
    '''\tloggerWrapper    *logger_wrapper.LoggerManager
\tcoordinator      *call_lifecycle.Coordinator
\tbrowser          call_browser.Manager
}''',
    "service browser field",
)
replace_once(
    "pkg/call/service/call_service.go",
    '''func (c *callService) RuntimeStatus(instance *instance_model.Instance) (call_runtime.Snapshot, error) {
\tclient, err := c.ensureClientConnected(instance.Id)
\tif err != nil {
\t\treturn call_runtime.Snapshot{InstanceID: instance.Id}, err
\t}

\truntime := c.coordinator.RuntimeFor(instance.Id, client)
\treturn runtime.Snapshot(), nil
}
''',
    '''func (c *callService) RuntimeStatus(instance *instance_model.Instance) (call_runtime.Snapshot, error) {
\tclient, err := c.ensureClientConnected(instance.Id)
\tif err != nil {
\t\treturn call_runtime.Snapshot{InstanceID: instance.Id}, err
\t}

\truntime := c.coordinator.RuntimeFor(instance.Id, client)
\treturn runtime.Snapshot(), nil
}

func (c *callService) CreateWebRTC(ctx context.Context, callID string, request call_browser.CreateRequest, instance *instance_model.Instance) (call_browser.CreateResponse, error) {
\tclient, err := c.ensureClientConnected(instance.Id)
\tif err != nil {
\t\treturn call_browser.CreateResponse{}, err
\t}
\truntime := c.coordinator.RuntimeFor(instance.Id, client)
\tcall, ok := runtime.Call(callID)
\tif !ok {
\t\treturn call_browser.CreateResponse{}, fmt.Errorf("call %s not found", callID)
\t}
\tif call.State != call_runtime.StateActive {
\t\treturn call_browser.CreateResponse{}, fmt.Errorf("%w: call %s is %s", ErrCallNotActive, callID, call.State)
\t}
\treturn c.browser.Create(ctx, instance.Id, callID, request)
}

func (c *callService) WebRTCSessions(callID string, instance *instance_model.Instance) ([]call_browser.SessionInfo, error) {
\tif callID == "" {
\t\treturn nil, fmt.Errorf("callId is required")
\t}
\treturn c.browser.Sessions(instance.Id, callID)
}

func (c *callService) CloseWebRTC(callID, sessionID string, instance *instance_model.Instance) error {
\tif callID == "" || sessionID == "" {
\t\treturn call_browser.ErrSessionNotFound
\t}
\treturn c.browser.CloseSession(instance.Id, callID, sessionID)
}
''',
    "service browser methods",
)
replace_once(
    "pkg/call/service/call_service.go",
    '''\treturn &callService{
\t\tclientPointer:    clientPointer,
\t\twhatsmeowService: whatsmeowService,
\t\tloggerWrapper:    loggerWrapper,
\t\tcoordinator:      coordinator,
\t}
}''',
    '''\tservice := &callService{
\t\tclientPointer:    clientPointer,
\t\twhatsmeowService: whatsmeowService,
\t\tloggerWrapper:    loggerWrapper,
\t\tcoordinator:      coordinator,
\t}
\tservice.browser = call_browser.NewManager(coordinator.FeedPCM)
\tcoordinator.SetBrowserPCM(service.browser.HandlePCM)
\tcoordinator.SetMediaCleanupHooks(service.browser.CloseCall, service.browser.CloseInstance)
\treturn service
}''',
    "service constructor",
)

# Handler endpoints and status mapping.
replace_once(
    "pkg/call/handler/call_handler.go",
    '''import (
\t"net/http"

\tcall_service "github.com/evolution-foundation/evolution-go/pkg/call/service"
''',
    '''import (
\t"context"
\t"errors"
\t"net/http"
\t"time"

\tcall_service "github.com/evolution-foundation/evolution-go/pkg/call/service"
\tcall_browser "github.com/evolution-foundation/evolution-go/pkg/call/voip/browser"
''',
    "handler imports",
)
replace_once(
    "pkg/call/handler/call_handler.go",
    '''\tRejectCall(ctx *gin.Context)
\tStatus(ctx *gin.Context)
}''',
    '''\tRejectCall(ctx *gin.Context)
\tStatus(ctx *gin.Context)
\tCreateWebRTC(ctx *gin.Context)
\tListWebRTC(ctx *gin.Context)
\tCloseWebRTC(ctx *gin.Context)
}''',
    "handler interface",
)
replace_once(
    "pkg/call/handler/call_handler.go",
    '''func NewCallHandler(callService call_service.CallService) CallHandler {
\treturn &callHandler{callService: callService}
}
''',
    '''func browserHTTPStatus(err error) int {
\tswitch {
\tcase errors.Is(err, call_browser.ErrWebRTCDisabled):
\t\treturn http.StatusNotImplemented
\tcase errors.Is(err, call_browser.ErrInvalidOffer), errors.Is(err, call_browser.ErrInvalidPCMMessage):
\t\treturn http.StatusBadRequest
\tcase errors.Is(err, call_browser.ErrSessionNotFound):
\t\treturn http.StatusNotFound
\tcase errors.Is(err, call_browser.ErrSessionLimit), errors.Is(err, call_service.ErrCallNotActive):
\t\treturn http.StatusConflict
\tcase errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
\t\treturn http.StatusGatewayTimeout
\tdefault:
\t\treturn http.StatusInternalServerError
\t}
}

// Create browser WebRTC PCM session
// @Summary Create an experimental browser PCM bridge
// @Description Exchanges a complete SDP offer/answer. Requires the voip_pion build and an active WhatsApp call.
// @Tags Call
// @Accept json
// @Produce json
// @Param callId path string true "Call ID"
// @Param offer body call_browser.CreateRequest true "Browser SDP offer"
// @Success 201 {object} call_browser.CreateResponse
// @Router /call/{callId}/webrtc [post]
func (g *callHandler) CreateWebRTC(ctx *gin.Context) {
\tinstance, ok := instanceFromContext(ctx)
\tif !ok {
\t\treturn
\t}
\tcallID := ctx.Param("callId")
\tif callID == "" {
\t\tctx.JSON(http.StatusBadRequest, gin.H{"error": "callId is required"})
\t\treturn
\t}
\tvar request call_browser.CreateRequest
\tif err := ctx.ShouldBindJSON(&request); err != nil {
\t\tctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
\t\treturn
\t}
\trequestContext, cancel := context.WithTimeout(ctx.Request.Context(), 30*time.Second)
\tdefer cancel()
\tresponse, err := g.callService.CreateWebRTC(requestContext, callID, request, instance)
\tif err != nil {
\t\tctx.JSON(browserHTTPStatus(err), gin.H{"error": err.Error()})
\t\treturn
\t}
\tctx.JSON(http.StatusCreated, response)
}

// List browser WebRTC PCM sessions
// @Summary List browser PCM bridge sessions
// @Tags Call
// @Produce json
// @Param callId path string true "Call ID"
// @Success 200 {object} gin.H
// @Router /call/{callId}/webrtc [get]
func (g *callHandler) ListWebRTC(ctx *gin.Context) {
\tinstance, ok := instanceFromContext(ctx)
\tif !ok {
\t\treturn
\t}
\tsessions, err := g.callService.WebRTCSessions(ctx.Param("callId"), instance)
\tif err != nil {
\t\tctx.JSON(browserHTTPStatus(err), gin.H{"error": err.Error()})
\t\treturn
\t}
\tctx.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// Close browser WebRTC PCM session
// @Summary Close a browser PCM bridge session
// @Tags Call
// @Produce json
// @Param callId path string true "Call ID"
// @Param sessionId path string true "WebRTC session ID"
// @Success 200 {object} gin.H
// @Router /call/{callId}/webrtc/{sessionId} [delete]
func (g *callHandler) CloseWebRTC(ctx *gin.Context) {
\tinstance, ok := instanceFromContext(ctx)
\tif !ok {
\t\treturn
\t}
\tif err := g.callService.CloseWebRTC(ctx.Param("callId"), ctx.Param("sessionId"), instance); err != nil {
\t\tctx.JSON(browserHTTPStatus(err), gin.H{"error": err.Error()})
\t\treturn
\t}
\tctx.JSON(http.StatusOK, gin.H{"message": "browser media session closed"})
}

func NewCallHandler(callService call_service.CallService) CallHandler {
\treturn &callHandler{callService: callService}
}
''',
    "handler browser methods",
)

# Routes.
replace_once(
    "pkg/routes/routes.go",
    '''\t\troutes.POST("/:callId/accept", r.callHandler.AcceptCall)
\t\troutes.DELETE("/:callId", r.callHandler.TerminateCall)
\t\troutes.POST("/reject", r.jidValidationMiddleware.ValidateNumberField(), r.callHandler.RejectCall)
''',
    '''\t\troutes.POST("/:callId/accept", r.callHandler.AcceptCall)
\t\troutes.POST("/:callId/webrtc", r.callHandler.CreateWebRTC)
\t\troutes.GET("/:callId/webrtc", r.callHandler.ListWebRTC)
\t\troutes.DELETE("/:callId/webrtc/:sessionId", r.callHandler.CloseWebRTC)
\t\troutes.DELETE("/:callId", r.callHandler.TerminateCall)
\t\troutes.POST("/reject", r.jidValidationMiddleware.ValidateNumberField(), r.callHandler.RejectCall)
''',
    "browser routes",
)

# Remove migration artifacts from the commit produced by the workflow.
for relative in ["tools/integrate_browser_webrtc.py", ".github/workflows/integrate-browser-webrtc.yml"]:
    target = ROOT / relative
    if target.exists():
        target.unlink()
