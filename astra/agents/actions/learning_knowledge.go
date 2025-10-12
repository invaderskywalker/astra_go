// Package actions: named DataActions for learning_knowledge, doctrine-aligned
package actions

import (
	"astra/astra/sources/psql/models"
	"context"
	"fmt"

	"github.com/google/uuid"
)

// CreateLearningKnowledgeParams represents parameters for creating a LearningKnowledge entry.
type CreateLearningKnowledgeParams struct {
	KnowledgeType string `json:"knowledge_type"`
	KnowledgeBlob string `json:"knowledge_blob"`
}

// UpdateLearningKnowledgeParams represents parameters for updating a LearningKnowledge entry.
type UpdateLearningKnowledgeParams struct {
	Id      string                 `json:"id"`
	Updates map[string]interface{} `json:"updates"`
}

// GetAllLearningKnowledgeByTypeParams represents params to fetch by type.
type GetAllLearningKnowledgeByTypeParams struct {
	KnowledgeType string `json:"knowledge_type"`
}

// CreateLearningKnowledgeAction: Named DataAction wrapping DAO
func (a *DataActions) CreateLearningKnowledgeAction(p CreateLearningKnowledgeParams) error {
	lk := models.LearningKnowledge{
		UserID:        a.UserID,
		KnowledgeType: p.KnowledgeType,
		KnowledgeBlob: p.KnowledgeBlob,
	}
	ctx := context.Background()
	return a.learningDao.CreateLearningKnowledge(ctx, &lk)
}

// UpdateLearningKnowledgeAction: Named DataAction for updates
func (a *DataActions) UpdateLearningKnowledgeAction(p UpdateLearningKnowledgeParams) error {
	id, err := uuid.Parse(p.Id)
	if err != nil {
		return fmt.Errorf("invalid uuid: %w", err)
	}
	ctx := context.Background()
	return a.learningDao.UpdateLearningKnowledge(ctx, id, p.Updates)
}

// GetAllLearningKnowledgeForUserAction: Named DataAction (returns slice)
func (a *DataActions) GetAllLearningKnowledgeForUserAction() ([]models.LearningKnowledge, error) {
	ctx := context.Background()
	return a.learningDao.GetAllLearningKnowledgeByUser(ctx, a.UserID)
}

// GetAllLearningKnowledgeForUserByTypeAction: Filtered fetch
func (a *DataActions) GetAllLearningKnowledgeForUserByTypeAction(p GetAllLearningKnowledgeByTypeParams) ([]models.LearningKnowledge, error) {
	ctx := context.Background()
	return a.learningDao.GetLearningKnowledgeByKnowledgeType(ctx, a.UserID, p.KnowledgeType)
}
