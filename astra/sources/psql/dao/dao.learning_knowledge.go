package dao

import (
	"astra/astra/sources/psql/models"
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LearningKnowledgeDAO struct {
	DB *gorm.DB
}

func NewLearningKnowledgeDAO(db *gorm.DB) *LearningKnowledgeDAO {
	return &LearningKnowledgeDAO{DB: db}
}

// CreateLearningKnowledge inserts a new LearningKnowledge record.
func (dao *LearningKnowledgeDAO) CreateLearningKnowledge(ctx context.Context, lk *models.LearningKnowledge) error {
	return dao.DB.WithContext(ctx).Create(lk).Error
}

// GetLearningKnowledgeByID fetches a LearningKnowledge by UUID.
func (dao *LearningKnowledgeDAO) GetLearningKnowledgeByID(ctx context.Context, id uuid.UUID) (*models.LearningKnowledge, error) {
	var lk models.LearningKnowledge
	err := dao.DB.WithContext(ctx).First(&lk, "id = ?", id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &lk, nil
}

// GetAllLearningKnowledgeByUser fetches all LearningKnowledge items for a user, ordered by CreatedAt desc.
func (dao *LearningKnowledgeDAO) GetAllLearningKnowledgeByUser(ctx context.Context, userID int) ([]models.LearningKnowledge, error) {
	var knowledge []models.LearningKnowledge
	err := dao.DB.WithContext(ctx).Where("user_id = ?", userID).Order("created_at desc").Find(&knowledge).Error
	if err != nil {
		return nil, err
	}
	return knowledge, nil
}

// UpdateLearningKnowledge updates an existing LearningKnowledge record by ID.
func (dao *LearningKnowledgeDAO) UpdateLearningKnowledge(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	return dao.DB.WithContext(ctx).Model(&models.LearningKnowledge{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteLearningKnowledge deletes a LearningKnowledge record by ID.
func (dao *LearningKnowledgeDAO) DeleteLearningKnowledge(ctx context.Context, id uuid.UUID) error {
	return dao.DB.WithContext(ctx).Where("id = ?", id).Delete(&models.LearningKnowledge{}).Error
}

// GetLearningKnowledgeByFilters fetches LearningKnowledge items matching arbitrary filters (e.g., {"user_id":, "knowledge_type":, ...}).
func (dao *LearningKnowledgeDAO) GetLearningKnowledgeByKnowledgeType(ctx context.Context, user_id int, knowledge_type string) ([]models.LearningKnowledge, error) {
	var knowledge []models.LearningKnowledge
	db := dao.DB.WithContext(ctx)
	db = db.Where("knowledge_type = ?", knowledge_type)
	db = db.Where("user_id = ?", user_id)
	err := db.Order("created_at desc").Find(&knowledge).Error
	if err != nil {
		return nil, err
	}
	return knowledge, nil
}
