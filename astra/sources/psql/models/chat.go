package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ChatMessage struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	SessionID string    `json:"session_id" gorm:"type:varchar(255);not null"`
	UserID    int       `json:"user_id" gorm:"not null"`
	User      User      `json:"-" gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
	Role      string    `json:"role" gorm:"type:varchar(50);not null"`
	Content   string    `json:"content" gorm:"type:text;not null"`
	Timestamp time.Time `json:"timestamp" gorm:"not null;default:CURRENT_TIMESTAMP"`
}

func (ChatMessage) BeforeCreate(tx *gorm.DB) (err error) {
	// Ensure UUID extension is enabled
	return tx.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`).Error
}
