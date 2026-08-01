// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package media

import (
	"fmt"
	"sync"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/media/mlow"
)

type mlowCodec struct {
	mu     sync.Mutex
	enc    *mlow.MlowEncoder
	dec    *mlow.MlowDecoder
	closed bool
}

func NewMLowCodec(opts CodecOptions) (Codec, error) {
	_ = opts
	return &mlowCodec{enc: mlow.NewMlowEncoder(), dec: mlow.NewMlowDecoder()}, nil
}

func (c *mlowCodec) Encode(pcm []float32) ([]byte, error) {
	if len(pcm) == 0 {
		return nil, nil
	}
	if len(pcm) != MLowFrameSize {
		return nil, fmt.Errorf("%w: got %d samples, want %d", ErrInvalidPCMFrame, len(pcm), MLowFrameSize)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.enc == nil {
		return nil, ErrCodecClosed
	}
	input := append([]float32(nil), pcm...)
	defer zeroFloat32(input)
	encoded, err := c.enc.Encode(input)
	if err != nil {
		return nil, fmt.Errorf("encode MLow frame: %w", err)
	}
	return append([]byte(nil), encoded...), nil
}

func (c *mlowCodec) Decode(frame []byte) ([]float32, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.dec == nil {
		return nil, ErrCodecClosed
	}
	input := append([]byte(nil), frame...)
	defer zeroBytes(input)
	decoded := c.dec.Decode(input)
	return NormalizeFrame(decoded, MLowFrameSize), nil
}

func (c *mlowCodec) FrameSize() int  { return MLowFrameSize }
func (c *mlowCodec) SampleRate() int { return MLowSampleRate }

func (c *mlowCodec) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.closed = true
	c.enc = nil
	c.dec = nil
	c.mu.Unlock()
}

func zeroFloat32(values []float32) {
	for index := range values {
		values[index] = 0
	}
}
