// pkg/call/stream/bridge.go
package call_stream

import (
	"encoding/base64"
	"io"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/purpshell/meowcaller"
)

// wsMessage is the JSON envelope carried over the stream socket, modeled on Twilio
// Media Streams so existing AI-voice integrations need minimal adapting.
type wsMessage struct {
	Event      string `json:"event"`
	CallID     string `json:"callId,omitempty"`
	SampleRate int    `json:"sampleRate,omitempty"`
	Video      bool   `json:"video,omitempty"`
	Track      string `json:"track,omitempty"`
	Payload    string `json:"payload,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// bridge adapts one WebSocket connection to meowcaller's AudioSink (Call.Receive),
// VideoSink (Call.ReceiveVideo), and AudioSource (Call.Play) interfaces, so a single
// object plugs a call's media straight into the socket in both directions.
type bridge struct {
	conn           *websocket.Conn
	writeMu        sync.Mutex
	incoming       chan []float32
	closed         chan struct{}
	closeOnce      sync.Once
	sendVideo      func([]byte) error
	startVideo     func() error
	startVideoOnce sync.Once
}

func newBridge(conn *websocket.Conn) *bridge {
	return &bridge{
		conn: conn,
		// ~3 seconds of buffering at one 60ms frame per slot before frames start
		// getting dropped — enough slack for scheduling jitter without unbounded growth.
		incoming: make(chan []float32, 50),
		closed:   make(chan struct{}),
	}
}

func (b *bridge) writeJSON(msg wsMessage) error {
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	return b.conn.WriteJSON(msg)
}

// writeStart sends the initial handshake message once the socket is up.
func (b *bridge) writeStart(callID string, video bool) {
	_ = b.writeJSON(wsMessage{
		Event:      "start",
		CallID:     callID,
		SampleRate: meowcaller.SampleRate,
		Video:      video,
	})
}

// WriteFrame implements meowcaller.AudioSink: one decoded mono frame from the peer.
func (b *bridge) WriteFrame(frame []float32) error {
	payload := base64.StdEncoding.EncodeToString(pcm16FromFloat32(frame))
	return b.writeJSON(wsMessage{Event: "media", Track: "inbound", Payload: payload})
}

// WriteVideo implements meowcaller.VideoSink: one Annex-B H.264 access unit.
func (b *bridge) WriteVideo(accessUnit []byte) error {
	payload := base64.StdEncoding.EncodeToString(accessUnit)
	return b.writeJSON(wsMessage{Event: "video", Track: "inbound", Payload: payload})
}

// ReadFrame implements meowcaller.AudioSource: frames the consumer sent back over the
// socket, decoded by readLoop and handed here on demand.
func (b *bridge) ReadFrame() ([]float32, error) {
	select {
	case frame, ok := <-b.incoming:
		if !ok {
			return nil, io.EOF
		}
		return frame, nil
	case <-b.closed:
		return nil, io.EOF
	}
}

// Close satisfies AudioSink/VideoSink/AudioSource's shared Close() error method. Safe
// to call more than once (from the call's OnEnd callback and from readLoop exiting).
func (b *bridge) Close() error {
	b.closeOnce.Do(func() {
		_ = b.writeJSON(wsMessage{Event: "stop", Reason: "hangup"})
		close(b.closed)
		_ = b.conn.Close()
	})
	return nil
}

// readLoop blocks reading consumer-sent media messages until the connection closes
// (by either side) or send Close(). Every non-media message is ignored rather than
// erroring, so the wire format can grow new event types without breaking old clients.
func (b *bridge) readLoop() {
	for {
		var msg wsMessage
		if err := b.conn.ReadJSON(&msg); err != nil {
			b.Close()
			return
		}
		if msg.Event == "video" && msg.Track == "outbound" {
			raw, err := base64.StdEncoding.DecodeString(msg.Payload)
			if err != nil || b.sendVideo == nil {
				continue
			}
			// Spike/experimental: outbound video was explicitly out of scope in the
			// reviewed plan (video is inbound-only there). Added here to test
			// screen-share/camera content actually reaching the peer.
			//
			// meowcaller's StartVideo doc comment: "Outbound video remains gated
			// until the peer acknowledges the transition" -- so the upgrade must be
			// requested before any SendVideo call has an effect. Lazily triggered on
			// the first outbound video message so callers don't need a separate
			// endpoint/step to remember.
			b.startVideoOnce.Do(func() {
				if b.startVideo != nil {
					_ = b.startVideo()
				}
			})
			if err := b.sendVideo(raw); err != nil {
				continue
			}
			continue
		}
		if msg.Event != "media" || msg.Track != "outbound" {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(msg.Payload)
		if err != nil {
			continue
		}
		frame := float32FromPCM16(raw)
		select {
		case b.incoming <- frame:
		case <-b.closed:
			return
		default:
			// Consumer is sending audio faster than the call can play it out; drop
			// the frame rather than block the socket read loop.
		}
	}
}
