package media

import "testing"

func TestWhatsAppOpusSessionUsesDEBEExtension(t *testing.T) {
	session, err := NewWhatsAppOpusRTPSession(1234)
	if err != nil {
		t.Fatal(err)
	}
	packet := session.CreatePacket([]byte{1, 2, 3}, true)
	if packet.Header == nil || !packet.Header.Extension {
		t.Fatal("expected RTP extension")
	}
	if packet.Header.ExtensionProfile != whatsAppRTPDEBEProfile {
		t.Fatalf("unexpected profile: %x", packet.Header.ExtensionProfile)
	}
	encoded, err := packet.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ParseRTPPacket(encoded)
	if err != nil {
		t.Fatal(err)
	}
	defer decoded.Wipe()
	if !decoded.Header.Extension || decoded.Header.ExtensionProfile != whatsAppRTPDEBEProfile {
		t.Fatal("extension did not survive RTP round trip")
	}
}
