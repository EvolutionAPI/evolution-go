package whatsmeow_service

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCanAutoReconnectRequiresPairedJID(t *testing.T) {
	for _, jid := range []string{"", " ", "	"} {
		if canAutoReconnect(jid) {
			t.Fatalf("unpaired jid %q must not auto-reconnect", jid)
		}
	}
	if !canAutoReconnect("paired-device@s.whatsapp.net") {
		t.Fatal("paired jid must auto-reconnect")
	}
}

func TestReserveReconnectDeduplicatesConcurrentAttempts(t *testing.T) {
	service := &whatsmeowService{reconnectState: make(map[string]*instanceReconnectState)}
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for attempt := 0; attempt < 100; attempt++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := service.reserveReconnect("tenant-2"); ok {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := accepted.Load(); got != 1 {
		t.Fatalf("expected exactly one concurrent reconnect, got %d", got)
	}

	service.finishReconnect("tenant-2", false)
	if _, ok := service.reserveReconnect("tenant-2"); !ok {
		t.Fatal("a new retry must be accepted after the prior attempt finishes")
	}
}

func TestReconnectDelayIsBoundedAndJitteredAcrossTwentyTenants(t *testing.T) {
	seen := map[time.Duration]struct{}{}
	for tenant := 1; tenant <= 20; tenant++ {
		delay := reconnectDelay(fmt.Sprintf("tenant-%d", tenant), 0)
		if delay < 2*time.Second || delay >= 2400*time.Millisecond {
			t.Fatalf("tenant %d first delay out of bounds: %s", tenant, delay)
		}
		seen[delay] = struct{}{}
	}
	if len(seen) < 5 {
		t.Fatalf("expected retry jitter across tenants, got only %d distinct delays", len(seen))
	}
	if delay := reconnectDelay("tenant-20", 1000); delay < 5*time.Minute || delay >= 6*time.Minute {
		t.Fatalf("capped retry delay out of bounds: %s", delay)
	}
}
