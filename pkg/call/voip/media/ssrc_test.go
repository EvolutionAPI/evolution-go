package media

import "testing"

func TestGenerateSecureSSRCIsDeterministic(t *testing.T) {
	first, err := GenerateSecureSSRC("call-123", "5511999999999:1@s.whatsapp.net", 0)
	if err != nil {
		t.Fatal(err)
	}
	second, err := GenerateSecureSSRC("call-123", "5511999999999:1@s.whatsapp.net", 0)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("SSRC is not deterministic: %d != %d", first, second)
	}
}

func TestGenerateSecureSSRCChangesWithInputs(t *testing.T) {
	base, err := GenerateSecureSSRC("call-123", "5511999999999:1@s.whatsapp.net", 0)
	if err != nil {
		t.Fatal(err)
	}
	counter, _ := GenerateSecureSSRC("call-123", "5511999999999:1@s.whatsapp.net", 1)
	peer, _ := GenerateSecureSSRC("call-123", "5511888888888:1@s.whatsapp.net", 0)
	otherCall, _ := GenerateSecureSSRC("call-456", "5511999999999:1@s.whatsapp.net", 0)
	for name, value := range map[string]uint32{"counter": counter, "peer": peer, "call": otherCall} {
		if value == base {
			t.Fatalf("%s input did not change SSRC", name)
		}
	}
}

func TestGenerateSecureSSRCValidatesInputs(t *testing.T) {
	if _, err := GenerateSecureSSRC("", "device", 0); err == nil {
		t.Fatal("expected empty call ID error")
	}
	if _, err := GenerateSecureSSRC("call", "", 0); err == nil {
		t.Fatal("expected empty device JID error")
	}
}
