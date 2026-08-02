package media

import (
	"math"
	"sync"
	"testing"
	"time"
)

type fakeAudioCodec struct {
	mu      sync.Mutex
	frames  [][]float32
	closed  bool
	decoded float32
}

func (c *fakeAudioCodec) Encode(pcm []float32) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	frame := append([]float32(nil), pcm...)
	c.frames = append(c.frames, frame)
	return []byte{byte(len(c.frames)), 0x7f}, nil
}

func (c *fakeAudioCodec) Decode(frame []byte) ([]float32, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pcm := make([]float32, MLowFrameSize)
	for index := range pcm {
		pcm[index] = c.decoded
	}
	return pcm, nil
}

func (c *fakeAudioCodec) FrameSize() int  { return MLowFrameSize }
func (c *fakeAudioCodec) SampleRate() int { return MLowSampleRate }
func (c *fakeAudioCodec) Close() {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
}

type sentAudioFrame struct {
	payload  []byte
	duration uint32
	marker   bool
}

func TestAudioRegistryBuffersSanitizesAndSendsPCM(t *testing.T) {
	codec := &fakeAudioCodec{}
	var mu sync.Mutex
	var sent []sentAudioFrame
	options := &AudioRegistryOptions{
		CodecFactory: func(CodecOptions) (Codec, error) { return codec, nil },
		DisableSilence: true,
	}
	registry := NewAudioRegistry(func(_ string, _ string, payload []byte, duration uint32, marker bool) error {
		mu.Lock()
		sent = append(sent, sentAudioFrame{payload: append([]byte(nil), payload...), duration: duration, marker: marker})
		mu.Unlock()
		return nil
	}, options)
	defer registry.Close("instance")

	first := make([]float32, MLowFrameSize/2)
	first[0] = float32(math.NaN())
	first[1] = 2
	if err := registry.FeedPCM("instance", "call", first); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if len(sent) != 0 {
		t.Fatalf("partial PCM unexpectedly sent %d frames", len(sent))
	}
	mu.Unlock()

	second := make([]float32, MLowFrameSize/2)
	second[0] = -2
	if err := registry.FeedPCM("instance", "call", second); err != nil {
		t.Fatal(err)
	}
	if err := registry.FeedPCM("instance", "call", make([]float32, MLowFrameSize)); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 2 {
		t.Fatalf("sent %d frames, want 2", len(sent))
	}
	if !sent[0].marker || sent[1].marker {
		t.Fatalf("unexpected marker sequence: %+v", sent)
	}
	if sent[0].duration != MLowFrameSize {
		t.Fatalf("duration=%d, want %d", sent[0].duration, MLowFrameSize)
	}

	codec.mu.Lock()
	defer codec.mu.Unlock()
	if len(codec.frames) != 2 {
		t.Fatalf("codec received %d frames", len(codec.frames))
	}
	if codec.frames[0][0] != 0 || codec.frames[0][1] != 1 || codec.frames[0][MLowFrameSize/2] != -1 {
		t.Fatalf("PCM sanitization failed: %v %v %v", codec.frames[0][0], codec.frames[0][1], codec.frames[0][MLowFrameSize/2])
	}
}

func TestAudioRegistryDecodesRTPToPCMCallback(t *testing.T) {
	codec := &fakeAudioCodec{decoded: 0.25}
	options := &AudioRegistryOptions{
		CodecFactory: func(CodecOptions) (Codec, error) { return codec, nil },
		DisableSilence: true,
	}
	registry := NewAudioRegistry(func(string, string, []byte, uint32, bool) error { return nil }, options)
	defer registry.Close("instance")

	called := make(chan []float32, 1)
	registry.SetOnPCM(func(instanceID, callID string, pcm []float32) {
		if instanceID != "instance" || callID != "call" {
			t.Errorf("unexpected callback identity %s/%s", instanceID, callID)
		}
		called <- append([]float32(nil), pcm...)
	})
	packet := &RTPPacket{Header: &RTPHeader{Version: 2}, Payload: []byte{1, 2, 3}}
	if err := registry.HandleRTP("instance", "call", packet); err != nil {
		t.Fatal(err)
	}
	select {
	case pcm := <-called:
		if len(pcm) != MLowFrameSize || pcm[0] != 0.25 {
			t.Fatalf("unexpected decoded PCM: len=%d first=%v", len(pcm), pcm[0])
		}
	case <-time.After(time.Second):
		t.Fatal("PCM callback was not invoked")
	}
}

func TestAudioRegistrySendsSilenceWhileIdle(t *testing.T) {
	codec := &fakeAudioCodec{}
	sent := make(chan sentAudioFrame, 4)
	options := &AudioRegistryOptions{
		CodecFactory: func(CodecOptions) (Codec, error) { return codec, nil },
		SilenceTick:  5 * time.Millisecond,
		SilenceAfter: 5 * time.Millisecond,
	}
	registry := NewAudioRegistry(func(_ string, _ string, payload []byte, duration uint32, marker bool) error {
		sent <- sentAudioFrame{payload: append([]byte(nil), payload...), duration: duration, marker: marker}
		return nil
	}, options)
	if err := registry.Prepare("instance", "call"); err != nil {
		t.Fatal(err)
	}
	defer registry.Close("instance")

	select {
	case frame := <-sent:
		if !frame.marker || frame.duration != MLowFrameSize {
			t.Fatalf("unexpected silence frame: %+v", frame)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("silence keepalive was not sent")
	}
}

func TestAudioRegistryRemoveWaitsForInFlightSend(t *testing.T) {
	codec := &fakeAudioCodec{}
	entered := make(chan struct{})
	release := make(chan struct{})
	options := &AudioRegistryOptions{
		CodecFactory: func(CodecOptions) (Codec, error) { return codec, nil },
		DisableSilence: true,
	}
	registry := NewAudioRegistry(func(string, string, []byte, uint32, bool) error {
		close(entered)
		<-release
		return nil
	}, options)

	feedDone := make(chan error, 1)
	go func() {
		feedDone <- registry.FeedPCM("instance", "call", make([]float32, MLowFrameSize))
	}()
	<-entered
	removeDone := make(chan struct{})
	go func() {
		registry.Remove("instance", "call")
		close(removeDone)
	}()

	select {
	case <-removeDone:
		t.Fatal("Remove returned while the sender was still in flight")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-feedDone; err != nil {
		t.Fatal(err)
	}
	select {
	case <-removeDone:
	case <-time.After(time.Second):
		t.Fatal("Remove did not finish after the sender returned")
	}

	codec.mu.Lock()
	closed := codec.closed
	codec.mu.Unlock()
	if !closed {
		t.Fatal("codec was not closed")
	}
}
