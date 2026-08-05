// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package signaling

import (
	"strings"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/wanode"
	waBinary "go.mau.fi/whatsmeow/binary"
)

// NodeContainsVideo reports whether a call protocol node advertises video.
func NodeContainsVideo(node *waBinary.Node) bool {
	if node == nil {
		return false
	}
	if strings.EqualFold(node.Tag, "video") {
		return true
	}
	for key := range node.Attrs {
		keyLower := strings.ToLower(key)
		valueString := strings.ToLower(strings.TrimSpace(wanode.AttrString(node.Attrs, key)))
		if (keyLower == "media" || keyLower == "type") && valueString == "video" {
			return true
		}
	}
	children := wanode.NodeChildren(node)
	for index := range children {
		if NodeContainsVideo(&children[index]) {
			return true
		}
	}
	return false
}

// IsAlreadyEndedOffer reports whether WhatsApp delivered a call-history update
// using an offer-shaped stanza. These are terminal notifications (for example,
// a call accepted on another device), not an incoming call that can be
// preaccepted or answered.
func IsAlreadyEndedOffer(node *waBinary.Node) bool {
	if node == nil {
		return false
	}
	attrs := node.AttrGetter()
	return attrs.OptionalString("is_call_ended") == "1" || attrs.OptionalString("terminate_reason") != ""
}

func findEncryptedCallKeyNode(inner *waBinary.Node) *waBinary.Node {
	if inner == nil {
		return nil
	}
	children := wanode.NodeChildren(inner)
	for index := range children {
		if children[index].Tag == "enc" && wanode.HasAttr(children[index].Attrs, "type") {
			return &children[index]
		}
	}
	for index := range children {
		if found := findEncryptedCallKeyNode(&children[index]); found != nil {
			return found
		}
	}
	return nil
}
