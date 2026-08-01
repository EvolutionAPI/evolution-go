// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package media

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

var (
	ErrAudioSessionNotReady   = errors.New("audio codec session is not ready")
	ErrAudioSenderUnavailable = errors.New("encoded audio sender is unavailable")
)

type CodecFactory func(options CodecOptions) (Codec, error)

type EncodedAudioSender func(instanceID, callID string, payload []byte, durationSamples uint32, marker bool) error

type PCMCallback func(instanceID, callID string, pcm []float32)

type AudioRegistryOptions struct {
	CodecFactory   CodecFactory
	CodecOptions   CodecOptions
	SilenceTick    time.Duration
	SilenceAfter   time.Duration
	DisableSilence bool
}

func DefaultAudioRegistryOptions() AudioRegistryOptions {
	return AudioRegistryOptions{
		CodecFactory: NewMLowCodec,
		CodecOptions: DefaultCodecOptions,
		SilenceTick:  60 * time.Millisecond,
		SilenceAfter: 120 * time.Millisecond,
	}
}

type audioSession struct {
	mu sync.Mutex

	instanceID string
	callID     string
	codec      Codec
	sender     EncodedAudioSender

	encodeBuffer []float32
	encodePos    int
	marker       bool
	lastCapture  time.Time
	closed       bool

	silenceTick  time.Duration
	silenceAfter time.Duration
	stopCh       chan struct{}
	doneCh       chan struct{}
	stopOnce     sync.Once
}

func newAudioSession(instanceID, callID string, codec Codec, sender EncodedAudioSender, options AudioRegistryOptions) *audioSession {
	session := &audioSession{
		instanceID:   instanceID,
		callID:       callID,
		codec:        codec,
		sender:       sender,
		encodeBuffer: make([]float32, codec.FrameSize()),
		marker:       true,
		lastCapture:  time.Now(),
		silenceTick:  options.SilenceTick,
		silenceAfter: options.SilenceAfter,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
	if options.DisableSilence || options.SilenceTick <= 0 || options.SilenceAfter <= 0 {
		close(session.doneCh)
	} else {
		go session.silenceLoop()
	}
	return session
}

func (s *audioSession) feedPCM(pcm []float32) error {
	if s == nil {
		return ErrAudioSessionNotReady
	}
	if len(pcm) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.codec == nil {
		return ErrAudioSessionNotReady
	}
	if s.sender == nil {
		return ErrAudioSenderUnavailable
	}

	s.lastCapture = time.Now()
	offset := 0
	for offset < len(pcm) {
		remaining := s.codec.FrameSize() - s.encodePos
		count := min(remaining, len(pcm)-offset)
		for index := 0; index < count; index++ {
			s.encodeBuffer[s.encodePos+index] = sanitizePCMSample(pcm[offset+index])
		}
		s.encodePos += count
		offset += count
		if s.encodePos != s.codec.FrameSize() {
			continue
		}
		if err := s.encodeAndSendLocked(s.encodeBuffer); err != nil {
			return err
		}
		zeroFloat32(s.encodeBuffer)
		s.encodePos = 0
	}
	return nil
}

func (s *audioSession) handleRTP(packet *RTPPacket) ([]float32, error) {
	if s == nil || packet == nil || packet.Header == nil {
		return nil, ErrAudioSessionNotReady
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.codec == nil {
		return nil, ErrAudioSessionNotReady
	}
	decoded, err := s.codec.Decode(packet.Payload)
	if err != nil {
		return nil, fmt.Errorf("decode MLow payload: %w", err)
	}
	return NormalizeFrame(decoded, s.codec.FrameSize()), nil
}

func (s *audioSession) encodeAndSendLocked(frame []float32) error {
	encoded, err := s.codec.Encode(frame)
	if err != nil {
		return fmt.Errorf("encode PCM frame: %w", err)
	}
	if len(encoded) == 0 {
		return nil
	}
	defer zeroBytes(encoded)
	if err = s.sender(s.instanceID, s.callID, encoded, uint32(s.codec.FrameSize()), s.marker); err != nil {
		return fmt.Errorf("send encoded audio frame: %w", err)
	}
	s.marker = false
	return nil
}

func (s *audioSession) silenceLoop() {
	defer close(s.doneCh)
	ticker := time.NewTicker(s.silenceTick)
	defer ticker.Stop()
	silence := make([]float32, s.codec.FrameSize())
	defer zeroFloat32(silence)

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.mu.Lock()
			if s.closed {
				s.mu.Unlock()
				return
			}
			idle := time.Since(s.lastCapture) >= s.silenceAfter
			ready := s.codec != nil && s.sender != nil
			if idle && ready {
				_ = s.encodeAndSendLocked(silence)
			}
			s.mu.Unlock()
		}
	}
}

func (s *audioSession) close() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	<-s.doneCh

	s.mu.Lock()
	if !s.closed {
		s.closed = true
		if s.codec != nil {
			s.codec.Close()
		}
		zeroFloat32(s.encodeBuffer)
		s.codec = nil
		s.sender = nil
		s.encodeBuffer = nil
		s.encodePos = 0
		s.marker = false
		s.lastCapture = time.Time{}
	}
	s.mu.Unlock()
}

func sanitizePCMSample(sample float32) float32 {
	value := float64(sample)
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if sample > 1 {
		return 1
	}
	if sample < -1 {
		return -1
	}
	return sample
}

// AudioRegistry owns one codec and PCM accumulator per call. It is independent
// from HTTP and device APIs so browser, native and test bridges can reuse it.
type AudioRegistry struct {
	mu sync.RWMutex

	options  AudioRegistryOptions
	sender   EncodedAudioSender
	sessions map[string]map[string]*audioSession
	onPCM    PCMCallback
}

func NewAudioRegistry(sender EncodedAudioSender, options *AudioRegistryOptions) *AudioRegistry {
	resolved := DefaultAudioRegistryOptions()
	if options != nil {
		resolved = *options
		if resolved.CodecFactory == nil {
			resolved.CodecFactory = NewMLowCodec
		}
		if resolved.SilenceTick == 0 {
			resolved.SilenceTick = 60 * time.Millisecond
		}
		if resolved.SilenceAfter == 0 {
			resolved.SilenceAfter = 120 * time.Millisecond
		}
	}
	return &AudioRegistry{
		options:  resolved,
		sender:   sender,
		sessions: make(map[string]map[string]*audioSession),
	}
}

func (r *AudioRegistry) SetOnPCM(callback PCMCallback) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.onPCM = callback
	r.mu.Unlock()
}

func (r *AudioRegistry) Prepare(instanceID, callID string) error {
	if r == nil || instanceID == "" || callID == "" {
		return ErrAudioSessionNotReady
	}
	r.mu.RLock()
	if calls := r.sessions[instanceID]; calls != nil && calls[callID] != nil {
		r.mu.RUnlock()
		return nil
	}
	factory := r.options.CodecFactory
	r.mu.RUnlock()
	if factory == nil {
		return ErrAudioSessionNotReady
	}

	codec, err := factory(r.options.CodecOptions)
	if err != nil {
		return fmt.Errorf("create MLow codec: %w", err)
	}
	candidate := newAudioSession(instanceID, callID, codec, r.sender, r.options)

	r.mu.Lock()
	calls := r.sessions[instanceID]
	if calls == nil {
		calls = make(map[string]*audioSession)
		r.sessions[instanceID] = calls
	}
	if existing := calls[callID]; existing != nil {
		r.mu.Unlock()
		candidate.close()
		return nil
	}
	calls[callID] = candidate
	r.mu.Unlock()
	return nil
}

func (r *AudioRegistry) FeedPCM(instanceID, callID string, pcm []float32) error {
	session, err := r.session(instanceID, callID, true)
	if err != nil {
		return err
	}
	return session.feedPCM(pcm)
}

func (r *AudioRegistry) HandleRTP(instanceID, callID string, packet *RTPPacket) error {
	session, err := r.session(instanceID, callID, true)
	if err != nil {
		return err
	}
	pcm, err := session.handleRTP(packet)
	if err != nil {
		return err
	}
	defer zeroFloat32(pcm)

	r.mu.RLock()
	callback := r.onPCM
	r.mu.RUnlock()
	if callback != nil {
		callback(instanceID, callID, append([]float32(nil), pcm...))
	}
	return nil
}

func (r *AudioRegistry) session(instanceID, callID string, lazy bool) (*audioSession, error) {
	if r == nil {
		return nil, ErrAudioSessionNotReady
	}
	r.mu.RLock()
	calls := r.sessions[instanceID]
	session := calls[callID]
	r.mu.RUnlock()
	if session != nil {
		return session, nil
	}
	if lazy {
		if err := r.Prepare(instanceID, callID); err != nil {
			return nil, err
		}
		r.mu.RLock()
		session = r.sessions[instanceID][callID]
		r.mu.RUnlock()
		if session != nil {
			return session, nil
		}
	}
	return nil, ErrAudioSessionNotReady
}

func (r *AudioRegistry) Remove(instanceID, callID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	calls := r.sessions[instanceID]
	session := calls[callID]
	delete(calls, callID)
	if len(calls) == 0 {
		delete(r.sessions, instanceID)
	}
	r.mu.Unlock()
	if session != nil {
		session.close()
	}
}

func (r *AudioRegistry) Close(instanceID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	sessions := r.sessions[instanceID]
	delete(r.sessions, instanceID)
	r.mu.Unlock()
	for callID, session := range sessions {
		if session != nil {
			session.close()
		}
		delete(sessions, callID)
	}
}
