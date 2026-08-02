package media

import (
	"bytes"
	"sync"
	"testing"
)

func TestPacketRegistryConcurrentProtectAndRemove(t *testing.T) {
	registry := NewPacketRegistry(&fakePacketSource{callKey: bytes.Repeat([]byte{0x8d}, 32)})
	const (
		instanceID = "race-instance"
		callID     = "race-call"
	)
	if err := registry.PrepareWithDevices(instanceID, callID, "self@lid", "peer@lid", 303, 404); err != nil {
		t.Fatal(err)
	}

	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		for index := 0; index < 200; index++ {
			_, _ = registry.ProtectOpus(instanceID, callID, []byte{byte(index)}, 960, index == 0)
		}
	}()
	go func() {
		defer wait.Done()
		for index := 0; index < 20; index++ {
			registry.Remove(instanceID, callID)
			_ = registry.PrepareWithDevices(instanceID, callID, "self@lid", "peer@lid", 303, 404)
		}
	}()
	wait.Wait()
	registry.Close(instanceID)
}
