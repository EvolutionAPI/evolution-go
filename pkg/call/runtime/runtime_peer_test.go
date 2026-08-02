package call_runtime

import (
	"testing"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func TestTransitionPreservesKnownPhoneNumber(t *testing.T) {
	runtime := New("instance", nil)
	runtime.Transition(
		"call",
		"556298612492@s.whatsapp.net",
		DirectionOutgoing,
		StateRinging,
		nil,
		"",
	)
	runtime.Transition(
		"call",
		"75741748277476:3@lid",
		DirectionOutgoing,
		StateConnecting,
		nil,
		"",
	)
	call, _ := runtime.Call("call")
	if call.Peer != "556298612492@s.whatsapp.net" {
		t.Fatalf("phone number was replaced by LID: %s", call.Peer)
	}
}

func TestIncomingCallRejectAfterAcceptMeansPeerEnded(t *testing.T) {
	runtime := New("instance", nil)
	runtime.Transition(
		"call",
		"556298612492@s.whatsapp.net",
		DirectionIncoming,
		StateConnecting,
		nil,
		"",
	)
	runtime.handleEvent(&events.CallReject{BasicCallMeta: types.BasicCallMeta{
		CallID:      "call",
		CallCreator: types.NewJID("66155398054068", types.HiddenUserServer),
	}})
	call, _ := runtime.Call("call")
	if call.State != StateEnded || call.EndReason != "peer_ended" {
		t.Fatalf("unexpected reject classification: state=%s reason=%s", call.State, call.EndReason)
	}
}
