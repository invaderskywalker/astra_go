// astra/sources/psql/models/long_term_knowledge.go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LongTermKnowledge struct {
	ID            uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID        int       `json:"user_id" gorm:"not null"`
	User          User      `json:"-" gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
	KnowledgeType string    `json:"knowledge_type" gorm:"type:varchar(255);not null"`
	KnowledgeBlob string    `json:"knowledge_blob" gorm:"type:text;not null"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (LongTermKnowledge) TableName() string {
	return "long_term_knowledge"
}

func (ltk *LongTermKnowledge) BeforeCreate(tx *gorm.DB) (err error) {
	return tx.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`).Error
}
