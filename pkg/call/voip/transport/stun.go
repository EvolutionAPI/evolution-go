// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package transport

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"net"
	"strings"
)

const (
	stunMagicCookie     = 0x2112a442
	stunFingerprintXOR  = 0x5354554e
	stunBindingRequest  = 0x0001
	stunAllocateRequest = 0x0003
	whatsAppPing        = 0x0801

	attrUsername            = 0x0006
	attrMessageIntegrity    = 0x0008
	attrXORRelayedAddress   = 0x0016
	attrPriority            = 0x0024
	attrSenderSubscriptions = 0x4000
	attrSSRCList            = 0x4024
	attrICEControlling      = 0x802a
	attrFingerprint         = 0x8028

	defaultICEPriority = 16_777_215
)

func generateTransactionID() ([]byte, error) {
	id := make([]byte, 12)
	if _, err := rand.Read(id); err != nil {
		return nil, fmt.Errorf("generate STUN transaction ID: %w", err)
	}
	return id, nil
}

func encodeAttribute(attributeType int, data []byte) []byte {
	header := make([]byte, 4)
	binary.BigEndian.PutUint16(header[0:], uint16(attributeType))
	binary.BigEndian.PutUint16(header[2:], uint16(len(data)))
	padding := (4 - (len(data) % 4)) % 4
	output := append(header, data...)
	return append(output, make([]byte, padding)...)
}

func buildSTUNMessage(messageType int, attributes, transactionID, integrityKey []byte, includeFingerprint bool) []byte {
	attributesData := append([]byte(nil), attributes...)

	if len(integrityKey) > 0 {
		messageLengthForHMAC := len(attributesData) + 24
		hmacHeader := make([]byte, 20)
		binary.BigEndian.PutUint16(hmacHeader[0:], uint16(messageType))
		binary.BigEndian.PutUint16(hmacHeader[2:], uint16(messageLengthForHMAC))
		binary.BigEndian.PutUint32(hmacHeader[4:], stunMagicCookie)
		copy(hmacHeader[8:], transactionID)

		mac := hmac.New(sha1.New, integrityKey)
		_, _ = mac.Write(hmacHeader)
		_, _ = mac.Write(attributesData)
		attributesData = append(attributesData, encodeAttribute(attrMessageIntegrity, mac.Sum(nil))...)
	}

	if includeFingerprint {
		messageLengthForCRC := len(attributesData) + 8
		crcHeader := make([]byte, 20)
		binary.BigEndian.PutUint16(crcHeader[0:], uint16(messageType))
		binary.BigEndian.PutUint16(crcHeader[2:], uint16(messageLengthForCRC))
		binary.BigEndian.PutUint32(crcHeader[4:], stunMagicCookie)
		copy(crcHeader[8:], transactionID)

		crcInput := append(append([]byte(nil), crcHeader...), attributesData...)
		fingerprint := crc32.ChecksumIEEE(crcInput) ^ stunFingerprintXOR
		fingerprintBuffer := make([]byte, 4)
		binary.BigEndian.PutUint32(fingerprintBuffer, fingerprint)
		attributesData = append(attributesData, encodeAttribute(attrFingerprint, fingerprintBuffer)...)
	}

	header := make([]byte, 20)
	binary.BigEndian.PutUint16(header[0:], uint16(messageType))
	binary.BigEndian.PutUint16(header[2:], uint16(len(attributesData)))
	binary.BigEndian.PutUint32(header[4:], stunMagicCookie)
	copy(header[8:], transactionID)
	return append(header, attributesData...)
}

func encodeXORRelayedAddress(ip string, port int) ([]byte, error) {
	parsedIP := net.ParseIP(ip).To4()
	if parsedIP == nil {
		return nil, fmt.Errorf("relay IP %q is not IPv4", ip)
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("relay port %d is invalid", port)
	}

	data := make([]byte, 8)
	data[1] = 0x01
	binary.BigEndian.PutUint16(data[2:], uint16(port)^uint16(stunMagicCookie>>16))
	ipNumber := binary.BigEndian.Uint32(parsedIP)
	binary.BigEndian.PutUint32(data[4:], ipNumber^stunMagicCookie)
	return data, nil
}

// BuildAllocateForRelay builds the WhatsApp relay allocation request.
func BuildAllocateForRelay(senderSubscriptions, ssrcList, hmacKey []byte, relayIP string, relayPort int) ([]byte, error) {
	transactionID, err := generateTransactionID()
	if err != nil {
		return nil, err
	}
	parts := [][]byte{
		encodeAttribute(attrSenderSubscriptions, senderSubscriptions),
		encodeAttribute(attrSSRCList, ssrcList),
	}
	if relayIP != "" && relayPort != 0 {
		address, addressErr := encodeXORRelayedAddress(relayIP, relayPort)
		if addressErr != nil {
			return nil, addressErr
		}
		parts = append(parts, encodeAttribute(attrXORRelayedAddress, address))
	}
	return buildSTUNMessage(stunAllocateRequest, concat(parts...), transactionID, hmacKey, false), nil
}

// BuildBindingRequestWithSubscriptions creates a STUN binding request carrying
// WhatsApp sender subscriptions.
func BuildBindingRequestWithSubscriptions(username, hmacKey, senderSubscriptions []byte, includeICEControlling, includeFingerprint bool) ([]byte, error) {
	transactionID, err := generateTransactionID()
	if err != nil {
		return nil, err
	}
	var parts [][]byte
	if len(username) > 0 {
		parts = append(parts, encodeAttribute(attrUsername, username))
	}
	priority := make([]byte, 4)
	binary.BigEndian.PutUint32(priority, defaultICEPriority)
	parts = append(parts, encodeAttribute(attrPriority, priority))
	if includeICEControlling {
		tieBreaker := make([]byte, 8)
		if _, err = rand.Read(tieBreaker); err != nil {
			return nil, fmt.Errorf("generate ICE tie breaker: %w", err)
		}
		parts = append(parts, encodeAttribute(attrICEControlling, tieBreaker))
	}
	if len(senderSubscriptions) > 0 {
		parts = append(parts, encodeAttribute(attrSenderSubscriptions, senderSubscriptions))
	}
	return buildSTUNMessage(stunBindingRequest, concat(parts...), transactionID, hmacKey, includeFingerprint), nil
}

// BuildWhatsAppPing returns the proprietary keepalive frame used by relays.
func BuildWhatsAppPing() ([]byte, error) {
	transactionID, err := generateTransactionID()
	if err != nil {
		return nil, err
	}
	header := make([]byte, 20)
	binary.BigEndian.PutUint16(header[0:], whatsAppPing)
	binary.BigEndian.PutUint32(header[4:], stunMagicCookie)
	copy(header[8:], transactionID)
	return header, nil
}

func IsSTUNPacket(data []byte) bool { return len(data) >= 2 && data[0]&0xc0 == 0 }
func IsRTPPacket(data []byte) bool  { return len(data) >= 2 && data[0]&0xc0 == 0x80 }

type STUNAttribute struct {
	Type     int
	TypeName string
	Length   int
	Data     []byte
}

type STUNResponseInfo struct {
	RawType             int
	Method              string
	Class               string
	IsSuccess           bool
	IsError             bool
	ErrorCode           int
	ErrorReason         string
	StableRoutingConnID uint64
	TransactionID       string
	Length              int
	Attributes          []STUNAttribute
}

var stunAttributeNames = map[int]string{
	0x0001: "MAPPED-ADDRESS", 0x0006: "USERNAME", 0x0008: "MESSAGE-INTEGRITY",
	0x0009: "ERROR-CODE", 0x0016: "XOR-RELAYED-ADDRESS", 0x0020: "XOR-MAPPED-ADDRESS",
	0x0024: "PRIORITY", 0x4000: "SENDER-SUBSCRIPTIONS", 0x4001: "RECEIVER-SUBSCRIPTION",
	0x4002: "SUBSCRIPTION-ACK", 0x4024: "SSRC-LIST", 0x4033: "STABLE-ROUTING-CONN-ID",
	0x8028: "FINGERPRINT", 0x8029: "ICE-CONTROLLED", 0x802a: "ICE-CONTROLLING",
}

func ParseSTUNResponse(data []byte) *STUNResponseInfo {
	if len(data) < 20 || binary.BigEndian.Uint32(data[4:]) != stunMagicCookie {
		return nil
	}

	rawType := int(binary.BigEndian.Uint16(data[0:]))
	messageLength := int(binary.BigEndian.Uint16(data[2:]))
	if 20+messageLength > len(data) {
		return nil
	}
	classNumber := (((rawType >> 8) & 0x1) << 1) | ((rawType >> 4) & 0x1)
	classes := []string{"request", "indication", "success", "error"}
	class := "unknown"
	if classNumber < len(classes) {
		class = classes[classNumber]
	}
	methodBits := ((rawType & 0x3e00) >> 2) | ((rawType & 0x00e0) >> 1) | (rawType & 0x000f)
	method := map[int]string{0x001: "binding", 0x003: "allocate", 0x004: "refresh", 0x006: "send", 0x007: "data", 0x008: "create-permission", 0x009: "channel-bind"}[methodBits]
	if method == "" {
		method = "unknown"
	}
	if rawType == 0x0801 {
		method = "wa-ping"
	} else if rawType == 0x0802 {
		method = "wa-pong"
	}

	info := &STUNResponseInfo{
		RawType:       rawType,
		Method:        method,
		Class:         class,
		IsSuccess:     class == "success",
		IsError:       class == "error",
		TransactionID: hex.EncodeToString(data[8:20]),
		Length:        len(data),
	}

	for offset := 20; offset+4 <= 20+messageLength; {
		attributeType := int(binary.BigEndian.Uint16(data[offset:]))
		attributeLength := int(binary.BigEndian.Uint16(data[offset+2:]))
		attributeEnd := offset + 4 + attributeLength
		if attributeEnd > len(data) || attributeEnd > 20+messageLength {
			return nil
		}
		attributeData := append([]byte(nil), data[offset+4:attributeEnd]...)
		name := stunAttributeNames[attributeType]
		if name == "" {
			name = fmt.Sprintf("0x%04x", attributeType)
		}
		info.Attributes = append(info.Attributes, STUNAttribute{Type: attributeType, TypeName: name, Length: attributeLength, Data: attributeData})
		if attributeType == 0x0009 && attributeLength >= 4 {
			info.ErrorCode = int(attributeData[2]&0x07)*100 + int(attributeData[3])
			if attributeLength > 4 {
				info.ErrorReason = string(attributeData[4:])
			}
		}
		if attributeType == 0x4033 && class == "success" && attributeLength == 8 {
			info.StableRoutingConnID = binary.BigEndian.Uint64(attributeData)
		}
		offset = attributeEnd + ((4 - (attributeLength % 4)) % 4)
	}
	return info
}

func ClassifyPacket(data []byte) string {
	if len(data) < 2 {
		return fmt.Sprintf("tiny(%dB)", len(data))
	}
	switch (data[0] & 0xc0) >> 6 {
	case 0:
		if info := ParseSTUNResponse(data); info != nil {
			result := fmt.Sprintf("STUN %s %s (0x%04x, %dB)", info.Method, info.Class, info.RawType, info.Length)
			if len(info.Attributes) > 0 {
				names := make([]string, len(info.Attributes))
				for index, attribute := range info.Attributes {
					names[index] = attribute.TypeName
				}
				result += " [" + strings.Join(names, ", ") + "]"
			}
			return result
		}
		return fmt.Sprintf("STUN? 0x%x (%dB)", int(data[0])<<8|int(data[1]), len(data))
	case 2:
		sequence := 0
		if len(data) >= 4 {
			sequence = int(binary.BigEndian.Uint16(data[2:4]))
		}
		return fmt.Sprintf("RTP/SRTP PT=%d M=%d seq=%d (%dB)", data[1]&0x7f, data[1]>>7, sequence, len(data))
	case 1:
		return fmt.Sprintf("DTLS? 0x%x (%dB)", data[0], len(data))
	default:
		return fmt.Sprintf("unknown 0x%x (%dB)", data[0], len(data))
	}
}
