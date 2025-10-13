// astra/controllers/long_term.go
package controllers

import (
	"astra/astra/sources/psql/dao"
	"astra/astra/sources/psql/models"
	"context"
)

type LongTermController struct {
	dao *dao.LongTermKnowledgeDAO
}

func NewLongTermController(dao *dao.LongTermKnowledgeDAO) *LongTermController {
	return &LongTermController{dao: dao}
}

func (c *LongTermController) GetAllLongTermKnowledgeByUser(ctx context.Context, userID int) ([]models.LongTermKnowledge, error) {
	return c.dao.GetAllLongTermKnowledgeByUser(ctx, userID)
}

func (c *LongTermController) GetAllLongTermKnowledgeByUserAndType(ctx context.Context, userID int, knowledgeType string) ([]models.LongTermKnowledge, error) {
	return c.dao.GetLongTermKnowledgeByKnowledgeType(ctx, userID, knowledgeType)
}
