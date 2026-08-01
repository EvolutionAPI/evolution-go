#!/usr/bin/env python3
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
AUDIO = ROOT / "pkg/call/voip/media/audio_pipeline.go"
COORDINATOR = ROOT / "pkg/call/lifecycle/coordinator.go"
TEST = ROOT / "pkg/call/voip/media/audio_jitter_integration_test.go"
WORKFLOW = ROOT / ".github/workflows/integrate-audio-jitter.yml"


def replace_once(text: str, old: str, new: str, label: str) -> str:
    count = text.count(old)
    if count != 1:
        raise RuntimeError(f"{label}: expected one match, found {count}")
    return text.replace(old, new, 1)


audio = AUDIO.read_text(encoding="utf-8")
audio = replace_once(
    audio,
    '''type AudioRegistryOptions struct {
\tCodecFactory   CodecFactory
\tCodecOptions   CodecOptions
\tSilenceTick    time.Duration
\tSilenceAfter   time.Duration
\tDisableSilence bool
}''',
    '''type AudioRegistryOptions struct {
\tCodecFactory   CodecFactory
\tCodecOptions   CodecOptions
\tSilenceTick    time.Duration
\tSilenceAfter   time.Duration
\tDisableSilence bool
\tJitter         JitterBufferOptions
}''',
    "audio options",
)
audio = replace_once(
    audio,
    '''\t\tCodecFactory: NewMLowCodec,
\t\tCodecOptions: DefaultCodecOptions,
\t\tSilenceTick:  60 * time.Millisecond,
\t\tSilenceAfter: 120 * time.Millisecond,
''',
    '''\t\tCodecFactory: NewMLowCodec,
\t\tCodecOptions: DefaultCodecOptions,
\t\tSilenceTick:  60 * time.Millisecond,
\t\tSilenceAfter: 120 * time.Millisecond,
\t\tJitter:       DefaultJitterBufferOptions(),
''',
    "default jitter options",
)
audio = replace_once(
    audio,
    '''\tinstanceID string
\tcallID     string
\tcodec      Codec
\tsender     EncodedAudioSender

\tencodeBuffer []float32''',
    '''\tinstanceID string
\tcallID     string
\tcodec      Codec
\tsender     EncodedAudioSender
\tonPCM      func([]float32)
\tjitter     *JitterBuffer

\tencodeBuffer []float32''',
    "audio session fields",
)
audio = replace_once(
    audio,
    '''func newAudioSession(instanceID, callID string, codec Codec, sender EncodedAudioSender, options AudioRegistryOptions) *audioSession {
\tsession := &audioSession{''',
    '''func newAudioSession(instanceID, callID string, codec Codec, sender EncodedAudioSender, onPCM func([]float32), options AudioRegistryOptions) *audioSession {
\tsession := &audioSession{''',
    "audio session signature",
)
audio = replace_once(
    audio,
    '''\t\tcodec:        codec,
\t\tsender:       sender,
\t\tencodeBuffer: make([]float32, codec.FrameSize()),''',
    '''\t\tcodec:        codec,
\t\tsender:       sender,
\t\tonPCM:        onPCM,
\t\tencodeBuffer: make([]float32, codec.FrameSize()),''',
    "audio session initialization",
)
audio = replace_once(
    audio,
    '''\tif options.DisableSilence || options.SilenceTick <= 0 || options.SilenceAfter <= 0 {
\t\tclose(session.doneCh)
\t} else {
\t\tgo session.silenceLoop()
\t}
\treturn session
}''',
    '''\tsession.jitter = NewJitterBuffer(&options.Jitter, session.handleJitterFrame)
\tif options.DisableSilence || options.SilenceTick <= 0 || options.SilenceAfter <= 0 {
\t\tclose(session.doneCh)
\t} else {
\t\tgo session.silenceLoop()
\t}
\treturn session
}''',
    "jitter creation",
)
audio = replace_once(
    audio,
    '''func (s *audioSession) handleRTP(packet *RTPPacket) ([]float32, error) {
\tif s == nil || packet == nil || packet.Header == nil {
\t\treturn nil, ErrAudioSessionNotReady
\t}
\ts.mu.Lock()
\tdefer s.mu.Unlock()
\tif s.closed || s.codec == nil {
\t\treturn nil, ErrAudioSessionNotReady
\t}
\tdecoded, err := s.codec.Decode(packet.Payload)
\tif err != nil {
\t\treturn nil, fmt.Errorf("decode MLow payload: %w", err)
\t}
\treturn NormalizeFrame(decoded, s.codec.FrameSize()), nil
}''',
    '''func (s *audioSession) handleRTP(packet *RTPPacket) error {
\tif s == nil || packet == nil || packet.Header == nil {
\t\treturn ErrAudioSessionNotReady
\t}
\ts.mu.Lock()
\tif s.closed || s.codec == nil || s.jitter == nil {
\t\ts.mu.Unlock()
\t\treturn ErrAudioSessionNotReady
\t}
\tjitter := s.jitter
\ts.mu.Unlock()
\treturn jitter.Push(packet)
}

func (s *audioSession) handleJitterFrame(frame JitterFrame) {
\tif s == nil {
\t\treturn
\t}
\ts.mu.Lock()
\tif s.closed || s.codec == nil {
\t\ts.mu.Unlock()
\t\treturn
\t}
\tpayload := frame.Payload
\tif frame.Concealed {
\t\tpayload = nil
\t}
\tdecoded, err := s.codec.Decode(payload)
\tif err != nil {
\t\ts.mu.Unlock()
\t\treturn
\t}
\tpcm := NormalizeFrame(decoded, s.codec.FrameSize())
\tcallback := s.onPCM
\ts.mu.Unlock()
\tdefer zeroFloat32(pcm)
\tif callback != nil {
\t\tcallback(append([]float32(nil), pcm...))
\t}
}

func (s *audioSession) jitterStats() JitterBufferStats {
\tif s == nil {
\t\treturn JitterBufferStats{}
\t}
\ts.mu.Lock()
\tjitter := s.jitter
\ts.mu.Unlock()
\tif jitter == nil {
\t\treturn JitterBufferStats{}
\t}
\treturn jitter.Stats()
}''',
    "replace direct RTP decode",
)
audio = replace_once(
    audio,
    '''func (s *audioSession) close() {
\tif s == nil {
\t\treturn
\t}
\ts.stopOnce.Do(func() { close(s.stopCh) })
\t<-s.doneCh

\ts.mu.Lock()
\tif !s.closed {
\t\ts.closed = true
\t\tif s.codec != nil {
\t\t\ts.codec.Close()
\t\t}
\t\tzeroFloat32(s.encodeBuffer)
\t\ts.codec = nil
\t\ts.sender = nil
\t\ts.encodeBuffer = nil
\t\ts.encodePos = 0
\t\ts.marker = false
\t\ts.lastCapture = time.Time{}
\t}
\ts.mu.Unlock()
}''',
    '''func (s *audioSession) close() {
\tif s == nil {
\t\treturn
\t}
\ts.stopOnce.Do(func() { close(s.stopCh) })
\t<-s.doneCh

\ts.mu.Lock()
\tif s.closed {
\t\ts.mu.Unlock()
\t\treturn
\t}
\ts.closed = true
\tjitter := s.jitter
\ts.jitter = nil
\ts.mu.Unlock()

\tif jitter != nil {
\t\tjitter.Close()
\t}

\ts.mu.Lock()
\tif s.codec != nil {
\t\ts.codec.Close()
\t}
\tzeroFloat32(s.encodeBuffer)
\ts.codec = nil
\ts.sender = nil
\ts.onPCM = nil
\ts.encodeBuffer = nil
\ts.encodePos = 0
\ts.marker = false
\ts.lastCapture = time.Time{}
\ts.mu.Unlock()
}''',
    "jitter teardown",
)
audio = replace_once(
    audio,
    '''\tcandidate := newAudioSession(instanceID, callID, codec, r.sender, r.options)
''',
    '''\tcandidate := newAudioSession(instanceID, callID, codec, r.sender, func(pcm []float32) {
\t\tr.emitPCM(instanceID, callID, pcm)
\t}, r.options)
''',
    "audio session candidate",
)
audio = replace_once(
    audio,
    '''func (r *AudioRegistry) HandleRTP(instanceID, callID string, packet *RTPPacket) error {
\tsession, err := r.session(instanceID, callID, true)
\tif err != nil {
\t\treturn err
\t}
\tpcm, err := session.handleRTP(packet)
\tif err != nil {
\t\treturn err
\t}
\tdefer zeroFloat32(pcm)

\tr.mu.RLock()
\tcallback := r.onPCM
\tr.mu.RUnlock()
\tif callback != nil {
\t\tcallback(instanceID, callID, append([]float32(nil), pcm...))
\t}
\treturn nil
}''',
    '''func (r *AudioRegistry) HandleRTP(instanceID, callID string, packet *RTPPacket) error {
\tsession, err := r.session(instanceID, callID, true)
\tif err != nil {
\t\treturn err
\t}
\terr = session.handleRTP(packet)
\tif errors.Is(err, ErrJitterDuplicatePacket) || errors.Is(err, ErrJitterLatePacket) {
\t\treturn nil
\t}
\treturn err
}

func (r *AudioRegistry) emitPCM(instanceID, callID string, pcm []float32) {
\tr.mu.RLock()
\tcallback := r.onPCM
\tr.mu.RUnlock()
\tif callback != nil {
\t\tcallback(instanceID, callID, append([]float32(nil), pcm...))
\t}
}

func (r *AudioRegistry) JitterStats(instanceID, callID string) (JitterBufferStats, bool) {
\tsession, err := r.session(instanceID, callID, false)
\tif err != nil {
\t\treturn JitterBufferStats{}, false
\t}
\treturn session.jitterStats(), true
}''',
    "registry jitter handling",
)
AUDIO.write_text(audio, encoding="utf-8")

coordinator = COORDINATOR.read_text(encoding="utf-8")
coordinator = replace_once(
    coordinator,
    '''func (c *Coordinator) SetOnPCM(callback func(instanceID, callID string, pcm []float32)) {
\tif c != nil {
\t\tc.audio.SetOnPCM(callback)
\t}
}
''',
    '''func (c *Coordinator) SetOnPCM(callback func(instanceID, callID string, pcm []float32)) {
\tif c != nil {
\t\tc.audio.SetOnPCM(callback)
\t}
}

func (c *Coordinator) JitterStats(instanceID, callID string) (call_media.JitterBufferStats, bool) {
\tif c == nil {
\t\treturn call_media.JitterBufferStats{}, false
\t}
\treturn c.audio.JitterStats(instanceID, callID)
}
''',
    "coordinator jitter stats",
)
COORDINATOR.write_text(coordinator, encoding="utf-8")

TEST.write_text('''package media

import (
    "sync"
    "testing"
    "time"
)

type jitterIntegrationCodec struct {
    mu sync.Mutex
    decodedPayloads [][]byte
    closed bool
}

func (c *jitterIntegrationCodec) Encode(pcm []float32) ([]byte, error) {
    return []byte{1}, nil
}

func (c *jitterIntegrationCodec) Decode(payload []byte) ([]float32, error) {
    c.mu.Lock()
    c.decodedPayloads = append(c.decodedPayloads, append([]byte(nil), payload...))
    c.mu.Unlock()
    value := float32(-1)
    if len(payload) > 0 {
        value = float32(payload[0])
    }
    pcm := make([]float32, MLowFrameSize)
    for i := range pcm {
        pcm[i] = value
    }
    return pcm, nil
}

func (c *jitterIntegrationCodec) FrameSize() int { return MLowFrameSize }
func (c *jitterIntegrationCodec) SampleRate() int { return MLowSampleRate }
func (c *jitterIntegrationCodec) Close() {
    c.mu.Lock()
    c.closed = true
    c.mu.Unlock()
}

func TestAudioRegistryReordersAndConcealsBeforePCM(t *testing.T) {
    codec := &jitterIntegrationCodec{}
    options := &AudioRegistryOptions{
        CodecFactory: func(CodecOptions) (Codec, error) { return codec, nil },
        DisableSilence: true,
        Jitter: JitterBufferOptions{
            FrameDuration: 3 * time.Millisecond,
            InitialDelayPackets: 2,
            MaxPackets: 8,
            MaxConcealmentPackets: 2,
        },
    }
    registry := NewAudioRegistry(func(string, string, []byte, uint32, bool) error { return nil }, options)
    defer registry.Close("instance")

    pcm := make(chan float32, 4)
    registry.SetOnPCM(func(_, _ string, samples []float32) {
        pcm <- samples[0]
    })

    if err := registry.HandleRTP("instance", "call", jitterPacket(102, 2920, 3)); err != nil {
        t.Fatal(err)
    }
    if err := registry.HandleRTP("instance", "call", jitterPacket(100, 1000, 1)); err != nil {
        t.Fatal(err)
    }

    want := []float32{1, -1, 3}
    for index, expected := range want {
        select {
        case actual := <-pcm:
            if actual != expected {
                t.Fatalf("frame %d decoded %v, want %v", index, actual, expected)
            }
        case <-time.After(time.Second):
            t.Fatalf("timed out waiting for PCM frame %d", index)
        }
    }

    stats, ok := registry.JitterStats("instance", "call")
    if !ok || stats.Delivered != 2 || stats.Concealed != 1 {
        t.Fatalf("unexpected jitter stats: ok=%v stats=%+v", ok, stats)
    }
}

func TestAudioRegistryIgnoresDuplicateAndLatePackets(t *testing.T) {
    codec := &jitterIntegrationCodec{}
    options := &AudioRegistryOptions{
        CodecFactory: func(CodecOptions) (Codec, error) { return codec, nil },
        DisableSilence: true,
        Jitter: JitterBufferOptions{
            FrameDuration: 3 * time.Millisecond,
            InitialDelayPackets: 1,
            MaxPackets: 8,
            MaxConcealmentPackets: 1,
        },
    }
    registry := NewAudioRegistry(func(string, string, []byte, uint32, bool) error { return nil }, options)
    defer registry.Close("instance")

    packet := jitterPacket(200, 1000, 1)
    if err := registry.HandleRTP("instance", "call", packet); err != nil {
        t.Fatal(err)
    }
    if err := registry.HandleRTP("instance", "call", packet); err != nil {
        t.Fatalf("duplicate should be ignored, got %v", err)
    }
    time.Sleep(15 * time.Millisecond)
    if err := registry.HandleRTP("instance", "call", packet); err != nil {
        t.Fatalf("late packet should be ignored, got %v", err)
    }

    stats, ok := registry.JitterStats("instance", "call")
    if !ok || stats.Duplicate != 1 || stats.Late != 1 {
        t.Fatalf("unexpected jitter stats: ok=%v stats=%+v", ok, stats)
    }
}

func TestAudioRegistryCloseStopsPlayoutBeforeCodecClose(t *testing.T) {
    codec := &jitterIntegrationCodec{}
    options := &AudioRegistryOptions{
        CodecFactory: func(CodecOptions) (Codec, error) { return codec, nil },
        DisableSilence: true,
        Jitter: JitterBufferOptions{
            FrameDuration: time.Millisecond,
            InitialDelayPackets: 1,
            MaxPackets: 8,
            MaxConcealmentPackets: 2,
        },
    }
    registry := NewAudioRegistry(func(string, string, []byte, uint32, bool) error { return nil }, options)
    if err := registry.HandleRTP("instance", "call", jitterPacket(300, 1000, 1)); err != nil {
        t.Fatal(err)
    }
    registry.Remove("instance", "call")

    codec.mu.Lock()
    closed := codec.closed
    codec.mu.Unlock()
    if !closed {
        t.Fatal("codec was not closed after jitter playout stopped")
    }
}
''', encoding="utf-8")

Path(__file__).unlink()
WORKFLOW.unlink()
