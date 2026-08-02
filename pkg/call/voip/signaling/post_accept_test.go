package signaling

import (
	"testing"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/wanode"
	"go.mau.fi/whatsmeow/types"
)

func TestBuildPostAcceptTransportStanza(t *testing.T) {
	peer := types.NewJID("5511999999999", types.HiddenUserServer)
	creator := types.NewJID("5511000000000", types.HiddenUserServer)
	node := BuildPostAcceptTransportStanza(peer, creator, "CALL-POST")

	children := wanode.NodeChildren(&node)
	if len(children) != 1 || children[0].Tag != "transport" {
		t.Fatalf("unexpected transport stanza: %#v", children)
	}
	transport := children[0]
	if wanode.AttrString(transport.Attrs, "call-id") != "CALL-POST" {
		t.Fatalf("unexpected call ID: %s", wanode.AttrString(transport.Attrs, "call-id"))
	}
	if wanode.AttrString(transport.Attrs, "transport-message-type") != "1" {
		t.Fatalf("unexpected transport type: %s", wanode.AttrString(transport.Attrs, "transport-message-type"))
	}
	if wanode.AttrString(transport.Attrs, "p2p-cand-round") != "1" {
		t.Fatalf("unexpected candidate round: %s", wanode.AttrString(transport.Attrs, "p2p-cand-round"))
	}
	netChildren := wanode.NodeChildren(&transport)
	if len(netChildren) != 1 || netChildren[0].Tag != "net" {
		t.Fatalf("transport is missing net child: %#v", netChildren)
	}
	if wanode.AttrString(netChildren[0].Attrs, "protocol") != "0" {
		t.Fatalf("unexpected transport protocol: %s", wanode.AttrString(netChildren[0].Attrs, "protocol"))
	}
}

func TestBuildMuteV2Stanza(t *testing.T) {
	peer := types.NewJID("5511999999999", types.HiddenUserServer)
	creator := types.NewJID("5511000000000", types.HiddenUserServer)
	node := BuildMuteV2Stanza(peer, creator, "CALL-MUTE", 0)

	children := wanode.NodeChildren(&node)
	if len(children) != 1 || children[0].Tag != "mute_v2" {
		t.Fatalf("unexpected mute stanza: %#v", children)
	}
	if wanode.AttrString(children[0].Attrs, "mute-state") != "0" {
		t.Fatalf("unexpected mute state: %s", wanode.AttrString(children[0].Attrs, "mute-state"))
	}
	if wanode.AttrString(children[0].Attrs, "call-id") != "CALL-MUTE" {
		t.Fatalf("unexpected call ID: %s", wanode.AttrString(children[0].Attrs, "call-id"))
	}
}

func TestBuildAcceptReceiptStanza(t *testing.T) {
	peer := types.NewADJID("5511999999999", 0, 7)
	creator := types.NewJID("5511999999999", types.HiddenUserServer)
	own := types.NewADJID("5511000000000", 0, 3)

	node, err := BuildAcceptReceiptStanza(peer, "ACCEPT-STANZA-ID", "CALL-RECEIPT", creator, own)
	if err != nil {
		t.Fatal(err)
	}
	if node.Tag != "receipt" {
		t.Fatalf("unexpected receipt tag: %s", node.Tag)
	}
	if got := wanode.AttrString(node.Attrs, "id"); got != "ACCEPT-STANZA-ID" {
		t.Fatalf("receipt did not preserve outer stanza ID: %s", got)
	}
	if got, ok := node.Attrs["to"].(types.JID); !ok || got.String() != peer.String() {
		t.Fatalf("unexpected receipt target: %#v", node.Attrs["to"])
	}
	if got, ok := node.Attrs["from"].(types.JID); !ok || got.String() != own.String() {
		t.Fatalf("unexpected receipt sender: %#v", node.Attrs["from"])
	}

	children := wanode.NodeChildren(&node)
	if len(children) != 1 || children[0].Tag != "accept" {
		t.Fatalf("unexpected receipt content: %#v", children)
	}
	if got := wanode.AttrString(children[0].Attrs, "call-id"); got != "CALL-RECEIPT" {
		t.Fatalf("unexpected receipt call ID: %s", got)
	}
	if got, ok := children[0].Attrs["call-creator"].(types.JID); !ok || got.String() != creator.String() {
		t.Fatalf("unexpected receipt creator: %#v", children[0].Attrs["call-creator"])
	}
}

func TestBuildAcceptReceiptStanzaRejectsSyntheticOrIncompleteInput(t *testing.T) {
	peer := types.NewJID("5511999999999", types.HiddenUserServer)
	creator := types.NewJID("5511999999999", types.HiddenUserServer)
	own := types.NewJID("5511000000000", types.HiddenUserServer)

	tests := []struct {
		name            string
		peer            types.JID
		acceptMessageID string
		callID          string
		creator         types.JID
		own             types.JID
	}{
		{name: "missing outer stanza ID", peer: peer, callID: "CALL", creator: creator, own: own},
		{name: "missing call ID", peer: peer, acceptMessageID: "ACCEPT", creator: creator, own: own},
		{name: "missing peer", acceptMessageID: "ACCEPT", callID: "CALL", creator: creator, own: own},
		{name: "missing creator", peer: peer, acceptMessageID: "ACCEPT", callID: "CALL", own: own},
		{name: "missing own JID", peer: peer, acceptMessageID: "ACCEPT", callID: "CALL", creator: creator},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := BuildAcceptReceiptStanza(
				test.peer,
				test.acceptMessageID,
				test.callID,
				test.creator,
				test.own,
			); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
