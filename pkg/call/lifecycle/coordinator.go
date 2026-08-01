// Package lifecycle coordinates call state and private incoming-call material
// for each Evolution WhatsApp client.
package lifecycle

import (
	"context"
	"sync"

	call_runtime "github.com/evolution-foundation/evolution-go/pkg/call/runtime"
	call_incoming "github.com/evolution-foundation/evolution-go/pkg/call/voip/incoming"
	"go.mau.fi/whatsmeow"
)

// Coordinator owns the call registries shared by the WhatsApp lifecycle and
// the HTTP call service. It is safe for concurrent use.
type Coordinator struct {
	mu              sync.RWMutex
	runtimes        *call_runtime.Registry
	incoming        *call_incoming.Registry
	incomingEnabled map[string]bool
}

func NewCoordinator() *Coordinator {
	return &Coordinator{
		runtimes:        call_runtime.NewRegistry(),
		incoming:        call_incoming.NewRegistry(),
		incomingEnabled: make(map[string]bool),
	}
}

// AttachClient is called by the WhatsApp client lifecycle. Public call state is
// always monitored, while private offer preparation is disabled for instances
// configured to reject incoming calls automatically.
func (c *Coordinator) AttachClient(instanceID string, client *whatsmeow.Client, prepareIncoming bool) {
	if c == nil || instanceID == "" || client == nil {
		return
	}

	c.mu.Lock()
	c.incomingEnabled[instanceID] = prepareIncoming
	c.mu.Unlock()

	c.runtimes.Attach(instanceID, client)
	if prepareIncoming {
		c.incoming.Attach(instanceID, client)
	} else {
		c.incoming.Close(instanceID)
	}
}

// DetachClient removes handlers, configuration and private call keys.
func (c *Coordinator) DetachClient(instanceID string) {
	if c == nil || instanceID == "" {
		return
	}
	c.mu.Lock()
	delete(c.incomingEnabled, instanceID)
	c.mu.Unlock()
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
	if !configured || prepareIncoming {
		c.incoming.Attach(instanceID, client)
	}
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

func (c *Coordinator) AcceptIncoming(ctx context.Context, instanceID, callID string) error {
	return c.incoming.Accept(ctx, instanceID, callID)
}

func (c *Coordinator) TerminateIncoming(ctx context.Context, instanceID, callID string) error {
	return c.incoming.Terminate(ctx, instanceID, callID)
}

func (c *Coordinator) RemoveIncoming(instanceID, callID string) {
	c.incoming.Remove(instanceID, callID)
}
