package message_repository

import (
	"encoding/json"
	"testing"

	message_model "github.com/EvolutionAPI/evolution-go/pkg/message/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInsertMessagePreservesReferralOnStatusUpdate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&message_model.Message{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := NewMessageRepository(db)
	referral := json.RawMessage(`{"ctwaClid":"abc123","showAdAttribution":true}`)

	initial := message_model.Message{
		MessageID: "msg-1",
		Timestamp: "2026-05-09 10:00:00",
		Status:    "Received",
		Source:    "1551999999999",
		Referral:  referral,
	}

	if err := repo.InsertMessage(initial); err != nil {
		t.Fatalf("insert initial message: %v", err)
	}

	updated := message_model.Message{
		MessageID: "msg-1",
		Timestamp: "2026-05-09 10:05:00",
		Status:    "Read",
		Source:    "1551999999999",
	}

	if err := repo.InsertMessage(updated); err != nil {
		t.Fatalf("insert updated message: %v", err)
	}

	got, err := repo.GetMessageByID("msg-1")
	if err != nil {
		t.Fatalf("get message: %v", err)
	}

	if got == nil {
		t.Fatal("expected message, got nil")
	}

	if got.Status != "Read" {
		t.Fatalf("expected status Read, got %q", got.Status)
	}

	if string(got.Referral) != string(referral) {
		t.Fatalf("expected referral %s, got %s", referral, got.Referral)
	}
}
