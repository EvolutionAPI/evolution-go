// Package lifecycle coordinates call state and private incoming-call material
// for each Evolution WhatsApp client.
package lifecycle

import (
	"context"

	call_runtime "github.com/evolution-foundation/evolution-go/pkg/call/runtime"
	call_incoming "github.com/evolution-foundation/evolution-go/pkg/call/voip/incoming"
	"go.mau.fi/whatsmeow"
)

// Coordinator owns the call registries shared by the WhatsApp lifecycle and
// the HTTP call service. It is safe for concurrent use.
type Coordinator struct {
	runtimes *call_runtime.Registry
	incoming *call_incoming.Registry
}

func NewCoordinator() *Coordinator {
	return &Coordinator{
		runtimes: call_runtime.NewRegistry(),
		incoming: call_incoming.NewRegistry(),
	}
}

// Attach installs both call event handlers on the authenticated client. The
// operation is idempotent for the same instance/client pair and replaces old
// handlers when whatsmeow creates a new client during reconnect.
func (c *Coordinator) Attach(instanceID string, client *whatsmeow.Client) {
	if c == nil || instanceID == "" || client == nil {
		return
	}
	c.runtimes.Attach(instanceID, client)
	c.incoming.Attach(instanceID, client)
}

// Detach removes handlers and erases private call keys for an instance.
func (c *Coordinator) Detach(instanceID string) {
	if c == nil || instanceID == "" {
		return
	}
	c.runtimes.Remove(instanceID)
	c.incoming.Close(instanceID)
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
