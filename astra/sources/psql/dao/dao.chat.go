package dao

import (
	"astra/astra/sources/psql/models"
	"astra/astra/utils/types"
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

// --- NEW: List session summaries for a user (for threads panel) ---
func (dao *ChatMessageDAO) ListSessionsForUser(ctx context.Context, userID int) ([]types.ChatSessionSummary, error) {
	// Fetch most recent message for each session_id
	var results []types.ChatSessionSummary
	// Postgres: select session_id, MAX(timestamp) from chat_message where user_id=? group by session_id order by MAX(timestamp) DESC
	type row struct {
		SessionID string
		Timestamp string
		Content   string
		Role      string
	}

	rows, err := dao.DB.WithContext(ctx).
		Raw(`SELECT a.session_id, a.timestamp, a.content, a.role FROM chat_messages a
		 JOIN (SELECT session_id, MAX(timestamp) as max_time
		   FROM chat_messages WHERE user_id = ? GROUP BY session_id) as b
		 ON a.session_id = b.session_id AND a.timestamp = b.max_time
		 WHERE a.user_id = ?
		 ORDER BY a.timestamp DESC`, userID, userID).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.SessionID, &r.Timestamp, &r.Content, &r.Role); err != nil {
			continue
		}
		results = append(results, types.ChatSessionSummary{
			SessionID:       r.SessionID,
			LastMessage:     r.Content,
			LastMessageRole: r.Role,
			LastActivity:    r.Timestamp,
		})
	}
	return results, nil
}

// --- NEW: Get all messages for a session for this user ---
func (dao *ChatMessageDAO) GetMessagesBySession(ctx context.Context, sessionID string, userID int) ([]models.ChatMessage, error) {
	var msgs []models.ChatMessage
	err := dao.DB.WithContext(ctx).
		Where("session_id = ? AND user_id = ?", sessionID, userID).
		Order("timestamp ASC").Find(&msgs).Error
	if err != nil {
		return nil, err
	}
	return msgs, nil
}

// --- NEW: Security check: verify session_id belongs to user ---
func (dao *ChatMessageDAO) SessionBelongsToUser(ctx context.Context, sessionID string, userID int) (bool, error) {
	var cnt int64
	err := dao.DB.WithContext(ctx).
		Model(&models.ChatMessage{}).
		Where("session_id = ? AND user_id = ?", sessionID, userID).Count(&cnt).Error
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}
