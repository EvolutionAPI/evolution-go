// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package wanode

import (
	"fmt"
	"strconv"

	waBinary "go.mau.fi/whatsmeow/binary"
)

func NodeChildren(node *waBinary.Node) []waBinary.Node {
	if node == nil {
		return nil
	}
	children, _ := node.Content.([]waBinary.Node)
	return children
}

func NodeBytes(node *waBinary.Node) []byte {
	if node == nil {
		return nil
	}
	value, _ := node.Content.([]byte)
	return value
}

func AttrString(attrs waBinary.Attrs, key string) string {
	value, ok := attrs[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case int64:
		return strconv.FormatInt(typed, 10)
	case int:
		return strconv.Itoa(typed)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case bool:
		return strconv.FormatBool(typed)
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func AttrInt(attrs waBinary.Attrs, key string, fallback int) int {
	value, err := strconv.Atoi(AttrString(attrs, key))
	if err != nil {
		return fallback
	}
	return value
}

func HasAttr(attrs waBinary.Attrs, key string) bool {
	value, ok := attrs[key]
	return ok && value != nil && AttrString(attrs, key) != ""
}
