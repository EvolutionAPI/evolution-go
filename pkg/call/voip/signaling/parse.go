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
