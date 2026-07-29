package whatsmeow_service

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestReserveRuntimeAllowsOneRuntimePerInstance(t *testing.T) {
	service := &whatsmeowService{runtimeToken: make(map[string]uint64)}

	var accepted atomic.Int32
	var wg sync.WaitGroup
	for attempt := 0; attempt < 100; attempt++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := service.reserveRuntime("tenant-7"); ok {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := accepted.Load(); got != 1 {
		t.Fatalf("expected exactly one runtime per instance, got %d", got)
	}
}

func TestReserveRuntimeIsPerInstance(t *testing.T) {
	service := &whatsmeowService{runtimeToken: make(map[string]uint64)}

	if _, ok := service.reserveRuntime("tenant-a"); !ok {
		t.Fatal("first instance must be able to start")
	}
	if _, ok := service.reserveRuntime("tenant-b"); !ok {
		t.Fatal("a different instance must not be blocked by another one's runtime")
	}
}

func TestReleaseRuntimeAllowsRestart(t *testing.T) {
	service := &whatsmeowService{runtimeToken: make(map[string]uint64)}

	token, ok := service.reserveRuntime("tenant-7")
	if !ok {
		t.Fatal("first reservation must succeed")
	}
	service.releaseRuntime("tenant-7", token)

	if _, ok := service.reserveRuntime("tenant-7"); !ok {
		t.Fatal("a new runtime must start once the previous one is released")
	}
}

// A late cleanup from a dead runtime must not free the reservation of the one
// that took its place — that is how a second concurrent client gets in.
func TestReleaseRuntimeIgnoresStaleToken(t *testing.T) {
	service := &whatsmeowService{runtimeToken: make(map[string]uint64)}

	stale, _ := service.reserveRuntime("tenant-7")
	service.releaseRuntime("tenant-7", stale)

	current, ok := service.reserveRuntime("tenant-7")
	if !ok {
		t.Fatal("replacement runtime must be able to reserve")
	}

	service.releaseRuntime("tenant-7", stale)

	if _, ok := service.reserveRuntime("tenant-7"); ok {
		t.Fatal("stale release freed the live runtime's reservation")
	}

	service.releaseRuntime("tenant-7", current)
	if _, ok := service.reserveRuntime("tenant-7"); !ok {
		t.Fatal("current token must still release the reservation")
	}
}
