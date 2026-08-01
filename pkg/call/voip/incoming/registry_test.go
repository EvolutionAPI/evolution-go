package incoming

import (
	"testing"

	"go.mau.fi/whatsmeow/types"
)

func TestSecretCopyIsIndependent(t *testing.T) {
	s := &session{secrets: make(map[string]*callSecret)}
	key := make([]byte, 32)
	for index := range key {
		key[index] = byte(index + 1)
	}
	s.store("call-1", &callSecret{
		callKey: key,
		peer:    types.NewJID("5511999999999", types.DefaultUserServer),
		creator: types.NewJID("5511999999999", types.HiddenUserServer),
	})

	copyValue, ok := s.copySecret("call-1")
	if !ok {
		t.Fatal("expected secret copy")
	}
	copyValue.callKey[0] = 99
	stored, _ := s.copySecret("call-1")
	if stored.callKey[0] != 1 {
		t.Fatal("mutating a secret copy changed the stored key")
	}
	zeroBytes(copyValue.callKey)
	zeroBytes(stored.callKey)
}

func TestRemoveZeroesStoredKey(t *testing.T) {
	s := &session{secrets: make(map[string]*callSecret)}
	key := make([]byte, 32)
	for index := range key {
		key[index] = 7
	}
	s.store("call-1", &callSecret{callKey: key})
	s.remove("call-1")

	if _, ok := s.copySecret("call-1"); ok {
		t.Fatal("secret was not removed")
	}
	for index, value := range key {
		if value != 0 {
			t.Fatalf("key byte %d was not zeroed: %d", index, value)
		}
	}
}

func TestClearZeroesAllKeys(t *testing.T) {
	s := &session{secrets: make(map[string]*callSecret)}
	first := []byte{1, 2, 3}
	second := []byte{4, 5, 6}
	s.store("first", &callSecret{callKey: first})
	s.store("second", &callSecret{callKey: second})
	s.clear()

	for _, key := range [][]byte{first, second} {
		for _, value := range key {
			if value != 0 {
				t.Fatalf("key was not zeroed: %v", key)
			}
	}
	}
}
