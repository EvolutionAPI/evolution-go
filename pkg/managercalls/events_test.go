package managercalls

import (
	"testing"
	"time"

	call_runtime "github.com/evolution-foundation/evolution-go/pkg/call/runtime"
)

func TestHubPublishesOnlyToTheSelectedInstance(t *testing.T) {
	hub := NewHub()
	selected := hub.Subscribe("instance-selected")
	defer selected.Cancel()
	other := hub.Subscribe("instance-other")
	defer other.Cancel()

	hub.Publish("instance-selected", call_runtime.Call{ID: "call-1", Direction: call_runtime.DirectionIncoming, State: call_runtime.StateRinging})

	select {
	case event := <-selected.Events:
		if event.Type != "call.offer" || event.InstanceID != "instance-selected" || event.Call.ID != "call-1" {
			t.Fatalf("unexpected event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("expected a call event for the selected instance")
	}

	select {
	case event := <-other.Events:
		t.Fatalf("unexpected event for another instance: %+v", event)
	default:
	}
}

func TestSubscriptionCancelStopsDelivery(t *testing.T) {
	hub := NewHub()
	subscription := hub.Subscribe("instance-1")
	subscription.Cancel()

	select {
	case <-subscription.Done:
	case <-time.After(time.Second):
		t.Fatal("expected the subscription to close")
	}

	hub.Publish("instance-1", call_runtime.Call{ID: "call-1", State: call_runtime.StateRinging})
	select {
	case event := <-subscription.Events:
		t.Fatalf("cancelled subscription received an event: %+v", event)
	default:
	}
}

func TestEventTypeReflectsCallLifecycle(t *testing.T) {
	tests := []struct {
		name string
		call call_runtime.Call
		want string
	}{
		{name: "incoming offer", call: call_runtime.Call{Direction: call_runtime.DirectionIncoming, State: call_runtime.StateRinging}, want: "call.offer"},
		{name: "accepted", call: call_runtime.Call{Direction: call_runtime.DirectionIncoming, State: call_runtime.StateConnecting}, want: "call.accept"},
		{name: "active", call: call_runtime.Call{Direction: call_runtime.DirectionOutgoing, State: call_runtime.StateActive}, want: "call.accept"},
		{name: "terminated", call: call_runtime.Call{State: call_runtime.StateEnded}, want: "call.terminate"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := eventType(test.call); got != test.want {
				t.Fatalf("unexpected event type: got %s want %s", got, test.want)
			}
		})
	}
}
