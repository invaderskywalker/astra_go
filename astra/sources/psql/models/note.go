// astra/sources/psql/models/note.go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Note struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID    int       `json:"user_id" gorm:"not null"`
	User      User      `json:"-" gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
	Title     string    `json:"title" gorm:"type:varchar(255);default:''"`
	Content   string    `json:"content" gorm:"type:text;not null"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Note) TableName() string {
	return "notes"
}

func (n *Note) BeforeCreate(tx *gorm.DB) (err error) {
	return tx.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`).Error
}
