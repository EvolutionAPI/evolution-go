package message_repository

import (
	"testing"

	message_model "github.com/EvolutionAPI/evolution-go/pkg/message/model"
)

func TestMessageUpdateColumns(t *testing.T) {
	t.Run("without referral", func(t *testing.T) {
		updates := messageUpdateColumns(message_model.Message{})

		expected := []string{"timestamp", "status", "source"}
		if len(updates) != len(expected) {
			t.Fatalf("expected %d update columns, got %d", len(expected), len(updates))
		}

		for i, column := range expected {
			if updates[i] != column {
				t.Fatalf("expected column %q at position %d, got %q", column, i, updates[i])
			}
		}
	})

	t.Run("with referral", func(t *testing.T) {
		updates := messageUpdateColumns(message_model.Message{Referral: []byte(`{"ctwaClid":"abc123"}`)})

		expected := []string{"timestamp", "status", "source", "referral"}
		if len(updates) != len(expected) {
			t.Fatalf("expected %d update columns, got %d", len(expected), len(updates))
		}

		for i, column := range expected {
			if updates[i] != column {
				t.Fatalf("expected column %q at position %d, got %q", column, i, updates[i])
			}
		}
	})
}
