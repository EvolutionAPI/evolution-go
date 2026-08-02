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
