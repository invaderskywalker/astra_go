// astra/sources/psql/dao/dao.long_term_knowledge.go
package dao

import (
	"astra/astra/sources/psql/models"
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LongTermKnowledgeDAO struct {
	DB *gorm.DB
}

func NewLongTermKnowledgeDAO(db *gorm.DB) *LongTermKnowledgeDAO {
	return &LongTermKnowledgeDAO{DB: db}
}

func (dao *LongTermKnowledgeDAO) CreateLongTermKnowledge(ctx context.Context, ltk *models.LongTermKnowledge) error {
	return dao.DB.WithContext(ctx).Create(ltk).Error
}

func (dao *LongTermKnowledgeDAO) GetLongTermKnowledgeByID(ctx context.Context, id uuid.UUID) (*models.LongTermKnowledge, error) {
	var ltk models.LongTermKnowledge
	err := dao.DB.WithContext(ctx).First(&ltk, "id = ?", id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ltk, nil
}

func (dao *LongTermKnowledgeDAO) GetAllLongTermKnowledgeByUser(ctx context.Context, userID int) ([]models.LongTermKnowledge, error) {
	var knowledge []models.LongTermKnowledge
	err := dao.DB.WithContext(ctx).Where("user_id = ?", userID).Order("created_at desc").Find(&knowledge).Error
	if err != nil {
		return nil, err
	}
	return knowledge, nil
}

func (dao *LongTermKnowledgeDAO) UpdateLongTermKnowledge(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	return dao.DB.WithContext(ctx).Model(&models.LongTermKnowledge{}).Where("id = ?", id).Updates(updates).Error
}

func (dao *LongTermKnowledgeDAO) DeleteLongTermKnowledge(ctx context.Context, id uuid.UUID) error {
	return dao.DB.WithContext(ctx).Where("id = ?", id).Delete(&models.LongTermKnowledge{}).Error
}

func (dao *LongTermKnowledgeDAO) GetLongTermKnowledgeByKnowledgeType(ctx context.Context, user_id int, knowledge_type string) ([]models.LongTermKnowledge, error) {
	var knowledge []models.LongTermKnowledge
	db := dao.DB.WithContext(ctx)
	db = db.Where("knowledge_type = ?", knowledge_type)
	db = db.Where("user_id = ?", user_id)
	err := db.Order("created_at desc").Find(&knowledge).Error
	if err != nil {
		return nil, err
	}
	return knowledge, nil
}
