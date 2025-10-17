// astra/sources/psql/models/session_summary.go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SessionSummary struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	SessionID string    `json:"session_id" gorm:"type:varchar(255);not null;unique"`
	UserID    int       `json:"user_id" gorm:"not null"`
	User      User      `json:"-" gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
	Summary   string    `json:"summary" gorm:"type:text;not null"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SessionSummary) TableName() string {
	return "session_summaries"
}

func (s *SessionSummary) BeforeCreate(tx *gorm.DB) (err error) {
	return tx.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`).Error
}
