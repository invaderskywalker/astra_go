package core

import (
	"astra/astra/agents/actions"
	"astra/astra/agents/configs"
	"astra/astra/agents/getters"
	"astra/astra/services/llm"
	"astra/astra/utils/logging"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	DefaultModel     = "llama3:8b"
	DefaultMaxTokens = 10000
	DefaultTemp      = 0.1
)

type BaseAgent struct {
	Name        string
	TenantID    int
	UserID      int
	LLM         *llm.OllamaClient
	Config      *configs.AgentConfig
	Plans       []map[string]interface{}
	SessionID   string
	LogInfo     map[string]interface{}
	dataGetters *getters.DataGetters
	dataActions *actions.DataActions
	stepCh      chan map[string]interface{}
	responseCh  chan string
	mu          sync.Mutex
}

func NewBaseAgent(userID int, sessionID string, agentName string, db *gorm.DB) *BaseAgent {
	cfg := configs.LoadConfig(agentName)

	agent := &BaseAgent{
		Name:        agentName,
		TenantID:    userID,
		UserID:      userID,
		LLM:         llm.NewOllamaClient(),
		Config:      cfg,
		SessionID:   sessionID,
		LogInfo:     map[string]interface{}{"tenant_id": userID, "user_id": userID, "session_id": sessionID},
		stepCh:      make(chan map[string]interface{}, 10),
		responseCh:  make(chan string, 10),
		dataActions: actions.NewDataActions(db), // Pass db
		// dataGetters: getters.NewDataGetters(), // Uncomment when implemented
	}
	logging.AppLogger.Info("BaseAgent initialized",
		zap.Int("user_id", userID),
		zap.String("agent_name", agentName),
	)
	go agent.handleEvents()
	return agent
}

func (a *BaseAgent) handleEvents() {
	for {
		select {
		case step := <-a.stepCh:
			fmt.Println("step ", step)
			// logging.AppLogger.Info("Step update", zap.Any("step", step))
		case resp := <-a.responseCh:
			fmt.Println("response ", resp)
			// logging.AppLogger.Info("Response chunk", zap.String("chunk", resp))
		}
	}
}

func (a *BaseAgent) constructPlanningPrompt(query string) map[string]interface{} {
	systemPrompt := fmt.Sprintf(`
You are %s. Analyze query: %s
Available sources: %v
Available actions: %v
Output JSON: %s
`, a.Config.Name, query, a.Config.AvailableGetters, a.Config.AvailableActions, a.Config.PlanningPrompt)

	req := llm.ChatRequest{
		Model:    DefaultModel,
		Messages: []llm.Message{{Role: "system", Content: systemPrompt}, {Role: "user", Content: query}},
		Stream:   false,
	}

	resp, err := a.LLM.Run(context.Background(), req)
	if err != nil {
		logging.ErrorLogger.Error("Planning error", zap.Error(err))
		return nil
	}

	var plan map[string]interface{}
	if err := json.Unmarshal([]byte(resp), &plan); err != nil {
		logging.ErrorLogger.Error("Plan unmarshal error", zap.Error(err))
		return nil
	}
	a.storeState("planning", plan)
	a.Plans = append(a.Plans, plan)
	return plan
}

func (a *BaseAgent) ProcessQuery(query string) <-chan string {
	ch := make(chan string)
	go func() {
		defer close(ch)

		a.stepCh <- map[string]interface{}{"message": "Creating execution plan"}
		plan := a.constructPlanningPrompt(query)
		if plan == nil {
			ch <- `{"error":"failed to create plan"}`
			return
		}

		a.stepCh <- map[string]interface{}{"message": "Executing plan"}
		results := a.executePlan(plan)

		a.stepCh <- map[string]interface{}{"message": "Preparing response"}
		respCh, err := a.LLM.RunStream(context.Background(), a.buildResponseReq(results))
		if err != nil {
			logging.ErrorLogger.Error("Response stream error", zap.Error(err))
			ch <- `{"error":"failed to stream response"}`
			return
		}
		for chunk := range respCh {
			a.responseCh <- chunk
			ch <- chunk
		}

		a.storeState("results", results)
	}()
	return ch
}

func (a *BaseAgent) executePlan(plan map[string]interface{}) map[string]interface{} {
	results := map[string]interface{}{"data_sources_results": map[string]interface{}{}, "action_results": map[string]interface{}{}}
	return results
}

func (a *BaseAgent) buildResponseReq(results map[string]interface{}) llm.ChatRequest {
	system := fmt.Sprintf(a.Config.ResponsePrompt, a.Config.Name, a.Config.Role, results)
	return llm.ChatRequest{
		Model:    DefaultModel,
		Messages: []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: fmt.Sprintf("Results: %+v", results)}},
		Stream:   true,
	}
}

func (a *BaseAgent) storeState(key string, value interface{}) {
	// Implement state storage (e.g., save to DB or file) if needed
}
