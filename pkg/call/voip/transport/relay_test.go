package transport

import (
	"errors"
	"testing"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
)

func TestBuildRelayConfigsFiltersAndCopies(t *testing.T) {
	token := []byte{1, 2, 3}
	auth := []byte{4, 5, 6}
	configs := BuildRelayConfigs([]core.RelayEndpoint{
		{
			IP:           "10.0.0.1",
			Port:         0,
			Protocol:     0,
			Key:          "relay-key",
			RawToken:     token,
			RawAuthToken: auth,
			AuthTokenID:  "a",
		},
		{
			IP:           "10.0.0.1",
			Protocol:     0,
			Key:          "relay-key",
			RawToken:     []byte{9},
			AuthTokenID:  "a",
		},
		{
			IP:       "10.0.0.2",
			Protocol: 1,
			Key:      "ignored",
			RawToken: []byte{7},
		},
		{
			IP:       "10.0.0.3",
			Protocol: 0,
			RawToken: []byte{8},
		},
	})

	if len(configs) != 1 {
		t.Fatalf("config count = %d, want 1", len(configs))
	}
	if configs[0].Port != core.WARelayPort || configs[0].Name != "10.0.0.1" {
		t.Fatalf("unexpected defaults: %+v", configs[0])
	}
	configs[0].RawToken[0] = 99
	configs[0].RawAuthToken[0] = 99
	if token[0] != 1 || auth[0] != 4 {
		t.Fatal("relay config shares private buffers with protocol endpoint")
	}
}

func TestZeroRelayConfigsOverwritesBuffers(t *testing.T) {
	token := []byte{1, 2}
	auth := []byte{3, 4}
	configs := []RelayConfig{{
		Token:        "token",
		AuthToken:    "auth",
		RawToken:     token,
		RawAuthToken: auth,
		Key:          "key",
	}}
	ZeroRelayConfigs(configs)
	for _, buffer := range [][]byte{token, auth} {
		for _, value := range buffer {
			if value != 0 {
				t.Fatalf("buffer was not overwritten: %v", buffer)
			}
		}
	}
	if configs[0].RawToken != nil || configs[0].RawAuthToken != nil || configs[0].Key != "" {
		t.Fatal("relay config references were not cleared")
	}
}

func TestDisabledRelayTransportFailsClosed(t *testing.T) {
	transport := NewDisabledRelayTransport()
	if !errors.Is(transport.ConfigureRelays(nil), ErrSCTPUnavailable) {
		t.Fatal("disabled transport must reject relay configuration")
	}
	if !errors.Is(transport.Broadcast([]byte{1}), ErrSCTPUnavailable) {
		t.Fatal("disabled transport must reject media writes")
	}
	if transport.HasConnection() || transport.ConnectedCount() != 0 {
		t.Fatal("disabled transport reported a connection")
	}
}
