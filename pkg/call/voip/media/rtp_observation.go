package media

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var ErrSelfRTPFrame = errors.New("relay frame belongs to the local RTP sender")

// relayRTPSSRC reads the clear RTP header carried inside an SRTP frame. The
// payload is encrypted, but the RTP version and SSRC remain available before
// authentication/decryption and can be used to correct relay subscriptions.
func relayRTPSSRC(frame []byte) (uint32, bool) {
	if len(frame) < 12 || frame[0]&0xc0 != 0x80 {
		return 0, false
	}
	ssrc := binary.BigEndian.Uint32(frame[8:12])
	return ssrc, ssrc != 0
}

// observePeerSSRC adopts the first real remote SSRC seen on the relay. The
// negotiation-derived value is only a prediction: account-level LIDs received
// in call events may omit the device suffix used by WhatsApp's SSRC derivation.
// After the first remote stream is fixed, later SSRC changes are rejected.
func (s *packetSession) observePeerSSRC(frame []byte) (previous, actual uint32, changed bool, err error) {
	if s == nil {
		return 0, 0, false, ErrPacketSessionNotReady
	}
	actual, ok := relayRTPSSRC(frame)
	if !ok {
		return 0, 0, false, ErrNonRTPFrame
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srtp == nil {
		return 0, 0, false, ErrPacketSessionNotReady
	}
	if actual == s.selfSSRC {
		return s.peerSSRC, actual, false, ErrSelfRTPFrame
	}
	if !s.peerObserved {
		previous = s.peerSSRC
		s.peerSSRC = actual
		s.peerObserved = true
		return previous, actual, previous != actual, nil
	}
	if actual != s.peerSSRC {
		return s.peerSSRC, actual, false, fmt.Errorf("unexpected RTP SSRC: got %d, want %d", actual, s.peerSSRC)
	}
	return s.peerSSRC, actual, false, nil
}
