// pkg/call/stream/codec.go
package call_stream

import "encoding/binary"

// pcm16FromFloat32 converts one mono PCM frame (as meowcaller delivers it: float32
// samples in [-1, 1]) into little-endian 16-bit PCM bytes, clamping out-of-range
// samples the same way meowcaller's own WAVRecorder does.
func pcm16FromFloat32(frame []float32) []byte {
	out := make([]byte, len(frame)*2)
	for i, s := range frame {
		v := s * 32768.0
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		binary.LittleEndian.PutUint16(out[2*i:], uint16(int16(v)))
	}
	return out
}

// float32FromPCM16 converts little-endian 16-bit PCM bytes (as sent by a stream
// consumer) back into mono float32 samples in [-1, 1] for meowcaller.Call.Play.
func float32FromPCM16(b []byte) []float32 {
	n := len(b) / 2
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		v := int16(binary.LittleEndian.Uint16(b[2*i:]))
		out[i] = float32(v) / 32768.0
	}
	return out
}
