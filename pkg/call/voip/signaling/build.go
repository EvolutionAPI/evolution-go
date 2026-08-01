// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package signaling

import (
	"context"
	"fmt"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/wanode"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

var capabilityOffer = []byte{0x01, 0x05, 0xf7, 0x09, 0xe4, 0xbb, 0x07}

func BuildOfferStanza(ctx context.Context, socket core.VoipSocket, callID string, callKey []byte, peer types.JID, video bool) (waBinary.Node, error) {
	creator := socket.OwnLID()
	if creator.IsEmpty() {
		creator = socket.OwnPN()
	}
	if creator.IsEmpty() {
		return waBinary.Node{}, fmt.Errorf("whatsapp client has no own JID")
	}

	resolvedPeer := socket.ResolveLIDForPN(ctx, peer)
	devices, err := socket.GetUSyncDevices(ctx, []types.JID{resolvedPeer})
	if err != nil {
		return waBinary.Node{}, fmt.Errorf("get peer devices: %w", err)
	}
	if len(devices) == 0 {
		return waBinary.Node{}, fmt.Errorf("no WhatsApp devices found for %s", peer.String())
	}
	if err := socket.AssertSessions(ctx, devices, false); err != nil {
		return waBinary.Node{}, fmt.Errorf("assert sessions: %w", err)
	}

	participants, includeIdentity, err := socket.CreateParticipantNodes(
		ctx,
		devices,
		callKey,
		waBinary.Attrs{"count": "0"},
	)
	if err != nil {
		return waBinary.Node{}, fmt.Errorf("encrypt call key: %w", err)
	}

	content := make([]waBinary.Node, 0, 8)
	if token, tokenErr := socket.GetTCToken(ctx, wanode.MustJID(wanode.CleanJID(resolvedPeer.String()))); tokenErr == nil && len(token) > 0 {
		content = append(content, waBinary.Node{Tag: "privacy", Content: token})
	}
	content = append(content,
		waBinary.Node{Tag: "audio", Attrs: waBinary.Attrs{"enc": "opus", "rate": "8000"}},
		waBinary.Node{Tag: "audio", Attrs: waBinary.Attrs{"enc": "opus", "rate": "16000"}},
	)
	if video {
		content = append(content, waBinary.Node{Tag: "video", Attrs: waBinary.Attrs{
			"enc":                "vp8",
			"dec":                "vp8",
			"orientation":        "0",
			"screen_width":       "1920",
			"screen_height":      "1080",
			"device_orientation": "0",
		}})
	}
	content = append(content,
		waBinary.Node{Tag: "net", Attrs: waBinary.Attrs{"medium": "3"}},
		waBinary.Node{Tag: "capability", Attrs: waBinary.Attrs{"ver": "1"}, Content: capabilityOffer},
		waBinary.Node{Tag: "destination", Content: participants},
		waBinary.Node{Tag: "encopt", Attrs: waBinary.Attrs{"keygen": "2"}},
	)
	if includeIdentity {
		if identity, ok := socket.AccountDeviceIdentityNode(); ok {
			content = append(content, identity)
		}
	}

	return waBinary.Node{
		Tag:   "call",
		Attrs: waBinary.Attrs{"to": resolvedPeer, "id": GenerateCallStanzaID()},
		Content: []waBinary.Node{{
			Tag:     "offer",
			Attrs:   waBinary.Attrs{"call-id": callID, "call-creator": creator},
			Content: content,
		}},
	}, nil
}

func BuildTerminateStanza(peer types.JID, callID string, creator types.JID) waBinary.Node {
	return wrap(peer, waBinary.Node{
		Tag:   "terminate",
		Attrs: waBinary.Attrs{"call-id": callID, "call-creator": creator},
	})
}

func BuildRejectStanza(peer types.JID, callID string, creator types.JID) waBinary.Node {
	return wrap(peer, waBinary.Node{
		Tag:   "reject",
		Attrs: waBinary.Attrs{"call-id": callID, "call-creator": creator},
	})
}

func wrap(to types.JID, inner waBinary.Node) waBinary.Node {
	return waBinary.Node{
		Tag:     "call",
		Attrs:   waBinary.Attrs{"to": to, "id": GenerateCallStanzaID()},
		Content: []waBinary.Node{inner},
	}
}
