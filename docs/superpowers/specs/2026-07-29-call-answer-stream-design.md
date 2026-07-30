# Design: Answer WhatsApp Calls + Bidirectional Media Stream

Date: 2026-07-29
Status: Approved for planning

## Problem

Evolution Go exposes `POST /call/reject` (see `pkg/call/handler/call_handler.go` and
`pkg/call/service/call_service.go`) but has no way to accept an incoming call or access
its audio/video. This is not a gap specific to Evolution Go: the underlying
`go.mau.fi/whatsmeow` client only parses call signaling (offer/accept/preaccept/
transport/terminate) and can reject — it never implemented accepting a call or handling
the actual encrypted media stream. Building that from scratch means reverse-engineering
WhatsApp's proprietary call media protocol, which is a multi-year, multi-attempt effort
documented in `tulir/whatsmeow#555`.

`github.com/purpshell/meowcaller` (MIT, actively maintained, pure Go, no CGO) already
did this work as an interoperability research project. It wraps a `*whatsmeow.Client`
and exposes: `OnIncomingCall`, `Call.Answer()`, `Call.Reject()`, `Call.Hangup()`,
`Call.AcceptVideo()`/`StartVideo()`, `Call.Play(AudioSource)`, `Call.Receive(AudioSink)`,
`Call.ReceiveVideo(VideoSink)`. Audio is delivered as 16 kHz mono `float32` PCM in
960-sample (60 ms) frames (`FrameSamples = 960`, `SampleRate = 16000` in `audio.go`);
video is H.264 Annex B access units (`VideoSink.WriteVideo([]byte)`).

Goal: add generic infrastructure to Evolution Go so an incoming call can be answered and
its audio/video piped to an external system (an AI voice pipeline, a human operator
console, a recorder — Evolution Go stays agnostic to what's on the other end). This
mirrors how Evolution already lets Chatwoot/Typebot/OpenAI plug into messaging via
webhook/websocket/RabbitMQ/NATS — except the existing event producers are unidirectional,
fire-and-forget JSON events, unsuited to a sustained, low-latency, two-way audio/video
stream. This feature therefore needs its own dedicated channel, not a new event type
bolted onto the existing producers.

Explicitly out of scope: no AI/STT/LLM/TTS logic lives in Evolution Go. This PR only
ships the plumbing; whatever consumes the stream is the caller's problem.

## Constraint that shapes the architecture

`meowcaller.NewClient(wa)` must be called **before** `wa.Connect()` — the doc comment on
`NewClient` is explicit: "Construct it before connecting whatsmeow so the raw call
adapter can be installed safely." In `pkg/whatsmeow/service/whatsmeow.go`, the
whatsmeow client is created and stored at lines 413–415
(`client := whatsmeow.NewClient(...)`; `w.clientPointer[cd.Instance.Id] = client`), and
`.Connect()` is called later in several branches (lines ~507–559 and ~2061). This means
the meowcaller client cannot be lazily created inside `pkg/call/service` the way
`ensureClientConnected` currently lazily starts instances — it must be created in
`whatsmeow.go` at client-creation time, alongside `clientPointer`.

## Components

Following the existing Handler → Service → Repository convention
(`docs/wiki/desenvolvimento/contributing.md`):

- **`pkg/whatsmeow/service`**: add `meowcallerPointer map[string]*meowcaller.Client`
  alongside the existing `clientPointer`. Populated at the same point `clientPointer` is
  populated, before `Connect()`. Exposes a getter so `pkg/call/service` can look up the
  meowcaller client for an instance.
- **`pkg/call/service`**: add a call registry (`map[callID]*meowcaller.Call`, mutex-
  guarded, matching the style of `CallRegistry` internal to meowcaller but scoped to
  what Evolution Go needs: lookup by callID for answer/hangup/stream-attach). New
  interface methods:
  - `AnswerCall(data *AnswerCallStruct, instance *Instance) (*meowcaller.Call, error)`
  - `HangupCall(data *HangupCallStruct, instance *Instance) error`
  - `GetActiveCall(instanceId, callId string) (*meowcaller.Call, error)` — used by the
    stream bridge.
- **`pkg/call/handler`**: new REST endpoints, same shape as the existing `RejectCall`:
  - `POST /call/answer` — body `{callCreator, callId}`. Calls `AnswerCall`; if the
    incoming call's `IsVideo()` is true, also calls `Call.AcceptVideo()`.
  - `POST /call/hangup` — body `{callId}`.
- **`pkg/call/stream`** (new package): the WebSocket bridge. One connection per call.
  Implements `meowcaller.AudioSink`/`VideoSink` (writes frames out to the socket) and
  `meowcaller.AudioSource` (reads frames the remote client sends back, feeds
  `Call.Play`).
- **`pkg/routes/routes.go`**: register `POST /call/answer`, `POST /call/hangup` in the
  existing `/call` group (auth middleware applies, same as `/call/reject`), and
  `GET /call/stream/:callId` as its own route (WS upgrade, can't sit behind the same
  auth middleware chain — needs upgrade-compatible auth, see below).

## Wire format (`GET /call/stream/:callId?apikey=...`)

Modeled on Twilio Media Streams, since AI voice vendors' existing WhatsApp/Twilio
bridges already speak a message shape like this — minimizes adapter work for whoever
consumes it:

```json
{"event": "start", "callId": "...", "sampleRate": 16000, "video": true}
{"event": "media", "track": "inbound",  "payload": "<base64 PCM16LE, 960 samples/frame>"}
{"event": "media", "track": "outbound", "payload": "<base64 PCM16LE, 960 samples/frame>"}
{"event": "video", "track": "inbound",  "payload": "<base64 H.264 Annex B access unit>"}
{"event": "stop", "reason": "hangup"}
```

The bridge converts `meowcaller`'s `[]float32` frames to PCM16LE for the `inbound`
track, and decodes base64 PCM16LE from `outbound` messages back to `[]float32` for
`Call.Play`. Video is inbound-only in this PR (receiving/recording the peer's video);
sending synthetic video back is not implemented (no known driver for it here — call
audio is the primary use case, video send can be a follow-up if someone needs it).

## Auth

WebSocket upgrades can't rely on custom headers from every possible client, so this
follows the existing pattern in `pkg/events/websocket/websocket_producer.go`: apikey as
a query parameter, validated against the instance before the upgrade completes.

## Lifecycle / error handling

- Remote hangs up → meowcaller's engine dispatches `CallTerminate` → bridge sends
  `{"event":"stop","reason":"hangup"}` and closes the socket.
- Consumer's WebSocket closes (crash, disconnect) → bridge calls `Call.Hangup()` so the
  WhatsApp call doesn't hang open with no one listening.
- `POST /call/answer` for an unknown/expired callId → `404`, same convention as
  `RejectCall`'s error handling.
- `GET /call/stream/:callId` for a call not yet answered, or already terminated →
  reject the upgrade with `4004`-equivalent close code.

## Testing

- Unit tests in `pkg/call/service` and `pkg/call/stream` against a small interface
  wrapping the `meowcaller.Client`/`Call` types (mockable), covering answer/hangup/
  registry lookup and the PCM16LE ⇄ float32 conversion in the bridge.
- Manual validation (real WhatsApp test number, since this can't be meaningfully faked):
  place a call to the number connected to a test instance, `POST /call/answer`, connect
  a throwaway WS client that dumps the `inbound` track to a WAV file, confirm the
  recording is intelligible speech.

## PR plan

Branch `feature/call-answer-stream` off `main` in the fork
(`RamonBritoDev/evolution-go`), Conventional Commits (`feat: ...`), PR against
`evolution-foundation/evolution-go` per `docs/wiki/desenvolvimento/contributing.md`.
The PR description calls out the new `meowcaller` dependency (MIT-licensed, compatible
with Evolution Go's Apache-2.0 license), links `meowcaller`'s `DISCLAIMER.md` (framed as
independent interoperability research, not affiliated with Meta, with explicit misuse
ground rules), and states clearly that this PR ships transport/plumbing only — no AI
logic — so reviewers aren't evaluating an opinionated AI stack.
