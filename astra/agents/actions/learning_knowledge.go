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
	ID      string         `json:"id"`
	Updates map[string]any `json:"updates"`
}
type GetAllLongTermKnowledgeByTypeParams struct {
	KnowledgeType string `json:"knowledge_type"`
}

func (a *DataActions) CreateLongTermKnowledgeAction(p CreateLongTermKnowledgeParams) ActionResult {
	if p.KnowledgeType == "" || p.KnowledgeBlob == "" {
		return ActionResult{Success: false, Error: "knowledge_type and knowledge_blob are required"}
	}
	item := models.LongTermKnowledge{UserID: a.UserID, KnowledgeType: p.KnowledgeType, KnowledgeBlob: p.KnowledgeBlob}
	if err := a.longTermKnowledgeDao.CreateLongTermKnowledge(context.Background(), &item); err != nil {
		return ActionResult{Success: false, Error: err.Error()}
	}
	return ActionResult{Success: true, Summary: "Knowledge saved"}
}

func (a *DataActions) UpdateLongTermKnowledgeAction(p UpdateLongTermKnowledgeParams) ActionResult {
	id, err := uuid.Parse(p.ID)
	if err != nil {
		return ActionResult{Success: false, Error: fmt.Sprintf("invalid knowledge id: %v", err)}
	}
	if err := a.longTermKnowledgeDao.UpdateLongTermKnowledge(context.Background(), id, p.Updates); err != nil {
		return ActionResult{Success: false, Error: err.Error()}
	}
	return ActionResult{Success: true, Summary: "Knowledge updated"}
}

func (a *DataActions) GetAllLongTermKnowledgeForUserAction(_ struct{}) ActionResult {
	items, err := a.longTermKnowledgeDao.GetAllLongTermKnowledgeByUser(context.Background(), a.UserID)
	if err != nil {
		return ActionResult{Success: false, Error: err.Error()}
	}
	return ActionResult{Success: true, Summary: fmt.Sprintf("Found %d knowledge item(s)", len(items)), Diagnostics: items}
}
func (a *DataActions) GetAllLongTermKnowledgeForUserByTypeAction(p GetAllLongTermKnowledgeByTypeParams) ActionResult {
	items, err := a.longTermKnowledgeDao.GetLongTermKnowledgeByKnowledgeType(context.Background(), a.UserID, p.KnowledgeType)
	if err != nil {
		return ActionResult{Success: false, Error: err.Error()}
	}
	return ActionResult{Success: true, Summary: fmt.Sprintf("Found %d knowledge item(s)", len(items)), Diagnostics: items}
}
func (a *DataActions) GetAllKnowledgeTypesForUser(_ struct{}) ActionResult {
	types, err := a.longTermKnowledgeDao.GetDistinctKnowledgeTypes(context.Background(), a.UserID)
	if err != nil {
		return ActionResult{Success: false, Error: err.Error()}
	}
	return ActionResult{Success: true, Summary: fmt.Sprintf("Found %d knowledge type(s)", len(types)), Diagnostics: types}
}

func (a *DataActions) registerKnowledgeActions() {
	a.register(ActionSpec{Name: "create_long_term_knowledge", Description: "Saves durable user knowledge.", Guidance: "Save only useful, non-sensitive facts supplied by the user.", Params: CreateLongTermKnowledgeParams{}, handler: decodeHandler(a.CreateLongTermKnowledgeAction)})
	a.register(ActionSpec{Name: "update_long_term_knowledge", Description: "Updates an existing saved knowledge item.", Guidance: "Use only with a known knowledge item id.", Params: UpdateLongTermKnowledgeParams{}, handler: decodeHandler(a.UpdateLongTermKnowledgeAction)})
	a.register(ActionSpec{Name: "list_long_term_knowledge", Description: "Lists the user's saved knowledge.", Guidance: "Use when existing user preferences or facts may be relevant.", Params: struct{}{}, handler: decodeHandler(a.GetAllLongTermKnowledgeForUserAction)})
	a.register(ActionSpec{Name: "list_long_term_knowledge_by_type", Description: "Lists saved knowledge in one category.", Guidance: "Use a specific type to reduce unnecessary context.", Params: GetAllLongTermKnowledgeByTypeParams{}, handler: decodeHandler(a.GetAllLongTermKnowledgeForUserByTypeAction)})
	a.register(ActionSpec{Name: "list_knowledge_types", Description: "Lists available saved-knowledge categories.", Guidance: "Use before querying a category whose name is unknown.", Params: struct{}{}, handler: decodeHandler(a.GetAllKnowledgeTypesForUser)})
}
