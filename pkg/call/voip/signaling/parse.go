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
	for key, value := range node.Attrs {
		keyLower := strings.ToLower(key)
		valueString := strings.ToLower(strings.TrimSpace(wanode.AttrString(node.Attrs, key)))
		if (keyLower == "media" || keyLower == "type") && valueString == "video" {
			return true
		}
	}
	for index := range wanode.NodeChildren(node) {
		children := wanode.NodeChildren(node)
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
	for _, childValue := range wanode.NodeChildren(inner) {
		child := childValue
		if child.Tag == "enc" && wanode.HasAttr(child.Attrs, "type") {
			return &child
		}
	}
	for _, childValue := range wanode.NodeChildren(inner) {
		child := childValue
		if found := findEncryptedCallKeyNode(&child); found != nil {
			return found
		}
	}
	return nil
}
