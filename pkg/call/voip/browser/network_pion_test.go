//go:build voip_pion

package browser

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
)

func TestReadBrowserNetworkConfig(t *testing.T) {
	tests := []struct {
		name       string
		environment map[string]string
		detected   string
		want       browserNetworkConfig
		wantError  bool
	}{
		{name: "disabled", environment: map[string]string{}},
		{
			name: "explicit endpoint",
			environment: map[string]string{
				browserPublicIPEnv: "203.0.113.20",
				browserMediaPortEnv: "50000",
			},
			want: browserNetworkConfig{enabled: true, publicIP: "203.0.113.20", mediaPort: 50000},
		},
		{
			name: "automatic address",
			environment: map[string]string{
				browserPublicIPEnv: "auto",
				browserMediaPortEnv: "40000",
			},
			detected: "198.51.100.8",
			want: browserNetworkConfig{enabled: true, publicIP: "198.51.100.8", mediaPort: 40000},
		},
		{
			name: "missing port",
			environment: map[string]string{browserPublicIPEnv: "203.0.113.20"},
			wantError: true,
		},
		{
			name: "invalid address",
			environment: map[string]string{
				browserPublicIPEnv: "not-an-ip",
				browserMediaPortEnv: "50000",
			},
			wantError: true,
		},
		{
			name: "invalid port",
			environment: map[string]string{
				browserPublicIPEnv: "203.0.113.20",
				browserMediaPortEnv: "70000",
			},
			wantError: true,
		},
		{
			name: "automatic detection failed",
			environment: map[string]string{
				browserPublicIPEnv: "auto",
				browserMediaPortEnv: "50000",
			},
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			getenv := func(key string) string { return test.environment[key] }
			config, err := readBrowserNetworkConfig(getenv, func() string { return test.detected })
			if test.wantError {
				if err == nil {
					t.Fatalf("expected configuration error, got %+v", config)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if config != test.want {
				t.Fatalf("unexpected configuration: got %+v want %+v", config, test.want)
			}
		})
	}
}

func TestBrowserAPIAdvertisesFixedUDPAndTCPPort(t *testing.T) {
	api, closers, mediaPort, err := buildBrowserAPI(browserNetworkConfig{
		enabled: true,
		publicIP: "127.0.0.1",
		mediaPort: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, closer := range closers {
			_ = closer.Close()
		}
	}()

	server, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err = client.CreateDataChannel(DataChannelLabel, nil); err != nil {
		t.Fatal(err)
	}

	offer, err := client.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	clientGathering := webrtc.GatheringCompletePromise(client)
	if err = client.SetLocalDescription(offer); err != nil {
		t.Fatal(err)
	}
	select {
	case <-clientGathering:
	case <-time.After(10 * time.Second):
		t.Fatal("client ICE gathering timed out")
	}

	if err = server.SetRemoteDescription(*client.LocalDescription()); err != nil {
		t.Fatal(err)
	}
	answer, err := server.CreateAnswer(nil)
	if err != nil {
		t.Fatal(err)
	}
	serverGathering := webrtc.GatheringCompletePromise(server)
	if err = server.SetLocalDescription(answer); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	select {
	case <-serverGathering:
	case <-ctx.Done():
		t.Fatal("server ICE gathering timed out")
	}

	sdp := server.LocalDescription().SDP
	endpoint := fmt.Sprintf(" 127.0.0.1 %d typ host", mediaPort)
	if !strings.Contains(sdp, endpoint) {
		t.Fatalf("SDP does not advertise fixed endpoint %q:\n%s", endpoint, sdp)
	}
	lowerSDP := strings.ToLower(sdp)
	if !strings.Contains(lowerSDP, " udp ") {
		t.Fatalf("SDP does not contain a UDP candidate:\n%s", sdp)
	}
	if !strings.Contains(lowerSDP, " tcp ") || !strings.Contains(lowerSDP, "tcptype passive") {
		t.Fatalf("SDP does not contain a passive ICE-TCP candidate:\n%s", sdp)
	}
}
