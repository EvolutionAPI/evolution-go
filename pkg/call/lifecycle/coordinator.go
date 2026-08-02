// Package lifecycle coordinates call state, private negotiation material and
// experimental media relays for each Evolution WhatsApp client.
package lifecycle

import (
	"context"
	"errors"
	"log/slog"
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
	mu sync.RWMutex

	runtimes *call_runtime.Registry
	incoming *call_incoming.Registry
	relays   *call_media.RelayRegistry
	packets  *call_media.PacketRegistry
	audio    *call_media.AudioRegistry

	onRTP                  func(instanceID, callID string, packet *call_media.RTPPacket)
	onPCM                  func(instanceID, callID string, pcm []float32)
	browserPCM             func(instanceID, callID string, pcm []float32)
	onCallMediaCleanup     func(instanceID, callID string)
	onInstanceMediaCleanup func(instanceID string)

	incomingEnabled   map[string]bool
	mediaErrorReported map[string]bool
}

func NewCoordinator() *Coordinator {
	incoming := call_incoming.NewRegistry()
	packets := call_media.NewPacketRegistry(incoming)
	coordinator := &Coordinator{
		runtimes:          call_runtime.NewRegistry(),
		incoming:          incoming,
		packets:           packets,
		incomingEnabled:   make(map[string]bool),
		mediaErrorReported: make(map[string]bool),
	}
	coordinator.audio = call_media.NewAudioRegistry(func(instanceID, callID string, payload []byte, durationSamples uint32, marker bool) error {
		return coordinator.SendOpus(instanceID, callID, payload, durationSamples, marker)
	}, nil)
	coordinator.audio.SetOnPCM(coordinator.dispatchPCM)
	coordinator.relays = call_media.NewRelayRegistry(incoming, nil, nil)
	coordinator.relays.SetOnRemoved(func(instanceID, callID string) {
		coordinator.audio.Remove(instanceID, callID)
		coordinator.packets.Remove(instanceID, callID)
		coordinator.clearMediaError(instanceID, callID)
		coordinator.notifyCallMediaCleanup(instanceID, callID)
	})
	coordinator.relays.SetOnCleanup(func(instanceID string) {
		coordinator.audio.Close(instanceID)
		coordinator.packets.Close(instanceID)
		coordinator.clearInstanceMediaErrors(instanceID)
		coordinator.notifyInstanceMediaCleanup(instanceID)
	})
	coordinator.relays.SetOnConnected(func(instanceID, callID string) {
		if err := coordinator.packets.Prepare(instanceID, callID); err != nil {
			coordinator.reportMediaError(instanceID, callID, "prepare RTP/SRTP session", err)
			return
		}
		if err := coordinator.audio.Prepare(instanceID, callID); err != nil {
			coordinator.reportMediaError(instanceID, callID, "prepare audio codec", err)
			coordinator.packets.Remove(instanceID, callID)
			return
		}
		if runtime, ok := coordinator.runtimes.Get(instanceID); ok {
			runtime.Transition(callID, "", "", call_runtime.StateActive, nil, "")
		}
		slog.Info("WhatsApp call media active", "instance", instanceID, "call_id", callID)
	})
	coordinator.packets.SetOnPeerSSRC(func(instanceID, callID string, previous, actual uint32) {
		if err := coordinator.relays.UpdatePeerSSRC(instanceID, callID, actual); err != nil {
			coordinator.reportMediaError(instanceID, callID, "refresh relay peer SSRC", err)
			return
		}
		slog.Info("WhatsApp peer SSRC adopted",
			"instance", instanceID,
			"call_id", callID,
			"predicted_ssrc", previous,
			"actual_ssrc", actual,
		)
	})
	coordinator.packets.SetOnRTP(func(instanceID, callID string, packet *call_media.RTPPacket) {
		if err := coordinator.audio.HandleRTP(instanceID, callID, packet); err != nil {
			coordinator.reportMediaError(instanceID, callID, "decode WhatsApp audio", err)
			return
		}
		coordinator.mu.RLock()
		callback := coordinator.onRTP
		coordinator.mu.RUnlock()
		if callback != nil {
			callback(instanceID, callID, packet)
		}
	})
	coordinator.relays.SetOnPacket(func(instanceID, callID string, packet []byte) {
		err := coordinator.packets.Handle(instanceID, callID, packet)
		if err == nil || errors.Is(err, call_media.ErrNonRTPFrame) || errors.Is(err, call_media.ErrPacketSessionNotReady) || errors.Is(err, call_media.ErrSelfRTPFrame) {
			return
		}
		coordinator.reportMediaError(instanceID, callID, "receive WhatsApp SRTP", err)
	})
	return coordinator
}

func mediaErrorKey(instanceID, callID string) string {
	return instanceID + "\x00" + callID
}

func (c *Coordinator) reportMediaError(instanceID, callID, stage string, err error) {
	if c == nil || err == nil {
		return
	}
	key := mediaErrorKey(instanceID, callID)
	c.mu.Lock()
	if c.mediaErrorReported[key] {
		c.mu.Unlock()
		return
	}
	c.mediaErrorReported[key] = true
	c.mu.Unlock()
	slog.Warn("WhatsApp inbound media failed", "instance", instanceID, "call_id", callID, "stage", stage, "err", err)
}

func (c *Coordinator) clearMediaError(instanceID, callID string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	delete(c.mediaErrorReported, mediaErrorKey(instanceID, callID))
	c.mu.Unlock()
}

func (c *Coordinator) clearInstanceMediaErrors(instanceID string) {
	if c == nil {
		return
	}
	prefix := instanceID + "\x00"
	c.mu.Lock()
	for key := range c.mediaErrorReported {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(c.mediaErrorReported, key)
		}
	}
	c.mu.Unlock()
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
	c.packets.Attach(instanceID, client)
	c.relays.Attach(instanceID, client)
}

// DetachClient removes handlers, relay connections, codec sessions, packet
// contexts, browser media, configuration and private call keys before the
// client is discarded.
func (c *Coordinator) DetachClient(instanceID string) {
	if c == nil || instanceID == "" {
		return
	}
	c.mu.Lock()
	delete(c.incomingEnabled, instanceID)
	c.mu.Unlock()
	c.relays.Close(instanceID)
	c.audio.Close(instanceID)
	c.packets.Close(instanceID)
	c.clearInstanceMediaErrors(instanceID)
	c.notifyInstanceMediaCleanup(instanceID)
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
	c.packets.Attach(instanceID, client)
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
	go func() {
		if err := c.relays.Start(instanceID, callID); err != nil {
			c.reportMediaError(instanceID, callID, "start incoming relay", err)
		}
	}()
	return nil
}

func (c *Coordinator) TerminateIncoming(ctx context.Context, instanceID, callID string) error {
	if err := c.incoming.Terminate(ctx, instanceID, callID); err != nil {
		return err
	}
	c.relays.Remove(instanceID, callID)
	c.audio.Remove(instanceID, callID)
	c.packets.Remove(instanceID, callID)
	c.clearMediaError(instanceID, callID)
	c.notifyCallMediaCleanup(instanceID, callID)
	return nil
}

// FeedPCM accepts mono float PCM at 16 kHz. Samples may arrive in arbitrary
// chunk sizes; the audio registry accumulates complete 960-sample MLow frames.
func (c *Coordinator) FeedPCM(instanceID, callID string, pcm []float32) error {
	if c == nil {
		return call_media.ErrAudioSessionNotReady
	}
	return c.audio.FeedPCM(instanceID, callID, pcm)
}

// SetOnPCM registers an optional external decoded-audio observer. Browser
// media keeps a separate internal sink so neither callback replaces the other.
func (c *Coordinator) SetOnPCM(callback func(instanceID, callID string, pcm []float32)) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.onPCM = callback
	c.mu.Unlock()
}

func (c *Coordinator) SetBrowserPCM(callback func(instanceID, callID string, pcm []float32)) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.browserPCM = callback
	c.mu.Unlock()
}

func (c *Coordinator) SetMediaCleanupHooks(onCall func(instanceID, callID string), onInstance func(instanceID string)) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.onCallMediaCleanup = onCall
	c.onInstanceMediaCleanup = onInstance
	c.mu.Unlock()
}

func (c *Coordinator) dispatchPCM(instanceID, callID string, pcm []float32) {
	c.mu.RLock()
	browserCallback := c.browserPCM
	externalCallback := c.onPCM
	c.mu.RUnlock()
	if browserCallback != nil {
		browserCallback(instanceID, callID, append([]float32(nil), pcm...))
	}
	if externalCallback != nil {
		externalCallback(instanceID, callID, append([]float32(nil), pcm...))
	}
}

func (c *Coordinator) notifyCallMediaCleanup(instanceID, callID string) {
	c.mu.RLock()
	callback := c.onCallMediaCleanup
	c.mu.RUnlock()
	if callback != nil {
		callback(instanceID, callID)
	}
}

func (c *Coordinator) notifyInstanceMediaCleanup(instanceID string) {
	c.mu.RLock()
	callback := c.onInstanceMediaCleanup
	c.mu.RUnlock()
	if callback != nil {
		callback(instanceID)
	}
}

// SendOpus protects one encoded MLow/Opus-compatible frame as SRTP and
// broadcasts it through the currently connected WhatsApp relays.
func (c *Coordinator) SendOpus(instanceID, callID string, payload []byte, durationSamples uint32, marker bool) error {
	if c == nil {
		return call_media.ErrPacketSessionNotReady
	}
	protected, err := c.packets.ProtectOpus(instanceID, callID, payload, durationSamples, marker)
	if err != nil {
		return err
	}
	defer wipe(protected)
	return c.relays.Broadcast(instanceID, callID, protected)
}

// SetOnRTP keeps the low-level authenticated RTP observation hook while the
// internal decoder remains permanently connected.
func (c *Coordinator) SetOnRTP(callback func(instanceID, callID string, packet *call_media.RTPPacket)) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.onRTP = callback
	c.mu.Unlock()
}

func (c *Coordinator) RemovePrivate(instanceID, callID string) {
	c.relays.Remove(instanceID, callID)
	c.audio.Remove(instanceID, callID)
	c.packets.Remove(instanceID, callID)
	c.incoming.Remove(instanceID, callID)
	c.clearMediaError(instanceID, callID)
	c.notifyCallMediaCleanup(instanceID, callID)
}

// RemoveIncoming is kept as a compatibility alias while call-service code is
// migrated to direction-neutral private negotiation storage.
func (c *Coordinator) RemoveIncoming(instanceID, callID string) {
	c.RemovePrivate(instanceID, callID)
}

func wipe(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
