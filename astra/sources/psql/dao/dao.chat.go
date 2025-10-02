package dao

import (
	"astra/astra/sources/psql/models"
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ChatMessageDAO struct {
	DB *gorm.DB
}

func NewChatMessageDAO(db *gorm.DB) *ChatMessageDAO {
	return &ChatMessageDAO{DB: db}
}

func (dao *ChatMessageDAO) CreateSessionID() string {
	return uuid.New().String()
}

func (dao *ChatMessageDAO) SaveMessage(ctx context.Context, sessionID string, userID int, role, content string) (*models.ChatMessage, error) {
	msg := models.ChatMessage{
		SessionID: sessionID,
		UserID:    userID,
		Role:      role,
		Content:   content,
	}
	err := dao.DB.WithContext(ctx).Create(&msg).Error
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

func (dao *ChatMessageDAO) GetChatHistoryBySession(ctx context.Context, sessionID string) ([]map[string]string, error) {
	var messages []models.ChatMessage
	err := dao.DB.WithContext(ctx).Where("session_id = ?", sessionID).Order("timestamp ASC").Find(&messages).Error
	if err != nil {
		return nil, err
	}
	history := make([]map[string]string, len(messages))
	for i, msg := range messages {
		history[i] = map[string]string{"role": msg.Role, "content": msg.Content}
	}
	return history, nil
}
