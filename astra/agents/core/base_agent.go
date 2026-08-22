// astra/agents/core/base_agent.go
package core

import (
	"astra/astra/agents/actions"
	"astra/astra/agents/configs"
	"astra/astra/agents/prompts"
	"astra/astra/services/llm"
	"astra/astra/sources/psql/dao"
	colorutil "astra/astra/utils/color"
	"astra/astra/utils/jsonutils"
	"astra/astra/utils/logging"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	DefaultModel       = "gpt-5.6-luna"
	DefaultMaxTokens   = 10000
	DefaultTemp        = 0.1
	NumRecentSummaries = 3 // Number of recent session summaries to inject into context
)

type BaseAgent struct {
	Name           string
	TenantID       int
	UserID         int
	LLM            llm.LLMClient
	Model          string
	Config         *configs.AgentConfig
	ExecutionPlans []map[string]interface{}
	RoughPlan      map[string]interface{}
	SessionID      string
	LogInfo        map[string]interface{}
	dataActions    *actions.DataActions
	stepCh         chan map[string]interface{}
	responseCh     chan string
	mu             sync.Mutex
	chatDAO        *dao.ChatMessageDAO
	summaryDAO     *dao.SessionSummaryDAO
	DB             *gorm.DB
}

func NewBaseAgent(userID int, sessionID string, agentName string, db *gorm.DB) *BaseAgent {
	return NewBaseAgentWithModel(userID, sessionID, agentName, db, llm.DefaultProvider(), llm.DefaultModel(llm.DefaultProvider()))
}

// NewBaseAgentWithModel creates an agent with an explicit provider/model pair.
func NewBaseAgentWithModel(userID int, sessionID string, agentName string, db *gorm.DB, provider, model string) *BaseAgent {
	cfg := configs.LoadConfig()
	chatDAO := dao.NewChatMessageDAO(db)
	summaryDAO := dao.NewSessionSummaryDAO(db)

	agent := &BaseAgent{
		Name:        agentName,
		TenantID:    userID,
		UserID:      userID,
		LLM:         llm.NewClient(provider),
		Model:       model,
		Config:      cfg,
		SessionID:   sessionID,
		LogInfo:     map[string]interface{}{"tenant_id": userID, "user_id": userID, "session_id": sessionID},
		stepCh:      make(chan map[string]interface{}, 10),
		responseCh:  make(chan string, 10),
		dataActions: actions.NewDataActionsForSession(db, userID, sessionID),
		chatDAO:     chatDAO,
		summaryDAO:  summaryDAO,
		DB:          db,
	}
	logging.AppLogger.Info("BaseAgent initialized",
		zap.Int("user_id", userID),
		zap.String("agent_name", agentName), zap.String("provider", provider), zap.String("model", model),
	)
	go agent.handleEvents()
	return agent
}

// handleEvents now includes colorized output for direct agent prints (step and response)
func (a *BaseAgent) handleEvents() {
	for {
		select {
		case step := <-a.stepCh:
			if msg, ok := step["message"].(string); ok {
				fmt.Println(colorutil.ColorInfo("[Astra Step] " + msg))
			}
			logging.AppLogger.Info("Step update", zap.Any("step", step))
		case resp := <-a.responseCh:
			fmt.Print(colorutil.ColorAgentResponse(resp))
		}
	}
}

// --- SESSION SUMMARY + RECENT SUMMARIES LOGIC ---
// Generates a short, structured summary given query, roughPlan, execPlans, and results.
func (a *BaseAgent) GenerateSessionSummary(query string, roughPlan interface{}, execPlans interface{}, results interface{}) string {
	// Compose a brief summary: request + top-level actions + outcome.
	// Reduce everything to 2-3 sentences.

	// Extract primary actions (if possible) from the rough plan
	actions := ""
	if rp, ok := roughPlan.(map[string]interface{}); ok {
		if steps, ok := rp["mind_map_steps_in_natural_language"].([]interface{}); ok {
			strs := make([]string, 0)
			for _, s := range steps {
				if sstr, ok := s.(string); ok {
					strs = append(strs, sstr)
				}
			}
			if len(strs) > 0 {
				actions = strings.Join(strs, "; ")
			}
		}
	}
	// Try to identify whether chat execution succeeded or failed
	outcome := "Success"
	if resSlice, ok := results.([]map[string]interface{}); ok && len(resSlice) > 0 {
		for _, step := range resSlice {
			if r, ok := step["result"].(map[string]interface{}); ok {
				if ar, ok := r["action_results"].(map[string]interface{}); ok {
					for _, v := range ar {
						if entry, ok := v.(map[string]interface{}); ok {
							if status, ok := entry["status"].(string); ok && status != "ok" {
								outcome = "Partial or failed: " + status
							}
						}
					}
				}
			}
		}
	}
	if actions == "" {
		actions = "No actions planned"
	}
	content := fmt.Sprintf("Request: %s\nActions: %s\nOutcome: %s", query, actions, outcome)
	return content
}

// Fetch N most recent session summaries for this user.
func (a *BaseAgent) GetRecentSessionSummaries(n int) ([]string, error) {
	ctx := context.Background()
	summaries, err := a.summaryDAO.ListRecentSessionSummaries(ctx, a.UserID, n)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0)
	for _, ss := range summaries {
		if ss.Summary != "" {
			result = append(result, fmt.Sprintf("Session (%s): %s", ss.SessionID, ss.Summary))
		}
	}
	return result, nil
}

// --- PLANNING/PROMPT GENERATION ---
func (a *BaseAgent) createRoughPlan(query string) (plan map[string]interface{}) {
	defer func() {
		if r := recover(); r != nil {
			logging.ErrorLogger.Error("Planning failure", zap.Any("recover", r))
			plan = map[string]interface{}{"error": fmt.Sprint(r)}
		}
	}()

	// Get recent summaries & inject into context
	recentSummaries := "No prior session summaries."
	// if summaries, err := a.GetRecentSessionSummaries(NumRecentSummaries); err == nil && len(summaries) > 0 {
	// 	recentSummaries = strings.Join(summaries, "\n-----\n")
	// }

	// Get lightweight action summaries (name + description) from runtime registry
	actionSummaries := a.dataActions.ListActions()

	// Build the system prompt (inject recent summaries + action summaries)
	systemPrompt := fmt.Sprintf(`
		You are a **planning assistant** 
		responsible for analyzing user queries 
		and determining the appropriate actions to use across stages

		## Context
		**Agent Name:** %s  
		**Agent Role:** %s  
		**Recent Summaries (last %d):**
		%s

		**Chat history** %s

		**Available Actions (full description with usage instruction):** 
		<available_actions_with_full_description>
		%s
		</available_actions_with_full_description>

		## Decision Process
			**Description:**  
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
			- DO NOT include natural language or markdown fences.
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
		NumRecentSummaries,
		recentSummaries,
		jsonutils.ToJSON(a.getHistory()),
		prompts.ActionCatalog(actionSummaries),
		a.Config.DecisionProcess.Description,
		prompts.PlanSchema,
		query,
	)
	systemPrompt = prompts.PlanningSystem(
		a.Config.AgentName,
		a.Config.AgentRole,
		jsonutils.ToJSON(a.getHistory()),
		prompts.ActionCatalog(actionSummaries),
		prompts.PlanSchema,
	)

	currentDateStr := time.Now().Format("January 2, 2006")
	datePreamble := fmt.Sprintf("Today's date is: %s.\n\n", currentDateStr)

	user_message := datePreamble + fmt.Sprintf(`
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
		prompts.PlanSchema,
	)

	req := llm.ChatRequest{
		Model: a.Model,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: user_message},
		},
		Stream: false,
	}

	resp, err := a.LLM.Run(context.Background(), req)
	// fmt.Println("\nreateRoughPlan plan created --- ", resp)
	if err != nil {
		panic(fmt.Errorf("failed to create plan: %w", err))
	}

	respJSON := jsonutils.ExtractJSON(resp)
	// 🔥 ADD THIS
	// if !utf8.ValidString(respJSON) {
	// 	respJSON = strings.ToValidUTF8(respJSON, "")
	// }
	if err := json.Unmarshal([]byte(respJSON), &plan); err != nil {
		panic(fmt.Errorf("invalid plan format: %w", err))
	}
	a.RoughPlan = plan
	return plan
}

// generateNextExecutionPlan and other methods remain unchanged
func (a *BaseAgent) generateNextExecutionPlan(roughPlan map[string]interface{}, stepIndex int, results any) (plan map[string]interface{}) {
	defer func() {
		if r := recover(); r != nil {
			logging.ErrorLogger.Error("generateNextExecutionPlan failure", zap.Any("recover", r))
			plan = map[string]interface{}{"error": fmt.Sprint(r)}
		}
	}()

	fullActions := a.dataActions.ListActions()

	systemPrompt := fmt.Sprintf(`
		You are Astra’s  sequential execution Planner.

		Context:
		- Full mind map plan: %s
		- Previous execution results: %s
		- Decision Process Description: %s
		**Available Actions (full description with usage instruction):**
		<available_actions_with_full_description>
		%s
		</available_actions_with_full_description>

		Task:
		You are provided with a full mind map of responding to user query.
		And you are provided with all actions that you can take and all previous execution determined by you and their results.

		Think properly and present only the next single concrete execution plan (single JSON object).

		Rules:
		- Output exactly one JSON object and nothing else.
		- If no concrete action is required, set "action" to an empty string and return the schema.

		## Output Schema (stick to this)
		%s
		`,
		jsonutils.ToJSON(roughPlan),
		jsonutils.ToJSON(results),
		a.Config.DecisionProcess.Description,
		prompts.ActionCatalog(fullActions),
		prompts.ExecutionSchema,
	)
	systemPrompt = prompts.ExecutionSystem(
		jsonutils.ToJSON(roughPlan),
		jsonutils.ToJSON(results),
		prompts.ActionCatalog(fullActions),
		prompts.ExecutionSchema,
	)

	currentDateStr := time.Now().Format("January 2, 2006")
	datePreamble := fmt.Sprintf("Today's date is: %s.\n\n", currentDateStr)

	userPrompt := datePreamble + fmt.Sprintf(`
		Please analyze and create a good thoughtful 
		execution plan and output a single object
		Please stick to the json output format and include all output in the JSON

		****important*****
		- Respond ONLY with valid JSON only stick to this format: %s
		- Any text outside the JSON is considered an error.
		- Dont keep repeating any action - be sensible, you are not some small time rookie, you are supposed to my JARVIS
		`,
		prompts.ExecutionSchema,
	)

	req := llm.ChatRequest{
		Model: a.Model,
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

	// fmt.Println("\n exec plan created --- ", resp)

	respJSON := jsonutils.ExtractJSON(resp)
	fmt.Println("\n exec plan created extracted json --- ", respJSON)
	if err := json.Unmarshal([]byte(respJSON), &plan); err != nil {
		panic(fmt.Errorf("invalid plan format: %w", err))
	}
	a.ExecutionPlans = append(a.ExecutionPlans, plan)
	return plan
}

func (a *BaseAgent) ProcessQuery(query string) <-chan string {
	ch := make(chan string)
	a.storeState("user_query", query)
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
		const maxExecutionSteps = 12
		for stepIndex <= maxExecutionSteps {
			a.stepCh <- map[string]interface{}{"message": "Planning step", "step_index": stepIndex}
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
			var planToExec map[string]interface{} = expanded
			step, ok := expanded["next_step"].(map[string]interface{})
			if !ok {
				ch <- a.formatEvent("error", map[string]interface{}{"message": "invalid execution plan: next_step is required"})
				return
			}
			actionName, _ := step["action"].(string)
			fmt.Println("action name .. ", actionName)
			if actionName == "" {
				fmt.Println("breaking no action name")
				break
			}
			ch <- a.formatEvent("intermediate", map[string]interface{}{
				"phase": "executing_step", "index": stepIndex,
			})
			a.stepCh <- map[string]interface{}{"message": "Executing expanded step", "step_index": stepIndex}
			if actionName == "think_aloud_reasoning" {
				var params map[string]interface{}
				if p, ok := step["action_params"].(map[string]interface{}); ok {
					params = p
				} else {
					params = map[string]interface{}{}
					if step["action_params"] != nil {
						bytes, _ := json.Marshal(step["action_params"])
						_ = json.Unmarshal(bytes, &params)
					}
				}
				contextInfo, _ := params["context"].(string)
				goal, _ := params["goal"].(string)
				goal += " Ensure the upcoming action is safe, meaningful, and consistent. Identify what will change and why."
				finalThought := a.thinkAloud(map[string]interface{}{"steps": results}, contextInfo, goal)
				results = append(results, map[string]interface{}{
					"step_index":    stepIndex,
					"executed_plan": planToExec,
					"result":        finalThought,
				})
				continue
			}
			if actionName == "read_image_with_vision" {
				var params actions.ReadImageWithVisionParams

				// decode params safely
				if p, ok := step["action_params"].(map[string]interface{}); ok {
					bytes, _ := json.Marshal(p)
					_ = json.Unmarshal(bytes, &params)
				}

				a.stepCh <- map[string]interface{}{
					"message": "Reading image(s) with vision",
					"count":   len(params.ImagePaths),
				}

				visionResults := a.readImageWithVision(params)

				results = append(results, map[string]interface{}{
					"step_index":    stepIndex,
					"executed_plan": planToExec,
					"result": map[string]interface{}{
						"vision_results": visionResults,
					},
				})

				continue
			}
			// fmt.Println("executing plan ... ")
			execRes := a.executePlan(planToExec)
			// fmt.Println("executed plan ... ")
			ch <- a.formatEvent("intermediate", map[string]interface{}{
				"phase":   "executed_step",
				"index":   stepIndex,
				"execRes": execRes,
			})
			results = append(results, map[string]interface{}{
				"step_index":    stepIndex,
				"executed_plan": planToExec,
				"result":        execRes,
			})
			stepIndex++
		}
		if stepIndex > maxExecutionSteps {
			ch <- a.formatEvent("error", map[string]interface{}{"message": "execution stopped after reaching the safety limit"})
		}
		fullPlan := map[string]interface{}{
			"rough_plan":      a.RoughPlan,
			"execution_plans": a.ExecutionPlans,
		}
		a.storeState("full_plan", fullPlan)
		// generate  LLM response
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
		resp := ""
		var utf8Buf string

		for chunk := range respCh {
			resp += chunk
			utf8Buf += chunk

			for utf8.ValidString(utf8Buf) {
				last := len(utf8Buf)
				for last > 0 && !utf8.ValidString(utf8Buf[:last]) {
					last--
				}
				if last == 0 {
					break
				}

				safe := utf8Buf[:last]
				utf8Buf = utf8Buf[last:]

				// console (optional)
				a.responseCh <- safe

				// websocket
				ch <- a.formatEvent("response_chunk", map[string]interface{}{
					"chunk": safe,
				})
			}
		}
		a.storeState("response", resp)
		// --- SESSION SUMMARY PERSISTENCE ---
		// a.stepCh <- map[string]interface{}{"message": "Generating and persisting session summary"}
		// summaryText := a.GenerateSessionSummary(query, roughPlan, a.ExecutionPlans, results)
		// ctx := context.Background()
		// _, err = a.summaryDAO.UpsertSessionSummary(ctx, a.SessionID, a.UserID, summaryText)
		// if err != nil {
		// 	ch <- a.formatEvent("error", map[string]interface{}{
		// 		"message": fmt.Sprintf("Failed to upsert session summary: %v", err),
		// 	})
		// }
		ch <- a.formatEvent("completed", map[string]interface{}{
			"message": "Process completed successfully",
			"steps":   len(results),
		})
	}()
	return ch
}

func (a *BaseAgent) executePlan(plan map[string]interface{}) (results map[string]interface{}) {
	results = map[string]interface{}{
		"action_results": map[string]interface{}{},
	}
	step, ok := plan["next_step"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{"error": "invalid plan format: missing detailed_plan"}
	}
	var stepID string = ""
	if v, ok := step["step_id"].(string); ok {
		stepID = v
	}
	actionName, _ := step["action"].(string)
	var params map[string]interface{}
	if p, ok := step["action_params"].(map[string]interface{}); ok {
		params = p
	} else {
		params = map[string]interface{}{}
		if step["action_params"] != nil {
			bytes, _ := json.Marshal(step["action_params"])
			_ = json.Unmarshal(bytes, &params)
		}
	}
	if actionName == "" {
		results["action_results"].(map[string]interface{})[stepID] = map[string]interface{}{
			"status": "skipped", "note": "no action specified",
		}
		return
	}
	a.stepCh <- map[string]interface{}{"message": "Executing step", "step_id": stepID, "action": actionName}
	fmt.Println("executePlan ", actionName, params)
	out, err := a.dataActions.ExecuteAction(actionName, params)
	fmt.Println("a.dataActions.ExecuteAction(actionName, params)", out, err)
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

	// Final system prompt: explicit, structured, include schemas and content
	systemPrompt := fmt.Sprintf(`
		You are 
		Agent identity:
		Agent Name: %s
		Agent Role: %s

		Ongoing Conv: %s

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
		jsonutils.ToJSON(a.getHistory()),
		query,
		jsonutils.ToJSON(a.RoughPlan),
		jsonutils.ToJSON(a.ExecutionPlans),
		jsonutils.ToJSON(results),
	)
	systemPrompt = prompts.ResponseSystem(query, jsonutils.ToJSON(results))
	userMessage := fmt.Sprintf("Please generate the final reply to the user for query: %s Output Format RICH TEXT properly structured", query)
	return llm.ChatRequest{
		Model: a.Model,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
		Stream: true,
	}
}

func (a *BaseAgent) storeState(key string, value interface{}) {
	a.dataActions.RecordSessionEvent(key, value)
	ctx := context.Background()
	contentBytes, err := json.Marshal(value)
	if err != nil {
		logging.ErrorLogger.Error("Failed to marshal state value", zap.String("key", key), zap.Error(err))
		return
	}
	content := string(contentBytes)
	_, err = a.chatDAO.SaveMessage(ctx, a.SessionID, a.UserID, key, content)
	if err != nil {
		logging.ErrorLogger.Error("Failed to save message", zap.String("key", key), zap.Error(err))
	}
}

func (a *BaseAgent) getHistory() []map[string]string {
	ctx := context.Background()
	history, err := a.chatDAO.GetChatHistoryBySession(ctx, a.SessionID)
	if err != nil {
		return []map[string]string{}
	}
	return history
}

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
		return fmt.Sprintf(`{"agent_name":"%s","session_id":"%s","type":"%s","payload":"unserializable","timestamp":"%s"}`,
			a.Name, a.SessionID, eventType, time.Now().UTC().Format(time.RFC3339))
	}
	return string(b)
}

func (a *BaseAgent) thinkAloud(results map[string]interface{}, contextInfo, goal string) string {
	a.stepCh <- map[string]interface{}{
		"message": "Starting internal thought process",
		"context": contextInfo,
		"goal":    goal,
	}
	systemPrompt := fmt.Sprintf(`
        You are Astra's internal reasoning module.
        Before taking a real-world action, you think carefully about what might happen, how to do this action.
        Your goal is to reason step-by-step, stream your thought process,
        and finally summarize your decision in one paragraph.
        Context: %s
        Goal: %s
		Thoughtful Mind map - %s
		Execution on that mind map with results - %s
        Behavior:
        - Think out loud.
        - Stream thoughts one by one.
        - Conclude with "FINAL THOUGHT:" followed by your summary.
        - Do not produce JSON, just human-readable reasoning.
    `,
		contextInfo,
		goal,
		jsonutils.ToJSON(a.RoughPlan),
		jsonutils.ToJSON(results),
	)
	currentDateStr := time.Now().Format("January 2, 2006")
	datePreamble := fmt.Sprintf("Today's date is: %s.\n\n", currentDateStr)
	userPrompt := datePreamble + " Begin your internal reasoning stream now and if doing code edits. reason clearly what edit , where etc"
	req := llm.ChatRequest{
		Model: a.Model,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream: true,
	}
	respCh, err := a.LLM.RunStream(context.Background(), req)
	if err != nil {
		a.stepCh <- map[string]interface{}{"message": "thinking stream failed", "error": err.Error()}
		return "thinking failed"
	}
	finalThought := ""
	for chunk := range respCh {
		a.responseCh <- chunk
		finalThought += chunk
	}
	a.stepCh <- map[string]interface{}{
		"message":       "Finished thinking",
		"final_thought": finalThought,
	}
	return finalThought
}

func (a *BaseAgent) readImageWithVision(
	params actions.ReadImageWithVisionParams,
) []actions.VisionImageResult {

	results := make([]actions.VisionImageResult, 0, len(params.ImagePaths))

	for _, imagePath := range params.ImagePaths {
		res := actions.VisionImageResult{
			ImagePath:  imagePath,
			VisionUsed: true,
		}

		data, err := os.ReadFile(imagePath)
		if err != nil {
			res.Error = err.Error()
			results = append(results, res)
			continue
		}

		imageB64 := base64.StdEncoding.EncodeToString(data)

		fmt.Println("Image bytes:", len(data))

		systemPrompt := `
			You are a visual perception assistant.

			Your task is to DESCRIBE what is visible in the image.
			Do NOT infer intent, function, or business meaning.
			Do NOT guess labels, components, or relationships
			unless they are explicitly visible.

			Allowed:
			- Describe shapes, boxes, arrows, lines
			- Transcribe visible text exactly as shown
			- Describe spatial relationships (above, connected to, grouped)
			- Describe flow direction if arrows exist
			- State uncertainty clearly

			Disallowed:
			- Interpretation or analysis
			- Optimization suggestions
			- Assumptions about architecture or domain
			- Conclusions or recommendations

			Output rules:
			- Plain text only
			- Bullet points allowed
			- No markdown
			- No speculation
		`

		userInstruction := params.UserInstruction
		if userInstruction == "" {
			userInstruction = "Describe what you see in this image."
		}
		// userContent := []map[string]interface{}{
		// 	{
		// 		"type": "text",
		// 		"text": userInstruction,
		// 	},
		// 	{
		// 		"type": "image_url",
		// 		"image_url": map[string]string{
		// 			"url": "data:image/png;base64," + imageB64,
		// 		},
		// 	},
		// }
		// userContentBytes, _ := json.Marshal(userContent)

		req := llm.ChatRequest{
			Model: a.Model,
			Messages: []llm.Message{
				{
					Role:    "system",
					Content: systemPrompt,
				},
				{
					Role: "user",
					Content: []map[string]interface{}{
						{
							"type": "text",
							"text": userInstruction,
						},
						{
							"type": "image_url",
							"image_url": map[string]string{
								"url": "data:image/jpeg;base64," + imageB64,
							},
						},
					},
				},
			},
			Stream: false,
		}

		resp, err := a.LLM.Run(context.Background(), req)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.VisionDescription = resp
		}

		fmt.Println("resp vison read .. ", resp)

		results = append(results, res)
	}

	return results
}
