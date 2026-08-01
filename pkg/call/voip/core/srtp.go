// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package core

const (
	PayloadTypeWhatsAppOpus uint8 = 120

	SRTPSendAuthTagLen = 4
	SRTPRecvAuthTagLen = 4
	SRTPAuthTagLen     = 4

	SRTPLabelEncryption byte = 0x00
	SRTPLabelAuth       byte = 0x01
	SRTPLabelSalt       byte = 0x02
)

// SRTPKeyingMaterial contains the RFC 3711 master key and master salt.
// Callers own these buffers and must call Wipe after the material is consumed.
type SRTPKeyingMaterial struct {
	MasterKey  []byte
	MasterSalt []byte
}

func (m *SRTPKeyingMaterial) Wipe() {
	if m == nil {
		return
	}
	zeroBytes(m.MasterKey)
	zeroBytes(m.MasterSalt)
	m.MasterKey = nil
	m.MasterSalt = nil
}
