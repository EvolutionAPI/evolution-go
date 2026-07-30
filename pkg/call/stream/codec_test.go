// pkg/call/stream/codec_test.go
package call_stream

import "testing"

func TestPCM16RoundTrip(t *testing.T) {
	frame := []float32{0, 0.5, -0.5, 1, -1, 0.25}

	pcm := pcm16FromFloat32(frame)
	if len(pcm) != len(frame)*2 {
		t.Fatalf("expected %d bytes, got %d", len(frame)*2, len(pcm))
	}

	back := float32FromPCM16(pcm)
	if len(back) != len(frame) {
		t.Fatalf("expected %d samples back, got %d", len(frame), len(back))
	}

	for i, want := range frame {
		got := back[i]
		diff := got - want
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.001 {
			t.Errorf("sample %d: want %v, got %v (diff %v)", i, want, got, diff)
		}
	}
}

func TestPCM16ClampsOutOfRange(t *testing.T) {
	frame := []float32{2.0, -2.0} // out of [-1, 1] range
	pcm := pcm16FromFloat32(frame)
	back := float32FromPCM16(pcm)

	if back[0] < 0.99 {
		t.Errorf("expected clamp to max positive, got %v", back[0])
	}
	if back[1] > -0.99 {
		t.Errorf("expected clamp to max negative, got %v", back[1])
	}
}
