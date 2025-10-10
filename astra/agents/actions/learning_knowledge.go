package actions

import (
	"astra/astra/sources/psql/models"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Params for creating a LearningKnowledge record
// All fields except ID, CreatedAt are required.
type CreateLearningKnowledgeParams struct {
	UserID        int    `json:"user_id"`
	KnowledgeType string `json:"knowledge_type"`
	KnowledgeBlob string `json:"knowledge_blob"`
}

type UpdateLearningKnowledgeParams struct {
	ID            string `json:"id"` // uuid string
	KnowledgeType string `json:"knowledge_type"`
	KnowledgeBlob string `json:"knowledge_blob"`
}

type FetchLearningKnowledgeParams struct {
	ID     string `json:"id,omitempty"` // uuid string, optional
	UserID int    `json:"user_id,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type LearningKnowledgeResult struct {
	LearningKnowledge models.LearningKnowledge `json:"learning_knowledge"`
}
type LearningKnowledgeListResult struct {
	Results []models.LearningKnowledge `json:"results"`
}

// CreateLearningKnowledge stores a new LearningKnowledge entry
type DataActionsWithDB interface {
	GetDB() *gorm.DB
}

func (a *DataActions) CreateLearningKnowledge(params CreateLearningKnowledgeParams) (LearningKnowledgeResult, error) {
	db := a.db
	lk := models.LearningKnowledge{
		UserID:        params.UserID,
		KnowledgeType: params.KnowledgeType,
		KnowledgeBlob: params.KnowledgeBlob,
	}
	if err := db.Create(&lk).Error; err != nil {
		return LearningKnowledgeResult{}, err
	}
	return LearningKnowledgeResult{LearningKnowledge: lk}, nil
}

func (a *DataActions) UpdateLearningKnowledge(params UpdateLearningKnowledgeParams) (LearningKnowledgeResult, error) {
	db := a.db
	var lk models.LearningKnowledge
	id, err := parseUUIDString(params.ID)
	if err != nil {
		return LearningKnowledgeResult{}, err
	}
	if err := db.First(&lk, "id = ?", id).Error; err != nil {
		return LearningKnowledgeResult{}, err
	}
	lk.KnowledgeType = params.KnowledgeType
	lk.KnowledgeBlob = params.KnowledgeBlob
	if err := db.Save(&lk).Error; err != nil {
		return LearningKnowledgeResult{}, err
	}
	return LearningKnowledgeResult{LearningKnowledge: lk}, nil
}

func (a *DataActions) FetchLearningKnowledge(params FetchLearningKnowledgeParams) (LearningKnowledgeListResult, error) {
	db := a.db
	var results []models.LearningKnowledge
	q := db.Model(&models.LearningKnowledge{})
	if params.ID != "" {
		id, err := parseUUIDString(params.ID)
		if err != nil {
			return LearningKnowledgeListResult{}, err
		}
		q = q.Where("id = ?", id)
	}
	if params.UserID != 0 {
		q = q.Where("user_id = ?", params.UserID)
	}
	if params.Limit > 0 {
		q = q.Limit(params.Limit)
	}
	if err := q.Order("created_at desc").Find(&results).Error; err != nil {
		return LearningKnowledgeListResult{}, err
	}
	return LearningKnowledgeListResult{Results: results}, nil
}

// Utility - parse uuid from string
func parseUUIDString(id string) (uuid.UUID, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return uuid.UUID{}, errors.New("invalid UUID: " + id)
	}
	return uid, nil
}
