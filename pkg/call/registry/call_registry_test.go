package call_registry

import (
	"testing"

	"github.com/purpshell/meowcaller"
)

func TestStoreAndGet(t *testing.T) {
	r := NewCallRegistry()
	call := &meowcaller.Call{}

	r.Store("instance-a", call)

	got, ok := r.Get("instance-a", callIDOf(call))
	if !ok {
		t.Fatal("expected call to be found")
	}
	if got != call {
		t.Fatal("expected the same call pointer back")
	}
}

func TestGetWrongInstanceFails(t *testing.T) {
	r := NewCallRegistry()
	call := &meowcaller.Call{}
	r.Store("instance-a", call)

	_, ok := r.Get("instance-b", callIDOf(call))
	if ok {
		t.Fatal("expected lookup from a different instance to fail")
	}
}

func TestDeleteRemovesEntry(t *testing.T) {
	r := NewCallRegistry()
	call := &meowcaller.Call{}
	r.Store("instance-a", call)
	r.Delete(callIDOf(call))

	_, ok := r.Get("instance-a", callIDOf(call))
	if ok {
		t.Fatal("expected entry to be gone after Delete")
	}
}

// callIDOf mirrors what CallRegistry.Store keys entries by: meowcaller.Call.ID().
// A zero-value *meowcaller.Call has an empty string ID, which is a perfectly valid
// (if degenerate) key for exercising Store/Get/Delete without a live call.
func callIDOf(call *meowcaller.Call) string {
	return call.ID()
}
