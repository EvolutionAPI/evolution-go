package browser

import (
	"encoding/binary"
	"fmt"
	"math"
)

const (
	pcmHeaderSize = 16
	pcmVersion    = 1
	pcmKind       = 1
	maxPCMSamples = PCMFrameSamples * 4
)

var pcmMagic = [4]byte{'E', 'V', 'P', 'C'}

func EncodePCMFrame(pcm []float32) ([]byte, error) {
	if len(pcm) == 0 || len(pcm) > maxPCMSamples {
		return nil, fmt.Errorf("%w: sample count %d", ErrInvalidPCMMessage, len(pcm))
	}
	output := make([]byte, pcmHeaderSize+len(pcm)*4)
	copy(output[:4], pcmMagic[:])
	output[4] = pcmVersion
	output[5] = pcmKind
	binary.LittleEndian.PutUint16(output[6:8], 0)
	binary.LittleEndian.PutUint32(output[8:12], PCMSampleRate)
	binary.LittleEndian.PutUint32(output[12:16], uint32(len(pcm)))
	offset := pcmHeaderSize
	for _, sample := range pcm {
		binary.LittleEndian.PutUint32(output[offset:offset+4], math.Float32bits(sample))
		offset += 4
	}
	return output, nil
}

func DecodePCMFrame(frame []byte) ([]float32, error) {
	if len(frame) < pcmHeaderSize {
		return nil, fmt.Errorf("%w: frame has %d bytes", ErrInvalidPCMMessage, len(frame))
	}
	if string(frame[:4]) != string(pcmMagic[:]) || frame[4] != pcmVersion || frame[5] != pcmKind {
		return nil, fmt.Errorf("%w: unsupported framing", ErrInvalidPCMMessage)
	}
	if binary.LittleEndian.Uint16(frame[6:8]) != 0 {
		return nil, fmt.Errorf("%w: unsupported flags", ErrInvalidPCMMessage)
	}
	if binary.LittleEndian.Uint32(frame[8:12]) != PCMSampleRate {
		return nil, fmt.Errorf("%w: sample rate must be %d", ErrInvalidPCMMessage, PCMSampleRate)
	}
	sampleCount := int(binary.LittleEndian.Uint32(frame[12:16]))
	if sampleCount <= 0 || sampleCount > maxPCMSamples {
		return nil, fmt.Errorf("%w: sample count %d", ErrInvalidPCMMessage, sampleCount)
	}
	expected := pcmHeaderSize + sampleCount*4
	if len(frame) != expected {
		return nil, fmt.Errorf("%w: frame has %d bytes, want %d", ErrInvalidPCMMessage, len(frame), expected)
	}
	pcm := make([]float32, sampleCount)
	offset := pcmHeaderSize
	for index := range pcm {
		pcm[index] = math.Float32frombits(binary.LittleEndian.Uint32(frame[offset : offset+4]))
		offset += 4
	}
	return pcm, nil
}

func zeroPCM(values []float32) {
	for index := range values {
		values[index] = 0
	}
}

func zeroFrame(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
