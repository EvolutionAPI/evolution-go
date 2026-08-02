//go:build voip_pion

package browser

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/pion/webrtc/v4"
)

const (
	browserPublicIPEnv = "CALL_WEBRTC_PUBLIC_IP"
	browserMediaPortEnv = "CALL_WEBRTC_MEDIA_PORT"
)

type browserNetworkConfig struct {
	enabled   bool
	publicIP string
	mediaPort int
}

type publicIPDetector func() string

type environmentReader func(string) string

var browserAPISingleton struct {
	once sync.Once
	api  *webrtc.API
	err  error
}

// newBrowserPeerConnection uses one process-wide Pion API so every browser
// session can share the configured UDP/TCP ICE muxes and fixed media port.
func newBrowserPeerConnection() (*webrtc.PeerConnection, error) {
	api, err := configuredBrowserAPI()
	if err != nil {
		return nil, err
	}
	return api.NewPeerConnection(webrtc.Configuration{})
}

func configuredBrowserAPI() (*webrtc.API, error) {
	browserAPISingleton.once.Do(func() {
		config, err := readBrowserNetworkConfig(os.Getenv, detectPublicIPv4)
		if err != nil {
			browserAPISingleton.err = err
			return
		}
		api, _, actualPort, err := buildBrowserAPI(config)
		if err != nil {
			browserAPISingleton.err = err
			return
		}
		browserAPISingleton.api = api
		if config.enabled {
			slog.Info("browser WebRTC fixed ICE endpoint enabled",
				"public_ip", config.publicIP,
				"media_port", actualPort,
				"udp", true,
				"ice_tcp", true,
			)
		}
	})
	if browserAPISingleton.err != nil {
		return nil, browserAPISingleton.err
	}
	if browserAPISingleton.api == nil {
		return nil, fmt.Errorf("browser WebRTC API is not initialized")
	}
	return browserAPISingleton.api, nil
}

func readBrowserNetworkConfig(getenv environmentReader, detect publicIPDetector) (browserNetworkConfig, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	publicIP := strings.TrimSpace(getenv(browserPublicIPEnv))
	portValue := strings.TrimSpace(getenv(browserMediaPortEnv))
	if publicIP == "" && portValue == "" {
		return browserNetworkConfig{}, nil
	}
	if publicIP == "" || portValue == "" {
		return browserNetworkConfig{}, fmt.Errorf("%s and %s must be configured together", browserPublicIPEnv, browserMediaPortEnv)
	}
	if strings.EqualFold(publicIP, "auto") {
		if detect == nil {
			return browserNetworkConfig{}, fmt.Errorf("detect public IPv4 address: detector is unavailable")
		}
		publicIP = strings.TrimSpace(detect())
		if publicIP == "" {
			return browserNetworkConfig{}, fmt.Errorf("detect public IPv4 address for %s=auto", browserPublicIPEnv)
		}
	}
	parsedIP := net.ParseIP(publicIP)
	if parsedIP == nil || parsedIP.To4() == nil {
		return browserNetworkConfig{}, fmt.Errorf("%s must be an IPv4 address or auto", browserPublicIPEnv)
	}
	mediaPort, err := strconv.Atoi(portValue)
	if err != nil || mediaPort < 1 || mediaPort > 65535 {
		return browserNetworkConfig{}, fmt.Errorf("%s must be an integer between 1 and 65535", browserMediaPortEnv)
	}
	return browserNetworkConfig{
		enabled:   true,
		publicIP: parsedIP.To4().String(),
		mediaPort: mediaPort,
	}, nil
}

// buildBrowserAPI binds UDP and TCP on the same port. A zero mediaPort is
// accepted only for tests; environment parsing always requires a fixed port.
func buildBrowserAPI(config browserNetworkConfig) (*webrtc.API, []io.Closer, int, error) {
	if !config.enabled {
		return webrtc.NewAPI(), nil, 0, nil
	}
	if parsedIP := net.ParseIP(config.publicIP); parsedIP == nil || parsedIP.To4() == nil {
		return nil, nil, 0, fmt.Errorf("invalid browser WebRTC advertised IPv4 address %q", config.publicIP)
	}
	if config.mediaPort < 0 || config.mediaPort > 65535 {
		return nil, nil, 0, fmt.Errorf("invalid browser WebRTC media port %d", config.mediaPort)
	}

	udpConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: config.mediaPort})
	if err != nil {
		return nil, nil, 0, fmt.Errorf("bind browser WebRTC UDP port %d: %w", config.mediaPort, err)
	}
	actualPort := udpConn.LocalAddr().(*net.UDPAddr).Port
	tcpListener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4zero, Port: actualPort})
	if err != nil {
		_ = udpConn.Close()
		return nil, nil, 0, fmt.Errorf("bind browser WebRTC ICE-TCP port %d: %w", actualPort, err)
	}

	settingEngine := webrtc.SettingEngine{}
	settingEngine.SetNAT1To1IPs([]string{config.publicIP}, webrtc.ICECandidateTypeHost)
	settingEngine.SetNetworkTypes([]webrtc.NetworkType{
		webrtc.NetworkTypeUDP4,
		webrtc.NetworkTypeTCP4,
	})
	settingEngine.SetICEUDPMux(webrtc.NewICEUDPMux(nil, udpConn))
	settingEngine.SetICETCPMux(webrtc.NewICETCPMux(nil, tcpListener, 8))

	api := webrtc.NewAPI(webrtc.WithSettingEngine(settingEngine))
	return api, []io.Closer{tcpListener, udpConn}, actualPort, nil
}

// detectPublicIPv4 resolves the local IPv4 selected by the default route. On a
// host-networked VPS this is normally the public address. Behind NAT, set the
// externally routed address explicitly instead of using auto.
func detectPublicIPv4() string {
	connection, err := net.Dial("udp4", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer connection.Close()
	address, ok := connection.LocalAddr().(*net.UDPAddr)
	if !ok || address.IP == nil {
		return ""
	}
	return address.IP.To4().String()
}
