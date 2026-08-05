package incoming

import (
	"errors"
	"testing"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

func newTestSession() *session {
	return &session{materials: make(map[string]*callMaterial), prepareIncoming: true}
}

func TestMaterialCopyIsIndependent(t *testing.T) {
	s := newTestSession()
	key := make([]byte, 32)
	for index := range key {
		key[index] = byte(index + 1)
	}
	originalToken := []byte{7, 8, 9}
	s.store("call-1", &callMaterial{
		callKey: key,
		peer:    types.NewJID("5511999999999", types.DefaultUserServer),
		creator: types.NewJID("5511999999999", types.HiddenUserServer),
		relayData: &core.RelayData{Endpoints: []core.RelayEndpoint{{
			IP:       "1.2.3.4",
			RawToken: originalToken,
		}}},
	})

	copyValue, ok := s.copyMaterial("call-1")
	if !ok {
		t.Fatal("expected material copy")
	}
	copyValue.callKey[0] = 99
	copyValue.relayData.Endpoints[0].RawToken[0] = 99
	stored, _ := s.copyMaterial("call-1")
	if stored.callKey[0] != 1 {
		t.Fatal("mutating a material copy changed the stored key")
	}
	if stored.relayData.Endpoints[0].RawToken[0] != 7 {
		t.Fatal("mutating a relay copy changed the stored token")
	}
	zeroMaterial(copyValue)
	zeroMaterial(stored)
}

func TestRemoveZeroesStoredMaterial(t *testing.T) {
	s := newTestSession()
	key := make([]byte, 32)
	for index := range key {
		key[index] = 7
	}
	token := []byte{4, 5, 6}
	s.store("call-1", &callMaterial{
		callKey: key,
		relayData: &core.RelayData{Endpoints: []core.RelayEndpoint{{
			RawToken: token,
		}}},
	})
	s.remove("call-1")

	if _, ok := s.copyMaterial("call-1"); ok {
		t.Fatal("material was not removed")
	}
	for index, value := range key {
		if value != 0 {
			t.Fatalf("key byte %d was not zeroed: %d", index, value)
		}
	}
	for index, value := range token {
		if value != 0 {
			t.Fatalf("token byte %d was not zeroed: %d", index, value)
		}
	}
}

func TestClearZeroesAllKeys(t *testing.T) {
	s := newTestSession()
	first := []byte{1, 2, 3}
	second := []byte{4, 5, 6}
	s.store("first", &callMaterial{callKey: first})
	s.store("second", &callMaterial{callKey: second})
	s.clear()

	for _, key := range [][]byte{first, second} {
		for _, value := range key {
			if value != 0 {
				t.Fatalf("key was not zeroed: %v", key)
			}
		}
	}
}

func TestTransportRelayUpdatePreservesCallKey(t *testing.T) {
	s := newTestSession()
	key := make([]byte, 32)
	for index := range key {
		key[index] = byte(index + 1)
	}
	s.store("call-1", &callMaterial{callKey: key})

	node := &waBinary.Node{Content: []waBinary.Node{{
		Tag: "relay",
		Attrs: waBinary.Attrs{
			"ip":    "10.0.0.8",
			"port":  "3480",
			"token": "relay-token",
		},
	}}}
	s.captureRelays("call-1", node)

	stored, ok := s.copyMaterial("call-1")
	if !ok {
		t.Fatal("expected stored material")
	}
	defer zeroMaterial(stored)
	if len(stored.callKey) != 32 || stored.callKey[0] != 1 {
		t.Fatal("relay update erased the call key")
	}
	if stored.relayData == nil || len(stored.relayData.Endpoints) != 1 {
		t.Fatal("relay update was not stored")
	}
	if stored.relayData.Endpoints[0].IP != "10.0.0.8" {
		t.Fatalf("unexpected relay IP: %s", stored.relayData.Endpoints[0].IP)
	}
}

func TestStoreMergesRelayCapturedBeforeKey(t *testing.T) {
	s := newTestSession()
	node := &waBinary.Node{Content: []waBinary.Node{{
		Tag: "relay",
		Attrs: waBinary.Attrs{
			"ip":    "10.0.0.9",
			"port":  "3480",
			"token": "relay-token",
		},
	}}}
	s.captureRelays("call-1", node)

	key := make([]byte, 32)
	key[0] = 42
	s.store("call-1", &callMaterial{callKey: key})

	stored, ok := s.copyMaterial("call-1")
	if !ok {
		t.Fatal("expected stored material")
	}
	defer zeroMaterial(stored)
	if stored.callKey[0] != 42 {
		t.Fatal("call key was not stored")
	}
	if stored.relayData == nil || stored.relayData.Endpoints[0].IP != "10.0.0.9" {
		t.Fatal("relay captured before key was not preserved")
	}
}

func TestSessionReportsPreparationResultsWithoutPrivateMaterial(t *testing.T) {
	reported := make(chan error, 2)
	s := newTestSession()
	s.setOnPreparation(func(_ string, err error) { reported <- err })

	s.reportPreparation("call-1", nil)
	failed := errors.New("preaccept failed")
	s.reportPreparation("call-1", failed)

	if err := <-reported; err != nil {
		t.Fatalf("successful preparation reported error: %v", err)
	}
	if err := <-reported; !errors.Is(err, failed) {
		t.Fatalf("failed preparation error = %v, want %v", err, failed)
	}
}

func TestRegistryForwardsPreparationResultWithInstanceID(t *testing.T) {
	registry := NewRegistry()
	type result struct {
		instanceID string
		callID     string
		err        error
	}
	reported := make(chan result, 1)
	registry.SetOnPreparation(func(instanceID, callID string, err error) {
		reported <- result{instanceID: instanceID, callID: callID, err: err}
	})

	failed := errors.New("call key unavailable")
	registry.notifyPreparation("instance-1", "call-1", failed)

	got := <-reported
	if got.instanceID != "instance-1" || got.callID != "call-1" || !errors.Is(got.err, failed) {
		t.Fatalf("unexpected preparation result: %+v", got)
	}
}
