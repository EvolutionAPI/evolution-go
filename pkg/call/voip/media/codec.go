// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package media

import "errors"

const (
	MLowSampleRate = 16000
	MLowFrameSize  = 960
)

var (
	ErrCodecClosed     = errors.New("audio codec is closed")
	ErrInvalidPCMFrame = errors.New("invalid PCM frame size")
)

type Codec interface {
	Encode(pcm []float32) ([]byte, error)
	Decode(frame []byte) ([]float32, error)
	FrameSize() int
	SampleRate() int
	Close()
}

type CodecOptions struct {
	Bitrate    int
	Complexity int
	FEC        bool
}

var DefaultCodecOptions = CodecOptions{Bitrate: 6000, Complexity: 5, FEC: false}

func NormalizeFrame(pcm []float32, samples int) []float32 {
	if samples <= 0 {
		return nil
	}
	if len(pcm) == samples {
		return append([]float32(nil), pcm...)
	}
	normalized := make([]float32, samples)
	copy(normalized, pcm)
	return normalized
}
