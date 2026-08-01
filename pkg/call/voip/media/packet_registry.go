package media

import (
	"errors"
	"fmt"
	"sync"

	call_state "github.com/evolution-foundation/evolution-go/pkg/call/voip/call"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/wa"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

var (
	ErrPacketSessionNotReady = errors.New("RTP/SRTP packet session is not ready")
	ErrNonRTPFrame           = errors.New("relay frame is not RTP/SRTP")
)

type PacketSource interface {
	RelayData(instanceID, callID string) (*core.RelayData, bool)
	State(instanceID, callID string) (*call_state.Info, bool)
	SRTPKeying(instanceID, callID, selfDeviceJID, peerDeviceJID string) (core.SRTPKeyingMaterial, core.SRTPKeyingMaterial, error)
}

type packetSession struct {
	srtp     *SRTPSession
	rtp      *RTPSession
	selfSSRC uint32
	peerSSRC uint32
}

func newPacketSession(sendKeying, receiveKeying core.SRTPKeyingMaterial, selfSSRC, peerSSRC uint32) (*packetSession, error) {
	if selfSSRC == 0 || peerSSRC == 0 {
		return nil, fmt.Errorf("RTP SSRC values must be non-zero")
	}
	srtp, err := NewSRTPSession(sendKeying, receiveKeying, core.SRTPSendAuthTagLen, core.SRTPRecvAuthTagLen)
	if err != nil {
		return nil, err
	}
	rtp, err := NewWhatsAppOpusRTPSession(selfSSRC)
	if err != nil {
		srtp.Close()
		return nil, err
	}
	return &packetSession{srtp: srtp, rtp: rtp, selfSSRC: selfSSRC, peerSSRC: peerSSRC}, nil
}

func (s *packetSession) protectOpus(payload []byte, durationSamples uint32, marker bool) ([]byte, error) {
	if s == nil || s.srtp == nil || s.rtp == nil {
		return nil, ErrPacketSessionNotReady
	}
	packet := s.rtp.CreatePacketWithDuration(payload, durationSamples, marker)
	defer packet.Wipe()
	return s.srtp.Protect(packet)
}

func (s *packetSession) unprotect(frame []byte) (*RTPPacket, error) {
	if s == nil || s.srtp == nil {
		return nil, ErrPacketSessionNotReady
	}
	packet, err := s.srtp.Unprotect(frame)
	if err != nil {
		return nil, err
	}
	if packet.Header.SSRC != s.peerSSRC {
		got := packet.Header.SSRC
		packet.Wipe()
		return nil, fmt.Errorf("unexpected RTP SSRC: got %d, want %d", got, s.peerSSRC)
	}
	if packet.Header.PayloadType != core.PayloadTypeWhatsAppOpus {
		got := packet.Header.PayloadType
		packet.Wipe()
		return nil, fmt.Errorf("unexpected RTP payload type: %d", got)
	}
	return packet, nil
}

func (s *packetSession) close() {
	if s == nil {
		return
	}
	if s.srtp != nil {
		s.srtp.Close()
	}
	s.srtp = nil
	s.rtp = nil
	s.selfSSRC = 0
	s.peerSSRC = 0
}

type PacketRegistry struct {
	mu       sync.RWMutex
	source   PacketSource
	clients  map[string]*whatsmeow.Client
	sessions map[string]map[string]*packetSession
	onRTP    func(instanceID, callID string, packet *RTPPacket)
}

func NewPacketRegistry(source PacketSource) *PacketRegistry {
	return &PacketRegistry{
		source:   source,
		clients:  make(map[string]*whatsmeow.Client),
		sessions: make(map[string]map[string]*packetSession),
	}
}

func (r *PacketRegistry) SetOnRTP(callback func(instanceID, callID string, packet *RTPPacket)) {
	r.mu.Lock()
	r.onRTP = callback
	r.mu.Unlock()
}

func (r *PacketRegistry) Attach(instanceID string, client *whatsmeow.Client) {
	if r == nil || instanceID == "" || client == nil {
		return
	}
	r.mu.Lock()
	previous := r.clients[instanceID]
	r.clients[instanceID] = client
	if previous != nil && previous != client {
		sessions := r.sessions[instanceID]
		delete(r.sessions, instanceID)
		r.mu.Unlock()
		closePacketSessions(sessions)
		return
	}
	r.mu.Unlock()
}

func (r *PacketRegistry) Prepare(instanceID, callID string) error {
	if r == nil || r.source == nil {
		return ErrPacketSessionNotReady
	}
	r.mu.RLock()
	client := r.clients[instanceID]
	if calls := r.sessions[instanceID]; calls != nil && calls[callID] != nil {
		r.mu.RUnlock()
		return nil
	}
	r.mu.RUnlock()
	if client == nil {
		return fmt.Errorf("packet runtime is not attached for instance %s", instanceID)
	}

	state, ok := r.source.State(instanceID, callID)
	if !ok || state == nil {
		return fmt.Errorf("call %s has no private state", callID)
	}
	relayData, ok := r.source.RelayData(instanceID, callID)
	if !ok || relayData == nil {
		return fmt.Errorf("call %s has no relay data", callID)
	}
	defer core.ZeroRelayData(relayData)

	ownJID := ownClientJID(client)
	peerJID, err := types.ParseJID(state.PeerJID)
	if err != nil || ownJID.IsEmpty() || peerJID.IsEmpty() {
		return fmt.Errorf("resolve RTP participants for call %s", callID)
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
	return r.PrepareWithDevices(instanceID, callID, selfDevice, peerDevice, selfSSRC, peerSSRC)
}

func (r *PacketRegistry) PrepareWithDevices(instanceID, callID, selfDeviceJID, peerDeviceJID string, selfSSRC, peerSSRC uint32) error {
	if r == nil || r.source == nil {
		return ErrPacketSessionNotReady
	}
	sendKeying, receiveKeying, err := r.source.SRTPKeying(instanceID, callID, selfDeviceJID, peerDeviceJID)
	if err != nil {
		return err
	}
	defer sendKeying.Wipe()
	defer receiveKeying.Wipe()

	candidate, err := newPacketSession(sendKeying, receiveKeying, selfSSRC, peerSSRC)
	if err != nil {
		return err
	}

	r.mu.Lock()
	calls := r.sessions[instanceID]
	if calls == nil {
		calls = make(map[string]*packetSession)
		r.sessions[instanceID] = calls
	}
	previous := calls[callID]
	calls[callID] = candidate
	r.mu.Unlock()
	if previous != nil {
		previous.close()
	}
	return nil
}

func (r *PacketRegistry) ProtectOpus(instanceID, callID string, payload []byte, durationSamples uint32, marker bool) ([]byte, error) {
	session, err := r.packetSession(instanceID, callID, true)
	if err != nil {
		return nil, err
	}
	return session.protectOpus(payload, durationSamples, marker)
}

func (r *PacketRegistry) Unprotect(instanceID, callID string, frame []byte) (*RTPPacket, error) {
	if len(frame) < 2 || frame[0]&0xc0 != 0x80 {
		return nil, ErrNonRTPFrame
	}
	session, err := r.packetSession(instanceID, callID, true)
	if err != nil {
		return nil, err
	}
	return session.unprotect(frame)
}

func (r *PacketRegistry) Handle(instanceID, callID string, frame []byte) error {
	packet, err := r.Unprotect(instanceID, callID, frame)
	if err != nil {
		return err
	}
	defer packet.Wipe()
	r.mu.RLock()
	callback := r.onRTP
	r.mu.RUnlock()
	if callback != nil {
		callback(instanceID, callID, packet)
	}
	return nil
}

func (r *PacketRegistry) packetSession(instanceID, callID string, lazyPrepare bool) (*packetSession, error) {
	r.mu.RLock()
	calls := r.sessions[instanceID]
	session := calls[callID]
	r.mu.RUnlock()
	if session != nil {
		return session, nil
	}
	if lazyPrepare {
		if err := r.Prepare(instanceID, callID); err != nil {
			return nil, err
		}
		r.mu.RLock()
		session = r.sessions[instanceID][callID]
		r.mu.RUnlock()
		if session != nil {
			return session, nil
		}
	}
	return nil, ErrPacketSessionNotReady
}

func (r *PacketRegistry) Remove(instanceID, callID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	calls := r.sessions[instanceID]
	session := calls[callID]
	delete(calls, callID)
	if len(calls) == 0 {
		delete(r.sessions, instanceID)
	}
	r.mu.Unlock()
	if session != nil {
		session.close()
	}
}

func (r *PacketRegistry) Close(instanceID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	delete(r.clients, instanceID)
	sessions := r.sessions[instanceID]
	delete(r.sessions, instanceID)
	r.mu.Unlock()
	closePacketSessions(sessions)
}

func closePacketSessions(sessions map[string]*packetSession) {
	for callID, session := range sessions {
		if session != nil {
			session.close()
		}
		delete(sessions, callID)
	}
}

func ownClientJID(client *whatsmeow.Client) types.JID {
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
