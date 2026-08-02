package media

import (
	"errors"
	"fmt"
	"log/slog"
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

type packetSRTPCandidate struct {
	receiveJID string
	session    *SRTPSession
}

type packetSession struct {
	mu sync.RWMutex

	srtpCandidates  []packetSRTPCandidate
	activeCandidate int
	receiveObserved bool
	rtp             *RTPSession
	selfSSRC        uint32
	peerSSRC        uint32
	peerObserved    bool
}

func newPacketSession(sendKeying, receiveKeying core.SRTPKeyingMaterial, selfSSRC, peerSSRC uint32) (*packetSession, error) {
	return newPacketSessionCandidates([]packetSRTPCandidateKeying{{receiveJID: "", send: sendKeying, receive: receiveKeying}}, selfSSRC, peerSSRC)
}

type packetSRTPCandidateKeying struct {
	receiveJID string
	send       core.SRTPKeyingMaterial
	receive    core.SRTPKeyingMaterial
}

func newPacketSessionCandidates(keyings []packetSRTPCandidateKeying, selfSSRC, peerSSRC uint32) (*packetSession, error) {
	if selfSSRC == 0 || peerSSRC == 0 {
		return nil, fmt.Errorf("RTP SSRC values must be non-zero")
	}
	if len(keyings) == 0 {
		return nil, fmt.Errorf("at least one SRTP receive candidate is required")
	}

	candidates := make([]packetSRTPCandidate, 0, len(keyings))
	for _, keying := range keyings {
		srtp, err := NewSRTPSession(keying.send, keying.receive, core.SRTPSendAuthTagLen, core.SRTPRecvAuthTagLen)
		if err != nil {
			for index := range candidates {
				candidates[index].session.Close()
			}
			return nil, err
		}
		candidates = append(candidates, packetSRTPCandidate{receiveJID: keying.receiveJID, session: srtp})
	}

	rtp, err := NewWhatsAppOpusRTPSession(selfSSRC)
	if err != nil {
		for index := range candidates {
			candidates[index].session.Close()
		}
		return nil, err
	}
	return &packetSession{
		srtpCandidates: candidates,
		rtp:            rtp,
		selfSSRC:       selfSSRC,
		peerSSRC:       peerSSRC,
	}, nil
}

func (s *packetSession) protectOpus(payload []byte, durationSamples uint32, marker bool) ([]byte, error) {
	if s == nil {
		return nil, ErrPacketSessionNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.srtpCandidates) == 0 || s.srtpCandidates[0].session == nil || s.rtp == nil {
		return nil, ErrPacketSessionNotReady
	}
	packet := s.rtp.CreatePacketWithDuration(payload, durationSamples, marker)
	defer packet.Wipe()
	return s.srtpCandidates[0].session.Protect(packet)
}

func (s *packetSession) unprotect(frame []byte) (*RTPPacket, uint32, uint32, bool, string, string, bool, error) {
	if s == nil {
		return nil, 0, 0, false, "", "", false, ErrPacketSessionNotReady
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.srtpCandidates) == 0 {
		return nil, 0, 0, false, "", "", false, ErrPacketSessionNotReady
	}

	previous, actual, first, err := s.peerSSRCCandidate(frame)
	if err != nil {
		return nil, previous, actual, false, "", "", false, err
	}

	order := make([]int, 0, len(s.srtpCandidates))
	if s.activeCandidate >= 0 && s.activeCandidate < len(s.srtpCandidates) {
		order = append(order, s.activeCandidate)
	}
	for index := range s.srtpCandidates {
		if index != s.activeCandidate {
			order = append(order, index)
		}
	}

	var packet *RTPPacket
	var authErrors []error
	selected := -1
	for _, index := range order {
		candidate := s.srtpCandidates[index]
		if candidate.session == nil {
			continue
		}
		packet, err = candidate.session.Unprotect(frame)
		if err == nil {
			selected = index
			break
		}
		if !isSRTPAuthenticationFailure(err) {
			return nil, previous, actual, false, "", "", false, err
		}
		authErrors = append(authErrors, fmt.Errorf("receive_jid=%s: %w", candidate.receiveJID, err))
	}
	if selected < 0 {
		return nil, previous, actual, false, "", "", false,
			fmt.Errorf("SRTP authentication failed for %d receive key candidates: %w", len(authErrors), errors.Join(authErrors...))
	}

	if packet.Header.SSRC != actual {
		got := packet.Header.SSRC
		packet.Wipe()
		return nil, previous, actual, false, "", "", false, fmt.Errorf("authenticated RTP SSRC mismatch: header=%d frame=%d", got, actual)
	}
	if packet.Header.PayloadType != core.PayloadTypeWhatsAppOpus {
		got := packet.Header.PayloadType
		packet.Wipe()
		return nil, previous, actual, false, "", "", false, fmt.Errorf("unexpected RTP payload type: %d", got)
	}

	previousReceiveJID := ""
	if s.receiveObserved && s.activeCandidate >= 0 && s.activeCandidate < len(s.srtpCandidates) {
		previousReceiveJID = s.srtpCandidates[s.activeCandidate].receiveJID
	}
	selectedReceiveJID := s.srtpCandidates[selected].receiveJID
	receiveChanged := !s.receiveObserved || selected != s.activeCandidate
	s.activeCandidate = selected
	s.receiveObserved = true

	ssrcChanged := false
	if first {
		previous, ssrcChanged = s.commitPeerSSRC(actual)
	}
	return packet, previous, actual, ssrcChanged, previousReceiveJID, selectedReceiveJID, receiveChanged, nil
}

func isSRTPAuthenticationFailure(err error) bool {
	var srtpErr *SRTPError
	return errors.As(err, &srtpErr) && srtpErr.Type == SRTPErrAuthFailed
}

func (s *packetSession) close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.srtpCandidates {
		if s.srtpCandidates[index].session != nil {
			s.srtpCandidates[index].session.Close()
		}
		s.srtpCandidates[index].session = nil
		s.srtpCandidates[index].receiveJID = ""
	}
	s.srtpCandidates = nil
	s.activeCandidate = 0
	s.receiveObserved = false
	s.rtp = nil
	s.selfSSRC = 0
	s.peerSSRC = 0
	s.peerObserved = false
}

type PacketRegistry struct {
	mu         sync.RWMutex
	source     PacketSource
	clients    map[string]*whatsmeow.Client
	sessions   map[string]map[string]*packetSession
	onRTP      func(instanceID, callID string, packet *RTPPacket)
	onPeerSSRC func(instanceID, callID string, previous, actual uint32)
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

func (r *PacketRegistry) SetOnPeerSSRC(callback func(instanceID, callID string, previous, actual uint32)) {
	r.mu.Lock()
	r.onPeerSSRC = callback
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
		attachPeerCallKeyObserver(r, instanceID, client)
		return
	}
	r.mu.Unlock()
	attachPeerCallKeyObserver(r, instanceID, client)
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
	creatorJID, _ := types.ParseJID(state.CallCreator)
	selfDevice, peerDevice := selectCallDeviceJIDs(relayData.ParticipantJIDs, ownJID, peerJID, creatorJID)
	selfSSRC, err := GenerateSecureSSRC(callID, selfDevice, 0)
	if err != nil {
		return err
	}
	peerSSRC, err := GenerateSecureSSRC(callID, peerDevice, 0)
	if err != nil {
		return err
	}

	receiveJIDs := receiveSRTPJIDCandidates(state, peerDevice)
	return r.PrepareWithDeviceCandidates(instanceID, callID, selfDevice, receiveJIDs, selfSSRC, peerSSRC)
}

func receiveSRTPJIDCandidates(state *call_state.Info, relayPeerDevice string) []string {
	if state == nil {
		return uniqueDeviceJIDs(relayPeerDevice)
	}
	peerAccount := ensureDeviceJIDString(state.PeerJID)
	creator := ""
	if state.Direction == core.CallDirectionIncoming {
		creator = ensureDeviceJIDString(state.CallCreator)
		return uniqueDeviceJIDs(relayPeerDevice, creator, peerAccount)
	}
	return uniqueDeviceJIDs(peerAccount, relayPeerDevice)
}

func uniqueDeviceJIDs(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	output := make([]string, 0, len(values))
	for _, value := range values {
		value = ensureDeviceJIDString(value)
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

func (r *PacketRegistry) PrepareWithDevices(instanceID, callID, selfDeviceJID, peerDeviceJID string, selfSSRC, peerSSRC uint32) error {
	return r.PrepareWithDeviceCandidates(instanceID, callID, selfDeviceJID, []string{peerDeviceJID}, selfSSRC, peerSSRC)
}

func (r *PacketRegistry) PrepareWithDeviceCandidates(instanceID, callID, selfDeviceJID string, receiveJIDs []string, selfSSRC, peerSSRC uint32) error {
	if r == nil || r.source == nil {
		return ErrPacketSessionNotReady
	}
	receiveJIDs = uniqueDeviceJIDs(receiveJIDs...)
	if len(receiveJIDs) == 0 {
		return fmt.Errorf("call %s has no SRTP receive JID candidates", callID)
	}

	keyings, err := buildPacketSRTPCandidates(r, instanceID, callID, selfDeviceJID, receiveJIDs)
	if err != nil {
		return err
	}
	defer wipePacketCandidateKeyings(keyings)

	candidate, err := newPacketSessionCandidates(keyings, selfSSRC, peerSSRC)
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

	labels := make([]string, 0, len(keyings))
	for _, keying := range keyings {
		labels = append(labels, keying.receiveJID)
	}
	slog.Info("WhatsApp SRTP receive candidates prepared",
		"instance", instanceID,
		"call_id", callID,
		"self_jid", selfDeviceJID,
		"receive_jids", labels,
	)
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
	packet, previous, actual, ssrcChanged, previousReceiveJID, selectedReceiveJID, receiveChanged, err := session.unprotect(frame)
	if err != nil {
		return nil, err
	}
	if receiveChanged {
		slog.Info("WhatsApp SRTP receive key selected",
			"instance", instanceID,
			"call_id", callID,
			"previous_receive_jid", previousReceiveJID,
			"receive_jid", selectedReceiveJID,
		)
	}
	if ssrcChanged {
		r.mu.RLock()
		callback := r.onPeerSSRC
		r.mu.RUnlock()
		if callback != nil {
			callback(instanceID, callID, previous, actual)
		}
	}
	return packet, nil
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
	removePeerCallKey(r, instanceID, callID)
}

func (r *PacketRegistry) Close(instanceID string) {
	if r == nil {
		return
	}
	detachPeerCallKeyObserver(r, instanceID)
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
