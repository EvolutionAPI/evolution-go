// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package signaling

import (
	"encoding/base64"
	"sort"
	"strconv"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/wanode"
	waBinary "go.mau.fi/whatsmeow/binary"
)

// ParsedRelayAck contains the complete relay metadata returned in an offer ACK
// or embedded in an incoming offer using WhatsApp's structured te2 encoding.
type ParsedRelayAck struct {
	Relays          []core.RelayEndpoint
	ParticipantJIDs []string
	UUID            string
	SelfPID         *int
	PeerPID         *int
	HBHKey          []byte
}

// ExtractRelayEndpoints parses the older attribute-based relay form:
// <relay ip="..." port="..." token="...">. Some offers wrap candidates in a
// <relays> element, so both layouts are supported.
func ExtractRelayEndpoints(node *waBinary.Node) []core.RelayEndpoint {
	var relays []core.RelayEndpoint

	parseRelay := func(candidate *waBinary.Node) {
		ip := wanode.AttrString(candidate.Attrs, "ip")
		token := wanode.AttrString(candidate.Attrs, "token")
		if ip == "" || token == "" {
			return
		}

		key := firstAttr(candidate.Attrs, "relay-key", "relay_key", "key")
		endpoint := core.RelayEndpoint{
			IP:        ip,
			Port:      firstAttrInt(candidate.Attrs, core.WARelayPort, "port"),
			Token:     token,
			AuthToken: firstAttr(candidate.Attrs, "auth-token", "auth_token"),
			Key:       key,
			RelayID:   firstAttrInt(candidate.Attrs, 0, "relay-id", "relay_id"),
			Protocol:  firstAttrInt(candidate.Attrs, 0, "protocol"),
			RelayName: firstAttr(candidate.Attrs, "relay-name", "relay_name"),
		}
		if value, ok := firstOptionalInt(candidate.Attrs, "c2r-rtt", "c2r_rtt"); ok {
			endpoint.C2RRtt = &value
		}
		relays = append(relays, endpoint)
	}

	for _, childValue := range wanode.NodeChildren(node) {
		child := childValue
		switch child.Tag {
		case "relay":
			parseRelay(&child)
		case "relays":
			for _, relayValue := range wanode.NodeChildren(&child) {
				relay := relayValue
				if relay.Tag == "relay" {
					parseRelay(&relay)
				}
			}
		}
	}

	sortRelaysByRTT(relays)
	return relays
}

// ParseRelayFromAck parses WhatsApp's structured relay response. Binary token
// material is copied so callers never retain slices backed by a protocol node.
func ParseRelayFromAck(node *waBinary.Node) ParsedRelayAck {
	result := ParsedRelayAck{}
	participantSeen := make(map[string]struct{})

	addParticipant := func(jid string) {
		if jid == "" {
			return
		}
		if _, exists := participantSeen[jid]; exists {
			return
		}
		participantSeen[jid] = struct{}{}
		result.ParticipantJIDs = append(result.ParticipantJIDs, jid)
	}

	for _, childValue := range wanode.NodeChildren(node) {
		child := childValue
		if child.Tag == "user" {
			for _, deviceValue := range wanode.NodeChildren(&child) {
				device := deviceValue
				if device.Tag == "device" {
					addParticipant(wanode.AttrString(device.Attrs, "jid"))
				}
			}
		}
		if child.Tag != "relay" {
			continue
		}

		result.UUID = wanode.AttrString(child.Attrs, "uuid")
		if value, ok := firstOptionalInt(child.Attrs, "self_pid", "self-pid"); ok {
			result.SelfPID = &value
		}
		if value, ok := firstOptionalInt(child.Attrs, "peer_pid", "peer-pid"); ok {
			result.PeerPID = &value
		}

		relayChildren := wanode.NodeChildren(&child)
		for _, relayChildValue := range relayChildren {
			relayChild := relayChildValue
			if relayChild.Tag == "participant" {
				addParticipant(wanode.AttrString(relayChild.Attrs, "jid"))
			}
		}

		var relayKey string
		tokens := make(map[string]string)
		authTokens := make(map[string]string)
		rawTokens := make(map[string][]byte)
		rawAuthTokens := make(map[string][]byte)

		for _, relayChildValue := range relayChildren {
			relayChild := relayChildValue
			switch relayChild.Tag {
			case "key":
				if value := wanode.NodeBytes(&relayChild); value != nil {
					relayKey = string(value)
				}
			case "hbh_key":
				result.HBHKey = decodeHBHKey(wanode.NodeBytes(&relayChild))
			case "token":
				if value := wanode.NodeBytes(&relayChild); value != nil {
					id := attrStringOr(relayChild.Attrs, "id", "0")
					rawTokens[id] = append([]byte(nil), value...)
					tokens[id] = base64.StdEncoding.EncodeToString(value)
				}
			case "auth_token":
				if value := wanode.NodeBytes(&relayChild); value != nil {
					id := attrStringOr(relayChild.Attrs, "id", "0")
					rawAuthTokens[id] = append([]byte(nil), value...)
					authTokens[id] = base64.StdEncoding.EncodeToString(value)
				}
			}
		}

		for _, relayChildValue := range relayChildren {
			relayChild := relayChildValue
			if relayChild.Tag != "te2" {
				continue
			}
			address := wanode.NodeBytes(&relayChild)
			if len(address) != 6 {
				continue
			}

			tokenID := attrStringOr(relayChild.Attrs, "token_id", "0")
			authTokenID := firstAttr(relayChild.Attrs, "auth_token_id", "auth-token-id")
			endpoint := core.RelayEndpoint{
				IP:           ipv4String(address[:4]),
				Port:         int(address[4])<<8 | int(address[5]),
				Token:        tokens[tokenID],
				AuthToken:    authTokens[authTokenID],
				RawToken:     append([]byte(nil), rawTokens[tokenID]...),
				RawAuthToken: append([]byte(nil), rawAuthTokens[authTokenID]...),
				Key:          relayKey,
				RelayID:      firstAttrInt(relayChild.Attrs, 0, "relay_id", "relay-id"),
				Protocol:     firstAttrInt(relayChild.Attrs, 0, "protocol"),
				RelayName:    firstAttr(relayChild.Attrs, "relay_name", "relay-name"),
				AddressBytes: append([]byte(nil), address...),
				AuthTokenID:  authTokenID,
			}
			if endpoint.AuthTokenID == "" {
				endpoint.AuthTokenID = tokenID
			}
			if value, ok := firstOptionalInt(relayChild.Attrs, "c2r_rtt", "c2r-rtt"); ok {
				endpoint.C2RRtt = &value
			}
			result.Relays = append(result.Relays, endpoint)
		}
	}

	sortRelaysByRTT(result.Relays)
	return result
}

func decodeHBHKey(value []byte) []byte {
	if len(value) == 30 {
		return append([]byte(nil), value...)
	}
	decoded, err := base64.StdEncoding.DecodeString(string(value))
	if err == nil && len(decoded) == 30 {
		return append([]byte(nil), decoded...)
	}
	return nil
}

func attrStringOr(attrs waBinary.Attrs, key, fallback string) string {
	if value := wanode.AttrString(attrs, key); value != "" {
		return value
	}
	return fallback
}

func firstAttr(attrs waBinary.Attrs, keys ...string) string {
	for _, key := range keys {
		if value := wanode.AttrString(attrs, key); value != "" {
			return value
		}
	}
	return ""
}

func firstAttrInt(attrs waBinary.Attrs, fallback int, keys ...string) int {
	for _, key := range keys {
		if wanode.HasAttr(attrs, key) {
			return wanode.AttrInt(attrs, key, fallback)
		}
	}
	return fallback
}

func firstOptionalInt(attrs waBinary.Attrs, keys ...string) (int, bool) {
	for _, key := range keys {
		if wanode.HasAttr(attrs, key) {
			return wanode.AttrInt(attrs, key, 0), true
		}
	}
	return 0, false
}

func ipv4String(value []byte) string {
	return strconv.Itoa(int(value[0])) + "." + strconv.Itoa(int(value[1])) + "." +
		strconv.Itoa(int(value[2])) + "." + strconv.Itoa(int(value[3]))
}

func sortRelaysByRTT(relays []core.RelayEndpoint) {
	sort.SliceStable(relays, func(left, right int) bool {
		leftRTT := relays[left].C2RRtt
		rightRTT := relays[right].C2RRtt
		switch {
		case leftRTT == nil && rightRTT == nil:
			return false
		case leftRTT == nil:
			return false
		case rightRTT == nil:
			return true
		default:
			return *leftRTT < *rightRTT
		}
	})
}
