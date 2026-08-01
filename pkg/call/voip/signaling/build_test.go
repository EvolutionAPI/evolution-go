package signaling

import (
	"context"
	"testing"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/wanode"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

type fakeSocket struct {
	own          types.JID
	devices      []types.JID
	decryptedKey []byte
}

func (f *fakeSocket) OwnPN() types.JID { return f.own }
func (f *fakeSocket) OwnLID() types.JID { return types.JID{} }
func (f *fakeSocket) AccountDeviceIdentityNode() (waBinary.Node, bool) {
	return waBinary.Node{Tag: "device-identity"}, true
}
func (f *fakeSocket) SendNode(context.Context, waBinary.Node) error { return nil }
func (f *fakeSocket) Query(context.Context, waBinary.Node) (*waBinary.Node, error) {
	return nil, nil
}
func (f *fakeSocket) GetUSyncDevices(context.Context, []types.JID) ([]types.JID, error) {
	return f.devices, nil
}
func (f *fakeSocket) AssertSessions(context.Context, []types.JID, bool) error { return nil }
func (f *fakeSocket) CreateParticipantNodes(_ context.Context, devices []types.JID, _ []byte, _ waBinary.Attrs) ([]waBinary.Node, bool, error) {
	device := types.JID{}
	if len(devices) > 0 {
		device = devices[0]
	}
	return []waBinary.Node{{
		Tag:   "to",
		Attrs: waBinary.Attrs{"jid": device},
		Content: []waBinary.Node{{
			Tag:     "enc",
			Attrs:   waBinary.Attrs{"type": "msg"},
			Content: []byte{1, 2, 3},
		}},
	}}, true, nil
}
func (f *fakeSocket) DecryptCallKey(context.Context, types.JID, *waBinary.Node) ([]byte, error) {
	return append([]byte(nil), f.decryptedKey...), nil
}
func (f *fakeSocket) GetTCToken(context.Context, types.JID) ([]byte, error) { return nil, nil }
func (f *fakeSocket) ResolveLIDForPN(_ context.Context, jid types.JID) types.JID { return jid }

func TestGenerateCallKey(t *testing.T) {
	key, err := GenerateCallKey()
	if err != nil {
		t.Fatalf("GenerateCallKey() error = %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("GenerateCallKey() length = %d, want 32", len(key))
	}
}

func TestBuildOfferStanza(t *testing.T) {
	own := types.NewJID("5511000000000", types.DefaultUserServer)
	peer := types.NewJID("5511999999999", types.DefaultUserServer)
	device := types.NewJID("5511999999999", types.DefaultUserServer)
	socket := &fakeSocket{own: own, devices: []types.JID{device}}

	node, err := BuildOfferStanza(context.Background(), socket, "CALL-123", make([]byte, 32), peer, false)
	if err != nil {
		t.Fatalf("BuildOfferStanza() error = %v", err)
	}
	if node.Tag != "call" {
		t.Fatalf("root tag = %q, want call", node.Tag)
	}
	if to, ok := node.Attrs["to"].(types.JID); !ok || to != peer {
		t.Fatalf("root to = %#v, want %s", node.Attrs["to"], peer.String())
	}

	rootChildren := wanode.NodeChildren(&node)
	if len(rootChildren) != 1 || rootChildren[0].Tag != "offer" {
		t.Fatalf("unexpected root children: %#v", rootChildren)
	}
	offer := rootChildren[0]
	if wanode.AttrString(offer.Attrs, "call-id") != "CALL-123" {
		t.Fatalf("call-id = %q", wanode.AttrString(offer.Attrs, "call-id"))
	}
	if creator, ok := offer.Attrs["call-creator"].(types.JID); !ok || creator != own {
		t.Fatalf("call creator = %#v, want %s", offer.Attrs["call-creator"], own.String())
	}

	var audio16, destination, identity bool
	for _, child := range wanode.NodeChildren(&offer) {
		switch child.Tag {
		case "audio":
			if wanode.AttrString(child.Attrs, "rate") == "16000" {
				audio16 = true
			}
		case "destination":
			destination = len(wanode.NodeChildren(&child)) == 1
		case "device-identity":
			identity = true
		}
	}
	if !audio16 || !destination || !identity {
		t.Fatalf("offer missing required nodes: audio16=%v destination=%v identity=%v", audio16, destination, identity)
	}
}

func TestBuildOfferRequiresDevices(t *testing.T) {
	socket := &fakeSocket{own: types.NewJID("5511000000000", types.DefaultUserServer)}
	peer := types.NewJID("5511999999999", types.DefaultUserServer)
	if _, err := BuildOfferStanza(context.Background(), socket, "CALL-123", make([]byte, 32), peer, false); err == nil {
		t.Fatal("BuildOfferStanza() expected error when no peer devices are available")
	}
}

func TestBuildPreacceptStanza(t *testing.T) {
	peer := types.NewJID("5511999999999", types.DefaultUserServer)
	creator := types.NewJID("5511999999999", types.HiddenUserServer)
	node := BuildPreacceptStanza(peer, "CALL-IN", creator)
	children := wanode.NodeChildren(&node)
	if len(children) != 1 || children[0].Tag != "preaccept" {
		t.Fatalf("unexpected preaccept node: %#v", children)
	}
	if wanode.AttrString(children[0].Attrs, "call-id") != "CALL-IN" {
		t.Fatalf("unexpected call id: %s", wanode.AttrString(children[0].Attrs, "call-id"))
	}
}

func TestBuildAcceptStanza(t *testing.T) {
	own := types.NewJID("5511000000000", types.DefaultUserServer)
	peer := types.NewJID("5511999999999", types.DefaultUserServer)
	creator := types.NewJID("5511999999999", types.HiddenUserServer)
	socket := &fakeSocket{own: own, devices: []types.JID{creator}}

	node, err := BuildAcceptStanza(context.Background(), socket, "CALL-IN", make([]byte, 32), peer, creator, true)
	if err != nil {
		t.Fatalf("BuildAcceptStanza() error = %v", err)
	}
	children := wanode.NodeChildren(&node)
	if len(children) != 1 || children[0].Tag != "accept" {
		t.Fatalf("unexpected accept node: %#v", children)
	}
	var encrypted, video bool
	for _, child := range wanode.NodeChildren(&children[0]) {
		if child.Tag == "enc" {
			encrypted = true
		}
		if child.Tag == "video" {
			video = true
		}
	}
	if !encrypted || !video {
		t.Fatalf("accept missing nodes: encrypted=%v video=%v", encrypted, video)
	}
}

func TestDecryptCallKeyInNode(t *testing.T) {
	peer := types.NewJID("5511999999999", types.DefaultUserServer)
	key := make([]byte, 32)
	for index := range key {
		key[index] = byte(index + 1)
	}
	socket := &fakeSocket{decryptedKey: key}
	offer := &waBinary.Node{
		Tag: "offer",
		Content: []waBinary.Node{{
			Tag: "destination",
			Content: []waBinary.Node{{
				Tag: "to",
				Content: []waBinary.Node{{
					Tag:     "enc",
					Attrs:   waBinary.Attrs{"type": "msg"},
					Content: []byte{9},
				}},
			}},
		}},
	}

	decrypted, err := DecryptCallKeyInNode(context.Background(), socket, offer, peer)
	if err != nil {
		t.Fatalf("DecryptCallKeyInNode() error = %v", err)
	}
	if len(decrypted) != 32 || decrypted[31] != 32 {
		t.Fatalf("unexpected decrypted key: %v", decrypted)
	}
}

func TestNodeContainsVideo(t *testing.T) {
	offer := &waBinary.Node{Tag: "offer", Content: []waBinary.Node{{Tag: "video"}}}
	if !NodeContainsVideo(offer) {
		t.Fatal("expected video offer to be detected")
	}
}
