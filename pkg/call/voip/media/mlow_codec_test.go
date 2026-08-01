package media

import (
	"errors"
	"math"
	"sync"
	"testing"
)

func TestMLowCodecAdapterRoundtrip(t *testing.T) {
	codec, err := NewMLowCodec(DefaultCodecOptions)
	if err != nil {
		t.Fatal(err)
	}
	defer codec.Close()
	frame := make([]float32, MLowFrameSize)
	for i := range frame {
		frame[i] = 0.25 * float32(math.Sin(2*math.Pi*440*float64(i)/MLowSampleRate))
	}
	encoded, err := codec.Encode(frame)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) == 0 {
		t.Fatal("encoded frame is empty")
	}
	decoded, err := codec.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != MLowFrameSize {
		t.Fatalf("decoded %d samples, want %d", len(decoded), MLowFrameSize)
	}
}

func TestMLowCodecPLCAndValidation(t *testing.T) {
	codec, err := NewMLowCodec(DefaultCodecOptions)
	if err != nil {
		t.Fatal(err)
	}
	plc, err := codec.Decode(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plc) != MLowFrameSize {
		t.Fatalf("PLC returned %d samples", len(plc))
	}
	if _, err = codec.Encode(make([]float32, 12)); !errors.Is(err, ErrInvalidPCMFrame) {
		t.Fatalf("expected ErrInvalidPCMFrame, got %v", err)
	}
	codec.Close()
	if _, err = codec.Decode(nil); !errors.Is(err, ErrCodecClosed) {
		t.Fatalf("expected ErrCodecClosed, got %v", err)
	}
}

func TestMLowCodecSerializesConcurrentUse(t *testing.T) {
	codec, err := NewMLowCodec(DefaultCodecOptions)
	if err != nil {
		t.Fatal(err)
	}
	defer codec.Close()
	frame := make([]float32, MLowFrameSize)
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for attempt := 0; attempt < 4; attempt++ {
				encoded, encodeErr := codec.Encode(frame)
				if encodeErr != nil {
					t.Errorf("encode: %v", encodeErr)
					return
				}
				if _, decodeErr := codec.Decode(encoded); decodeErr != nil {
					t.Errorf("decode: %v", decodeErr)
					return
				}
			}
		}()
	}
	wg.Wait()
}
