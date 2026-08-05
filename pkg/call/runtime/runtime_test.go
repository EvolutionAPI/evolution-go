package call_runtime

import (
	"testing"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func TestRegistryAttachReusesRuntime(t *testing.T) {
	registry := NewRegistry()

	first := registry.Attach("instance-1", nil)
	second := registry.Attach("instance-1", nil)

	if first != second {
		t.Fatal("expected registry to reuse the runtime for the same instance")
	}
	if first.InstanceID() != "instance-1" {
		t.Fatalf("unexpected instance id: %s", first.InstanceID())
	}
}

func TestRuntimeUpsertPreservesCreatedAt(t *testing.T) {
	runtime := New("instance-1", nil)
	createdAt := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)

	runtime.UpsertCall(Call{
		ID:        "call-1",
		Peer:      "5511999999999@s.whatsapp.net",
		Direction: DirectionOutgoing,
		State:     StateRinging,
		CreatedAt: createdAt,
	})
	runtime.UpsertCall(Call{
		ID:        "call-1",
		Peer:      "5511999999999@s.whatsapp.net",
		Direction: DirectionOutgoing,
		State:     StateActive,
	})

	call, ok := runtime.Call("call-1")
	if !ok {
		t.Fatal("expected call to exist")
	}
	if !call.CreatedAt.Equal(createdAt) {
		t.Fatalf("createdAt changed: got %s want %s", call.CreatedAt, createdAt)
	}
	if call.State != StateActive {
		t.Fatalf("unexpected state: %s", call.State)
	}
	if call.UpdatedAt.IsZero() {
		t.Fatal("expected updatedAt to be populated")
	}
}

func TestRuntimeTransitionPreservesCapturedMetadata(t *testing.T) {
	runtime := New("instance-1", nil)
	video := true

	runtime.Transition(
		"call-1",
		"5511999999999@s.whatsapp.net",
		DirectionIncoming,
		StateRinging,
		&video,
		"",
	)
	runtime.Transition("call-1", "", DirectionOutgoing, StateActive, nil, "")

	call, ok := runtime.Call("call-1")
	if !ok {
		t.Fatal("expected call to exist")
	}
	if call.Peer != "5511999999999@s.whatsapp.net" {
		t.Fatalf("peer was erased: %s", call.Peer)
	}
	if call.Direction != DirectionIncoming {
		t.Fatalf("direction was overwritten: %s", call.Direction)
	}
	if !call.Video {
		t.Fatal("video metadata was erased")
	}
	if call.State != StateActive {
		t.Fatalf("unexpected state: %s", call.State)
	}
}

func TestRuntimeNotifiesPublicCallChanges(t *testing.T) {
	runtime := New("instance-1", nil)
	updates := make([]Call, 0, 2)
	runtime.SetOnChange(func(call Call) {
		updates = append(updates, call)
	})

	runtime.Transition("call-1", "5511999999999@s.whatsapp.net", DirectionIncoming, StateRinging, nil, "")
	runtime.Transition("call-1", "", DirectionIncoming, StateConnecting, nil, "")

	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}
	if updates[0].State != StateRinging || updates[1].State != StateConnecting {
		t.Fatalf("unexpected state updates: %+v", updates)
	}
	if updates[1].Peer != "5511999999999@s.whatsapp.net" {
		t.Fatalf("expected retained peer in callback, got %s", updates[1].Peer)
	}
}

func TestRuntimeClearsFailedAnswerAttemptWhileRinging(t *testing.T) {
	runtime := New("instance-1", nil)
	runtime.Transition("call-1", "5511999999999@s.whatsapp.net", DirectionIncoming, StateRinging, nil, "")
	runtime.MarkAnswered("call-1", "Manager V2")
	runtime.ClearAnswerMetadata("call-1")

	call, _ := runtime.Call("call-1")
	if call.AnsweredBy != "" || call.AnsweredAt != nil || call.State != StateRinging {
		t.Fatalf("failed answer attempt was not rolled back: %+v", call)
	}
}

func TestRuntimeTracksWhatsmeowCallLifecycle(t *testing.T) {
	runtime := New("instance-1", nil)
	creator := types.NewJID("5511999999999", types.DefaultUserServer)

	runtime.handleEvent(&events.CallOffer{
		BasicCallMeta: types.BasicCallMeta{
			From:        creator,
			CallCreator: creator,
			CallID:      "call-1",
		},
		Data: &waBinary.Node{
			Tag: "offer",
			Content: []waBinary.Node{
				{Tag: "video"},
			},
		},
	})

	call, ok := runtime.Call("call-1")
	if !ok {
		t.Fatal("expected call offer to create a runtime call")
	}
	if call.State != StateRinging || call.Direction != DirectionIncoming {
		t.Fatalf("unexpected offered call: %+v", call)
	}
	if !call.Video {
		t.Fatal("expected video call metadata")
	}

	// A Manager-side answer records the local actor before the companion
	// receives the CallAccept event. That keeps it distinct from a phone or
	// another linked device accepting the same ringing call.
	runtime.MarkAnswered("call-1", "Manager V2")
	runtime.handleEvent(&events.CallAccept{
		BasicCallMeta: types.BasicCallMeta{
			From:        creator,
			CallCreator: creator,
			CallID:      "call-1",
		},
	})

	call, _ = runtime.Call("call-1")
	if call.State != StateConnecting {
		t.Fatalf("expected connecting state before media, got %s", call.State)
	}
	if call.Direction != DirectionIncoming {
		t.Fatalf("incoming direction must be preserved, got %s", call.Direction)
	}

	runtime.handleEvent(&events.CallTerminate{
		BasicCallMeta: types.BasicCallMeta{
			From:        creator,
			CallCreator: creator,
			CallID:      "call-1",
		},
		Reason: "peer_hangup",
	})

	call, _ = runtime.Call("call-1")
	if call.State != StateEnded {
		t.Fatalf("expected ended state, got %s", call.State)
	}
	if call.EndReason != "peer_hangup" {
		t.Fatalf("unexpected end reason: %s", call.EndReason)
	}
}

func TestRuntimeMarksIncomingCallAnsweredElsewhere(t *testing.T) {
	runtime := New("instance-1", nil)
	creator := types.NewJID("5511999999999", types.DefaultUserServer)
	runtime.Transition("call-1", creator.String(), DirectionIncoming, StateRinging, nil, "")

	runtime.handleEvent(&events.CallAccept{
		BasicCallMeta: types.BasicCallMeta{
			From:        creator,
			CallCreator: creator,
			CallID:      "call-1",
		},
	})

	call, ok := runtime.Call("call-1")
	if !ok {
		t.Fatal("expected incoming call to be tracked")
	}
	if call.State != StateEnded || call.EndReason != "answered_elsewhere" {
		t.Fatalf("unexpected other-device outcome: %+v", call)
	}
	if call.AnsweredBy != "Outro dispositivo" || call.AnsweredAt == nil {
		t.Fatalf("answer metadata was not captured: %+v", call)
	}
}

func TestRuntimeDoesNotReviveTerminalCallFromDelayedSignaling(t *testing.T) {
	runtime := New("instance-1", nil)
	runtime.Transition("call-1", "5511999999999@s.whatsapp.net", DirectionIncoming, StateEnded, nil, "answered_elsewhere")
	runtime.Transition("call-1", "", DirectionIncoming, StateConnecting, nil, "")

	call, ok := runtime.Call("call-1")
	if !ok {
		t.Fatal("expected call to exist")
	}
	if call.State != StateEnded || call.EndReason != "answered_elsewhere" {
		t.Fatalf("terminal call was revived: %+v", call)
	}
}

func TestRuntimeKeepsOtherDeviceOutcomeAfterTerminate(t *testing.T) {
	runtime := New("instance-1", nil)
	runtime.Transition("call-1", "5511999999999@s.whatsapp.net", DirectionIncoming, StateEnded, nil, "answered_elsewhere")
	runtime.Transition("call-1", "", DirectionIncoming, StateEnded, nil, "peer_hangup")

	call, _ := runtime.Call("call-1")
	if call.EndReason != "answered_elsewhere" {
		t.Fatalf("external answer outcome was overwritten: %+v", call)
	}
}

func TestRuntimeMarksOpenCallsFailedOnDisconnect(t *testing.T) {
	runtime := New("instance-1", nil)
	runtime.Transition("call-1", "peer", DirectionOutgoing, StateConnecting, nil, "")
	runtime.Transition("call-2", "peer", DirectionIncoming, StateEnded, nil, "completed")

	runtime.handleEvent(&events.Disconnected{})

	openCall, _ := runtime.Call("call-1")
	if openCall.State != StateFailed {
		t.Fatalf("expected open call to fail, got %s", openCall.State)
	}
	if openCall.Error == "" {
		t.Fatal("expected disconnect error")
	}

	endedCall, _ := runtime.Call("call-2")
	if endedCall.State != StateEnded {
		t.Fatalf("ended call must not change, got %s", endedCall.State)
	}
}

func TestRuntimeSnapshotIsSortedAndIndependent(t *testing.T) {
	runtime := New("instance-1", nil)
	later := time.Date(2026, time.July, 31, 13, 0, 0, 0, time.UTC)
	earlier := later.Add(-time.Hour)

	runtime.UpsertCall(Call{ID: "later", CreatedAt: later, State: StateRinging})
	runtime.UpsertCall(Call{ID: "earlier", CreatedAt: earlier, State: StateEnded})

	snapshot := runtime.Snapshot()
	if snapshot.Connected {
		t.Fatal("nil client must be reported as disconnected")
	}
	if len(snapshot.Calls) != 2 {
		t.Fatalf("unexpected call count: %d", len(snapshot.Calls))
	}
	if snapshot.Calls[0].ID != "earlier" || snapshot.Calls[1].ID != "later" {
		t.Fatalf("calls are not sorted by creation time: %+v", snapshot.Calls)
	}

	snapshot.Calls[0].State = StateFailed
	stored, _ := runtime.Call("earlier")
	if stored.State != StateEnded {
		t.Fatal("snapshot mutation changed runtime state")
	}
}

func TestRegistryRemove(t *testing.T) {
	registry := NewRegistry()
	registry.Attach("instance-1", nil)
	registry.Remove("instance-1")

	if _, ok := registry.Get("instance-1"); ok {
		t.Fatal("expected runtime to be removed")
	}
}
