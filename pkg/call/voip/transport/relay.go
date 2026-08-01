// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package transport

import (
	"errors"
	"fmt"
	"sync"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
)

var ErrSCTPUnavailable = errors.New("WhatsApp SCTP relay transport is not enabled")

type RelayConfig struct {
	IP           string
	Port         int
	Token        string
	AuthToken    string
	RawAuthToken []byte
	RawToken     []byte
	Key          string
	RelayID      int
	Name         string
	AuthTokenID  string
}

// RelayTransport is the media-relay boundary used by the future CallManager.
// Implementations may use Pion, another WebRTC stack, or a deterministic fake.
type RelayTransport interface {
	SetSSRC(ssrc uint32)
	SetSubscriptionSSRC(ssrc uint32)
	SetOnConnected(fn func(ip string, port int))
	SetOnReceive(fn func(data []byte))
	ResendSubscriptions()
	ConfigureRelays(relays []RelayConfig) error
	Broadcast(data []byte) error
	HasConnection() bool
	ConnectedCount() int
	Cleanup()
}

// BuildRelayConfigs converts protocol candidates into independent SCTP configs.
// Only UDP relay protocol 0 entries with credentials and a raw token are usable.
func BuildRelayConfigs(endpoints []core.RelayEndpoint) []RelayConfig {
	seen := make(map[string]struct{})
	configs := make([]RelayConfig, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if endpoint.Protocol != 0 || endpoint.IP == "" || endpoint.Key == "" || len(endpoint.RawToken) == 0 {
			continue
		}
		port := endpoint.Port
		if port == 0 {
			port = core.WARelayPort
		}
		identity := fmt.Sprintf("%s:%d#%s", endpoint.IP, port, endpoint.AuthTokenID)
		if _, exists := seen[identity]; exists {
			continue
		}
		seen[identity] = struct{}{}

		name := endpoint.RelayName
		if name == "" {
			name = endpoint.IP
		}
		configs = append(configs, RelayConfig{
			IP:           endpoint.IP,
			Port:         port,
			Token:        endpoint.Token,
			AuthToken:    endpoint.AuthToken,
			RawAuthToken: append([]byte(nil), endpoint.RawAuthToken...),
			RawToken:     append([]byte(nil), endpoint.RawToken...),
			Key:          endpoint.Key,
			RelayID:      endpoint.RelayID,
			Name:         name,
			AuthTokenID:  endpoint.AuthTokenID,
		})
	}
	return configs
}

func ZeroRelayConfigs(configs []RelayConfig) {
	for index := range configs {
		zeroBytes(configs[index].RawToken)
		zeroBytes(configs[index].RawAuthToken)
		configs[index].RawToken = nil
		configs[index].RawAuthToken = nil
		configs[index].Token = ""
		configs[index].AuthToken = ""
		configs[index].Key = ""
		configs[index].Name = ""
		configs[index].AuthTokenID = ""
	}
}

// DisabledRelayTransport is the safe default until the Pion SCTP implementation
// is connected. It preserves callbacks and SSRC configuration but opens no socket.
type DisabledRelayTransport struct {
	mu               sync.RWMutex
	ssrc             uint32
	subscriptionSSRC uint32
	onConnected      func(string, int)
	onReceive        func([]byte)
}

func NewDisabledRelayTransport() *DisabledRelayTransport { return &DisabledRelayTransport{} }
func (d *DisabledRelayTransport) SetSSRC(ssrc uint32) {
	d.mu.Lock()
	d.ssrc = ssrc
	d.mu.Unlock()
}
func (d *DisabledRelayTransport) SetSubscriptionSSRC(ssrc uint32) {
	d.mu.Lock()
	d.subscriptionSSRC = ssrc
	d.mu.Unlock()
}
func (d *DisabledRelayTransport) SetOnConnected(fn func(string, int)) {
	d.mu.Lock()
	d.onConnected = fn
	d.mu.Unlock()
}
func (d *DisabledRelayTransport) SetOnReceive(fn func([]byte)) {
	d.mu.Lock()
	d.onReceive = fn
	d.mu.Unlock()
}
func (d *DisabledRelayTransport) ResendSubscriptions()                  {}
func (d *DisabledRelayTransport) ConfigureRelays([]RelayConfig) error   { return ErrSCTPUnavailable }
func (d *DisabledRelayTransport) Broadcast([]byte) error                { return ErrSCTPUnavailable }
func (d *DisabledRelayTransport) HasConnection() bool                   { return false }
func (d *DisabledRelayTransport) ConnectedCount() int                   { return 0 }
func (d *DisabledRelayTransport) Cleanup()                              {}

var _ RelayTransport = (*DisabledRelayTransport)(nil)

func zeroBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
