// astra/agents/actions/long_term_knowledge.go
package actions

import (
	"astra/astra/sources/psql/models"
	"context"
	"fmt"

	"github.com/google/uuid"
)

type CreateLongTermKnowledgeParams struct {
	KnowledgeType string `json:"knowledge_type"`
	KnowledgeBlob string `json:"knowledge_blob"`
}

type UpdateLongTermKnowledgeParams struct {
	Id      string                 `json:"id"`
	Updates map[string]interface{} `json:"updates"`
}

type GetAllLongTermKnowledgeByTypeParams struct {
	KnowledgeType string `json:"knowledge_type"`
}

func (a *DataActions) CreateLongTermKnowledgeAction(p CreateLongTermKnowledgeParams) error {
	ltk := models.LongTermKnowledge{
		UserID:        a.UserID,
		KnowledgeType: p.KnowledgeType,
		KnowledgeBlob: p.KnowledgeBlob,
	}
	ctx := context.Background()
	return a.longTermKnowledgeDao.CreateLongTermKnowledge(ctx, &ltk)
}

func (a *DataActions) UpdateLongTermKnowledgeAction(p UpdateLongTermKnowledgeParams) error {
	id, err := uuid.Parse(p.Id)
	if err != nil {
		return fmt.Errorf("invalid uuid: %w", err)
	}
	ctx := context.Background()
	return a.longTermKnowledgeDao.UpdateLongTermKnowledge(ctx, id, p.Updates)
}

func (a *DataActions) GetAllLongTermKnowledgeForUserAction() ([]models.LongTermKnowledge, error) {
	ctx := context.Background()
	return a.longTermKnowledgeDao.GetAllLongTermKnowledgeByUser(ctx, a.UserID)
}

func (a *DataActions) GetAllLongTermKnowledgeForUserByTypeAction(p GetAllLongTermKnowledgeByTypeParams) ([]models.LongTermKnowledge, error) {
	ctx := context.Background()
	return a.longTermKnowledgeDao.GetLongTermKnowledgeByKnowledgeType(ctx, a.UserID, p.KnowledgeType)
}
