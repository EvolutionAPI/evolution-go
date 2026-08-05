package signaling

import (
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
)

func TestIsAlreadyEndedOffer(t *testing.T) {
	tests := []struct {
		name string
		node *waBinary.Node
		want bool
	}{
		{name: "nil", want: false},
		{name: "live offer", node: &waBinary.Node{Tag: "offer"}, want: false},
		{name: "ended flag", node: &waBinary.Node{Tag: "offer", Attrs: waBinary.Attrs{"is_call_ended": "1"}}, want: true},
		{name: "termination reason", node: &waBinary.Node{Tag: "offer", Attrs: waBinary.Attrs{"terminate_reason": "accepted_elsewhere"}}, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsAlreadyEndedOffer(test.node); got != test.want {
				t.Fatalf("IsAlreadyEndedOffer() = %v, want %v", got, test.want)
			}
		})
	}
}
