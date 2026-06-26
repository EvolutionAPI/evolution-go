package message_model

import (
	"encoding/json"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Message struct {
	Id        string          `json:"id" gorm:"type:uuid;primaryKey"`
	MessageID string          `json:"message_id" gorm:"unique"`
	Timestamp string          `json:"timestamp"`
	Status    string          `json:"status"`
	Source    string          `json:"source"`
	Referral  json.RawMessage `json:"referral,omitempty" gorm:"type:jsonb"`
}

func (m *Message) BeforeCreate(tx *gorm.DB) (err error) {
	m.Id = uuid.New().String()
	return
}
