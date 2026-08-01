package call_runtime

import (
	"testing"
	"time"
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
