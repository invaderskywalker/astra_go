package actions

import (
	"astra/astra/sources/psql/dao"
	"astra/astra/sources/psql/models"
	"context"

	"github.com/google/uuid"
)

// CreateLearningKnowledgeAction creates a new LearningKnowledge entry for a user.
func CreateLearningKnowledgeAction(lk *models.LearningKnowledge, lkDao *dao.LearningKnowledgeDAO) error {
	return lkDao.CreateLearningKnowledge(context.Background(), lk)
}

// UpdateLearningKnowledgeAction updates an existing LearningKnowledge entry by id.
func UpdateLearningKnowledgeAction(id uuid.UUID, updates map[string]interface{}, lkDao *dao.LearningKnowledgeDAO) error {
	return lkDao.UpdateLearningKnowledge(context.Background(), id, updates)
}

// GetAllLearningKnowledgeForUserAction retrieves all LearningKnowledge entries for a user.
func GetAllLearningKnowledgeForUserAction(userID int, lkDao *dao.LearningKnowledgeDAO) ([]models.LearningKnowledge, error) {
	return lkDao.GetAllLearningKnowledgeByUser(context.Background(), userID)
}

// GetAllLearningKnowledgeForUserByTypeAction retrieves LearningKnowledge entries by user and knowledge_type.
func GetAllLearningKnowledgeForUserByTypeAction(userID int, knowledgeType string, lkDao *dao.LearningKnowledgeDAO) ([]models.LearningKnowledge, error) {
	return lkDao.GetLearningKnowledgeByKnowledgeType(context.Background(), userID, knowledgeType)
}
