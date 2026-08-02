//go:build voip_pion

package transport

import (
	"log/slog"
	"strings"
	"testing"
)

func TestPionFactorySelectsExperimentalTransport(t *testing.T) {
	transport := NewRelayTransport(slog.Default())
	if _, ok := transport.(*PionRelayTransport); !ok {
		t.Fatalf("expected Pion relay transport, got %T", transport)
	}
}

func TestModifySDPForRelay(t *testing.T) {
	input := strings.Join([]string{
		"v=0",
		"a=setup:actpass",
		"a=ice-ufrag:local-user",
		"a=ice-pwd:local-password",
		"a=fingerprint:sha-256 LOCAL",
		"a=max-message-size:65536",
		"a=ice-options:trickle",
		"a=candidate:1 1 udp 1 10.0.0.1 9999 typ host",
		"a=end-of-candidates",
		"",
	}, "\r\n")

	output := modifySDPForRelay(input, RelayConfig{
		IP:        "203.0.113.9",
		Port:      3480,
		Token:     "relay-token",
		AuthToken: "relay-auth",
		Key:       "relay-password",
	})

	for _, expected := range []string{
		"a=setup:passive",
		"a=ice-ufrag:relay-auth",
		"a=ice-pwd:relay-password",
		"a=max-message-size:1500",
		"203.0.113.9 3480 typ host",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("modified SDP does not contain %q:\n%s", expected, output)
		}
	}
	for _, removed := range []string{"local-user", "local-password", "10.0.0.1 9999", "ice-options:trickle"} {
		if strings.Contains(output, removed) {
			t.Fatalf("modified SDP still contains %q:\n%s", removed, output)
		}
	}
}

func TestRelayConfigCloneAndZero(t *testing.T) {
	original := RelayConfig{
		Token:        "token",
		AuthToken:    "auth",
		RawToken:     []byte{1, 2, 3},
		RawAuthToken: []byte{4, 5, 6},
		Key:          "password",
	}
	clone := cloneRelayConfig(original)
	clone.RawToken[0] = 99
	if original.RawToken[0] != 1 {
		t.Fatal("clone shares raw token storage")
	}
	zeroRelayConfig(&clone)
	if clone.Token != "" || clone.AuthToken != "" || clone.Key != "" || clone.RawToken != nil || clone.RawAuthToken != nil {
		t.Fatalf("relay config was not cleared: %#v", clone)
	}
}

func TestPionTransportStartsDisconnected(t *testing.T) {
	transport := NewPionRelayTransport(nil)
	transport.SetSSRC(123)
	transport.SetSubscriptionSSRC(456)
	if transport.HasConnection() || transport.ConnectedCount() != 0 {
		t.Fatal("new transport unexpectedly has a relay connection")
	}
	transport.Cleanup()
}
