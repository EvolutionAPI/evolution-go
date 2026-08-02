package call_runtime

import (
	"testing"
	"time"
)

func installWatchdogTimeouts(t *testing.T, ringing, connecting time.Duration) {
	t.Helper()
	previousRinging := ringingWatchdogTimeout
	previousConnecting := connectingWatchdogTimeout
	ringingWatchdogTimeout = ringing
	connectingWatchdogTimeout = connecting
	t.Cleanup(func() {
		ringingWatchdogTimeout = previousRinging
		connectingWatchdogTimeout = previousConnecting
	})
}

func waitForTimeout(t *testing.T, timedOut <-chan string) string {
	t.Helper()
	select {
	case callID := <-timedOut:
		return callID
	case <-time.After(2 * time.Second):
		t.Fatal("watchdog callback did not run")
		return ""
	}
}

func TestRuntimeWatchdogFailsStuckRingingCall(t *testing.T) {
	installWatchdogTimeouts(t, 20*time.Millisecond, time.Second)
	runtime := New("watchdog-instance", nil)
	defer runtime.Close()

	timedOut := make(chan string, 1)
	runtime.SetOnTimeout(func(instanceID, callID string) {
		if instanceID != "watchdog-instance" {
			t.Errorf("unexpected instance ID: %s", instanceID)
		}
		timedOut <- callID
	})
	runtime.Transition("ringing-call", "peer", DirectionOutgoing, StateRinging, nil, "")

	if got := waitForTimeout(t, timedOut); got != "ringing-call" {
		t.Fatalf("unexpected timed out call: %s", got)
	}
	call, ok := runtime.Call("ringing-call")
	if !ok {
		t.Fatal("timed out call was removed from public runtime")
	}
	if call.State != StateFailed || call.Error != "call ringing timed out" || call.EndReason != "timeout" {
		t.Fatalf("unexpected timeout state: %+v", call)
	}
}

func TestRuntimeWatchdogFailsStuckConnectingCall(t *testing.T) {
	installWatchdogTimeouts(t, time.Second, 20*time.Millisecond)
	runtime := New("watchdog-instance", nil)
	defer runtime.Close()

	timedOut := make(chan string, 1)
	runtime.SetOnTimeout(func(_, callID string) { timedOut <- callID })
	runtime.Transition("connecting-call", "peer", DirectionIncoming, StateConnecting, nil, "")

	if got := waitForTimeout(t, timedOut); got != "connecting-call" {
		t.Fatalf("unexpected timed out call: %s", got)
	}
	call, _ := runtime.Call("connecting-call")
	if call.State != StateFailed || call.Error != "call media negotiation timed out" {
		t.Fatalf("unexpected connecting timeout state: %+v", call)
	}
}

func TestRuntimeWatchdogIsCancelledWhenMediaBecomesActive(t *testing.T) {
	installWatchdogTimeouts(t, time.Second, 30*time.Millisecond)
	runtime := New("watchdog-instance", nil)
	defer runtime.Close()

	timedOut := make(chan string, 1)
	runtime.SetOnTimeout(func(_, callID string) { timedOut <- callID })
	runtime.Transition("active-call", "peer", DirectionOutgoing, StateConnecting, nil, "")
	runtime.Transition("active-call", "", "", StateActive, nil, "")

	time.Sleep(90 * time.Millisecond)
	select {
	case callID := <-timedOut:
		t.Fatalf("active call timed out: %s", callID)
	default:
	}
	call, _ := runtime.Call("active-call")
	if call.State != StateActive {
		t.Fatalf("unexpected active call state: %+v", call)
	}
}

func TestRuntimeWatchdogIgnoresReplacedTimer(t *testing.T) {
	installWatchdogTimeouts(t, 30*time.Millisecond, 80*time.Millisecond)
	runtime := New("watchdog-instance", nil)
	defer runtime.Close()

	timedOut := make(chan string, 2)
	runtime.SetOnTimeout(func(_, callID string) { timedOut <- callID })
	runtime.Transition("progressing-call", "peer", DirectionOutgoing, StateRinging, nil, "")
	time.Sleep(10 * time.Millisecond)
	runtime.Transition("progressing-call", "", "", StateConnecting, nil, "")

	// The old ringing timer must not fail the newer connecting state.
	time.Sleep(45 * time.Millisecond)
	select {
	case callID := <-timedOut:
		t.Fatalf("stale watchdog timer fired for %s", callID)
	default:
	}

	if got := waitForTimeout(t, timedOut); got != "progressing-call" {
		t.Fatalf("unexpected connecting timeout: %s", got)
	}
}

func TestRuntimeWatchdogDuplicateStateKeepsOriginalDeadline(t *testing.T) {
	installWatchdogTimeouts(t, time.Second, time.Second)
	runtime := New("watchdog-instance", nil)
	defer runtime.Close()

	runtime.Transition("duplicate-call", "peer", DirectionOutgoing, StateConnecting, nil, "")
	runtimeWatchdogs.Lock()
	first := runtimeWatchdogs.states[runtime].entries["duplicate-call"]
	runtimeWatchdogs.Unlock()

	runtime.Transition("duplicate-call", "peer", DirectionOutgoing, StateConnecting, nil, "")
	runtimeWatchdogs.Lock()
	second := runtimeWatchdogs.states[runtime].entries["duplicate-call"]
	runtimeWatchdogs.Unlock()

	if first.generation != second.generation || first.timer != second.timer {
		t.Fatalf("duplicate state replaced watchdog deadline: first=%d second=%d", first.generation, second.generation)
	}
}

func TestRuntimeCloseCancelsWatchdogs(t *testing.T) {
	installWatchdogTimeouts(t, 30*time.Millisecond, 30*time.Millisecond)
	runtime := New("watchdog-instance", nil)
	timedOut := make(chan string, 1)
	runtime.SetOnTimeout(func(_, callID string) { timedOut <- callID })
	runtime.Transition("closed-call", "peer", DirectionOutgoing, StateRinging, nil, "")
	runtime.Close()

	time.Sleep(90 * time.Millisecond)
	select {
	case callID := <-timedOut:
		t.Fatalf("closed runtime watchdog fired for %s", callID)
	default:
	}
}
