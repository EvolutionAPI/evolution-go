package browser

import (
	"errors"
	"math"
	"testing"
)

func TestPCMFrameRoundTrip(t *testing.T) {
	input := []float32{-1, -0.25, 0, 0.5, 1}
	frame, err := EncodePCMFrame(input)
	if err != nil {
		t.Fatal(err)
	}
	output, err := DecodePCMFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != len(input) {
		t.Fatalf("decoded %d samples, want %d", len(output), len(input))
	}
	for index := range input {
		if math.Float32bits(output[index]) != math.Float32bits(input[index]) {
			t.Fatalf("sample %d=%v, want %v", index, output[index], input[index])
		}
	}
}

func TestPCMFrameRejectsMalformedInput(t *testing.T) {
	frame, err := EncodePCMFrame(make([]float32, PCMFrameSamples))
	if err != nil {
		t.Fatal(err)
	}
	cases := [][]byte{
		nil,
		frame[:10],
		append([]byte(nil), frame[:len(frame)-1]...),
		append([]byte("BAD!"), frame[4:]...),
	}
	for _, value := range cases {
		if _, err = DecodePCMFrame(value); !errors.Is(err, ErrInvalidPCMMessage) {
			t.Fatalf("expected invalid PCM error, got %v", err)
		}
	}
}

func TestPCMFrameLimitsSamples(t *testing.T) {
	if _, err := EncodePCMFrame(nil); !errors.Is(err, ErrInvalidPCMMessage) {
		t.Fatalf("expected empty frame error, got %v", err)
	}
	if _, err := EncodePCMFrame(make([]float32, maxPCMSamples+1)); !errors.Is(err, ErrInvalidPCMMessage) {
		t.Fatalf("expected oversized frame error, got %v", err)
	}
}
