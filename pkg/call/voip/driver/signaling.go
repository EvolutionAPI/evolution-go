// Package driver coordinates VoIP protocol operations without owning Evolution sessions.
package driver

import (
	"context"
	"fmt"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/signaling"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/wa"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// SignalingDriver sends real WhatsApp call stanzas. Media transport is not yet
// attached, so a successful offer means the peer can ring and emit lifecycle
// events, not that bidirectional audio is available.
type SignalingDriver struct {
	socket core.VoipSocket
}

func NewSignalingDriver(client *whatsmeow.Client) *SignalingDriver {
	return &SignalingDriver{socket: wa.NewSocket(client)}
}

func (d *SignalingDriver) Start(ctx context.Context, peer types.JID, video bool) (string, types.JID, error) {
	if peer.IsEmpty() {
		return "", types.JID{}, fmt.Errorf("peer JID is empty")
	}
	callID := signaling.GenerateCallID()
	callKey, err := signaling.GenerateCallKey()
	if err != nil {
		return "", types.JID{}, err
	}
	resolvedPeer := d.socket.ResolveLIDForPN(ctx, peer)
	offer, err := signaling.BuildOfferStanza(ctx, d.socket, callID, callKey, resolvedPeer, video)
	if err != nil {
		return "", types.JID{}, err
	}
	if err := d.socket.SendNode(ctx, offer); err != nil {
		return "", types.JID{}, fmt.Errorf("send call offer: %w", err)
	}
	return callID, resolvedPeer, nil
}

func (d *SignalingDriver) EndOutgoing(ctx context.Context, callID string, peer types.JID) error {
	creator := d.socket.OwnLID()
	if creator.IsEmpty() {
		creator = d.socket.OwnPN()
	}
	if creator.IsEmpty() {
		return fmt.Errorf("whatsapp client has no own JID")
	}
	node := signaling.BuildTerminateStanza(peer, callID, creator)
	if err := d.socket.SendNode(ctx, node); err != nil {
		return fmt.Errorf("send call terminate: %w", err)
	}
	return nil
}
