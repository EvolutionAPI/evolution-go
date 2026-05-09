package message_repository

import (
	message_model "github.com/EvolutionAPI/evolution-go/pkg/message/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MessageRepository interface {
	InsertMessage(message message_model.Message) error
	GetMessageByID(messageID string) (*message_model.Message, error)
	DeleteAllMessages() (int64, error)
	GetLatestMessageID(source string) (string, string, error)
}

type messageRepository struct {
	db *gorm.DB
}

func (m *messageRepository) InsertMessage(message message_model.Message) error {
	updates := []string{"timestamp", "status", "source"}
	if len(message.Referral) > 0 {
		updates = append(updates, "referral")
	}

	return m.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "message_id"}},
		DoUpdates: clause.AssignmentColumns(updates),
	}).Create(&message).Error
}

func (m *messageRepository) GetMessageByID(messageID string) (*message_model.Message, error) {
	var message message_model.Message
	err := m.db.Where("message_id = ?", messageID).First(&message).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &message, nil
}

func (m *messageRepository) DeleteAllMessages() (int64, error) {
	result := m.db.Exec("DELETE FROM messages")
	return result.RowsAffected, result.Error
}

func (m *messageRepository) GetLatestMessageID(source string) (string, string, error) {
	var message message_model.Message
	err := m.db.Where("source = ?", source).Order("timestamp DESC").First(&message).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", "", nil
		}
		return "", "", err
	}

	return message.MessageID, message.Timestamp, nil
}

func NewMessageRepository(db *gorm.DB) MessageRepository {
	return &messageRepository{db: db}
}
