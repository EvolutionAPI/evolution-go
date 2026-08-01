// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package media

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
)

type SRTPErrorType string

const (
	SRTPErrPacketTooShort SRTPErrorType = "packet_too_short"
	SRTPErrAuthFailed     SRTPErrorType = "auth_failed"
	SRTPErrReplay         SRTPErrorType = "replay"
	SRTPErrEncryption     SRTPErrorType = "encryption"
	SRTPErrDecryption     SRTPErrorType = "decryption"
	SRTPErrInvalidKeying  SRTPErrorType = "invalid_keying"
	SRTPErrClosed         SRTPErrorType = "closed"
)

type SRTPError struct {
	Type SRTPErrorType
	Msg  string
}

func (e *SRTPError) Error() string {
	if e == nil {
		return "srtp error"
	}
	return fmt.Sprintf("srtp %s: %s", e.Type, e.Msg)
}

type SRTPContext struct {
	mu sync.Mutex

	sessionKey  []byte
	sessionSalt []byte
	authKey     []byte
	authTagLen  int

	initialized  bool
	highestIndex uint64
	replayWindow uint64
	closed       bool
}

func NewSRTPContext(keying core.SRTPKeyingMaterial, authTagLen int) (*SRTPContext, error) {
	if len(keying.MasterKey) != srtpMasterKeyLength {
		return nil, &SRTPError{Type: SRTPErrInvalidKeying, Msg: fmt.Sprintf("master key length is %d", len(keying.MasterKey))}
	}
	if len(keying.MasterSalt) != srtpMasterSaltLength {
		return nil, &SRTPError{Type: SRTPErrInvalidKeying, Msg: fmt.Sprintf("master salt length is %d", len(keying.MasterSalt))}
	}
	if authTagLen <= 0 {
		authTagLen = core.SRTPAuthTagLen
	}
	if authTagLen > sha1.Size {
		return nil, &SRTPError{Type: SRTPErrInvalidKeying, Msg: fmt.Sprintf("auth tag length is %d", authTagLen)}
	}

	sessionKey, err := deriveSRTPKey(keying.MasterKey, keying.MasterSalt, core.SRTPLabelEncryption, srtpMasterKeyLength)
	if err != nil {
		return nil, err
	}
	authKey, err := deriveSRTPKey(keying.MasterKey, keying.MasterSalt, core.SRTPLabelAuth, sha1.Size)
	if err != nil {
		zeroBytes(sessionKey)
		return nil, err
	}
	sessionSalt, err := deriveSRTPKey(keying.MasterKey, keying.MasterSalt, core.SRTPLabelSalt, srtpMasterSaltLength)
	if err != nil {
		zeroBytes(sessionKey)
		zeroBytes(authKey)
		return nil, err
	}
	return &SRTPContext{
		sessionKey:  sessionKey,
		sessionSalt: sessionSalt,
		authKey:     authKey,
		authTagLen:  authTagLen,
	}, nil
}

func (c *SRTPContext) Protect(packet *RTPPacket) ([]byte, error) {
	if packet == nil || packet.Header == nil {
		return nil, &SRTPError{Type: SRTPErrEncryption, Msg: "RTP packet or header is nil"}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, &SRTPError{Type: SRTPErrClosed, Msg: "context is closed"}
	}

	index := estimatePacketIndex(packet.Header.SequenceNumber, c.highestIndex, c.initialized)
	if c.initialized && index <= c.highestIndex {
		return nil, &SRTPError{Type: SRTPErrEncryption, Msg: "non-monotonic RTP sequence would reuse an SRTP index"}
	}

	plain, err := packet.Marshal()
	if err != nil {
		return nil, &SRTPError{Type: SRTPErrEncryption, Msg: err.Error()}
	}
	defer zeroBytes(plain)

	_, headerSize, err := ParseRTPHeader(plain)
	if err != nil {
		return nil, &SRTPError{Type: SRTPErrEncryption, Msg: err.Error()}
	}
	output := make([]byte, len(plain)+c.authTagLen)
	copy(output[:headerSize], plain[:headerSize])

	iv := c.generateIV(packet.Header.SSRC, index)
	if err = aesCTRXOR(c.sessionKey, iv, plain[headerSize:], output[headerSize:len(plain)]); err != nil {
		zeroBytes(output)
		return nil, &SRTPError{Type: SRTPErrEncryption, Msg: err.Error()}
	}
	zeroBytes(iv)

	roc := uint32(index >> 16)
	tag := c.computeAuthTag(output[:len(plain)], roc)
	copy(output[len(plain):], tag)
	zeroBytes(tag)

	c.highestIndex = index
	c.initialized = true
	return output, nil
}

func (c *SRTPContext) Unprotect(data []byte) (*RTPPacket, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, &SRTPError{Type: SRTPErrClosed, Msg: "context is closed"}
	}
	if len(data) < rtpMinHeaderSize+c.authTagLen {
		return nil, &SRTPError{Type: SRTPErrPacketTooShort, Msg: fmt.Sprintf("packet is %d bytes", len(data))}
	}

	header, headerSize, err := ParseRTPHeader(data)
	if err != nil {
		return nil, &SRTPError{Type: SRTPErrDecryption, Msg: err.Error()}
	}
	ciphertextEnd := len(data) - c.authTagLen
	if ciphertextEnd <= headerSize {
		return nil, &SRTPError{Type: SRTPErrPacketTooShort, Msg: "packet has no encrypted RTP payload"}
	}

	index := estimatePacketIndex(header.SequenceNumber, c.highestIndex, c.initialized)
	roc := uint32(index >> 16)
	expectedTag := c.computeAuthTag(data[:ciphertextEnd], roc)
	receivedTag := data[ciphertextEnd:]
	if len(expectedTag) != len(receivedTag) || subtle.ConstantTimeCompare(expectedTag, receivedTag) != 1 {
		zeroBytes(expectedTag)
		return nil, &SRTPError{Type: SRTPErrAuthFailed, Msg: "authentication tag mismatch"}
	}
	zeroBytes(expectedTag)

	if c.isReplay(index) {
		return nil, &SRTPError{Type: SRTPErrReplay, Msg: "packet index was already received or is outside the replay window"}
	}

	plain := make([]byte, ciphertextEnd)
	copy(plain[:headerSize], data[:headerSize])
	iv := c.generateIV(header.SSRC, index)
	if err = aesCTRXOR(c.sessionKey, iv, data[headerSize:ciphertextEnd], plain[headerSize:]); err != nil {
		zeroBytes(iv)
		zeroBytes(plain)
		return nil, &SRTPError{Type: SRTPErrDecryption, Msg: err.Error()}
	}
	zeroBytes(iv)

	packet, err := ParseRTPPacket(plain)
	zeroBytes(plain)
	if err != nil {
		return nil, &SRTPError{Type: SRTPErrDecryption, Msg: err.Error()}
	}
	c.commitReceivedIndex(index)
	return packet, nil
}

func (c *SRTPContext) SetAuthenticationKeying(keying core.SRTPKeyingMaterial) error {
	if len(keying.MasterKey) != srtpMasterKeyLength || len(keying.MasterSalt) != srtpMasterSaltLength {
		return &SRTPError{Type: SRTPErrInvalidKeying, Msg: "invalid authentication keying material"}
	}
	authKey, err := deriveSRTPKey(keying.MasterKey, keying.MasterSalt, core.SRTPLabelAuth, sha1.Size)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		zeroBytes(authKey)
		return &SRTPError{Type: SRTPErrClosed, Msg: "context is closed"}
	}
	zeroBytes(c.authKey)
	c.authKey = authKey
	return nil
}

func (c *SRTPContext) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	if !c.closed {
		zeroBytes(c.sessionKey)
		zeroBytes(c.sessionSalt)
		zeroBytes(c.authKey)
		c.sessionKey = nil
		c.sessionSalt = nil
		c.authKey = nil
		c.highestIndex = 0
		c.replayWindow = 0
		c.initialized = false
		c.closed = true
	}
	c.mu.Unlock()
}

func (c *SRTPContext) generateIV(ssrc uint32, index uint64) []byte {
	iv := make([]byte, aes.BlockSize)
	copy(iv, c.sessionSalt)
	var ssrcBuffer [4]byte
	binary.BigEndian.PutUint32(ssrcBuffer[:], ssrc)
	for offset := 0; offset < len(ssrcBuffer); offset++ {
		iv[4+offset] ^= ssrcBuffer[offset]
	}
	var indexBuffer [8]byte
	binary.BigEndian.PutUint64(indexBuffer[:], index)
	for offset := 0; offset < 6; offset++ {
		iv[8+offset] ^= indexBuffer[2+offset]
	}
	return iv
}

func (c *SRTPContext) computeAuthTag(data []byte, roc uint32) []byte {
	mac := hmac.New(sha1.New, c.authKey)
	_, _ = mac.Write(data)
	var rocBuffer [4]byte
	binary.BigEndian.PutUint32(rocBuffer[:], roc)
	_, _ = mac.Write(rocBuffer[:])
	return append([]byte(nil), mac.Sum(nil)[:c.authTagLen]...)
}

func (c *SRTPContext) isReplay(index uint64) bool {
	if !c.initialized || index > c.highestIndex {
		return false
	}
	delta := c.highestIndex - index
	if delta >= 64 {
		return true
	}
	return c.replayWindow&(uint64(1)<<delta) != 0
}

func (c *SRTPContext) commitReceivedIndex(index uint64) {
	if !c.initialized {
		c.initialized = true
		c.highestIndex = index
		c.replayWindow = 1
		return
	}
	if index > c.highestIndex {
		shift := index - c.highestIndex
		if shift >= 64 {
			c.replayWindow = 1
		} else {
			c.replayWindow = (c.replayWindow << shift) | 1
		}
		c.highestIndex = index
		return
	}
	delta := c.highestIndex - index
	c.replayWindow |= uint64(1) << delta
}

func estimatePacketIndex(sequence uint16, highest uint64, initialized bool) uint64 {
	if !initialized {
		return uint64(sequence)
	}
	roc := uint32(highest >> 16)
	lastSequence := uint16(highest)
	guessedROC := roc
	if lastSequence < 0x8000 {
		if int(sequence)-int(lastSequence) > 0x8000 && roc > 0 {
			guessedROC = roc - 1
		}
	} else if int(lastSequence)-int(sequence) > 0x8000 {
		guessedROC = roc + 1
	}
	return (uint64(guessedROC) << 16) | uint64(sequence)
}

type SRTPSession struct {
	send *SRTPContext
	recv *SRTPContext
}

func NewSRTPSession(sendKey, receiveKey core.SRTPKeyingMaterial, sendAuthLen, receiveAuthLen int) (*SRTPSession, error) {
	sendContext, err := NewSRTPContext(sendKey, sendAuthLen)
	if err != nil {
		return nil, err
	}
	receiveContext, err := NewSRTPContext(receiveKey, receiveAuthLen)
	if err != nil {
		sendContext.Close()
		return nil, err
	}
	return &SRTPSession{send: sendContext, recv: receiveContext}, nil
}

func (s *SRTPSession) Protect(packet *RTPPacket) ([]byte, error) {
	if s == nil || s.send == nil {
		return nil, &SRTPError{Type: SRTPErrClosed, Msg: "send context is unavailable"}
	}
	return s.send.Protect(packet)
}

func (s *SRTPSession) Unprotect(data []byte) (*RTPPacket, error) {
	if s == nil || s.recv == nil {
		return nil, &SRTPError{Type: SRTPErrClosed, Msg: "receive context is unavailable"}
	}
	return s.recv.Unprotect(data)
}

func (s *SRTPSession) SetSendAuthenticationKeying(keying core.SRTPKeyingMaterial) error {
	if s == nil || s.send == nil {
		return &SRTPError{Type: SRTPErrClosed, Msg: "send context is unavailable"}
	}
	return s.send.SetAuthenticationKeying(keying)
}

func (s *SRTPSession) Close() {
	if s == nil {
		return
	}
	if s.send != nil {
		s.send.Close()
	}
	if s.recv != nil {
		s.recv.Close()
	}
	s.send = nil
	s.recv = nil
}

func deriveSRTPKey(masterKey, masterSalt []byte, label byte, length int) ([]byte, error) {
	if len(masterKey) != srtpMasterKeyLength {
		return nil, &SRTPError{Type: SRTPErrInvalidKeying, Msg: fmt.Sprintf("master key length is %d", len(masterKey))}
	}
	if len(masterSalt) != srtpMasterSaltLength {
		return nil, &SRTPError{Type: SRTPErrInvalidKeying, Msg: fmt.Sprintf("master salt length is %d", len(masterSalt))}
	}
	if length <= 0 {
		return nil, &SRTPError{Type: SRTPErrInvalidKeying, Msg: fmt.Sprintf("derived key length is %d", length)}
	}
	iv := make([]byte, aes.BlockSize)
	copy(iv, masterSalt)
	iv[7] ^= label
	output := make([]byte, length)
	if err := aesCTRXOR(masterKey, iv, make([]byte, length), output); err != nil {
		zeroBytes(iv)
		zeroBytes(output)
		return nil, &SRTPError{Type: SRTPErrInvalidKeying, Msg: err.Error()}
	}
	zeroBytes(iv)
	return output, nil
}

func aesCTRXOR(key, iv, source, destination []byte) error {
	if len(iv) != aes.BlockSize {
		return fmt.Errorf("invalid AES CTR IV length: %d", len(iv))
	}
	if len(source) != len(destination) {
		return fmt.Errorf("AES CTR source and destination lengths differ")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	cipher.NewCTR(block, iv).XORKeyStream(destination, source)
	return nil
}
