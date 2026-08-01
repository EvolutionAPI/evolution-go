// Package lifecycle coordinates call state, private negotiation material and
// experimental media relays for each Evolution WhatsApp client.
package lifecycle

import (
	"context"
	"sync"

	call_runtime "github.com/evolution-foundation/evolution-go/pkg/call/runtime"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	call_incoming "github.com/evolution-foundation/evolution-go/pkg/call/voip/incoming"
	call_media "github.com/evolution-foundation/evolution-go/pkg/call/voip/media"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// Coordinator owns the call registries shared by the WhatsApp lifecycle and
// the HTTP call service. It is safe for concurrent use.
type Coordinator struct {
	mu              sync.RWMutex
	runtimes        *call_runtime.Registry
	incoming        *call_incoming.Registry
	relays          *call_media.RelayRegistry
	incomingEnabled map[string]bool
}

func NewCoordinator() *Coordinator {
	incoming := call_incoming.NewRegistry()
	coordinator := &Coordinator{
		runtimes:        call_runtime.NewRegistry(),
		incoming:        incoming,
		incomingEnabled: make(map[string]bool),
	}
	coordinator.relays = call_media.NewRelayRegistry(incoming, nil, nil)
	coordinator.relays.SetOnConnected(func(instanceID, callID string) {
		if runtime, ok := coordinator.runtimes.Get(instanceID); ok {
			runtime.Transition(callID, "", "", call_runtime.StateActive, nil, "")
		}
	})
	return coordinator
}

// AttachClient is called by the WhatsApp client lifecycle. Public call state is
// always monitored. Private outgoing negotiation remains available even when
// incoming offer preparation is disabled by automatic rejection settings.
func (c *Coordinator) AttachClient(instanceID string, client *whatsmeow.Client, prepareIncoming bool) {
	if c == nil || instanceID == "" || client == nil {
		return
	}

	c.mu.Lock()
	c.incomingEnabled[instanceID] = prepareIncoming
	c.mu.Unlock()

	c.runtimes.Attach(instanceID, client)
	c.incoming.Attach(instanceID, client, prepareIncoming)
	c.relays.Attach(instanceID, client)
}

// DetachClient removes handlers, relay connections, configuration and private
// call keys before the WhatsApp client is discarded.
func (c *Coordinator) DetachClient(instanceID string) {
	if c == nil || instanceID == "" {
		return
	}
	c.mu.Lock()
	delete(c.incomingEnabled, instanceID)
	c.mu.Unlock()
	c.relays.Close(instanceID)
	c.runtimes.Remove(instanceID)
	c.incoming.Close(instanceID)
}

// Attach keeps call-service operations idempotent without overriding the
// automatic-rejection policy configured by AttachClient.
func (c *Coordinator) Attach(instanceID string, client *whatsmeow.Client) {
	if c == nil || instanceID == "" || client == nil {
		return
	}
	c.runtimes.Attach(instanceID, client)

	c.mu.RLock()
	prepareIncoming, configured := c.incomingEnabled[instanceID]
	c.mu.RUnlock()
	if !configured {
		prepareIncoming = true
	}
	c.incoming.Attach(instanceID, client, prepareIncoming)
	c.relays.Attach(instanceID, client)
}

func (c *Coordinator) Detach(instanceID string) {
	c.DetachClient(instanceID)
}

func (c *Coordinator) Runtime(instanceID string) (*call_runtime.Runtime, bool) {
	if c == nil {
		return nil, false
	}
	return c.runtimes.Get(instanceID)
}

func (c *Coordinator) RuntimeFor(instanceID string, client *whatsmeow.Client) *call_runtime.Runtime {
	if c == nil || instanceID == "" {
		return nil
	}
	c.Attach(instanceID, client)
	runtime, _ := c.runtimes.Get(instanceID)
	return runtime
}

func (c *Coordinator) StoreOutgoing(instanceID, callID string, callKey []byte, peer, creator types.JID, video bool, relayData *core.RelayData) error {
	if c == nil {
		return nil
	}
	return c.incoming.StoreOutgoing(instanceID, callID, callKey, peer, creator, video, relayData)
}

// RelayData returns a defensive copy. The caller owns the copy and must call
// core.ZeroRelayData after use.
func (c *Coordinator) RelayData(instanceID, callID string) (*core.RelayData, bool) {
	if c == nil {
		return nil, false
	}
	return c.incoming.RelayData(instanceID, callID)
}

func (c *Coordinator) AcceptIncoming(ctx context.Context, instanceID, callID string) error {
	if err := c.incoming.Accept(ctx, instanceID, callID); err != nil {
		return err
	}
	go func() { _ = c.relays.Start(instanceID, callID) }()
	return nil
}

func (c *Coordinator) TerminateIncoming(ctx context.Context, instanceID, callID string) error {
	if err := c.incoming.Terminate(ctx, instanceID, callID); err != nil {
		return err
	}
	c.relays.Remove(instanceID, callID)
	return nil
}

func (c *Coordinator) RemovePrivate(instanceID, callID string) {
	c.relays.Remove(instanceID, callID)
	c.incoming.Remove(instanceID, callID)
}

// RemoveIncoming is kept as a compatibility alias while call-service code is
// migrated to direction-neutral private negotiation storage.
func (c *Coordinator) RemoveIncoming(instanceID, callID string) {
	c.RemovePrivate(instanceID, callID)
}
