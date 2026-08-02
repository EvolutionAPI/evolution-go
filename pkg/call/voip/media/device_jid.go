package media

import (
	"strings"

	"go.mau.fi/whatsmeow/types"
)

// ensureDeviceJIDString normalizes account-level JIDs to the device form used
// by WhatsApp's SSRC and per-JID SRTP derivation. Relay participant entries
// normally include a device number, while call accept events may only expose
// the account-level LID/PN.
func ensureDeviceJIDString(value string) string {
	at := strings.IndexByte(value, '@')
	if at <= 0 {
		return value
	}
	if colon := strings.IndexByte(value[:at], ':'); colon >= 0 {
		return value
	}
	return value[:at] + ":0" + value[at:]
}

func sameJIDAccount(left, right types.JID) bool {
	if left.IsEmpty() || right.IsEmpty() {
		return false
	}
	return left.User == right.User
}
