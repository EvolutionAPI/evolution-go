package media

import (
	"testing"

	"go.mau.fi/whatsmeow/types"
)

func TestSelectCallDeviceJIDsUsesConcreteRemoteParticipant(t *testing.T) {
	self, peer := selectCallDeviceJIDs(
		[]string{
			"15509143740569:3@lid",
			"66155398054068:2@lid",
		},
		types.NewJID("15509143740569", types.HiddenUserServer),
		types.NewJID("75741748277476", types.HiddenUserServer),
		types.NewJID("66155398054068", types.HiddenUserServer),
	)
	if self != "15509143740569:3@lid" {
		t.Fatalf("unexpected self device: %s", self)
	}
	if peer != "66155398054068:2@lid" {
		t.Fatalf("expected creator participant as peer, got %s", peer)
	}
}

func TestSelectCallDeviceJIDsNormalizesAccountFallback(t *testing.T) {
	self, peer := selectCallDeviceJIDs(
		nil,
		types.NewJID("self", types.HiddenUserServer),
		types.NewJID("peer", types.HiddenUserServer),
		types.JID{},
	)
	if self != "self:0@lid" || peer != "peer:0@lid" {
		t.Fatalf("unexpected normalized fallbacks: self=%s peer=%s", self, peer)
	}
}
