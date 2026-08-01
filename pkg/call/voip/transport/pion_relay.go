//go:build voip_pion

// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package transport

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	"github.com/pion/webrtc/v4"
)

const (
	relayConnectionTimeout = 20 * time.Second
	relayKeepaliveInterval = 1100 * time.Millisecond
)

type relayConnectionState uint8

const (
	relayStateConnecting relayConnectionState = iota
	relayStateOpen
	relayStateClosed
	relayStateFailed
)

type pionRelayConnection struct {
	mu         sync.RWMutex
	state      relayConnectionState
	pc         *webrtc.PeerConnection
	channel    *webrtc.DataChannel
	id         string
	info       RelayConfig
	localUfrag string
	keepalive  *time.Ticker
	stopCh     chan struct{}
	stopOnce   sync.Once
}

func (c *pionRelayConnection) isOpen() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state == relayStateOpen && c.channel != nil
}

func (c *pionRelayConnection) setOpen() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != relayStateConnecting {
		return false
	}
	c.state = relayStateOpen
	return true
}

func (c *pionRelayConnection) setTerminal(state relayConnectionState) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == relayStateClosed || c.state == relayStateFailed {
		return false
	}
	c.state = state
	return true
}

// PionRelayTransport opens WhatsApp relay DataChannels when the voip_pion build
// tag is enabled. The default build never includes this implementation.
type PionRelayTransport struct {
	mu               sync.RWMutex
	connections      map[string]*pionRelayConnection
	log              *slog.Logger
	ssrc             uint32
	subscriptionSSRC uint32
	onConnected      func(ip string, port int)
	onReceive        func(data []byte)
}

func NewPionRelayTransport(log *slog.Logger) *PionRelayTransport {
	if log == nil {
		log = slog.Default()
	}
	return &PionRelayTransport{
		connections: make(map[string]*pionRelayConnection),
		log:         log,
	}
}

// NewRelayTransport selects the real transport only in a voip_pion build.
func NewRelayTransport(log *slog.Logger) RelayTransport {
	return NewPionRelayTransport(log)
}

func (m *PionRelayTransport) SetSSRC(ssrc uint32) {
	m.mu.Lock()
	m.ssrc = ssrc
	m.mu.Unlock()
}

func (m *PionRelayTransport) SetSubscriptionSSRC(ssrc uint32) {
	m.mu.Lock()
	m.subscriptionSSRC = ssrc
	m.mu.Unlock()
}

func (m *PionRelayTransport) SetOnConnected(callback func(ip string, port int)) {
	m.mu.Lock()
	m.onConnected = callback
	m.mu.Unlock()
}

func (m *PionRelayTransport) SetOnReceive(callback func(data []byte)) {
	m.mu.Lock()
	m.onReceive = callback
	m.mu.Unlock()
}

func (m *PionRelayTransport) ResendSubscriptions() {
	for _, connection := range m.connectionSnapshot() {
		if connection.isOpen() {
			m.sendSTUNRegistration(connection)
		}
	}
}

func relayConnectionID(ip string, port int, authTokenID string) string {
	identity := fmt.Sprintf("%s:%d", ip, port)
	if authTokenID != "" {
		identity += "#" + authTokenID
	}
	return identity
}

func (m *PionRelayTransport) ConfigureRelays(relays []RelayConfig) error {
	if len(relays) == 0 {
		return fmt.Errorf("no relay configurations supplied")
	}

	var setupErrors []error
	for _, relay := range relays {
		config := cloneRelayConfig(relay)
		if config.Port == 0 {
			config.Port = core.WARelayPort
		}
		if config.IP == "" || config.Key == "" || len(config.RawToken) == 0 {
			zeroRelayConfig(&config)
			setupErrors = append(setupErrors, fmt.Errorf("relay configuration is missing IP, key or token"))
			continue
		}

		identity := relayConnectionID(config.IP, config.Port, config.AuthTokenID)
		connection := &pionRelayConnection{
			state:  relayStateConnecting,
			id:     identity,
			info:   config,
			stopCh: make(chan struct{}),
		}

		m.mu.Lock()
		if _, exists := m.connections[identity]; exists {
			m.mu.Unlock()
			zeroRelayConfig(&config)
			continue
		}
		m.connections[identity] = connection
		m.mu.Unlock()

		if err := m.connectToRelay(connection); err != nil {
			m.failConnection(connection)
			setupErrors = append(setupErrors, fmt.Errorf("configure relay %s: %w", identity, err))
		}
	}
	return errors.Join(setupErrors...)
}

func (m *PionRelayTransport) connectToRelay(connection *pionRelayConnection) error {
	info := connection.info
	m.log.Info("WhatsApp relay connecting", "id", connection.id, "name", info.Name)

	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return fmt.Errorf("create peer connection: %w", err)
	}
	connection.mu.Lock()
	connection.pc = peerConnection
	connection.mu.Unlock()

	peerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		m.log.Debug("WhatsApp relay ICE state", "id", connection.id, "state", state.String())
		switch state {
		case webrtc.ICEConnectionStateFailed, webrtc.ICEConnectionStateDisconnected:
			m.failConnection(connection)
		case webrtc.ICEConnectionStateClosed:
			m.closeConnection(connection.id)
		}
	})

	ordered := false
	channel, err := peerConnection.CreateDataChannel("wa-web-call", &webrtc.DataChannelInit{Ordered: &ordered})
	if err != nil {
		return fmt.Errorf("create relay data channel: %w", err)
	}
	connection.mu.Lock()
	connection.channel = channel
	connection.mu.Unlock()

	channel.OnOpen(func() {
		if !connection.setOpen() {
			return
		}
		m.sendSTUNRegistration(connection)
		m.startKeepalive(connection)
		m.mu.RLock()
		callback := m.onConnected
		m.mu.RUnlock()
		if callback != nil {
			callback(info.IP, info.Port)
		}
	})
	channel.OnClose(func() { m.closeConnection(connection.id) })
	channel.OnMessage(func(message webrtc.DataChannelMessage) {
		m.mu.RLock()
		callback := m.onReceive
		m.mu.RUnlock()
		if callback != nil {
			callback(append([]byte(nil), message.Data...))
		}
	})

	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		return fmt.Errorf("create relay SDP offer: %w", err)
	}
	if err = peerConnection.SetLocalDescription(offer); err != nil {
		return fmt.Errorf("set relay local description: %w", err)
	}
	connection.mu.Lock()
	connection.localUfrag = extractFirst(relayUfragPattern, offer.SDP)
	connection.mu.Unlock()

	answer := modifySDPForRelay(offer.SDP, info)
	if err = peerConnection.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answer}); err != nil {
		return fmt.Errorf("set relay remote description: %w", err)
	}

	go func() {
		timer := time.NewTimer(relayConnectionTimeout)
		defer timer.Stop()
		select {
		case <-timer.C:
			connection.mu.RLock()
			connecting := connection.state == relayStateConnecting
			connection.mu.RUnlock()
			if connecting {
				m.failConnection(connection)
			}
		case <-connection.stopCh:
		}
	}()
	return nil
}

var (
	relaySetupPattern       = regexp.MustCompile(`a=setup:actpass`)
	relayUfragLinePattern   = regexp.MustCompile(`a=ice-ufrag:[^\r\n]+`)
	relayPasswordPattern    = regexp.MustCompile(`a=ice-pwd:[^\r\n]+`)
	relayFingerprintPattern = regexp.MustCompile(`a=fingerprint:[^\r\n]+`)
	relayMaxMessagePattern  = regexp.MustCompile(`a=max-message-size:[^\r\n]+`)
	relayICEOptionsPattern  = regexp.MustCompile(`a=ice-options:[^\r\n]+\r?\n`)
	relayCandidatePattern   = regexp.MustCompile(`a=candidate:[^\r\n]+\r?\n`)
	relayEndCandidatePattern = regexp.MustCompile(`a=end-of-candidates\r?\n?`)
	relayUfragPattern       = regexp.MustCompile(`a=ice-ufrag:([^\r\n]+)`)
)

func modifySDPForRelay(sdp string, info RelayConfig) string {
	output := relaySetupPattern.ReplaceAllString(sdp, "a=setup:passive")
	iceUfrag := info.AuthToken
	if iceUfrag == "" {
		iceUfrag = info.Token
	}
	output = relayUfragLinePattern.ReplaceAllString(output, "a=ice-ufrag:"+iceUfrag)
	output = relayPasswordPattern.ReplaceAllString(output, "a=ice-pwd:"+info.Key)
	output = relayFingerprintPattern.ReplaceAllString(output, "a=fingerprint:"+core.WADTLSFingerprint)
	output = relayMaxMessagePattern.ReplaceAllString(output, "a=max-message-size:1500")
	output = relayICEOptionsPattern.ReplaceAllString(output, "")
	output = relayCandidatePattern.ReplaceAllString(output, "")
	output = relayEndCandidatePattern.ReplaceAllString(output, "")
	candidate := fmt.Sprintf("a=candidate:2 1 udp 2122262783 %s %d typ host generation 0 network-cost 5", info.IP, info.Port)
	return output + candidate + "\r\na=end-of-candidates\r\n"
}

func extractFirst(pattern *regexp.Regexp, value string) string {
	matches := pattern.FindStringSubmatch(value)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (m *PionRelayTransport) sendSTUNRegistration(connection *pionRelayConnection) {
	connection.mu.RLock()
	info := cloneRelayConfig(connection.info)
	localUfrag := connection.localUfrag
	open := connection.state == relayStateOpen && connection.channel != nil
	connection.mu.RUnlock()
	defer zeroRelayConfig(&info)
	if !open {
		return
	}

	remoteUfrag := info.AuthToken
	if remoteUfrag == "" {
		remoteUfrag = info.Token
	}
	if remoteUfrag == "" {
		return
	}

	m.mu.RLock()
	selfSSRC := m.ssrc
	peerSSRC := m.subscriptionSSRC
	m.mu.RUnlock()
	subscriptionSSRC := peerSSRC
	if subscriptionSSRC == 0 {
		subscriptionSSRC = selfSSRC
	}
	if subscriptionSSRC == 0 {
		return
	}

	subscriptions := BuildSenderSubscriptions(subscriptionSSRC)
	hmacKey := []byte(info.Key)
	sendBinding := func(username, key []byte, controlling, fingerprint bool) {
		message, err := BuildBindingRequestWithSubscriptions(username, key, subscriptions, controlling, fingerprint)
		if err == nil {
			_ = m.sendRaw(connection, message)
		}
	}
	if localUfrag != "" {
		sendBinding([]byte(remoteUfrag+":"+localUfrag), hmacKey, true, true)
	}
	if info.Token != "" && info.Token != remoteUfrag && localUfrag != "" {
		sendBinding([]byte(info.Token+":"+localUfrag), hmacKey, true, true)
	}
	sendBinding(nil, nil, false, false)

	if len(info.RawToken) > 0 {
		var peerSSRCs []uint32
		if peerSSRC != 0 {
			peerSSRCs = []uint32{peerSSRC}
		}
		ssrcList := BuildSSRCSubscriptionList([]uint32{selfSSRC}, peerSSRCs, 0, 0)
		allocation, err := BuildAllocateForRelay(info.RawToken, ssrcList, hmacKey, info.IP, info.Port)
		if err == nil {
			_ = m.sendRaw(connection, allocation)
		}
	}

	for _, delay := range []time.Duration{50, 150, 500, 3000} {
		delay := delay * time.Millisecond
		go func() {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-timer.C:
				if connection.isOpen() {
					m.sendSTUNRegistrationOnce(connection)
				}
			case <-connection.stopCh:
			}
		}()
	}
}

func (m *PionRelayTransport) sendSTUNRegistrationOnce(connection *pionRelayConnection) {
	connection.mu.RLock()
	info := cloneRelayConfig(connection.info)
	localUfrag := connection.localUfrag
	connection.mu.RUnlock()
	defer zeroRelayConfig(&info)

	m.mu.RLock()
	selfSSRC := m.ssrc
	peerSSRC := m.subscriptionSSRC
	m.mu.RUnlock()
	subscriptionSSRC := peerSSRC
	if subscriptionSSRC == 0 {
		subscriptionSSRC = selfSSRC
	}
	if subscriptionSSRC == 0 {
		return
	}
	remoteUfrag := info.AuthToken
	if remoteUfrag == "" {
		remoteUfrag = info.Token
	}
	if remoteUfrag == "" {
		return
	}
	message, err := BuildBindingRequestWithSubscriptions([]byte(remoteUfrag+":"+localUfrag), []byte(info.Key), BuildSenderSubscriptions(subscriptionSSRC), true, true)
	if err == nil {
		_ = m.sendRaw(connection, message)
	}
}

func (m *PionRelayTransport) startKeepalive(connection *pionRelayConnection) {
	ping, err := BuildWhatsAppPing()
	if err == nil {
		_ = m.sendRaw(connection, ping)
	}
	ticker := time.NewTicker(relayKeepaliveInterval)
	connection.mu.Lock()
	connection.keepalive = ticker
	connection.mu.Unlock()
	go func() {
		for {
			select {
			case <-ticker.C:
				if !connection.isOpen() {
					return
				}
				if ping, pingErr := BuildWhatsAppPing(); pingErr == nil {
					_ = m.sendRaw(connection, ping)
				}
			case <-connection.stopCh:
				return
			}
		}
	}()
}

func (m *PionRelayTransport) sendRaw(connection *pionRelayConnection, data []byte) error {
	connection.mu.RLock()
	channel := connection.channel
	open := connection.state == relayStateOpen && channel != nil
	connection.mu.RUnlock()
	if !open {
		return fmt.Errorf("relay %s is not open", connection.id)
	}
	if err := channel.Send(data); err != nil {
		return fmt.Errorf("send relay data: %w", err)
	}
	return nil
}

func (m *PionRelayTransport) Broadcast(data []byte) error {
	var sendErrors []error
	for _, connection := range m.connectionSnapshot() {
		if !connection.isOpen() {
			continue
		}
		if err := m.sendRaw(connection, data); err != nil {
			sendErrors = append(sendErrors, err)
		}
	}
	return errors.Join(sendErrors...)
}

func (m *PionRelayTransport) HasConnection() bool {
	return m.ConnectedCount() > 0
}

func (m *PionRelayTransport) ConnectedCount() int {
	count := 0
	for _, connection := range m.connectionSnapshot() {
		if connection.isOpen() {
			count++
		}
	}
	return count
}

func (m *PionRelayTransport) connectionSnapshot() []*pionRelayConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	connections := make([]*pionRelayConnection, 0, len(m.connections))
	for _, connection := range m.connections {
		connections = append(connections, connection)
	}
	return connections
}

func (m *PionRelayTransport) failConnection(connection *pionRelayConnection) {
	if !connection.setTerminal(relayStateFailed) {
		return
	}
	m.removeConnection(connection)
	m.teardown(connection)
}

func (m *PionRelayTransport) closeConnection(identity string) {
	m.mu.RLock()
	connection := m.connections[identity]
	m.mu.RUnlock()
	if connection == nil || !connection.setTerminal(relayStateClosed) {
		return
	}
	m.removeConnection(connection)
	m.teardown(connection)
}

func (m *PionRelayTransport) removeConnection(connection *pionRelayConnection) {
	m.mu.Lock()
	if current := m.connections[connection.id]; current == connection {
		delete(m.connections, connection.id)
	}
	m.mu.Unlock()
}

func (m *PionRelayTransport) teardown(connection *pionRelayConnection) {
	connection.stopOnce.Do(func() { close(connection.stopCh) })
	connection.mu.Lock()
	ticker := connection.keepalive
	channel := connection.channel
	peerConnection := connection.pc
	connection.keepalive = nil
	connection.channel = nil
	connection.pc = nil
	zeroRelayConfig(&connection.info)
	connection.mu.Unlock()
	if ticker != nil {
		ticker.Stop()
	}
	if channel != nil {
		_ = channel.Close()
	}
	if peerConnection != nil {
		_ = peerConnection.Close()
	}
}

func (m *PionRelayTransport) Cleanup() {
	m.mu.Lock()
	connections := make([]*pionRelayConnection, 0, len(m.connections))
	for _, connection := range m.connections {
		connections = append(connections, connection)
	}
	m.connections = make(map[string]*pionRelayConnection)
	m.ssrc = 0
	m.subscriptionSSRC = 0
	m.mu.Unlock()
	for _, connection := range connections {
		connection.setTerminal(relayStateClosed)
		m.teardown(connection)
	}
}

func cloneRelayConfig(config RelayConfig) RelayConfig {
	clone := config
	clone.RawToken = append([]byte(nil), config.RawToken...)
	clone.RawAuthToken = append([]byte(nil), config.RawAuthToken...)
	return clone
}

func zeroRelayConfig(config *RelayConfig) {
	if config == nil {
		return
	}
	zeroBytes(config.RawToken)
	zeroBytes(config.RawAuthToken)
	config.RawToken = nil
	config.RawAuthToken = nil
	config.Token = ""
	config.AuthToken = ""
	config.Key = ""
	config.Name = ""
	config.AuthTokenID = ""
}

var _ RelayTransport = (*PionRelayTransport)(nil)
