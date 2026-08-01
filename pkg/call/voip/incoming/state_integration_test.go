package incoming

import (
	"testing"

	call_state "github.com/evolution-foundation/evolution-go/pkg/call/voip/call"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	"go.mau.fi/whatsmeow/types"
)

func TestOutgoingMaterialStartsRingingAndAcceptsRemote(t *testing.T) {
	s := newTestSession()
	peer := types.NewJID("5511999999999", types.HiddenUserServer)
	creator := types.NewJID("5511000000000", types.DefaultUserServer)
	key := make([]byte, 32)
	key[0] = 9

	s.storeOutgoing("call-out", key, peer, creator, false, &core.RelayData{
		Endpoints: []core.RelayEndpoint{{IP: "10.0.0.1"}},
	})
	state, ok := s.state("call-out")
	if !ok || state.StateData.State != core.CallStateRinging {
		t.Fatalf("unexpected initial outgoing state: %+v", state)
	}
	if err := s.transition("call-out", call_state.Transition{Type: call_state.TransitionRemoteAccepted}); err != nil {
		t.Fatalf("remote accept transition failed: %v", err)
	}
	state, _ = s.state("call-out")
	if state.StateData.State != core.CallStateConnecting || state.StateData.AcceptedAt == nil {
		t.Fatalf("unexpected accepted state: %+v", state.StateData)
	}
}

func TestIncomingMaterialRejectsRemoteAcceptTransition(t *testing.T) {
	s := newTestSession()
	state := call_state.NewIncoming(
		"call-in",
		"peer@s.whatsapp.net",
		"peer@s.whatsapp.net",
		core.CallMediaTypeAudio,
	)
	s.store("call-in", &callMaterial{state: state})

	if err := s.transition("call-in", call_state.Transition{Type: call_state.TransitionRemoteAccepted}); err == nil {
		t.Fatal("incoming call unexpectedly accepted a remote-accepted transition")
	}
	stored, _ := s.state("call-in")
	if stored.StateData.State != core.CallStateIncomingRinging {
		t.Fatalf("invalid transition mutated state: %s", stored.StateData.State)
	}
}

func TestStateCopyIsIndependent(t *testing.T) {
	s := newTestSession()
	state := call_state.NewIncoming(
		"call-in",
		"peer@s.whatsapp.net",
		"peer@s.whatsapp.net",
		core.CallMediaTypeAudio,
	)
	s.store("call-in", &callMaterial{state: state})

	copyValue, ok := s.state("call-in")
	if !ok {
		t.Fatal("expected private state")
	}
	copyValue.StateData.State = core.CallStateEnded
	stored, _ := s.state("call-in")
	if stored.StateData.State != core.CallStateIncomingRinging {
		t.Fatal("mutating state copy changed private state")
	}
}
