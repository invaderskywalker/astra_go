package dao

import (
	models "astra/astra/sources/psql/model"
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ChatMessageDAO struct {
	DB *pgxpool.Pool
}

func NewChatMessageDAO(db *pgxpool.Pool) *ChatMessageDAO {
	return &ChatMessageDAO{DB: db}
}

func (dao *ChatMessageDAO) CreateSessionID() string {
	return uuid.New().String()
}

func (dao *ChatMessageDAO) SaveMessage(ctx context.Context, sessionID string, userID int, role, content string) (*models.ChatMessage, error) {
	query := "INSERT INTO chat_messages (session_id, user_id, role, content) VALUES ($1, $2, $3, $4) RETURNING id, session_id, user_id, role, content, timestamp"
	row := dao.DB.QueryRow(ctx, query, sessionID, userID, role, content)
	var msg models.ChatMessage
	err := row.Scan(&msg.ID, &msg.SessionID, &msg.UserID, &msg.Role, &msg.Content, &msg.Timestamp)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

func (dao *ChatMessageDAO) GetChatHistoryBySession(ctx context.Context, sessionID string) ([]map[string]string, error) {
	query := "SELECT role, content FROM chat_messages WHERE session_id = $1 ORDER BY timestamp ASC"
	rows, err := dao.DB.Query(ctx, query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var history []map[string]string
	for rows.Next() {
		var role, content string
		err := rows.Scan(&role, &content)
		if err != nil {
			return nil, err
		}
		history = append(history, map[string]string{"role": role, "content": content})
	}
	return history, nil
}
