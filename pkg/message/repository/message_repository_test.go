package message_repository

import (
	"bytes"
	"encoding/json"
	"log"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	message_model "github.com/EvolutionAPI/evolution-go/pkg/message/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestInsertMessagePreservesReferralOnStatusUpdate(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("open sqlmock db: %v", err)
	}
	defer sqlDB.Close()

	var logBuffer bytes.Buffer
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:             sqlDB,
		WithoutReturning: true,
	}), &gorm.Config{
		DryRun:                 true,
		SkipDefaultTransaction: true,
		Logger: gormlogger.New(
			log.New(&logBuffer, "", 0),
			gormlogger.Config{LogLevel: gormlogger.Info, Colorful: false},
		),
	})
	if err != nil {
		t.Fatalf("open gorm db: %v", err)
	}

	repo := NewMessageRepository(gormDB)
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

	initialSQL := logBuffer.String()
	if !strings.Contains(initialSQL, `"referral"="excluded"."referral"`) {
		t.Fatalf("expected initial upsert SQL to update referral, got %q", initialSQL)
	}

	logBuffer.Reset()

	updated := message_model.Message{
		MessageID: "msg-1",
		Timestamp: "2026-05-09 10:05:00",
		Status:    "Read",
		Source:    "1551999999999",
	}

	if err := repo.InsertMessage(updated); err != nil {
		t.Fatalf("insert updated message: %v", err)
	}

	updatedSQL := logBuffer.String()
	if strings.Contains(updatedSQL, `"referral"="excluded"."referral"`) {
		t.Fatalf("expected updated upsert SQL to omit referral update, got %q", updatedSQL)
	}
	if !strings.Contains(updatedSQL, `"timestamp"="excluded"."timestamp","status"="excluded"."status","source"="excluded"."source"`) {
		t.Fatalf("expected updated upsert SQL to keep core columns, got %q", updatedSQL)
	}
}
