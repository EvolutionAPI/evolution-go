// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package media

import (
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// GenerateSecureSSRC deterministically derives a WhatsApp media SSRC from the
// call ID, device JID and stream counter.
func GenerateSecureSSRC(callID, deviceJID string, counter uint32) (uint32, error) {
	if callID == "" {
		return 0, fmt.Errorf("call ID is empty")
	}
	if deviceJID == "" {
		return 0, fmt.Errorf("device JID is empty")
	}
	salt := make([]byte, 4)
	binary.LittleEndian.PutUint32(salt, counter)
	output, err := hkdf.Key(sha256.New, []byte(callID), salt, deviceJID, 4)
	if err != nil {
		return 0, fmt.Errorf("derive SSRC: %w", err)
	}
	ssrc := binary.LittleEndian.Uint32(output)
	if ssrc == 0 {
		return 0, fmt.Errorf("derived SSRC is zero")
	}
	return ssrc, nil
}
