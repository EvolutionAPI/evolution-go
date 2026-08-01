// Package wanode contains WhatsApp call-node and JID helpers.
// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package wanode

import (
	"strings"

	"go.mau.fi/whatsmeow/types"
)

func CleanJID(jid string) string {
	if index := strings.Index(jid, ":"); index >= 0 {
		if at := strings.Index(jid, "@"); at > index {
			return jid[:index] + jid[at:]
		}
	}
	return jid
}

func MustJID(value string) types.JID {
	jid, err := types.ParseJID(value)
	if err != nil {
		return types.JID{}
	}
	return jid
}
