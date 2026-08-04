package media

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	call_state "github.com/evolution-foundation/evolution-go/pkg/call/voip/call"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	"go.mau.fi/whatsmeow"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

type fakePostAcceptSocket struct {
	resolve func(context.Context, types.JID) types.JID
	send    func(context.Context, waBinary.Node) error
}

func (f *fakePostAcceptSocket) ResolveLIDForPN(ctx context.Context, peer types.JID) types.JID {
	if f != nil && f.resolve != nil {
		return f.resolve(ctx, peer)
	}
	return peer
}

func (f *fakePostAcceptSocket) SendNode(ctx context.Context, node waBinary.Node) error {
	if f != nil && f.send != nil {
		return f.send(ctx, node)
	}
	return nil
}

func installPostAcceptTestSocket(t *testing.T, socket postAcceptSocket) {
	t.Helper()
	previousFactory := newPostAcceptSocket
	previousScheduler := schedulePostAcceptRetry
	previousDelays := append([]time.Duration(nil), postAcceptRetryDelays...)
	outgoingPostAcceptProgress.reset()
	newPostAcceptSocket = func(*whatsmeow.Client) postAcceptSocket {
		return socket
	}
	// Execute retries synchronously so tests prove the state machine without
	// sleeping or leaving timers alive after cleanup.
	schedulePostAcceptRetry = func(_ time.Duration, callback func()) {
		callback()
	}
	t.Cleanup(func() {
		newPostAcceptSocket = previousFactory
		schedulePostAcceptRetry = previousScheduler
		postAcceptRetryDelays = previousDelays
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

func postAcceptChildTag(node waBinary.Node) string {
	children := node.GetChildren()
	if len(children) == 0 {
		return ""
	}
	return children[0].Tag
}

func TestOutgoingPostAcceptSuppressesDuplicateAccept(t *testing.T) {
	transportCalls := 0
	muteCalls := 0
	installPostAcceptTestSocket(t, &fakePostAcceptSocket{
		send: func(_ context.Context, node waBinary.Node) error {
			switch postAcceptChildTag(node) {
			case "transport":
				transportCalls++
			case "mute_v2":
				muteCalls++
			}
			return nil
		},
	})

	session := newPostAcceptTestSession(t, "call-duplicate-accept")
	session.sendOutgoingPostAccept("call-duplicate-accept")
	session.sendOutgoingPostAccept("call-duplicate-accept")

	if transportCalls != 1 || muteCalls != 1 {
		t.Fatalf("duplicate accept resent signaling: transport=%d mute=%d", transportCalls, muteCalls)
	}
}

func TestOutgoingPostAcceptAutomaticallyRetriesOnlyFailedMuteStage(t *testing.T) {
	transportCalls := 0
	muteCalls := 0
	installPostAcceptTestSocket(t, &fakePostAcceptSocket{
		send: func(_ context.Context, node waBinary.Node) error {
			switch postAcceptChildTag(node) {
			case "transport":
				transportCalls++
			case "mute_v2":
				muteCalls++
				if muteCalls == 1 {
					return errors.New("temporary mute sync failure")
				}
			}
			return nil
		},
	})

	session := newPostAcceptTestSession(t, "call-mute-retry")
	// No duplicate CallAccept is injected: the internal scheduler must recover.
	session.sendOutgoingPostAccept("call-mute-retry")

	if transportCalls != 1 {
		t.Fatalf("successful transport stage was resent: %d", transportCalls)
	}
	if muteCalls != 2 {
		t.Fatalf("failed mute stage was not automatically retried once: %d", muteCalls)
	}
}

func TestOutgoingPostAcceptStopsAfterRetryBudget(t *testing.T) {
	transportCalls := 0
	muteCalls := 0
	installPostAcceptTestSocket(t, &fakePostAcceptSocket{
		send: func(_ context.Context, node waBinary.Node) error {
			switch postAcceptChildTag(node) {
			case "transport":
				transportCalls++
				return errors.New("persistent transport failure")
			case "mute_v2":
				muteCalls++
			}
			return nil
		},
	})

	session := newPostAcceptTestSession(t, "call-retry-budget")
	session.sendOutgoingPostAccept("call-retry-budget")

	wantAttempts := 1 + len(postAcceptRetryDelays)
	if transportCalls != wantAttempts {
		t.Fatalf("unexpected retry count: got=%d want=%d", transportCalls, wantAttempts)
	}
	if muteCalls != 0 {
		t.Fatalf("mute stage ran before transport succeeded: %d", muteCalls)
	}
}

func TestOutgoingPostAcceptSerializesConcurrentAccepts(t *testing.T) {
	var transportCalls atomic.Int32
	var muteCalls atomic.Int32
	transportStarted := make(chan struct{})
	releaseTransport := make(chan struct{})
	var startOnce sync.Once

	installPostAcceptTestSocket(t, &fakePostAcceptSocket{
		send: func(_ context.Context, node waBinary.Node) error {
			switch postAcceptChildTag(node) {
			case "transport":
				transportCalls.Add(1)
				startOnce.Do(func() { close(transportStarted) })
				<-releaseTransport
			case "mute_v2":
				muteCalls.Add(1)
			}
			return nil
		},
	})

	session := newPostAcceptTestSession(t, "call-concurrent-accept")
	primaryDone := make(chan struct{})
	go func() {
		session.sendOutgoingPostAccept("call-concurrent-accept")
		close(primaryDone)
	}()
	<-transportStarted

	var duplicates sync.WaitGroup
	for range 16 {
		duplicates.Add(1)
		go func() {
			defer duplicates.Done()
			session.sendOutgoingPostAccept("call-concurrent-accept")
		}()
	}
	duplicates.Wait()
	close(releaseTransport)
	<-primaryDone

	if got := transportCalls.Load(); got != 1 {
		t.Fatalf("concurrent accepts sent transport %d times", got)
	}
	if got := muteCalls.Load(); got != 1 {
		t.Fatalf("concurrent accepts sent mute %d times", got)
	}
}
