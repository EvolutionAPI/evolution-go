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

// CloneRelayData makes a defensive deep copy of all relay metadata. This keeps
// private call state independent from buffers owned by incoming protocol nodes.
func CloneRelayData(data *RelayData) *RelayData {
	if data == nil {
		return nil
	}

	clone := &RelayData{
		ParticipantJIDs: append([]string(nil), data.ParticipantJIDs...),
		UUID:            data.UUID,
		SelfPID:         cloneInt(data.SelfPID),
		PeerPID:         cloneInt(data.PeerPID),
		HBHKey:          append([]byte(nil), data.HBHKey...),
	}
	clone.Endpoints = make([]RelayEndpoint, len(data.Endpoints))
	for index, endpoint := range data.Endpoints {
		clone.Endpoints[index] = endpoint
		clone.Endpoints[index].RawToken = append([]byte(nil), endpoint.RawToken...)
		clone.Endpoints[index].RawAuthToken = append([]byte(nil), endpoint.RawAuthToken...)
		clone.Endpoints[index].AddressBytes = append([]byte(nil), endpoint.AddressBytes...)
		clone.Endpoints[index].C2RRtt = cloneInt(endpoint.C2RRtt)
	}
	return clone
}

// ZeroRelayData overwrites byte material and clears references before private
// relay state is discarded. Strings cannot be overwritten in place in Go, so
// their references are dropped immediately.
func ZeroRelayData(data *RelayData) {
	if data == nil {
		return
	}

	zeroBytes(data.HBHKey)
	for index := range data.Endpoints {
		endpoint := &data.Endpoints[index]
		zeroBytes(endpoint.RawToken)
		zeroBytes(endpoint.RawAuthToken)
		zeroBytes(endpoint.AddressBytes)
		endpoint.Token = ""
		endpoint.AuthToken = ""
		endpoint.Key = ""
		endpoint.RelayName = ""
		endpoint.AuthTokenID = ""
		endpoint.RawToken = nil
		endpoint.RawAuthToken = nil
		endpoint.AddressBytes = nil
		endpoint.C2RRtt = nil
	}
	for index := range data.ParticipantJIDs {
		data.ParticipantJIDs[index] = ""
	}
	data.Endpoints = nil
	data.ParticipantJIDs = nil
	data.UUID = ""
	data.SelfPID = nil
	data.PeerPID = nil
	data.HBHKey = nil
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func zeroBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
