package signaling

import (
	"fmt"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/wanode"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

// BuildPostAcceptTransportStanza announces the relay media path after the
// remote party accepts an outgoing call. WhatsApp clients use message type 1
// and candidate round 1 at this stage of the negotiation.
func BuildPostAcceptTransportStanza(peer, creator types.JID, callID string) waBinary.Node {
	return waBinary.Node{
		Tag: "call",
		Attrs: waBinary.Attrs{
			"to": wanode.MustJID(wanode.CleanJID(peer.String())),
			"id": GenerateCallStanzaID(),
		},
		Content: []waBinary.Node{{
			Tag: "transport",
			Attrs: waBinary.Attrs{
				"call-id": callID,
				"call-creator": creator,
				"transport-message-type": "1",
				"p2p-cand-round": "1",
			},
			Content: []waBinary.Node{{
				Tag: "net",
				Attrs: waBinary.Attrs{"medium": "2", "protocol": "0"},
			}},
		}},
	}
}

// BuildMuteV2Stanza synchronizes the initial microphone state with the remote
// WhatsApp device after media negotiation.
func BuildMuteV2Stanza(peer, creator types.JID, callID string, muteState int) waBinary.Node {
	return waBinary.Node{
		Tag: "call",
		Attrs: waBinary.Attrs{
			"to": peer,
			"id": GenerateCallStanzaID(),
		},
		Content: []waBinary.Node{{
			Tag: "mute_v2",
			Attrs: waBinary.Attrs{
				"call-id": callID,
				"call-creator": creator,
				"mute-state": fmt.Sprintf("%d", muteState),
			},
		}},
	}
}

// BuildAcceptReceiptStanza builds the device receipt expected after a remote
// CallAccept. acceptMessageID MUST be the original ID from the outer incoming
// <call> stanza. It must never be generated locally or replaced by callID.
//
// The currently pinned whatsmeow event API does not expose that outer ID, so
// this helper is intentionally not wired into media signaling until the source
// event can provide it without reflection or unsafe access.
func BuildAcceptReceiptStanza(
	peer types.JID,
	acceptMessageID, callID string,
	creator, own types.JID,
) (waBinary.Node, error) {
	if peer.IsEmpty() {
		return waBinary.Node{}, fmt.Errorf("accept receipt peer JID is empty")
	}
	if own.IsEmpty() {
		return waBinary.Node{}, fmt.Errorf("accept receipt own JID is empty")
	}
	if creator.IsEmpty() {
		return waBinary.Node{}, fmt.Errorf("accept receipt creator JID is empty")
	}
	if acceptMessageID == "" {
		return waBinary.Node{}, fmt.Errorf("accept receipt requires original stanza ID")
	}
	if callID == "" {
		return waBinary.Node{}, fmt.Errorf("accept receipt call ID is empty")
	}

	return waBinary.Node{
		Tag: "receipt",
		Attrs: waBinary.Attrs{
			"to":   peer,
			"id":   acceptMessageID,
			"from": own,
		},
		Content: []waBinary.Node{{
			Tag: "accept",
			Attrs: waBinary.Attrs{
				"call-id":      callID,
				"call-creator": creator,
			},
		}},
	}, nil
}
