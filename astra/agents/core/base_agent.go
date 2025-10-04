package core

import (
	"astra/astra/agents/actions"
	"astra/astra/agents/configs"
	"astra/astra/services/llm"
	"astra/astra/utils/jsonutils"
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
	dataActions *actions.DataActions
	stepCh      chan map[string]interface{}
	responseCh  chan string
	mu          sync.Mutex
}

func NewBaseAgent(userID int, sessionID string, agentName string, db *gorm.DB) *BaseAgent {
	cfg := configs.LoadConfig()
	// fmt.Printf("cfg loaded: %+v\n", cfg) // Debug print
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
		dataActions: actions.NewDataActions(db),
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
			logging.AppLogger.Info("Step update", zap.Any("step", step))
		case resp := <-a.responseCh:
			logging.AppLogger.Info("Response chunk", zap.String("chunk", resp))
		}
	}
}

func (a *BaseAgent) createExecutionPlan(query string) (plan map[string]interface{}) {
	// Default error return if something goes wrong
	defer func() {
		if r := recover(); r != nil {
			logging.ErrorLogger.Error("Planning failure", zap.Any("recover", r))
			plan = map[string]interface{}{"error": fmt.Sprint(r)}
		}
	}()

	// Build a structured description of the decision process stages
	var stagesDesc string
	for i, stage := range a.Config.DecisionProcess.Stages {
		stagesDesc += fmt.Sprintf(
			"\nStage %d: %s\nPurpose: %s\nBehavior: %s\nOutputs: %v\n",
			i+1, stage.Name, stage.Purpose, stage.Behavior, stage.Outputs,
		)
	}

	// Build the system prompt
	systemPrompt := fmt.Sprintf(`
		You are a **planning assistant** 
		responsible for analyzing user queries 
		and determining the appropriate actions to use across stages

		## Context
		**Agent Name:** %s  
		**Agent Role:** %s  

		**Available Actions:** %v  

		## Decision Process
			**Description:**  
			%s  

			**Stages:**  
			%s  

		## Your Task
			Analyze the user query (and conversation context if available) to create an execution plan by:
			- Classifying the user's intent.
			- Determining the necessary actions to perform across stages.
			- Providing a clear rationale for your choices, including assumptions and dependencies.
			- Making the plan thoughtful and connecting it to the previous context.
			- If clarification is required, mark it explicitly in the plan and suggest clarification prompts.

		## Instructions
			- Always follow the defined decision process stages when structuring your plan.
			- Select **only the necessary** actions from the available list (avoid redundancy).
			- If a step requires multiple calls with different parameters, include it multiple times.
			- Ensure all required parameters for actions are specified clearly.
			- Provide reasoning for each step (planning rationale, assumptions, risks).
			- If user context is incomplete (e.g., missing company_name, company_website, designation), note this in assumptions or missing_fields.

		## Output Format
			You MUST respond with a valid JSON object in **exactly this schema**:

		%s

		## Important Notes
			- Respond strictly in valid JSON, with no extra commentary.
			- Include only non-null keys in the JSON.
			- Ensure actions and parameters align with the user's query and decision process.
			- Stick to the structured outputs specified in the decision process.

		---

			### User Query
			%s
		`,
		a.Config.AgentName,
		a.Config.AgentRole,
		a.Config.AvailableActions,
		a.Config.DecisionProcess.Description,
		stagesDesc,
		a.Config.OutputFormats.PlanOutputJSON,
		query,
	)

	user_message := fmt.Sprintf(`
		Please analyze and create an execution plan for the following user query:
			User Query: %s
			Remember to:
			- Apply decision process
			- Focus on addressing the specific query
			- Output valid JSON per the specified format.
			- Include params for all sources/actions triggered.
			Please stick to the json output format and include all output in the JSON
		`,
		query)

	req := llm.ChatRequest{
		Model:    DefaultModel,
		Messages: []llm.Message{{Role: "system", Content: systemPrompt}, {Role: "user", Content: user_message}},
		Stream:   false,
	}

	resp, err := a.LLM.Run(context.Background(), req)
	if err != nil {
		panic(fmt.Errorf("failed to create plan: %w", err))
	}

	print("resp ...", resp)

	respJSON := jsonutils.ExtractJSON(resp)
	if err := json.Unmarshal([]byte(respJSON), &plan); err != nil {
		panic(fmt.Errorf("invalid plan format: %w", err))
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
		plan := a.createExecutionPlan(query)
		if plan["error"] != nil {
			ch <- `{"error":"` + fmt.Sprint(plan["error"]) + `"}`
			return
		}

		a.stepCh <- map[string]interface{}{"message": "Executing plan"}
		results := a.executePlan(plan)

		a.stepCh <- map[string]interface{}{"message": "Preparing response"}
		respCh, err := a.LLM.RunStream(context.Background(), a.buildResponseReq(results, query))
		if err != nil {
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
	results := map[string]interface{}{
		"action_results": map[string]interface{}{},
	}

	steps, ok := plan["detailed_plan"].([]interface{})
	if !ok {
		return map[string]interface{}{"error": "invalid plan format"}
	}

	for _, s := range steps {
		step := s.(map[string]interface{})
		// stepID := step["step_id"].(string)
		// action := step["action"].(string)
		// params := step["action_params"].(map[string]interface{})

		fmt.Println("step", step)

		// // Dispatch to action executor
		// res := a.executeAction(action, params)
		// Store result
		// results["action_results"].(map[string]interface{})[stepID] = res
		// // If failed, trigger re-plan
		// if res["status"] == "fail" {
		// 	newPlan := a.replan(plan, step, res)
		// 	// Replace remaining steps
		// 	steps = newPlan["detailed_plan"].([]interface{})
		// }
	}
	return results
}

func (a *BaseAgent) buildResponseReq(results map[string]interface{}, query string) llm.ChatRequest {
	// Serialize results to JSON for cleaner LLM input
	resultsJSON, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		resultsJSON = []byte(`{"error": "failed to serialize results"}`)
	}

	systemPrompt := fmt.Sprintf(`
		You are %s.
			Role:
			%s
			User Query:
			%s
			Context:
			These are the execution results from the plan (in JSON):
			%s
		Now generate a clear, helpful final response for the user.
		`,
		a.Config.AgentName,
		a.Config.AgentRole,
		query,
		string(resultsJSON),
	)

	return llm.ChatRequest{
		Model: DefaultModel,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: query},
		},
		Stream: true,
	}
}

func (a *BaseAgent) storeState(key string, value interface{}) {
	fmt.Println("storeState ", key, value)
}
