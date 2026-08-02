package media

import (
	"context"
	"fmt"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/signaling"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/wa"
	"go.mau.fi/whatsmeow/types"
)

const postAcceptSignalingTimeout = 5 * time.Second

// sendOutgoingPostAccept completes the signaling sequence used by WhatsApp
// after the remote party accepts an outgoing call. The relay connection may
// start in parallel; these stanzas must not block media startup.
func (s *relaySession) sendOutgoingPostAccept(callID string) {
	if s == nil || s.source == nil || callID == "" {
		return
	}
	state, ok := s.source.State(s.instanceID, callID)
	if !ok || state == nil || state.Direction != core.CallDirectionOutgoing {
		return
	}

	s.mu.Lock()
	client := s.client
	s.mu.Unlock()
	if client == nil {
		return
	}

	peer, err := types.ParseJID(state.PeerJID)
	if err != nil || peer.IsEmpty() {
		if err == nil {
			err = fmt.Errorf("peer JID is empty")
		}
		s.log.Warn("WhatsApp post-accept signaling skipped", "instance", s.instanceID, "call_id", callID, "err", err)
		return
	}
	creator, err := types.ParseJID(state.CallCreator)
	if err != nil || creator.IsEmpty() {
		if err == nil {
			err = fmt.Errorf("creator JID is empty")
		}
		s.log.Warn("WhatsApp post-accept signaling skipped", "instance", s.instanceID, "call_id", callID, "err", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), postAcceptSignalingTimeout)
	defer cancel()
	socket := wa.NewSocket(client)
	peer = socket.ResolveLIDForPN(ctx, peer)

	if err = socket.SendNode(ctx, signaling.BuildPostAcceptTransportStanza(peer, creator, callID)); err != nil {
		s.log.Warn("WhatsApp post-accept transport failed", "instance", s.instanceID, "call_id", callID, "err", err)
		return
	}
	if err = socket.SendNode(ctx, signaling.BuildMuteV2Stanza(peer, creator, callID, 0)); err != nil {
		s.log.Warn("WhatsApp post-accept mute sync failed", "instance", s.instanceID, "call_id", callID, "err", err)
		return
	}
	s.log.Info("WhatsApp post-accept media signaling sent", "instance", s.instanceID, "call_id", callID, "peer", peer.String())
}
