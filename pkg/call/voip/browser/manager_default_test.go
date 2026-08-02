//go:build !voip_pion

package browser

import (
	"context"
	"errors"
	"testing"
)

func TestDefaultManagerIsDisabled(t *testing.T) {
	manager := NewManager(nil)
	_, err := manager.Create(context.Background(), "instance", "call", CreateRequest{})
	if !errors.Is(err, ErrWebRTCDisabled) {
		t.Fatalf("expected disabled error, got %v", err)
	}
}
