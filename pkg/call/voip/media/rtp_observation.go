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

// peerSSRCCandidate validates whether a relay frame can belong to the remote
// stream. The first non-local SSRC is allowed as a candidate, but it is only
// committed after SRTP authentication succeeds.
func (s *packetSession) peerSSRCCandidate(frame []byte) (previous, actual uint32, first bool, err error) {
	if s == nil {
		return 0, 0, false, ErrPacketSessionNotReady
	}
	actual, ok := relayRTPSSRC(frame)
	if !ok {
		return 0, 0, false, ErrNonRTPFrame
	}
	if actual == s.selfSSRC {
		return s.peerSSRC, actual, false, ErrSelfRTPFrame
	}
	if s.peerObserved && actual != s.peerSSRC {
		return s.peerSSRC, actual, false, fmt.Errorf("unexpected RTP SSRC: got %d, want %d", actual, s.peerSSRC)
	}
	return s.peerSSRC, actual, !s.peerObserved, nil
}

func (s *packetSession) commitPeerSSRC(actual uint32) (previous uint32, changed bool) {
	previous = s.peerSSRC
	if !s.peerObserved {
		s.peerSSRC = actual
		s.peerObserved = true
	}
	return previous, previous != actual
}
