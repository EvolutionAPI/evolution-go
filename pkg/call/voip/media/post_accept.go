package media

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/signaling"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/wa"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

const (
	postAcceptSignalingTimeout = 5 * time.Second
	postAcceptProgressTTL      = 10 * time.Minute
)

type postAcceptProgressKey struct {
	session *relaySession
	callID  string
}

type postAcceptProgress struct {
	running       bool
	transportSent bool
	muteSent      bool
	expiresAt     time.Time
}

type postAcceptProgressTracker struct {
	sync.Mutex
	calls map[postAcceptProgressKey]*postAcceptProgress
}

var outgoingPostAcceptProgress = postAcceptProgressTracker{
	calls: make(map[postAcceptProgressKey]*postAcceptProgress),
}

var resolvePostAcceptPeer = func(ctx context.Context, client *whatsmeow.Client, peer types.JID) types.JID {
	return wa.NewSocket(client).ResolveLIDForPN(ctx, peer)
}

var sendPostAcceptTransport = func(
	ctx context.Context,
	client *whatsmeow.Client,
	peer, creator types.JID,
	callID string,
) error {
	return wa.NewSocket(client).SendNode(ctx, signaling.BuildPostAcceptTransportStanza(peer, creator, callID))
}

var sendPostAcceptMute = func(
	ctx context.Context,
	client *whatsmeow.Client,
	peer, creator types.JID,
	callID string,
) error {
	return wa.NewSocket(client).SendNode(ctx, signaling.BuildMuteV2Stanza(peer, creator, callID, 0))
}

func (t *postAcceptProgressTracker) begin(session *relaySession, callID string) (postAcceptProgress, bool) {
	if t == nil || session == nil || callID == "" {
		return postAcceptProgress{}, false
	}

	now := time.Now()
	key := postAcceptProgressKey{session: session, callID: callID}
	t.Lock()
	defer t.Unlock()
	if t.calls == nil {
		t.calls = make(map[postAcceptProgressKey]*postAcceptProgress)
	}
	for existingKey, progress := range t.calls {
		if progress != nil && !progress.running && !progress.expiresAt.IsZero() && !progress.expiresAt.After(now) {
			delete(t.calls, existingKey)
		}
	}

	progress := t.calls[key]
	if progress == nil {
		progress = &postAcceptProgress{}
		t.calls[key] = progress
	}
	if progress.running || progress.muteSent {
		return postAcceptProgress{}, false
	}
	progress.running = true
	progress.expiresAt = time.Time{}
	return *progress, true
}

func (t *postAcceptProgressTracker) finish(session *relaySession, callID string, result postAcceptProgress) {
	if t == nil || session == nil || callID == "" {
		return
	}
	key := postAcceptProgressKey{session: session, callID: callID}
	expiresAt := time.Now().Add(postAcceptProgressTTL)

	t.Lock()
	progress := t.calls[key]
	if progress == nil {
		t.Unlock()
		return
	}
	progress.running = false
	progress.transportSent = result.transportSent
	progress.muteSent = result.muteSent
	progress.expiresAt = expiresAt
	t.Unlock()

	time.AfterFunc(postAcceptProgressTTL, func() {
		t.Lock()
		defer t.Unlock()
		current := t.calls[key]
		if current != nil && !current.running && !current.expiresAt.After(expiresAt) {
			delete(t.calls, key)
		}
	})
}

func (t *postAcceptProgressTracker) reset() {
	if t == nil {
		return
	}
	t.Lock()
	t.calls = make(map[postAcceptProgressKey]*postAcceptProgress)
	t.Unlock()
}

// sendOutgoingPostAccept completes the signaling sequence used by WhatsApp
// after the remote party accepts an outgoing call. The relay connection may
// start in parallel; these stanzas must not block media startup.
//
// WhatsApp can emit duplicate CallAccept events. Progress is therefore tracked
// per relay session and call: a successful transport announcement is not sent
// again when only the mute synchronization needs retrying.
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

	progress, acquired := outgoingPostAcceptProgress.begin(s, callID)
	if !acquired {
		return
	}
	defer func() {
		outgoingPostAcceptProgress.finish(s, callID, progress)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), postAcceptSignalingTimeout)
	defer cancel()
	peer = resolvePostAcceptPeer(ctx, client, peer)

	if !progress.transportSent {
		if err = sendPostAcceptTransport(ctx, client, peer, creator, callID); err != nil {
			s.log.Warn("WhatsApp post-accept transport failed", "instance", s.instanceID, "call_id", callID, "err", err)
			return
		}
		progress.transportSent = true
	}
	if !progress.muteSent {
		if err = sendPostAcceptMute(ctx, client, peer, creator, callID); err != nil {
			s.log.Warn("WhatsApp post-accept mute sync failed", "instance", s.instanceID, "call_id", callID, "err", err)
			return
		}
		progress.muteSent = true
	}
	s.log.Info("WhatsApp post-accept media signaling sent", "instance", s.instanceID, "call_id", callID, "peer", peer.String())
}
