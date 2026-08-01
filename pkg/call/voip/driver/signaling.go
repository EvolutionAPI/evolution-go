// Package driver coordinates VoIP protocol operations without owning Evolution sessions.
package driver

import (
	"context"
	"fmt"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/signaling"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/wa"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/wanode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// StartResult contains private negotiation material produced while starting a
// call. Callers must copy it into their private registry and then call Wipe.
type StartResult struct {
	CallID    string
	Peer      types.JID
	Creator   types.JID
	CallKey   []byte
	RelayData *core.RelayData
}

// Wipe removes private material from this transient result after it has been
// copied into the per-instance call registry.
func (r *StartResult) Wipe() {
	if r == nil {
		return
	}
	for index := range r.CallKey {
		r.CallKey[index] = 0
	}
	core.ZeroRelayData(r.RelayData)
	r.CallKey = nil
	r.RelayData = nil
}

// SignalingDriver sends real WhatsApp call stanzas. Media transport is not yet
// attached, so a successful offer means the peer can ring and emit lifecycle
// events, not that bidirectional audio is available.
type SignalingDriver struct {
	socket core.VoipSocket
}

func NewSignalingDriver(client *whatsmeow.Client) *SignalingDriver {
	return &SignalingDriver{socket: wa.NewSocket(client)}
}

func (d *SignalingDriver) Start(ctx context.Context, peer types.JID, video bool) (*StartResult, error) {
	if peer.IsEmpty() {
		return nil, fmt.Errorf("peer JID is empty")
	}

	creator := d.socket.OwnLID()
	if creator.IsEmpty() {
		creator = d.socket.OwnPN()
	}
	if creator.IsEmpty() {
		return nil, fmt.Errorf("whatsapp client has no own JID")
	}

	callID := signaling.GenerateCallID()
	callKey, err := signaling.GenerateCallKey()
	if err != nil {
		return nil, err
	}
	wipeOnError := func() {
		for index := range callKey {
			callKey[index] = 0
		}
	}

	resolvedPeer := d.socket.ResolveLIDForPN(ctx, peer)
	offer, err := signaling.BuildOfferStanza(ctx, d.socket, callID, callKey, resolvedPeer, video)
	if err != nil {
		wipeOnError()
		return nil, err
	}

	ack, err := d.socket.Query(ctx, offer)
	if err != nil {
		wipeOnError()
		return nil, fmt.Errorf("send call offer: %w", err)
	}

	var relayData *core.RelayData
	if ack != nil {
		if ackError := wanode.AttrString(ack.Attrs, "error"); ackError != "" {
			wipeOnError()
			return nil, fmt.Errorf("call offer rejected by WhatsApp: %s", ackError)
		}
		parsed := signaling.ParseRelayFromAck(ack)
		if len(parsed.Relays) > 0 || parsed.UUID != "" || len(parsed.HBHKey) > 0 {
			relayData = &core.RelayData{
				Endpoints:       parsed.Relays,
				ParticipantJIDs: parsed.ParticipantJIDs,
				UUID:            parsed.UUID,
				SelfPID:         parsed.SelfPID,
				PeerPID:         parsed.PeerPID,
				HBHKey:          parsed.HBHKey,
			}
		}
	}

	return &StartResult{
		CallID:    callID,
		Peer:      resolvedPeer,
		Creator:   creator,
		CallKey:   callKey,
		RelayData: relayData,
	}, nil
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
