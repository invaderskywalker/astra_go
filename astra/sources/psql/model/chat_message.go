package models

import (
	"time"

	"github.com/google/uuid"
)

type ChatMessage struct {
	ID        uuid.UUID `json:"id"`
	SessionID string    `json:"session_id"`
	UserID    int       `json:"user_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}
