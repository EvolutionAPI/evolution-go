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
