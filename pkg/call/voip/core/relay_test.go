package core

import "testing"

func TestCloneRelayDataIsDeepCopy(t *testing.T) {
	rtt := 18
	selfPID := 4
	original := &RelayData{
		Endpoints: []RelayEndpoint{{
			IP:           "1.2.3.4",
			RawToken:     []byte{1, 2, 3},
			RawAuthToken: []byte{4, 5, 6},
			AddressBytes: []byte{1, 2, 3, 4, 13, 152},
			C2RRtt:       &rtt,
		}},
		ParticipantJIDs: []string{"device@s.whatsapp.net"},
		UUID:            "relay-uuid",
		SelfPID:         &selfPID,
		HBHKey:          []byte{7, 8, 9},
	}

	clone := CloneRelayData(original)
	clone.Endpoints[0].RawToken[0] = 99
	clone.Endpoints[0].RawAuthToken[0] = 99
	clone.Endpoints[0].AddressBytes[0] = 99
	*clone.Endpoints[0].C2RRtt = 99
	clone.ParticipantJIDs[0] = "changed"
	clone.HBHKey[0] = 99
	*clone.SelfPID = 99

	if original.Endpoints[0].RawToken[0] != 1 || original.Endpoints[0].RawAuthToken[0] != 4 {
		t.Fatal("clone shares token buffers with original")
	}
	if original.Endpoints[0].AddressBytes[0] != 1 || *original.Endpoints[0].C2RRtt != 18 {
		t.Fatal("clone shares endpoint metadata with original")
	}
	if original.ParticipantJIDs[0] != "device@s.whatsapp.net" || original.HBHKey[0] != 7 || *original.SelfPID != 4 {
		t.Fatal("clone shares relay metadata with original")
	}
}

func TestZeroRelayDataOverwritesBuffers(t *testing.T) {
	rawToken := []byte{1, 2, 3}
	rawAuth := []byte{4, 5, 6}
	address := []byte{7, 8, 9, 10, 13, 152}
	hbh := []byte{11, 12, 13}
	data := &RelayData{
		Endpoints: []RelayEndpoint{{
			Token:        "token",
			AuthToken:    "auth",
			RawToken:     rawToken,
			RawAuthToken: rawAuth,
			AddressBytes: address,
			Key:          "key",
		}},
		ParticipantJIDs: []string{"device@s.whatsapp.net"},
		HBHKey:          hbh,
	}

	ZeroRelayData(data)
	for _, buffer := range [][]byte{rawToken, rawAuth, address, hbh} {
		for _, value := range buffer {
			if value != 0 {
				t.Fatalf("private relay buffer was not overwritten: %v", buffer)
			}
		}
	}
	if data.Endpoints != nil || data.ParticipantJIDs != nil || data.HBHKey != nil {
		t.Fatal("relay references were not cleared")
	}
}
