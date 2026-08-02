package media

import (
	"errors"
	"log/slog"
	"testing"

	call_state "github.com/evolution-foundation/evolution-go/pkg/call/voip/call"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	call_transport "github.com/evolution-foundation/evolution-go/pkg/call/voip/transport"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

type fakeNegotiationSource struct {
	state       *call_state.Info
	relayData   *core.RelayData
	connected   int
	captureHits int
}

func (f *fakeNegotiationSource) RelayData(_, _ string) (*core.RelayData, bool) {
	if f.relayData == nil {
		return nil, false
	}
	return core.CloneRelayData(f.relayData), true
}
func (f *fakeNegotiationSource) State(_, _ string) (*call_state.Info, bool) {
	if f.state == nil {
		return nil, false
	}
	return f.state.Clone(), true
}
func (f *fakeNegotiationSource) CaptureRelayNode(_, _ string, _ *waBinary.Node) {
	f.captureHits++
}
func (f *fakeNegotiationSource) EnsureRemoteAccepted(_, _ string) error {
	if f.state.StateData.State == core.CallStateRinging {
		return f.state.Apply(call_state.Transition{Type: call_state.TransitionRemoteAccepted})
	}
	return nil
}
func (f *fakeNegotiationSource) MarkMediaConnected(_, _ string) error {
	if err := f.state.Apply(call_state.Transition{Type: call_state.TransitionMediaConnected}); err != nil {
		return err
	}
	f.connected++
	return nil
}

type fakeRelayTransport struct {
	ssrc             uint32
	subscriptionSSRC uint32
	configs          []call_transport.RelayConfig
	onConnected      func(string, int)
	onReceive        func([]byte)
	configureErr     error
	cleaned          bool
}

func (f *fakeRelayTransport) SetSSRC(ssrc uint32)                       { f.ssrc = ssrc }
func (f *fakeRelayTransport) SetSubscriptionSSRC(ssrc uint32)           { f.subscriptionSSRC = ssrc }
func (f *fakeRelayTransport) SetOnConnected(callback func(string, int)) { f.onConnected = callback }
func (f *fakeRelayTransport) SetOnReceive(callback func([]byte))        { f.onReceive = callback }
func (f *fakeRelayTransport) ResendSubscriptions()                      {}
func (f *fakeRelayTransport) ConfigureRelays(configs []call_transport.RelayConfig) error {
	f.configs = append([]call_transport.RelayConfig(nil), configs...)
	return f.configureErr
}
func (f *fakeRelayTransport) Broadcast([]byte) error { return nil }
func (f *fakeRelayTransport) HasConnection() bool    { return false }
func (f *fakeRelayTransport) ConnectedCount() int    { return 0 }
func (f *fakeRelayTransport) Cleanup()               { f.cleaned = true }

func TestRelaySessionConfiguresTransportAndMarksMediaConnected(t *testing.T) {
	state := call_state.NewOutgoing(
		"call-1",
		"5511888888888:2@s.whatsapp.net",
		"5511999999999:1@s.whatsapp.net",
		core.CallMediaTypeAudio,
	)
	if err := state.Apply(call_state.Transition{Type: call_state.TransitionOfferSent}); err != nil {
		t.Fatal(err)
	}
	if err := state.Apply(call_state.Transition{Type: call_state.TransitionRemoteAccepted}); err != nil {
		t.Fatal(err)
	}

	source := &fakeNegotiationSource{
		state: state,
		relayData: &core.RelayData{
			Endpoints: []core.RelayEndpoint{{
				IP:       "203.0.113.10",
				Port:     3480,
				Protocol: 0,
				Key:      "relay-password",
				RawToken: []byte{1, 2, 3},
			}},
			ParticipantJIDs: []string{
				"5511999999999:7@s.whatsapp.net",
				"5511888888888:9@s.whatsapp.net",
			},
		},
	}
	fakeTransport := &fakeRelayTransport{}
	session := &relaySession{
		instanceID:  "instance-1",
		source:      source,
		factory:     func(*slog.Logger) call_transport.RelayTransport { return fakeTransport },
		log:         slog.Default(),
		transports:  make(map[string]call_transport.RelayTransport),
		configuring: make(map[string]bool),
		ownJID: func() types.JID {
			return types.NewJID("5511999999999", types.DefaultUserServer)
		},
	}
	connectedCallback := 0
	session.onConnected = func(_, _ string) { connectedCallback++ }

	if err := session.start("call-1"); err != nil {
		t.Fatal(err)
	}
	if fakeTransport.ssrc == 0 || fakeTransport.subscriptionSSRC == 0 {
		t.Fatalf("SSRCs were not configured: self=%d peer=%d", fakeTransport.ssrc, fakeTransport.subscriptionSSRC)
	}
	if len(fakeTransport.configs) != 1 || fakeTransport.configs[0].IP != "203.0.113.10" {
		t.Fatalf("unexpected relay configs: %#v", fakeTransport.configs)
	}
	if fakeTransport.onConnected == nil {
		t.Fatal("connected callback was not installed")
	}
	fakeTransport.onConnected("203.0.113.10", 3480)
	if source.state.StateData.State != core.CallStateActive || source.connected != 1 || connectedCallback != 1 {
		t.Fatalf("media connection was not propagated: state=%s source=%d callback=%d", source.state.StateData.State, source.connected, connectedCallback)
	}
}

func TestRelaySessionTreatsDisabledTransportAsNoop(t *testing.T) {
	state := call_state.NewIncoming("call-2", "5511888888888@s.whatsapp.net", "5511888888888@s.whatsapp.net", core.CallMediaTypeAudio)
	if err := state.Apply(call_state.Transition{Type: call_state.TransitionLocalAccepted}); err != nil {
		t.Fatal(err)
	}
	source := &fakeNegotiationSource{
		state: state,
		relayData: &core.RelayData{Endpoints: []core.RelayEndpoint{{
			IP: "203.0.113.11", Protocol: 0, Key: "key", RawToken: []byte{4, 5, 6},
		}}},
	}
	fakeTransport := &fakeRelayTransport{configureErr: call_transport.ErrSCTPUnavailable}
	session := &relaySession{
		instanceID:  "instance-1",
		source:      source,
		factory:     func(*slog.Logger) call_transport.RelayTransport { return fakeTransport },
		log:         slog.Default(),
		transports:  make(map[string]call_transport.RelayTransport),
		configuring: make(map[string]bool),
		ownJID: func() types.JID {
			return types.NewJID("5511999999999", types.DefaultUserServer)
		},
	}
	if err := session.start("call-2"); err != nil {
		t.Fatal(err)
	}
	if !fakeTransport.cleaned {
		t.Fatal("disabled transport was not cleaned up")
	}
	if _, exists := session.transports["call-2"]; exists {
		t.Fatal("disabled transport remained registered")
	}
}

func TestRelaySessionReturnsRealConfigurationErrors(t *testing.T) {
	state := call_state.NewIncoming("call-3", "5511888888888@s.whatsapp.net", "5511888888888@s.whatsapp.net", core.CallMediaTypeAudio)
	_ = state.Apply(call_state.Transition{Type: call_state.TransitionLocalAccepted})
	source := &fakeNegotiationSource{
		state: state,
		relayData: &core.RelayData{Endpoints: []core.RelayEndpoint{{
			IP: "203.0.113.12", Protocol: 0, Key: "key", RawToken: []byte{7},
		}}},
	}
	expected := errors.New("setup failed")
	session := &relaySession{
		instanceID: "instance-1", source: source,
		factory: func(*slog.Logger) call_transport.RelayTransport { return &fakeRelayTransport{configureErr: expected} },
		log:     slog.Default(), transports: make(map[string]call_transport.RelayTransport), configuring: make(map[string]bool),
		ownJID: func() types.JID { return types.NewJID("5511999999999", types.DefaultUserServer) },
	}
	if err := session.start("call-3"); !errors.Is(err, expected) {
		t.Fatalf("expected setup error, got %v", err)
	}
}

func TestSelectDeviceJIDsPrefersParticipantDevices(t *testing.T) {
	self, peer := selectDeviceJIDs(
		[]string{"5511999999999:7@s.whatsapp.net", "5511888888888:9@s.whatsapp.net"},
		types.NewJID("5511999999999", types.DefaultUserServer),
		types.NewJID("5511888888888", types.HiddenUserServer),
	)
	if self != "5511999999999:7@s.whatsapp.net" || peer != "5511888888888:9@s.whatsapp.net" {
		t.Fatalf("unexpected participant selection: self=%s peer=%s", self, peer)
	}
}
