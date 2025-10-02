// astra/agents/core/base_agent.go (new)
package core

import (
	"astra/astra/agents/actions"
	"astra/astra/agents/getters"
	"astra/astra/services/llm"
	"astra/astra/utils/logging"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

const (
	DefaultModel     = "llama3:8b"
	DefaultMaxTokens = 10000
	DefaultTemp      = 0.1
)

type BaseAgent struct {
	Name      string
	TenantID  int // Use userID as tenant for now
	UserID    int
	LLM       *llm.OllamaClient
	Config    map[string]interface{}
	Plans     []map[string]interface{}
	SessionID string
	LogInfo   map[string]interface{}

	dataGetters *getters.DataGetters
	dataActions *actions.DataActions

	// Event bus (simple channels)
	stepCh     chan map[string]interface{}
	responseCh chan string

	mu sync.Mutex
}

func NewBaseAgent(userID int, sessionID string, agentName string) *BaseAgent {
	cfgBytes, _ := os.ReadFile("astra/agents/config/agent.yaml")
	var cfg map[string]interface{}
	yaml.Unmarshal(cfgBytes, &cfg)

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
		dataGetters: getters.NewDataGetters(),
	}
	logging.Logger.Info("BaseAgent init", agent.LogInfo)
	go agent.handleEvents()
	return agent
}

func (a *BaseAgent) handleEvents() {
	for {
		select {
		case step := <-a.stepCh:
			// Emit to WS or log
			logging.Logger.Info("Step update", step)
		case resp := <-a.responseCh:
			// Stream response
			logging.Logger.Info("Response chunk", map[string]string{"chunk": resp})
		}
	}
}

func (a *BaseAgent) constructPlanningPrompt(query string) map[string]interface{} {
	// Mirror Python: Build system/user prompt from config
	systemPrompt := fmt.Sprintf(`
You are %s. Analyze query: %s
Available sources: %v
Available actions: %v
Output JSON: %s
`, a.Config["agent_name"], query, a.Config["available_data_sources"], a.Config["available_actions"], a.Config["llm1_plan_output_structure"])

	req := llm.ChatRequest{
		Model:    DefaultModel,
		Messages: []llm.Message{{Role: "system", Content: systemPrompt}, {Role: "user", Content: query}},
		Stream:   false,
	}

	resp, err := a.LLM.Run(context.Background(), req)
	if err != nil {
		logging.Logger.Error("Planning error", "error", err)
		return nil
	}

	var plan map[string]interface{}
	json.Unmarshal([]byte(resp), &plan)
	a.storeState("planning", plan)
	a.Plans = append(a.Plans, plan)
	return plan
}

func (a *BaseAgent) ProcessQuery(query string) <-chan string {
	ch := make(chan string)
	go func() {
		defer close(ch)

		// Plan
		a.stepCh <- map[string]interface{}{"message": "Creating execution plan"}
		plan := a.constructPlanningPrompt(query)

		// Execute
		a.stepCh <- map[string]interface{}{"message": "Executing plan"}
		results := a.executePlan(plan)

		// Response
		a.stepCh <- map[string]interface{}{"message": "Preparing response"}
		respCh, _ := a.LLM.RunStream(context.Background(), a.buildResponseReq(results)) // Build req like planning
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

	// // Parallel getters (goroutines)
	// var wg sync.WaitGroup
	// sources := plan["data_sources_to_trigger_with_source_params"].(map[string]interface{})
	// for name, paramsI := range sources {
	// 	params := paramsI.(map[string]interface{})
	// 	wg.Add(1)
	// 	go func(n string, p map[string]interface{}) {
	// 		defer wg.Done()
	// 		res := a.dataGetters.fnMaps[n](p)
	// 		a.mu.Lock()
	// 		results["data_sources_results"].(map[string]interface{})[n] = res
	// 		a.mu.Unlock()
	// 	}(name, params)
	// }
	// wg.Wait()

	// // Sequential actions
	// actions := plan["actions_to_trigger_with_action_params"].(map[string]interface{})
	// for name, paramsI := range actions {
	// 	params := paramsI.(map[string]interface{})
	// 	res := a.dataActions.fnMaps[name](params)
	// 	results["action_results"].(map[string]interface{})[name] = res
	// }

	return results
}

func (a *BaseAgent) buildResponseReq(results map[string]interface{}) llm.ChatRequest {
	// Mirror Python response prompt
	system := "You are " + a.Config["agent_name"].(string) + ". Generate response based on results."
	return llm.ChatRequest{
		Model:    DefaultModel,
		Messages: []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: fmt.Sprintf("Results: %+v", results)}},
		Stream:   true,
	}
}

func (a *BaseAgent) storeState(key string, value interface{}) {
	// Use agentDAO.InsertState(context.Background(), a.SessionID, a.UserID, key, json of value)
	// Omitted; inject DAO
}
