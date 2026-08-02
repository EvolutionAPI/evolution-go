package media

import (
	"context"
	"errors"
	"testing"

	call_state "github.com/evolution-foundation/evolution-go/pkg/call/voip/call"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

func installPostAcceptTestHooks(
	t *testing.T,
	transport func(context.Context, *whatsmeow.Client, types.JID, types.JID, string) error,
	mute func(context.Context, *whatsmeow.Client, types.JID, types.JID, string) error,
) {
	t.Helper()
	previousResolve := resolvePostAcceptPeer
	previousTransport := sendPostAcceptTransport
	previousMute := sendPostAcceptMute
	outgoingPostAcceptProgress.reset()

	resolvePostAcceptPeer = func(_ context.Context, _ *whatsmeow.Client, peer types.JID) types.JID {
		return peer
	}
	sendPostAcceptTransport = transport
	sendPostAcceptMute = mute

	t.Cleanup(func() {
		resolvePostAcceptPeer = previousResolve
		sendPostAcceptTransport = previousTransport
		sendPostAcceptMute = previousMute
		outgoingPostAcceptProgress.reset()
	})
}

func newPostAcceptTestSession(t *testing.T, callID string) *relaySession {
	t.Helper()
	state := call_state.NewOutgoing(
		callID,
		"5511888888888:2@s.whatsapp.net",
		"5511999999999:1@s.whatsapp.net",
		core.CallMediaTypeAudio,
	)
	source := &fakeNegotiationSource{state: state}
	session := newTestRelaySession(source, &fakeRelayTransport{})
	session.client = &whatsmeow.Client{}
	return session
}

func TestOutgoingPostAcceptSuppressesDuplicateAccept(t *testing.T) {
	transportCalls := 0
	muteCalls := 0
	installPostAcceptTestHooks(t,
		func(context.Context, *whatsmeow.Client, types.JID, types.JID, string) error {
			transportCalls++
			return nil
		},
		func(context.Context, *whatsmeow.Client, types.JID, types.JID, string) error {
			muteCalls++
			return nil
		},
	)

	session := newPostAcceptTestSession(t, "call-duplicate-accept")
	session.sendOutgoingPostAccept("call-duplicate-accept")
	session.sendOutgoingPostAccept("call-duplicate-accept")

	if transportCalls != 1 || muteCalls != 1 {
		t.Fatalf("duplicate accept resent signaling: transport=%d mute=%d", transportCalls, muteCalls)
	}
}

func TestOutgoingPostAcceptRetriesOnlyFailedMuteStage(t *testing.T) {
	transportCalls := 0
	muteCalls := 0
	installPostAcceptTestHooks(t,
		func(context.Context, *whatsmeow.Client, types.JID, types.JID, string) error {
			transportCalls++
			return nil
		},
		func(context.Context, *whatsmeow.Client, types.JID, types.JID, string) error {
			muteCalls++
			if muteCalls == 1 {
				return errors.New("temporary mute sync failure")
			}
			return nil
		},
	)

	session := newPostAcceptTestSession(t, "call-mute-retry")
	session.sendOutgoingPostAccept("call-mute-retry")
	session.sendOutgoingPostAccept("call-mute-retry")

	if transportCalls != 1 {
		t.Fatalf("successful transport stage was resent: %d", transportCalls)
	}
	if muteCalls != 2 {
		t.Fatalf("failed mute stage was not retried once: %d", muteCalls)
	}
}
