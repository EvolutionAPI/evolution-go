// Package signaling builds and parses WhatsApp <call> protocol nodes.
// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package signaling

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func GenerateCallID() string {
	buffer := make([]byte, 16)
	_, _ = rand.Read(buffer)
	return strings.ToUpper(hex.EncodeToString(buffer))
}

func GenerateCallStanzaID() string {
	buffer := make([]byte, 16)
	_, _ = rand.Read(buffer)
	return strings.ToUpper(hex.EncodeToString(buffer))
}

func GenerateCallKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate call key: %w", err)
	}
	return key, nil
}

func EncodeCallKeyMessage(callKey []byte) ([]byte, error) {
	message := &waE2E.Message{Call: &waE2E.Call{CallKey: callKey}}
	return proto.Marshal(message)
}

func DecodeCallKeyPlaintext(plaintext []byte) ([]byte, error) {
	var message waE2E.Message
	if err := proto.Unmarshal(plaintext, &message); err != nil {
		return nil, err
	}
	key := message.GetCall().GetCallKey()
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid call key: expected 32 bytes, got %d", len(key))
	}
	return key, nil
}
