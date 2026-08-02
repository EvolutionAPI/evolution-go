// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package media

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
)

const (
	rtpVersion       uint8 = 2
	rtpMinHeaderSize       = 12
	maxCSRCCount           = 15
)

type RTPHeader struct {
	Version          uint8
	Padding          bool
	Extension        bool
	Marker           bool
	PayloadType      uint8
	SequenceNumber   uint16
	Timestamp        uint32
	SSRC             uint32
	CSRC             []uint32
	ExtensionProfile uint16
	ExtensionData    []byte
}

func NewRTPHeader(payloadType uint8, sequence uint16, timestamp, ssrc uint32) *RTPHeader {
	return &RTPHeader{
		Version:        rtpVersion,
		PayloadType:    payloadType,
		SequenceNumber: sequence,
		Timestamp:      timestamp,
		SSRC:           ssrc,
	}
}

func (h *RTPHeader) encodedSize() (int, error) {
	if h == nil {
		return 0, fmt.Errorf("RTP header is nil")
	}
	if h.Version == 0 {
		h.Version = rtpVersion
	}
	if h.Version != rtpVersion {
		return 0, fmt.Errorf("invalid RTP version: %d", h.Version)
	}
	if len(h.CSRC) > maxCSRCCount {
		return 0, fmt.Errorf("too many RTP CSRC entries: %d", len(h.CSRC))
	}
	if h.PayloadType > 127 {
		return 0, fmt.Errorf("invalid RTP payload type: %d", h.PayloadType)
	}
	if h.Extension {
		if len(h.ExtensionData)%4 != 0 {
			return 0, fmt.Errorf("RTP extension length must be a multiple of four: %d", len(h.ExtensionData))
		}
		if len(h.ExtensionData)/4 > int(^uint16(0)) {
			return 0, fmt.Errorf("RTP extension is too large: %d", len(h.ExtensionData))
		}
	}

	size := rtpMinHeaderSize + len(h.CSRC)*4
	if h.Extension {
		size += 4 + len(h.ExtensionData)
	}
	return size, nil
}

func (h *RTPHeader) MarshalTo(buffer []byte) (int, error) {
	size, err := h.encodedSize()
	if err != nil {
		return 0, err
	}
	if len(buffer) < size {
		return 0, fmt.Errorf("buffer too small for RTP header: got %d, need %d", len(buffer), size)
	}

	buffer[0] = (h.Version&0x03)<<6 | boolBit(h.Padding)<<5 | boolBit(h.Extension)<<4 | byte(len(h.CSRC)&0x0f)
	buffer[1] = boolBit(h.Marker)<<7 | (h.PayloadType & 0x7f)
	binary.BigEndian.PutUint16(buffer[2:4], h.SequenceNumber)
	binary.BigEndian.PutUint32(buffer[4:8], h.Timestamp)
	binary.BigEndian.PutUint32(buffer[8:12], h.SSRC)

	offset := rtpMinHeaderSize
	for _, source := range h.CSRC {
		binary.BigEndian.PutUint32(buffer[offset:offset+4], source)
		offset += 4
	}
	if h.Extension {
		binary.BigEndian.PutUint16(buffer[offset:offset+2], h.ExtensionProfile)
		binary.BigEndian.PutUint16(buffer[offset+2:offset+4], uint16(len(h.ExtensionData)/4))
		copy(buffer[offset+4:offset+4+len(h.ExtensionData)], h.ExtensionData)
	}
	return size, nil
}

func ParseRTPHeader(buffer []byte) (*RTPHeader, int, error) {
	if len(buffer) < rtpMinHeaderSize {
		return nil, 0, fmt.Errorf("buffer too small for RTP header: %d", len(buffer))
	}
	version := (buffer[0] >> 6) & 0x03
	if version != rtpVersion {
		return nil, 0, fmt.Errorf("invalid RTP version: %d", version)
	}

	csrcCount := int(buffer[0] & 0x0f)
	offset := rtpMinHeaderSize + csrcCount*4
	if len(buffer) < offset {
		return nil, 0, fmt.Errorf("truncated RTP CSRC list")
	}

	header := &RTPHeader{
		Version:        version,
		Padding:        buffer[0]&0x20 != 0,
		Extension:      buffer[0]&0x10 != 0,
		Marker:         buffer[1]&0x80 != 0,
		PayloadType:    buffer[1] & 0x7f,
		SequenceNumber: binary.BigEndian.Uint16(buffer[2:4]),
		Timestamp:      binary.BigEndian.Uint32(buffer[4:8]),
		SSRC:           binary.BigEndian.Uint32(buffer[8:12]),
		CSRC:           make([]uint32, 0, csrcCount),
	}

	cursor := rtpMinHeaderSize
	for index := 0; index < csrcCount; index++ {
		header.CSRC = append(header.CSRC, binary.BigEndian.Uint32(buffer[cursor:cursor+4]))
		cursor += 4
	}
	if header.Extension {
		if len(buffer) < cursor+4 {
			return nil, 0, fmt.Errorf("truncated RTP extension header")
		}
		header.ExtensionProfile = binary.BigEndian.Uint16(buffer[cursor : cursor+2])
		extensionLength := int(binary.BigEndian.Uint16(buffer[cursor+2:cursor+4])) * 4
		cursor += 4
		if len(buffer) < cursor+extensionLength {
			return nil, 0, fmt.Errorf("truncated RTP extension data")
		}
		header.ExtensionData = append([]byte(nil), buffer[cursor:cursor+extensionLength]...)
		cursor += extensionLength
	}
	return header, cursor, nil
}

type RTPPacket struct {
	Header      *RTPHeader
	Payload     []byte
	PaddingSize uint8
}

func (p *RTPPacket) Marshal() ([]byte, error) {
	if p == nil || p.Header == nil {
		return nil, fmt.Errorf("RTP packet or header is nil")
	}
	headerSize, err := p.Header.encodedSize()
	if err != nil {
		return nil, err
	}
	paddingSize := int(p.PaddingSize)
	if p.Header.Padding && paddingSize == 0 {
		return nil, fmt.Errorf("RTP padding flag is set without padding bytes")
	}
	if !p.Header.Padding && paddingSize != 0 {
		return nil, fmt.Errorf("RTP padding bytes require the padding flag")
	}

	output := make([]byte, headerSize+len(p.Payload)+paddingSize)
	if _, err = p.Header.MarshalTo(output); err != nil {
		return nil, err
	}
	copy(output[headerSize:], p.Payload)
	if paddingSize > 0 {
		output[len(output)-1] = byte(paddingSize)
	}
	return output, nil
}

func ParseRTPPacket(buffer []byte) (*RTPPacket, error) {
	header, headerSize, err := ParseRTPHeader(buffer)
	if err != nil {
		return nil, err
	}
	if len(buffer) < headerSize {
		return nil, fmt.Errorf("invalid RTP header size")
	}
	payloadEnd := len(buffer)
	paddingSize := 0
	if header.Padding {
		if payloadEnd == headerSize {
			return nil, fmt.Errorf("RTP padding flag set on empty payload")
		}
		paddingSize = int(buffer[payloadEnd-1])
		if paddingSize == 0 || paddingSize > payloadEnd-headerSize {
			return nil, fmt.Errorf("invalid RTP padding size: %d", paddingSize)
		}
		payloadEnd -= paddingSize
	}
	return &RTPPacket{
		Header:      header,
		Payload:     append([]byte(nil), buffer[headerSize:payloadEnd]...),
		PaddingSize: uint8(paddingSize),
	}, nil
}

func (p *RTPPacket) Wipe() {
	if p == nil {
		return
	}
	zeroBytes(p.Payload)
	p.Payload = nil
	if p.Header != nil {
		zeroBytes(p.Header.ExtensionData)
		p.Header.ExtensionData = nil
		p.Header.CSRC = nil
	}
	p.Header = nil
	p.PaddingSize = 0
}

type RTPSession struct {
	mu               sync.Mutex
	ssrc             uint32
	payloadType      uint8
	sequenceNumber   uint16
	timestamp        uint32
	samplesPerPacket uint32
}

func NewRTPSession(ssrc uint32, payloadType uint8, samplesPerPacket uint32) (*RTPSession, error) {
	if ssrc == 0 {
		return nil, fmt.Errorf("RTP SSRC must be non-zero")
	}
	if payloadType > 127 {
		return nil, fmt.Errorf("invalid RTP payload type: %d", payloadType)
	}
	if samplesPerPacket == 0 {
		return nil, fmt.Errorf("samples per RTP packet must be non-zero")
	}
	sequence, err := randomUint16()
	if err != nil {
		return nil, err
	}
	timestamp, err := randomUint32()
	if err != nil {
		return nil, err
	}
	return &RTPSession{
		ssrc:             ssrc,
		payloadType:      payloadType,
		sequenceNumber:   sequence,
		timestamp:        timestamp,
		samplesPerPacket: samplesPerPacket,
	}, nil
}

func NewWhatsAppOpusRTPSession(ssrc uint32) (*RTPSession, error) {
	return NewRTPSession(ssrc, core.PayloadTypeWhatsAppOpus, 960)
}

func (s *RTPSession) CreatePacket(payload []byte, marker bool) *RTPPacket {
	return s.CreatePacketWithDuration(payload, s.samplesPerPacket, marker)
}

func (s *RTPSession) CreatePacketWithDuration(payload []byte, durationSamples uint32, marker bool) *RTPPacket {
	s.mu.Lock()
	header := NewRTPHeader(s.payloadType, s.sequenceNumber, s.timestamp, s.ssrc)
	header.Marker = marker
	s.sequenceNumber++
	s.timestamp += durationSamples
	s.mu.Unlock()
	return &RTPPacket{Header: header, Payload: append([]byte(nil), payload...)}
}

func boolBit(value bool) byte {
	if value {
		return 1
	}
	return 0
}

func randomUint16() (uint16, error) {
	var buffer [2]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return 0, fmt.Errorf("generate RTP sequence: %w", err)
	}
	return binary.BigEndian.Uint16(buffer[:]), nil
}

func randomUint32() (uint32, error) {
	var buffer [4]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return 0, fmt.Errorf("generate RTP timestamp: %w", err)
	}
	return binary.BigEndian.Uint32(buffer[:]), nil
}
