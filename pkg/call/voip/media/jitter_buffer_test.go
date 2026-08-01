package media

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func testJitterOptions() JitterBufferOptions {
	return JitterBufferOptions{
		FrameDuration:         3 * time.Millisecond,
		InitialDelayPackets:   2,
		MaxPackets:            8,
		MaxConcealmentPackets: 2,
	}
}

func jitterPacket(sequence uint16, timestamp uint32, value byte) *RTPPacket {
	return &RTPPacket{
		Header: &RTPHeader{
			Version:        2,
			SequenceNumber: sequence,
			Timestamp:      timestamp,
		},
		Payload: []byte{value},
	}
}

func readJitterFrames(t *testing.T, frames <-chan JitterFrame, count int) []JitterFrame {
	t.Helper()
	result := make([]JitterFrame, 0, count)
	deadline := time.After(time.Second)
	for len(result) < count {
		select {
		case frame := <-frames:
			result = append(result, frame)
		case <-deadline:
			t.Fatalf("timed out after %d/%d jitter frames", len(result), count)
		}
	}
	return result
}

func TestJitterBufferReordersPackets(t *testing.T) {
	options := testJitterOptions()
	frames := make(chan JitterFrame, 4)
	buffer := NewJitterBuffer(&options, func(frame JitterFrame) {
		frame.Payload = append([]byte(nil), frame.Payload...)
		frames <- frame
	})
	defer buffer.Close()

	if err := buffer.Push(jitterPacket(11, 1960, 11)); err != nil {
		t.Fatal(err)
	}
	if err := buffer.Push(jitterPacket(10, 1000, 10)); err != nil {
		t.Fatal(err)
	}

	got := readJitterFrames(t, frames, 2)
	if got[0].SequenceNumber != 10 || got[1].SequenceNumber != 11 {
		t.Fatalf("unexpected order: %d, %d", got[0].SequenceNumber, got[1].SequenceNumber)
	}
	if !reflect.DeepEqual(got[0].Payload, []byte{10}) || !reflect.DeepEqual(got[1].Payload, []byte{11}) {
		t.Fatalf("unexpected payloads: %v %v", got[0].Payload, got[1].Payload)
	}
}

func TestJitterBufferConcealsGapThenContinues(t *testing.T) {
	options := testJitterOptions()
	frames := make(chan JitterFrame, 6)
	buffer := NewJitterBuffer(&options, func(frame JitterFrame) {
		frame.Payload = append([]byte(nil), frame.Payload...)
		frames <- frame
	})
	defer buffer.Close()

	if err := buffer.Push(jitterPacket(20, 1000, 20)); err != nil {
		t.Fatal(err)
	}
	if err := buffer.Push(jitterPacket(22, 2920, 22)); err != nil {
		t.Fatal(err)
	}

	got := readJitterFrames(t, frames, 3)
	if got[0].SequenceNumber != 20 || got[0].Concealed {
		t.Fatalf("unexpected first frame: %+v", got[0])
	}
	if got[1].SequenceNumber != 21 || !got[1].Concealed || len(got[1].Payload) != 0 {
		t.Fatalf("missing packet was not concealed: %+v", got[1])
	}
	if got[1].Timestamp != 1960 {
		t.Fatalf("concealed timestamp=%d, want 1960", got[1].Timestamp)
	}
	if got[2].SequenceNumber != 22 || got[2].Concealed {
		t.Fatalf("unexpected recovery frame: %+v", got[2])
	}

	stats := buffer.Stats()
	if stats.Delivered != 2 || stats.Concealed != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestJitterBufferHandlesSequenceRollover(t *testing.T) {
	options := testJitterOptions()
	frames := make(chan JitterFrame, 4)
	buffer := NewJitterBuffer(&options, func(frame JitterFrame) { frames <- frame })
	defer buffer.Close()

	if err := buffer.Push(jitterPacket(0, 1960, 0)); err != nil {
		t.Fatal(err)
	}
	if err := buffer.Push(jitterPacket(65535, 1000, 1)); err != nil {
		t.Fatal(err)
	}

	got := readJitterFrames(t, frames, 2)
	if got[0].SequenceNumber != 65535 || got[1].SequenceNumber != 0 {
		t.Fatalf("rollover order is %d, %d", got[0].SequenceNumber, got[1].SequenceNumber)
	}
}

func TestJitterBufferRejectsDuplicateLateAndOverflow(t *testing.T) {
	options := testJitterOptions()
	options.MaxPackets = 2
	frames := make(chan JitterFrame, 4)
	buffer := NewJitterBuffer(&options, func(frame JitterFrame) { frames <- frame })
	defer buffer.Close()

	packet := jitterPacket(30, 1000, 1)
	if err := buffer.Push(packet); err != nil {
		t.Fatal(err)
	}
	if err := buffer.Push(packet); !errors.Is(err, ErrJitterDuplicatePacket) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
	if err := buffer.Push(jitterPacket(31, 1960, 2)); err != nil {
		t.Fatal(err)
	}
	if err := buffer.Push(jitterPacket(32, 2920, 3)); !errors.Is(err, ErrJitterBufferFull) {
		t.Fatalf("expected full error, got %v", err)
	}

	_ = readJitterFrames(t, frames, 2)
	if err := buffer.Push(jitterPacket(30, 1000, 1)); !errors.Is(err, ErrJitterLatePacket) {
		t.Fatalf("expected late error, got %v", err)
	}

	stats := buffer.Stats()
	if stats.Duplicate != 1 || stats.Overflow != 1 || stats.Late != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestJitterBufferStopsAfterBoundedConcealment(t *testing.T) {
	options := testJitterOptions()
	options.InitialDelayPackets = 1
	frames := make(chan JitterFrame, 8)
	buffer := NewJitterBuffer(&options, func(frame JitterFrame) { frames <- frame })
	defer buffer.Close()

	if err := buffer.Push(jitterPacket(40, 1000, 1)); err != nil {
		t.Fatal(err)
	}
	got := readJitterFrames(t, frames, 3)
	if got[0].Concealed || !got[1].Concealed || !got[2].Concealed {
		t.Fatalf("unexpected concealment sequence: %+v", got)
	}

	select {
	case extra := <-frames:
		t.Fatalf("unbounded concealment produced extra frame: %+v", extra)
	case <-time.After(25 * time.Millisecond):
	}
}

func TestJitterBufferCopiesAndWipesOwnedPayload(t *testing.T) {
	options := testJitterOptions()
	options.InitialDelayPackets = 1
	frames := make(chan JitterFrame, 1)
	buffer := NewJitterBuffer(&options, func(frame JitterFrame) {
		frame.Payload = append([]byte(nil), frame.Payload...)
		frames <- frame
	})
	payload := []byte{1, 2, 3}
	packet := jitterPacket(50, 1000, 0)
	packet.Payload = payload
	if err := buffer.Push(packet); err != nil {
		t.Fatal(err)
	}
	payload[0] = 9
	got := readJitterFrames(t, frames, 1)
	if !reflect.DeepEqual(got[0].Payload, []byte{1, 2, 3}) {
		t.Fatalf("jitter buffer retained caller payload: %v", got[0].Payload)
	}
	buffer.Close()
	if err := buffer.Push(packet); !errors.Is(err, ErrJitterBufferClosed) {
		t.Fatalf("expected closed error, got %v", err)
	}
}
