package call

import (
	"errors"
	"testing"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
)

func TestOutgoingCallLifecycle(t *testing.T) {
	call := NewOutgoing("call-1", "peer@s.whatsapp.net", "self@s.whatsapp.net", core.CallMediaTypeAudio)
	steps := []Transition{
		{Type: TransitionOfferSent},
		{Type: TransitionRemoteAccepted},
		{Type: TransitionMediaConnected},
		{Type: TransitionAudioMuteChanged, Muted: true},
		{Type: TransitionHold},
		{Type: TransitionResume},
		{Type: TransitionTerminated, Reason: core.EndCallReasonUserEnded},
	}
	for _, step := range steps {
		if err := call.Apply(step); err != nil {
			t.Fatalf("Apply(%s) error = %v", step.Type, err)
		}
	}
	if !call.IsEnded() || call.StateData.EndReason != core.EndCallReasonUserEnded {
		t.Fatalf("unexpected final state: %+v", call.StateData)
	}
	if !call.StateData.AudioMuted {
		t.Fatal("audio mute state was not preserved")
	}
}

func TestIncomingAcceptLifecycle(t *testing.T) {
	call := NewIncoming("call-2", "peer@s.whatsapp.net", "peer@s.whatsapp.net", core.CallMediaTypeVideo)
	if !call.CanAccept() {
		t.Fatal("incoming ringing call should be acceptable")
	}
	if err := call.Apply(Transition{Type: TransitionLocalAccepted}); err != nil {
		t.Fatalf("local accept error = %v", err)
	}
	if err := call.Apply(Transition{Type: TransitionMediaConnected}); err != nil {
		t.Fatalf("media connected error = %v", err)
	}
	if call.StateData.State != core.CallStateActive || call.StateData.VideoOff {
		t.Fatalf("unexpected active video state: %+v", call.StateData)
	}
}

func TestInvalidTransitionDoesNotMutateState(t *testing.T) {
	call := NewOutgoing("call-3", "peer@s.whatsapp.net", "self@s.whatsapp.net", core.CallMediaTypeAudio)
	err := call.Apply(Transition{Type: TransitionMediaConnected})
	var invalidTransition *InvalidTransition
	if !errors.As(err, &invalidTransition) {
		t.Fatalf("expected InvalidTransition, got %v", err)
	}
	if call.StateData.State != core.CallStateInitiating {
		t.Fatalf("invalid transition mutated state to %s", call.StateData.State)
	}
}

func TestCloneIsIndependent(t *testing.T) {
	call := NewOutgoing("call-4", "peer@s.whatsapp.net", "self@s.whatsapp.net", core.CallMediaTypeAudio)
	if err := call.Apply(Transition{Type: TransitionOfferSent}); err != nil {
		t.Fatal(err)
	}
	if err := call.Apply(Transition{Type: TransitionRemoteAccepted}); err != nil {
		t.Fatal(err)
	}
	clone := call.Clone()
	clone.StateData.State = core.CallStateEnded
	if clone.StateData.AcceptedAt != nil {
		clone.StateData.AcceptedAt = nil
	}
	if call.StateData.State != core.CallStateConnecting || call.StateData.AcceptedAt == nil {
		t.Fatal("mutating clone changed original")
	}
}
