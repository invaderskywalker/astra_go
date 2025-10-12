// astra/controllers/learning.go
package controllers

import (
	"astra/astra/sources/psql/dao"
	"astra/astra/sources/psql/models"
	"context"
)

type LearningController struct {
	dao *dao.LearningKnowledgeDAO
}

func NewLearningController(dao *dao.LearningKnowledgeDAO) *LearningController {
	return &LearningController{dao: dao}
}

func (c *LearningController) GetAllLearningKnowledgeByUser(ctx context.Context, userID int) ([]models.LearningKnowledge, error) {
	return c.dao.GetAllLearningKnowledgeByUser(ctx, userID)
}

func (c *LearningController) GetAllLearningKnowledgeByUserAndType(ctx context.Context, userID int, knowledgeType string) ([]models.LearningKnowledge, error) {
	return c.dao.GetLearningKnowledgeByKnowledgeType(ctx, userID, knowledgeType)
}
