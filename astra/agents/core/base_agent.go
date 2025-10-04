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
	Name           string
	TenantID       int
	UserID         int
	LLM            *llm.OllamaClient
	Config         *configs.AgentConfig
	ExecutionPlans []map[string]interface{}
	RoughPlan      map[string]interface{}
	SessionID      string
	LogInfo        map[string]interface{}
	dataActions    *actions.DataActions
	stepCh         chan map[string]interface{}
	responseCh     chan string
	mu             sync.Mutex
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
			fmt.Println("resp., ", resp)
		}
	}
}

func (a *BaseAgent) createRoughPlan(query string) (plan map[string]interface{}) {
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

	// Get lightweight action summaries (name + description) from runtime registry
	actionSummaries := a.dataActions.ListActionSummaries()
	actionsJSON, err := json.MarshalIndent(actionSummaries, "", "  ")
	actionsJSONStr := ""
	if err != nil {
		// Fallback to a simple fmt representation if marshal fails
		actionsJSONStr = fmt.Sprintf("%v", actionSummaries)
	} else {
		actionsJSONStr = string(actionsJSON)
	}

	// Build the system prompt (inject action summaries)
	systemPrompt := fmt.Sprintf(`
		You are a **planning assistant** 
		responsible for analyzing user queries 
		and determining the appropriate actions to use across stages

		## Context
		**Agent Name:** %s  
		**Agent Role:** %s  

		**Available Actions (name + description):** 
		%s

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
			- Do not include “meta” steps (like understanding, clarifying) in the execution template. 
			- Only include steps that require concrete actions from the available actions list.


		## Output Format Rules
			- Respond ONLY with a single JSON object. 
			- DO NOT include natural language, explanations, or markdown fences like.
			- The JSON must exactly follow this schema:

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
		actionsJSONStr,
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

			****important*****
			- Respond ONLY with valid JSON. 
			- Any text outside the JSON is considered an error.
		`,
		query,
	)

	req := llm.ChatRequest{
		Model: DefaultModel,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: user_message},
		},
		Stream: false,
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
	a.RoughPlan = plan
	return plan
}

func (a *BaseAgent) expandStep(stepText string, index int) (plan map[string]interface{}) {
	// Default error return if something goes wrong
	defer func() {
		if r := recover(); r != nil {
			logging.ErrorLogger.Error("ExpandStep failure", zap.Any("recover", r))
			plan = map[string]interface{}{"error": fmt.Sprint(r)}
		}
	}()

	// Get full action specs (params, returns, examples) from runtime registry
	fullActions := a.dataActions.ListActions()
	actionsJSON, err := json.MarshalIndent(fullActions, "", "  ")
	actionsJSONStr := ""
	if err != nil {
		actionsJSONStr = fmt.Sprintf("%v", fullActions)
	} else {
		actionsJSONStr = string(actionsJSON)
	}

	// Build the system prompt (inject full action specs)
	systemPrompt := fmt.Sprintf(`
		You are Astra’s Execution Planner.

			## Context
			You receive one natural-language step from a rough execution template.  
			Your job is to expand this step into a structured JSON execution plan.  
			This JSON will be given to an executor to either perform a real action (e.g. code edit, DB query, web scrape) or skip if no action is needed.

			## Rules
			- Always output **only one JSON object**, nothing else.  
			- Follow the schema exactly.  
			- If the step requires no concrete execution, set action and action params as empty. 
			- Otherwise, choose the correct action from the available list and specify precise action_params.  
			- Provide expected outputs and validation checks to help downstream validation.  
			- Do not include meta or planning notes.

			## Available Actions (full spec: params, returns, examples)
			%s

			## Output Schema
			%s

			## Full plan - %s
			## Your previous step expansion: %s

			## Current step to expand
			"%s"
		`,
		actionsJSONStr,
		a.Config.OutputFormats.ExecutionStepOutputJSON,
		a.RoughPlan,
		a.ExecutionPlans,
		stepText,
	)

	user_message := fmt.Sprintf(`
		Please analyze and create a good thoughtful execution plan for the following execution step:
			Step to focus on %s
			Please stick to the json output format and include all output in the JSON
			****important*****
			- Respond ONLY with valid JSON. 
			- Any text outside the JSON is considered an error.
		`,
		stepText,
	)

	req := llm.ChatRequest{
		Model: DefaultModel,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: user_message},
		},
		Stream: false,
	}
	resp, err := a.LLM.Run(context.Background(), req)
	if err != nil {
		panic(fmt.Errorf("failed to create plan: %w", err))
	}
	print("expandStep Resp ...", resp)
	respJSON := jsonutils.ExtractJSON(resp)
	if err := json.Unmarshal([]byte(respJSON), &plan); err != nil {
		panic(fmt.Errorf("invalid plan format: %w", err))
	}
	a.storeState("execution_step_expand", plan)
	a.ExecutionPlans = append(a.ExecutionPlans, plan)
	return plan
}

func (a *BaseAgent) ProcessQuery(query string) <-chan string {
	ch := make(chan string)
	go func() {
		defer close(ch)

		a.stepCh <- map[string]interface{}{"message": "Creating rough plan"}
		roughPlan := a.createRoughPlan(query)
		if roughPlan["error"] != nil {
			ch <- `{"error":"` + fmt.Sprint(roughPlan["error"]) + `"}`
			return
		}

		// Safely extract the steps list from the rough plan
		var stepsSlice []interface{}
		if dp, ok := roughPlan["decision_process_output"].(map[string]interface{}); ok {
			if rawSteps, ok := dp["mind_map_execution_statements"]; ok {
				if castSteps, ok := rawSteps.([]interface{}); ok {
					stepsSlice = castSteps
				}
			}
		}
		if stepsSlice == nil {
			ch <- `{"error":"no steps found in rough plan"}` // consistent error shape
			return
		}

		results := []map[string]interface{}{}

		for i, s := range stepsSlice {
			// Expect each step to be a plain string
			stepText, ok := s.(string)
			if !ok {
				a.stepCh <- map[string]interface{}{"message": "Skipping invalid step format", "index": i}
				continue
			}

			a.stepCh <- map[string]interface{}{"message": "Expanding step", "step_index": i + 1, "step": stepText}
			expanded := a.expandStep(stepText, i+1)
			if expanded == nil {
				// safety
				results = append(results, map[string]interface{}{
					"step_index": i + 1, "status": "error", "error": "expandStep returned nil",
				})
				continue
			}
			if errVal, ok := expanded["error"]; ok && errVal != nil {
				// expansion error: stream and abort further processing
				errStr := fmt.Sprint(errVal)
				ch <- `{"error":"` + errStr + `"}`
				return
			}

			// Determine if expanded is a full plan or a single-step object.
			var planToExec map[string]interface{}
			if _, hasDetailed := expanded["detailed_plan"]; hasDetailed {
				// assume expanded already fits executePlan input
				planToExec = expanded
			} else {
				// wrap single-step expansion into a plan
				planToExec = map[string]interface{}{"detailed_plan": []interface{}{expanded}}
			}

			a.stepCh <- map[string]interface{}{"message": "Executing expanded step", "step_index": i + 1}
			execRes := a.executePlan(planToExec)
			// append result and continue
			results = append(results, map[string]interface{}{
				"step_index": i + 1,
				"step_text":  stepText,
				"result":     execRes,
			})

			// Optional: if you want to trigger re-plan on certain conditions, examine execRes here
			// Example (simple): if execRes contains an error, you may choose to stop and replan.
			// Not implemented: replan loop (keeps flow linear for now).
		}

		a.stepCh <- map[string]interface{}{"message": "Preparing summary"}

		// Build the response request with full context & results
		respReq := a.buildResponseReq(map[string]interface{}{"steps": results}, query)

		// Stream final LLM response built from results
		respCh, err := a.LLM.RunStream(context.Background(), respReq)
		if err != nil {
			a.stepCh <- map[string]interface{}{"message": "LLM stream start failed", "error": err.Error()}
			ch <- `{"error":"failed to stream response"}`
			return
		}
		for chunk := range respCh {
			// forward chunks to both internal logger and caller channel
			a.responseCh <- chunk
			ch <- chunk
		}
	}()
	return ch
}

func (a *BaseAgent) executePlan(plan map[string]interface{}) map[string]interface{} {
	results := map[string]interface{}{
		"action_results": map[string]interface{}{},
	}

	steps, ok := plan["detailed_plan"].([]interface{})
	if !ok {
		return map[string]interface{}{"error": "invalid plan format: missing detailed_plan"}
	}

	for i, s := range steps {
		step, ok := s.(map[string]interface{})
		if !ok {
			// store parse error
			results["action_results"].(map[string]interface{})[fmt.Sprintf("step_%d", i)] = map[string]interface{}{
				"status": "error", "error": "invalid step format",
			}
			continue
		}

		// fetch identifiers and fields safely
		stepID := ""
		if v, ok := step["step_id"].(string); ok {
			stepID = v
		} else {
			// fallback to index-based id
			stepID = fmt.Sprintf("step_%d", i+1)
		}
		actionName, _ := step["action"].(string)

		// action_params may be missing or of different type; ensure we pass map[string]interface{}
		var params map[string]interface{}
		if p, ok := step["action_params"].(map[string]interface{}); ok {
			params = p
		} else {
			// try marshal/unmarshal to normalize if it's e.g. map[interface{}]interface{} or something else
			params = map[string]interface{}{}
			if step["action_params"] != nil {
				bytes, _ := json.Marshal(step["action_params"])
				_ = json.Unmarshal(bytes, &params)
			}
		}

		// If actionName empty → skip (no-op)
		if actionName == "" {
			results["action_results"].(map[string]interface{})[stepID] = map[string]interface{}{
				"status": "skipped", "note": "no action specified",
			}
			continue
		}

		// Execute the action via dataActions
		a.stepCh <- map[string]interface{}{"message": "Executing step", "step_id": stepID, "action": actionName}
		out, err := a.dataActions.ExecuteAction(actionName, params)
		if err != nil {
			results["action_results"].(map[string]interface{})[stepID] = map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
			}
			// Optionally: trigger replan logic here if needed (not implemented)
			continue
		}

		results["action_results"].(map[string]interface{})[stepID] = map[string]interface{}{
			"status": "ok",
			"output": out,
		}
	}

	return results
}

// Updated buildResponseReq - includes RoughPlan, ExecutionPlans, Available Actions, Decision Process and Results
func (a *BaseAgent) buildResponseReq(results map[string]interface{}, query string) llm.ChatRequest {
	// Prepare serialized pieces (best-effort - fall back to string representation on error)
	resultsJSON, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		resultsJSON = []byte(fmt.Sprintf(`"__failed_to_serialize_results__": %q`, err.Error()))
	}

	roughPlanJSON, err := json.MarshalIndent(a.RoughPlan, "", "  ")
	if err != nil {
		// if nil or serialization fails, show a short message
		roughPlanJSON = []byte(fmt.Sprintf(`"__rough_plan_unavailable__": %q`, err.Error()))
	}

	execPlansJSON, err := json.MarshalIndent(a.ExecutionPlans, "", "  ")
	if err != nil {
		execPlansJSON = []byte(fmt.Sprintf(`"__execution_plans_unavailable__": %q`, err.Error()))
	}

	// action summaries (name + description) to keep prompt size bounded
	actionSummaries := a.dataActions.ListActionSummaries()
	actionsJSON, err := json.MarshalIndent(actionSummaries, "", "  ")
	if err != nil {
		actionsJSON = []byte(fmt.Sprintf(`"__actions_unavailable__": %q`, err.Error()))
	}

	// build a human-readable stages description from config
	var stagesDesc string
	for i, stage := range a.Config.DecisionProcess.Stages {
		stagesDesc += fmt.Sprintf("Stage %d: %s\n  Purpose: %s\n  Behavior: %s\n  Outputs: %v\n\n",
			i+1, stage.Name, stage.Purpose, stage.Behavior, stage.Outputs)
	}

	// Final system prompt: explicit, structured, include schemas and content
	systemPrompt := fmt.Sprintf(`
		You are %s — %s.

		You will produce a clear, helpful final response to the user's query.  Use the provided execution results and context to generate:
		1) A concise summary of what was done and why (1-3 short paragraphs).
		2) A list of outcomes and any important outputs or files created/changed.
		3) Any actions that failed or need human attention (with short remediation steps).
		4) Next recommended steps (if any) and any required clarifications.

		Important: your reply should be primarily plain text suitable for a user. You may include a short JSON block (optional) listing final structured outputs if helpful, but it is not required. Prefer clarity and actionable language.

		--- Context and artifacts (for your reference) ---

		Agent identity:
		Agent Name: %s
		Agent Role: %s
		Session ID: %s

		User Query:
		%s

		Available action summaries (name + description):
		%s

		Decision process (description & stages):
		%s

		Rough plan template (the plan the agent created from the query; may include planned steps and rationale):
		%s

		Previously expanded execution plans (detailed steps produced for execution):
		%s

		Execution results (what ran and observed outputs - use this as the definitive log of what happened):
		%s

		Output schemas (for reference, used when the agent originally created the plan):
		Plan output schema: %s

		Execution step output schema: %s

		---

		Behavior requirements:
		- Be accurate and concise.
		- Highlight failures and partial results first.
		- For each failed or partial item, include a recommended remediation or a short verification step.
		- If user follow-up / clarification is required, clearly list the questions.
		- If everything succeeded, state that the plan completed successfully and summarize the key outputs.

		Now produce the final user-facing response using the above context.
		`,

		a.Config.AgentName,
		a.Config.AgentRole,
		a.SessionID,
		query,
		string(actionsJSON),
		a.Config.DecisionProcess.Description,
		string(roughPlanJSON),
		string(execPlansJSON),
		string(resultsJSON),
		a.Config.OutputFormats.PlanOutputJSON,
		a.Config.OutputFormats.ExecutionStepOutputJSON,
	)

	// The user-facing content will be sent as the "user" message; system prompt contains the context.
	userMessage := fmt.Sprintf("Please generate the final reply to the user for query: %s", query)

	return llm.ChatRequest{
		Model: DefaultModel,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
		Stream: true,
	}
}

func (a *BaseAgent) storeState(key string, value interface{}) {
	// fmt.Println("storeState ", key, value)
}
