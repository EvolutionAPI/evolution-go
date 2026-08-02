package media

import "go.mau.fi/whatsmeow/types"

// selectCallDeviceJIDs chooses concrete device JIDs from the relay participant
// list. The account-level peer LID and the call creator may use different user
// identifiers, so an exact peer match is preferred but the first non-local
// participant is a safer fallback than deriving media keys from user@lid.
func selectCallDeviceJIDs(participants []string, ownJID, peerJID, creatorJID types.JID) (string, string) {
	selfDevice := ensureDeviceJIDString(ownJID.String())
	peerDevice := ensureDeviceJIDString(peerJID.String())
	creatorIsRemote := !creatorJID.IsEmpty() && !sameJIDAccount(creatorJID, ownJID)

	var exactPeer string
	var creatorPeer string
	var fallbackPeer string
	for _, participant := range participants {
		jid, err := types.ParseJID(participant)
		if err != nil || jid.IsEmpty() {
			continue
		}
		device := ensureDeviceJIDString(jid.String())
		if sameJIDAccount(jid, ownJID) {
			selfDevice = device
			continue
		}
		if fallbackPeer == "" {
			fallbackPeer = device
		}
		if sameJIDAccount(jid, peerJID) {
			exactPeer = device
		}
		if creatorIsRemote && sameJIDAccount(jid, creatorJID) {
			creatorPeer = device
		}
	}

	switch {
	case exactPeer != "":
		peerDevice = exactPeer
	case creatorPeer != "":
		peerDevice = creatorPeer
	case fallbackPeer != "":
		peerDevice = fallbackPeer
	case creatorIsRemote:
		peerDevice = ensureDeviceJIDString(creatorJID.String())
	}
	return selfDevice, peerDevice
}
