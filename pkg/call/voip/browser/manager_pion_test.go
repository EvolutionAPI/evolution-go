//go:build voip_pion

package browser

import (
	"context"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
)

func TestPionManagerBridgesPCMOverDataChannel(t *testing.T) {
	fed := make(chan []float32, 2)
	manager := NewManager(func(_, _ string, pcm []float32) error {
		fed <- append([]float32(nil), pcm...)
		return nil
	})
	defer manager.CloseInstance("instance")

	client, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	protocol := DataChannelProtocol
	channel, err := client.CreateDataChannel(DataChannelLabel, &webrtc.DataChannelInit{Protocol: &protocol})
	if err != nil {
		t.Fatal(err)
	}
	opened := make(chan struct{})
	received := make(chan []float32, 2)
	channel.OnOpen(func() { close(opened) })
	channel.OnMessage(func(message webrtc.DataChannelMessage) {
		pcm, decodeErr := DecodePCMFrame(message.Data)
		if decodeErr != nil {
			t.Errorf("decode server PCM: %v", decodeErr)
			return
		}
		received <- pcm
	})

	offer, err := client.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	gather := webrtc.GatheringCompletePromise(client)
	if err = client.SetLocalDescription(offer); err != nil {
		t.Fatal(err)
	}
	select {
	case <-gather:
	case <-time.After(10 * time.Second):
		t.Fatal("client ICE gathering timed out")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	response, err := manager.Create(ctx, "instance", "call", CreateRequest{Offer: SDPDescription{
		Type: "offer",
		SDP:  client.LocalDescription().SDP,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err = client.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: response.Answer.SDP}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-opened:
	case <-time.After(10 * time.Second):
		t.Fatal("PCM data channel did not open")
	}

	browserPCM := make([]float32, PCMFrameSamples)
	browserPCM[0] = 0.25
	frame, err := EncodePCMFrame(browserPCM)
	if err != nil {
		t.Fatal(err)
	}
	if err = channel.Send(frame); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-fed:
		if len(got) != PCMFrameSamples || got[0] != 0.25 {
			t.Fatalf("unexpected fed PCM: len=%d first=%v", len(got), got[0])
		}
	case <-time.After(10 * time.Second):
		t.Fatal("server did not receive browser PCM")
	}

	serverPCM := make([]float32, PCMFrameSamples)
	serverPCM[0] = -0.5
	manager.HandlePCM("instance", "call", serverPCM)
	select {
	case got := <-received:
		if len(got) != PCMFrameSamples || got[0] != -0.5 {
			t.Fatalf("unexpected browser PCM: len=%d first=%v", len(got), got[0])
		}
	case <-time.After(10 * time.Second):
		t.Fatal("browser did not receive server PCM")
	}

	sessions, err := manager.Sessions("instance", "call")
	if err != nil || len(sessions) != 1 || !sessions[0].ChannelOpen {
		t.Fatalf("unexpected sessions: %+v err=%v", sessions, err)
	}
	if err = manager.CloseSession("instance", "call", response.SessionID); err != nil {
		t.Fatal(err)
	}
	sessions, err = manager.Sessions("instance", "call")
	if err != nil || len(sessions) != 0 {
		t.Fatalf("session was not removed: %+v err=%v", sessions, err)
	}
}
