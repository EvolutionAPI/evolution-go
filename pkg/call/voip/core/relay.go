// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package core

const WARelayPort = 3480

// RelayEndpoint describes one WhatsApp media relay candidate. Raw token fields
// are kept for the future SCTP transport while the base64 forms remain useful
// for diagnostics that do not expose the original byte slices by reference.
type RelayEndpoint struct {
	IP           string
	Port         int
	Token        string
	AuthToken    string
	RawToken     []byte
	RawAuthToken []byte
	Key          string
	RelayID      int
	Protocol     int
	C2RRtt       *int
	RelayName    string
	AddressBytes []byte
	AuthTokenID  string
}

// RelayData is the transport metadata associated with a call. It intentionally
// remains outside public runtime snapshots until a redacted API representation
// is designed.
type RelayData struct {
	Endpoints       []RelayEndpoint
	ParticipantJIDs []string
	UUID            string
	SelfPID         *int
	PeerPID         *int
	HBHKey          []byte
}
