// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package media

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrJitterBufferClosed    = errors.New("jitter buffer is closed")
	ErrJitterDuplicatePacket = errors.New("duplicate RTP packet")
	ErrJitterLatePacket      = errors.New("RTP packet arrived after its playout deadline")
	ErrJitterBufferFull      = errors.New("jitter buffer is full")
)

type JitterBufferOptions struct {
	FrameDuration         time.Duration
	InitialDelayPackets   int
	MaxPackets            int
	MaxConcealmentPackets int
}

func DefaultJitterBufferOptions() JitterBufferOptions {
	return JitterBufferOptions{
		FrameDuration:         60 * time.Millisecond,
		InitialDelayPackets:   2,
		MaxPackets:            64,
		MaxConcealmentPackets: 5,
	}
}

type JitterFrame struct {
	SequenceNumber uint16
	Timestamp      uint32
	Marker         bool
	Payload        []byte
	Concealed      bool
}

type JitterBufferStats struct {
	Received  uint64
	Delivered uint64
	Concealed uint64
	Duplicate uint64
	Late      uint64
	Overflow  uint64
}

type bufferedRTP struct {
	extendedSequence uint64
	sequenceNumber   uint16
	timestamp        uint32
	marker           bool
	payload          []byte
}

type JitterBuffer struct {
	mu sync.Mutex

	options JitterBufferOptions
	onFrame func(JitterFrame)
	packets map[uint64]*bufferedRTP

	initialized        bool
	started            bool
	highestSequence    uint64
	nextSequence       uint64
	lastTimestamp      uint32
	hasTimestamp       bool
	firstArrival       time.Time
	consecutiveMissing int
	closed             bool
	stats              JitterBufferStats

	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
}

func NewJitterBuffer(options *JitterBufferOptions, onFrame func(JitterFrame)) *JitterBuffer {
	resolved := DefaultJitterBufferOptions()
	if options != nil {
		resolved = *options
	}
	if resolved.FrameDuration <= 0 {
		resolved.FrameDuration = 60 * time.Millisecond
	}
	if resolved.InitialDelayPackets <= 0 {
		resolved.InitialDelayPackets = 1
	}
	if resolved.MaxPackets <= 0 {
		resolved.MaxPackets = 64
	}
	if resolved.MaxConcealmentPackets <= 0 {
		resolved.MaxConcealmentPackets = 1
	}

	buffer := &JitterBuffer{
		options: resolved,
		onFrame: onFrame,
		packets: make(map[uint64]*bufferedRTP),
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
	go buffer.playoutLoop()
	return buffer
}

func (b *JitterBuffer) Push(packet *RTPPacket) error {
	if b == nil {
		return ErrJitterBufferClosed
	}
	if packet == nil || packet.Header == nil {
		return fmt.Errorf("push jitter packet: RTP packet or header is nil")
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return ErrJitterBufferClosed
	}

	extended := b.extendSequenceLocked(packet.Header.SequenceNumber)
	if b.started && extended < b.nextSequence {
		b.stats.Late++
		return ErrJitterLatePacket
	}
	if _, exists := b.packets[extended]; exists {
		b.stats.Duplicate++
		return ErrJitterDuplicatePacket
	}
	if len(b.packets) >= b.options.MaxPackets {
		b.stats.Overflow++
		return ErrJitterBufferFull
	}

	if !b.initialized {
		b.initialized = true
		b.highestSequence = extended
		b.firstArrival = time.Now()
	} else if extended > b.highestSequence {
		b.highestSequence = extended
	}

	b.packets[extended] = &bufferedRTP{
		extendedSequence: extended,
		sequenceNumber:   packet.Header.SequenceNumber,
		timestamp:        packet.Header.Timestamp,
		marker:           packet.Header.Marker,
		payload:          append([]byte(nil), packet.Payload...),
	}
	b.stats.Received++

	if !b.started && len(b.packets) >= b.options.InitialDelayPackets {
		b.startLocked()
	}
	return nil
}

func (b *JitterBuffer) Stats() JitterBufferStats {
	if b == nil {
		return JitterBufferStats{}
	}
	b.mu.Lock()
	stats := b.stats
	b.mu.Unlock()
	return stats
}

func (b *JitterBuffer) Buffered() int {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	count := len(b.packets)
	b.mu.Unlock()
	return count
}

func (b *JitterBuffer) Close() {
	if b == nil {
		return
	}
	b.stopOnce.Do(func() { close(b.stopCh) })
	<-b.doneCh
}

func (b *JitterBuffer) playoutLoop() {
	defer close(b.doneCh)
	ticker := time.NewTicker(b.options.FrameDuration)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			b.mu.Lock()
			b.closed = true
			b.clearPacketsLocked()
			b.mu.Unlock()
			return
		case now := <-ticker.C:
			frame, ok := b.dequeue(now)
			if !ok {
				continue
			}
			if b.onFrame != nil {
				b.onFrame(frame)
			}
			zeroBytes(frame.Payload)
		}
	}
}

func (b *JitterBuffer) dequeue(now time.Time) (JitterFrame, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed || !b.initialized {
		return JitterFrame{}, false
	}
	if !b.started {
		startupDelay := time.Duration(b.options.InitialDelayPackets) * b.options.FrameDuration
		if len(b.packets) < b.options.InitialDelayPackets && now.Sub(b.firstArrival) < startupDelay {
			return JitterFrame{}, false
		}
		b.startLocked()
	}

	if packet := b.packets[b.nextSequence]; packet != nil {
		delete(b.packets, b.nextSequence)
		frame := JitterFrame{
			SequenceNumber: packet.sequenceNumber,
			Timestamp:      packet.timestamp,
			Marker:         packet.marker,
			Payload:        packet.payload,
		}
		packet.payload = nil
		b.nextSequence++
		b.lastTimestamp = frame.Timestamp
		b.hasTimestamp = true
		b.consecutiveMissing = 0
		b.stats.Delivered++
		return frame, true
	}

	sequence := uint16(b.nextSequence)
	timestamp := b.estimatedTimestampLocked()
	b.nextSequence++
	b.lastTimestamp = timestamp
	b.hasTimestamp = true
	b.consecutiveMissing++
	b.stats.Concealed++
	frame := JitterFrame{SequenceNumber: sequence, Timestamp: timestamp, Concealed: true}

	if b.consecutiveMissing >= b.options.MaxConcealmentPackets {
		b.resynchronizeLocked()
	}
	return frame, true
}

func (b *JitterBuffer) startLocked() {
	if len(b.packets) == 0 {
		return
	}
	b.nextSequence = b.minimumSequenceLocked()
	b.started = true
	b.consecutiveMissing = 0
}

func (b *JitterBuffer) resynchronizeLocked() {
	b.consecutiveMissing = 0
	if len(b.packets) == 0 {
		b.initialized = false
		b.started = false
		b.highestSequence = 0
		b.nextSequence = 0
		b.lastTimestamp = 0
		b.hasTimestamp = false
		b.firstArrival = time.Time{}
		return
	}
	b.nextSequence = b.minimumSequenceLocked()
}

func (b *JitterBuffer) extendSequenceLocked(sequence uint16) uint64 {
	if !b.initialized {
		// Start in epoch one so an out-of-order packet from the previous epoch can
		// still be represented when sequence zero arrives before 65535.
		return 1<<16 | uint64(sequence)
	}
	rollover := b.highestSequence >> 16
	highestLow := uint16(b.highestSequence)
	candidate := rollover<<16 | uint64(sequence)

	if sequence < highestLow && highestLow-sequence > 0x8000 {
		candidate += 1 << 16
	} else if sequence > highestLow && sequence-highestLow > 0x8000 && rollover > 0 {
		candidate -= 1 << 16
	}
	return candidate
}

func (b *JitterBuffer) minimumSequenceLocked() uint64 {
	var minimum uint64
	first := true
	for sequence := range b.packets {
		if first || sequence < minimum {
			minimum = sequence
			first = false
		}
	}
	return minimum
}

func (b *JitterBuffer) estimatedTimestampLocked() uint32 {
	if b.hasTimestamp {
		return b.lastTimestamp + uint32(MLowFrameSize)
	}
	if next := b.packets[b.nextSequence+1]; next != nil {
		return next.timestamp - uint32(MLowFrameSize)
	}
	return 0
}

func (b *JitterBuffer) clearPacketsLocked() {
	for sequence, packet := range b.packets {
		if packet != nil {
			zeroBytes(packet.payload)
			packet.payload = nil
		}
		delete(b.packets, sequence)
	}
}
