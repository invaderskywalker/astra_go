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
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	// DefaultModel     = "llama3:8b"
	DefaultModel     = "gpt-4.1"
	DefaultMaxTokens = 10000
	DefaultTemp      = 0.1
)

type BaseAgent struct {
	Name           string
	TenantID       int
	UserID         int
	LLM            *llm.GPTClient
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
		LLM:         llm.NewGPTClient(),
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
	var stagesDesc string = ""
	// Get lightweight action summaries (name + description) from runtime registry
	actionSummaries := a.dataActions.ListActionSummaries()

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

		## A rough example of mind_map_steps_in_natural_language  
		to ensure your output format is correct.
			"mind_map_steps_in_natural_language": [
				"plain english statement 1",
				"plain english next step",...
			]


		---

		### User Query
		%s
		`,
		a.Config.AgentName,
		a.Config.AgentRole,
		jsonutils.ToJSON(actionSummaries),
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
			- Respond ONLY with valid JSON only stick to this format: %s
			- Any text outside the JSON is considered an error.
		`,
		query,
		a.Config.OutputFormats.PlanOutputJSON,
	)

	// print("debug --createRoughPlan- prompt.. ", systemPrompt, user_message)

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

func (a *BaseAgent) generateNextExecutionPlan(roughPlan map[string]interface{}, stepIndex int, results any) (plan map[string]interface{}) {
	// Default error return if something goes wrong
	defer func() {
		if r := recover(); r != nil {
			logging.ErrorLogger.Error("generateNextExecutionPlan failure", zap.Any("recover", r))
			plan = map[string]interface{}{"error": fmt.Sprint(r)}
		}
	}()

	// Get full action specs (params, returns, examples) from runtime registry
	fullActions := a.dataActions.ListActions()
	actionsJSON, _ := json.MarshalIndent(fullActions, "", "  ")
	actionsJSONStr := string(actionsJSON)

	var systemPrompt string
	var userPrompt string

	systemPrompt = fmt.Sprintf(`
		You are Astra’s  sequential execution Planner.

		Context:
		- Full mind map plan: %s
		- Previous execution results: %s
		- Available actions (full spec): %s

		Task:
		You are provided with a full mind map of responding 
		to user query.
		And you are provided with all actions that you can take and 
		all previous execution determined by you and their results.

		Think properly and present only the next single 
		concrete execution plan (single JSON object).

		

		Rules:
		- Output exactly one JSON object and nothing else.
		- If no concrete action is required, set "action" to an empty string and return the schema.

		## Output Schema (stick to this)
			%s
		`,
		jsonutils.ToJSON(roughPlan),
		jsonutils.ToJSON(results),
		actionsJSONStr,
		a.Config.OutputFormats.ExecutionStepOutputJSON,
	)

	// fmt.Println("debug generateNextExecutionPlan prompt ", systemPrompt)

	userPrompt = fmt.Sprintf(`
		Please analyze and create a good thoughtful 
		execution plan and output a single object
		Please stick to the json output format and include all output in the JSON

		****important*****
		- Respond ONLY with valid JSON only stick to this format: %s
		- Any text outside the JSON is considered an error.
		`,
		a.Config.OutputFormats.ExecutionStepOutputJSON,
	)

	req := llm.ChatRequest{
		Model: DefaultModel,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream: false,
	}

	resp, err := a.LLM.Run(context.Background(), req)
	if err != nil {
		panic(fmt.Errorf("failed to create plan: %w", err))
	}

	fmt.Println("\n exec done --- ", resp)

	respJSON := jsonutils.ExtractJSON(resp)
	if err := json.Unmarshal([]byte(respJSON), &plan); err != nil {
		panic(fmt.Errorf("invalid plan format: %w", err))
	}

	// persist for traceability
	a.storeState("execution_step_expand", plan)
	a.ExecutionPlans = append(a.ExecutionPlans, plan)
	return plan
}

func (a *BaseAgent) ProcessQuery(query string) <-chan string {
	ch := make(chan string)

	go func() {
		defer close(ch)

		// Step 1: Create the rough plan
		a.stepCh <- map[string]interface{}{"message": "Creating rough plan"}
		roughPlan := a.createRoughPlan(query)
		if roughPlan["error"] != nil {
			ch <- a.formatEvent("error", map[string]interface{}{
				"message": fmt.Sprint(roughPlan["error"]),
			})
			return
		}
		a.RoughPlan = roughPlan
		ch <- a.formatEvent("intermediate", map[string]interface{}{
			"message": "Plan created successfully",
		})

		ch <- a.formatEvent("intermediate", map[string]interface{}{
			"message": jsonutils.ToJSON(roughPlan),
		})

		results := []map[string]interface{}{}
		stepIndex := 1

		// Step 3: Begin iterative execution loop
		for {

			a.stepCh <- map[string]interface{}{"message": "Planning step", "step_index": stepIndex}

			// Generate plan for current step
			expanded := a.generateNextExecutionPlan(a.RoughPlan, stepIndex, results)
			if expanded == nil {
				ch <- a.formatEvent("error", map[string]interface{}{
					"message": "generateNextExecutionPlan returned nil",
				})
				return
			}
			if errVal, ok := expanded["error"]; ok && errVal != nil {
				ch <- a.formatEvent("error", map[string]interface{}{
					"message": fmt.Sprint(errVal),
				})
				return
			}

			shouldContinue := false
			if sc, ok := expanded["should_continue"].(bool); ok {
				shouldContinue = sc
			}
			if !shouldContinue {
				break
			}

			ch <- a.formatEvent("intermediate", map[string]interface{}{
				"phase":    "expanded_step",
				"index":    stepIndex,
				"expanded": expanded,
			})

			// Step 4: Execute the plan
			var planToExec map[string]interface{} = expanded
			// if _, hasDetailed := expanded["detailed_plan"]; hasDetailed {
			// 	planToExec = expanded
			// } else {
			// 	planToExec = map[string]interface{}{"detailed_plan": []interface{}{expanded}}
			// }

			ch <- a.formatEvent("intermediate", map[string]interface{}{
				"phase": "executing_step", "index": stepIndex,
			})
			a.stepCh <- map[string]interface{}{"message": "Executing expanded step", "step_index": stepIndex}

			execRes := a.executePlan(planToExec)
			fmt.Println("result of execution ", execRes)

			ch <- a.formatEvent("intermediate", map[string]interface{}{
				"phase":   "executed_step",
				"index":   stepIndex,
				"execRes": execRes,
			})

			results = append(results, map[string]interface{}{
				"step_index": stepIndex,
				// "step_text":  stepText,
				"result": execRes,
			})

			// // Step 5: Reflection — check whether to continue and next_step
			// reflection := a.generateNextExecutionPlan(a.RoughPlan, 0, results)
			// if reflection == nil {
			// 	ch <- a.formatEvent("error", map[string]interface{}{
			// 		"message": "reflection returned nil",
			// 	})
			// 	return
			// }
			// if errVal, ok := reflection["error"]; ok && errVal != nil {
			// 	ch <- a.formatEvent("error", map[string]interface{}{
			// 		"message": fmt.Sprint(errVal),
			// 	})
			// 	return
			// }

			// // Handle should_continue
			// shouldContinue := false
			// if sc, ok := reflection["should_continue"].(bool); ok {
			// 	shouldContinue = sc
			// }
			// if !shouldContinue {
			// 	break
			// }

			// // Handle next_step (object)
			// nextStepObj, ok := reflection["next_step"].(map[string]interface{})
			// if ok && len(nextStepObj) > 0 {
			// 	stepSummary := fmt.Sprintf("%v (action: %v)", nextStepObj["step_id"], nextStepObj["action"])

			// 	if dp, ok := a.RoughPlan["decision_process_output"].(map[string]interface{}); ok {
			// 		if raw, ok := dp["mind_map_steps_in_natural_language"].([]interface{}); ok {
			// 			dp["mind_map_steps_in_natural_language"] = append(raw, stepSummary)
			// 		} else {
			// 			dp["mind_map_steps_in_natural_language"] = []interface{}{stepSummary}
			// 		}
			// 	} else {
			// 		a.RoughPlan["decision_process_output"] = map[string]interface{}{
			// 			"mind_map_steps_in_natural_language": []interface{}{stepSummary},
			// 		}
			// 	}

			// 	// Move to the newly added step
			// 	stepIndex = len(a.RoughPlan["decision_process_output"].(map[string]interface{})["mind_map_steps_in_natural_language"].([]interface{}))
			// }

			// // Recalculate mindSteps (because we may have appended)
			// // if dp, ok := a.RoughPlan["decision_process_output"].(map[string]interface{}); ok {
			// // 	if raw, ok := dp["mind_map_steps_in_natural_language"].([]interface{}); ok {
			// // 		mindSteps = raw
			// // 	}
			// // }
		}

		// Step 6: Summarize and generate final LLM response
		a.stepCh <- map[string]interface{}{"message": "Preparing summary"}
		respReq := a.buildResponseReq(map[string]interface{}{"steps": results}, query)

		respCh, err := a.LLM.RunStream(context.Background(), respReq)
		if err != nil {
			a.stepCh <- map[string]interface{}{"message": "LLM stream start failed", "error": err.Error()}
			ch <- a.formatEvent("error", map[string]interface{}{
				"message": "failed to stream response", "error": err.Error(),
			})
			return
		}

		for chunk := range respCh {
			a.responseCh <- chunk
			ch <- a.formatEvent("response_chunk", map[string]interface{}{"chunk": chunk})
		}

		ch <- a.formatEvent("completed", map[string]interface{}{
			"message": "Process completed successfully",
			"steps":   len(results),
		})
	}()

	return ch
}

func (a *BaseAgent) executePlan(plan map[string]interface{}) (results map[string]interface{}) {
	fmt.Println("executePlan.  ", plan)
	results = map[string]interface{}{
		"action_results": map[string]interface{}{},
	}

	step, ok := plan["next_step"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{"error": "invalid plan format: missing detailed_plan"}
	}

	// fetch identifiers and fields safely
	var stepID string = ""
	if v, ok := step["step_id"].(string); ok {
		stepID = v
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
		return
	}

	// Execute the action via dataActions
	a.stepCh <- map[string]interface{}{"message": "Executing step", "step_id": stepID, "action": actionName}
	out, err := a.dataActions.ExecuteAction(actionName, params)
	if err != nil {
		results["action_results"].(map[string]interface{})[stepID] = map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		}
		return
	}

	results["action_results"].(map[string]interface{})[stepID] = map[string]interface{}{
		"status": "ok",
		"output": out,
	}

	return results
}

func (a *BaseAgent) buildResponseReq(results map[string]interface{}, query string) llm.ChatRequest {
	// build a human-readable stages description from config
	var stagesDesc string
	for i, stage := range a.Config.DecisionProcess.Stages {
		stagesDesc += fmt.Sprintf("Stage %d: %s\n  Purpose: %s\n  Behavior: %s\n  Outputs: %v\n\n",
			i+1, stage.Name, stage.Purpose, stage.Behavior, "")
	}

	// Final system prompt: explicit, structured, include schemas and content
	systemPrompt := fmt.Sprintf(`
		You are 
		Agent identity:
		Agent Name: %s
		Agent Role: %s

		You will produce a clear, helpful final response to the user's query.  
		Use the provided execution results and context to generate:
		1) A concise summary of what was done and why (1-3 short points).
		2) Respond to user query.

		--- Context and artifacts (for your reference) ---

		User Query:
		%s


		Rough plan (the plan the agent created from the query):
		%s

		All detailed execution plans (detailed steps produced for execution):
		%s

		Execution results (what ran and observed outputs - use this as the definitive log of what happened):
		%s



		---

		Behavior requirements:
		- Be accurate and concise.
		- Highlight failures and partial results first.
		- For each failed or partial item, include a recommended remediation or a short verification step.
		- If user follow-up / clarification is required, clearly ask the questions.
		- If everything succeeded, state that the plan completed successfully and summarize the key outputs.

		Now produce the final high quality user-facing response using the above context.
		`,

		a.Config.AgentName,
		a.Config.AgentRole,
		query,
		jsonutils.ToJSON(a.RoughPlan),
		jsonutils.ToJSON(a.ExecutionPlans),
		jsonutils.ToJSON(results),
	)

	// The user-facing content will be sent as the "user" message; system prompt contains the context.
	userMessage := fmt.Sprintf("Please generate the final reply to the user for query: %s Output Format RICH TEXT properly structured", query)

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

// --- 2) small helper: formatEvent ---
// Put this method next to other BaseAgent methods.
func (a *BaseAgent) formatEvent(eventType string, payload interface{}) string {
	env := map[string]interface{}{
		"agent_name": a.Name,
		"session_id": a.SessionID,
		"type":       eventType,
		"payload":    payload,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	b, err := json.Marshal(env)
	if err != nil {
		// fallback minimal envelope
		return fmt.Sprintf(`{"agent_name":"%s","session_id":"%s","type":"%s","payload":"unserializable","timestamp":"%s"}`,
			a.Name, a.SessionID, eventType, time.Now().UTC().Format(time.RFC3339))
	}
	return string(b)
}
