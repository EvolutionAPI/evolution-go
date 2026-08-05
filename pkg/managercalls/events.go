// Package managercalls exposes authenticated, Manager V2-only call events.
// It is separate from the integration WebSocket producer because the Manager
// must not depend on a customer's WebSocket subscription settings.
package managercalls

import (
	"sync"
	"time"

	call_runtime "github.com/evolution-foundation/evolution-go/pkg/call/runtime"
)

// Event contains the safe public state needed by the Manager to update a call
// card immediately. It intentionally excludes private signaling and media
// data.
type Event struct {
	Type       string            `json:"type"`
	InstanceID string            `json:"instanceId"`
	Call       call_runtime.Call `json:"call"`
	OccurredAt time.Time         `json:"occurredAt"`
}

type subscription struct {
	events chan Event
	done   chan struct{}
	once   sync.Once
}

// Subscription belongs to one browser connection.
type Subscription struct {
	Events <-chan Event
	Done   <-chan struct{}
	cancel func()
}

// Cancel unregisters the browser connection. It is safe to call repeatedly.
func (s *Subscription) Cancel() {
	if s != nil && s.cancel != nil {
		s.cancel()
	}
}

// Hub fans out a call update to all Manager V2 windows watching the same
// instance. Slow subscribers keep only recent updates; /call/status polling
// remains the recovery path for a missed event.
type Hub struct {
	mu            sync.RWMutex
	subscribers   map[string]map[*subscription]struct{}
	channelBuffer int
}

func NewHub() *Hub {
	return &Hub{
		subscribers:   make(map[string]map[*subscription]struct{}),
		channelBuffer: 16,
	}
}

func (h *Hub) Subscribe(instanceID string) *Subscription {
	if h == nil {
		return &Subscription{Events: make(chan Event), Done: closedChannel()}
	}

	entry := &subscription{
		events: make(chan Event, h.channelBuffer),
		done:   make(chan struct{}),
	}
	h.mu.Lock()
	if h.subscribers[instanceID] == nil {
		h.subscribers[instanceID] = make(map[*subscription]struct{})
	}
	h.subscribers[instanceID][entry] = struct{}{}
	h.mu.Unlock()

	return &Subscription{
		Events: entry.events,
		Done:   entry.done,
		cancel: func() { h.unsubscribe(instanceID, entry) },
	}
}

func (h *Hub) unsubscribe(instanceID string, entry *subscription) {
	if h == nil || entry == nil {
		return
	}
	entry.once.Do(func() {
		h.mu.Lock()
		if entries := h.subscribers[instanceID]; entries != nil {
			delete(entries, entry)
			if len(entries) == 0 {
				delete(h.subscribers, instanceID)
			}
		}
		h.mu.Unlock()
		close(entry.done)
	})
}

// Publish is non-blocking with respect to WhatsApp event processing. A full
// subscriber buffer is compacted so the newest state is preferred.
func (h *Hub) Publish(instanceID string, call call_runtime.Call) {
	if h == nil || instanceID == "" || call.ID == "" {
		return
	}
	event := Event{
		Type:       eventType(call),
		InstanceID: instanceID,
		Call:       call,
		OccurredAt: time.Now().UTC(),
	}

	h.mu.RLock()
	for entry := range h.subscribers[instanceID] {
		select {
		case entry.events <- event:
		default:
			select {
			case <-entry.events:
			default:
			}
			select {
			case entry.events <- event:
			default:
			}
		}
	}
	h.mu.RUnlock()
}

func eventType(call call_runtime.Call) string {
	if call.Direction == call_runtime.DirectionIncoming && call.State == call_runtime.StateRinging {
		return "call.offer"
	}
	if call.State == call_runtime.StateConnecting || call.State == call_runtime.StateActive {
		return "call.accept"
	}
	if call.State == call_runtime.StateEnded || call.State == call_runtime.StateFailed {
		return "call.terminate"
	}
	return "call.updated"
}

func closedChannel() <-chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}
