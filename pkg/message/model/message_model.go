package message_model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Message struct {
	Id        string `json:"id" gorm:"type:uuid;primaryKey"`
	MessageID string `json:"message_id" gorm:"unique"`
	Timestamp string `json:"timestamp"`
	Status    string `json:"status"`
	Source    string `json:"source"`
	Referral  []byte `json:"referral,omitempty"`
}

func (m *Message) BeforeCreate(tx *gorm.DB) (err error) {
	m.Id = uuid.New().String()
	return
}
