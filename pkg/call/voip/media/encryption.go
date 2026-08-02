// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package media

import (
	"crypto/hkdf"
	"crypto/sha256"
	"fmt"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
)

const (
	whatsAppCallKeyLength = 32
	srtpHKDFOutputLength  = 46
	srtpMasterKeyLength   = 16
	srtpMasterSaltLength  = 14
)

// DerivePerJIDSRTPKey derives the SRTP master key and salt bound to one
// WhatsApp device JID. The caller owns the returned buffers and must wipe them.
func DerivePerJIDSRTPKey(callKey []byte, deviceJID string) (core.SRTPKeyingMaterial, error) {
	if len(callKey) != whatsAppCallKeyLength {
		return core.SRTPKeyingMaterial{}, fmt.Errorf("invalid WhatsApp call key length: %d", len(callKey))
	}
	if deviceJID == "" {
		return core.SRTPKeyingMaterial{}, fmt.Errorf("device JID is empty")
	}

	output, err := hkdf.Key(sha256.New, callKey, nil, deviceJID, srtpHKDFOutputLength)
	if err != nil {
		return core.SRTPKeyingMaterial{}, fmt.Errorf("derive SRTP key for device: %w", err)
	}
	defer zeroBytes(output)

	material := core.SRTPKeyingMaterial{
		MasterKey:  append([]byte(nil), output[:srtpMasterKeyLength]...),
		MasterSalt: append([]byte(nil), output[srtpMasterKeyLength:srtpMasterKeyLength+srtpMasterSaltLength]...),
	}
	return material, nil
}

func zeroBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
