//go:build !voip_pion

package transport

import "log/slog"

// NewRelayTransport returns the safe no-network implementation unless the
// experimental voip_pion build tag is explicitly enabled.
func NewRelayTransport(_ *slog.Logger) RelayTransport {
	return NewDisabledRelayTransport()
}
