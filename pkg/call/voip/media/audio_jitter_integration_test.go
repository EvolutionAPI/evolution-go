package media

import (
	"sync"
	"testing"
	"time"
)

type jitterIntegrationCodec struct {
	mu              sync.Mutex
	decodedPayloads [][]byte
	closed          bool
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
	for index := range pcm {
		pcm[index] = value
	}
	return pcm, nil
}

func (c *jitterIntegrationCodec) FrameSize() int  { return MLowFrameSize }
func (c *jitterIntegrationCodec) SampleRate() int { return MLowSampleRate }
func (c *jitterIntegrationCodec) Close() {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
}

func TestAudioRegistryReordersAndConcealsBeforePCM(t *testing.T) {
	codec := &jitterIntegrationCodec{}
	options := &AudioRegistryOptions{
		CodecFactory:  func(CodecOptions) (Codec, error) { return codec, nil },
		DisableSilence: true,
		Jitter: JitterBufferOptions{
			FrameDuration:         3 * time.Millisecond,
			InitialDelayPackets:   2,
			MaxPackets:            8,
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
		CodecFactory:  func(CodecOptions) (Codec, error) { return codec, nil },
		DisableSilence: true,
		Jitter: JitterBufferOptions{
			FrameDuration:         3 * time.Millisecond,
			InitialDelayPackets:   1,
			MaxPackets:            8,
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
		CodecFactory:  func(CodecOptions) (Codec, error) { return codec, nil },
		DisableSilence: true,
		Jitter: JitterBufferOptions{
			FrameDuration:         time.Millisecond,
			InitialDelayPackets:   1,
			MaxPackets:            8,
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
