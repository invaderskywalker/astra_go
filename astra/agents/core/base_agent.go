// astra/agents/core/base_agent.go
package core

import (
	"astra/astra/agents/actions"
	"astra/astra/agents/prompts"
	"astra/astra/agents/workspace"
	"astra/astra/services/llm"
	"astra/astra/sources/psql/dao"
	"astra/astra/utils/jsonutils"
	"astra/astra/utils/logging"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sort"
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
	Name             string
	TenantID         int
	UserID           int
	LLM              llm.LLMClient
	Model            string
	ExecutionPlans   []map[string]interface{}
	RoughPlan        map[string]interface{}
	SessionID        string
	WorkspaceRoot    string
	LogInfo          map[string]interface{}
	dataActions      *actions.DataActions
	activatedActions map[string]bool
	stepCh           chan map[string]interface{}
	responseCh       chan string
	queryQueue       chan queuedQuery
	mu               sync.Mutex
	controlMu        sync.Mutex
	controlCond      *sync.Cond
	paused           bool
	running          bool
	cancelCurrent    context.CancelFunc
	chatDAO          *dao.ChatMessageDAO
	summaryDAO       *dao.SessionSummaryDAO
	DB               *gorm.DB
}

type queuedQuery struct {
	query string
	out   chan string
}

func NewBaseAgent(userID int, sessionID string, agentName string, db *gorm.DB) *BaseAgent {
	return NewBaseAgentWithModel(userID, sessionID, agentName, db, llm.DefaultProvider(), llm.DefaultModel(llm.DefaultProvider()))
}

// NewBaseAgentWithModel creates an agent with an explicit provider/model pair.
func NewBaseAgentWithModel(userID int, sessionID string, agentName string, db *gorm.DB, provider, model string) *BaseAgent {
	return NewBaseAgentWithWorkspace(userID, sessionID, agentName, db, provider, model, "")
}

// NewBaseAgentWithWorkspace creates an agent whose model context and local
// actions share one canonical workspace root.
func NewBaseAgentWithWorkspace(userID int, sessionID string, agentName string, db *gorm.DB, provider, model, workspaceRoot string) *BaseAgent {
	ws, err := workspace.NewWorkspace(workspaceRoot)
	if err != nil {
		panic(fmt.Sprintf("initialize workspace: %v", err))
	}
	chatDAO := dao.NewChatMessageDAO(db)
	summaryDAO := dao.NewSessionSummaryDAO(db)

	agent := &BaseAgent{
		Name:             agentName,
		TenantID:         userID,
		UserID:           userID,
		LLM:              llm.NewClient(provider),
		Model:            model,
		SessionID:        sessionID,
		WorkspaceRoot:    ws.Root,
		LogInfo:          map[string]interface{}{"tenant_id": userID, "user_id": userID, "session_id": sessionID},
		stepCh:           make(chan map[string]interface{}, 10),
		responseCh:       make(chan string, 10),
		queryQueue:       make(chan queuedQuery, 32),
		dataActions:      actions.NewDataActionsForSessionAt(db, userID, sessionID, ws.Root),
		activatedActions: make(map[string]bool),
		chatDAO:          chatDAO,
		summaryDAO:       summaryDAO,
		DB:               db,
	}
	agent.controlCond = sync.NewCond(&agent.controlMu)
	logging.AppLogger.Info("BaseAgent initialized",
		zap.Int("user_id", userID),
		zap.String("agent_name", agentName), zap.String("provider", provider), zap.String("model", model),
	)
	go agent.handleEvents()
	go agent.processQueue()
	return agent
}

// Pause requests cooperative suspension at the next safe checkpoint. An
// in-flight HTTP/tool call is allowed to finish without being interrupted in
// the middle of a write.
func (a *BaseAgent) Pause() {
	a.controlMu.Lock()
	a.paused = true
	a.controlMu.Unlock()
}

func (a *BaseAgent) Resume() {
	a.controlMu.Lock()
	a.paused = false
	a.controlCond.Broadcast()
	a.controlMu.Unlock()
}

// Stop cancels the current request's context. Providers and context-aware
// calls can return immediately; non-context-aware actions finish their current
// atomic operation and stop before the next checkpoint.
func (a *BaseAgent) Stop() bool {
	a.controlMu.Lock()
	cancel := a.cancelCurrent
	a.controlMu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	a.controlMu.Lock()
	a.controlCond.Broadcast()
	a.controlMu.Unlock()
	return true
}

func (a *BaseAgent) beginRun() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	a.controlMu.Lock()
	a.running = true
	a.cancelCurrent = cancel
	a.controlMu.Unlock()
	return ctx, cancel
}

func (a *BaseAgent) endRun() {
	a.controlMu.Lock()
	a.running = false
	a.cancelCurrent = nil
	a.controlCond.Broadcast()
	a.controlMu.Unlock()
}

func (a *BaseAgent) checkpoint(ctx context.Context, ch chan<- string) bool {
	a.controlMu.Lock()
	announced := false
	for a.paused && ctx.Err() == nil {
		if !announced {
			select {
			case ch <- a.formatEvent("paused", map[string]interface{}{"message": "Astra is paused. Use :resume to continue."}):
			default:
			}
			announced = true
		}
		a.controlCond.Wait()
	}
	stopped := ctx.Err() != nil
	a.controlMu.Unlock()
	return !stopped
}

// SetModel switches the model for future queued requests. Callers should only
// invoke this when no request is currently running; the interactive CLI enforces
// that rule and reports the active queue state to the user.
func (a *BaseAgent) SetModel(provider, model string) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	model = strings.TrimSpace(model)
	if provider != "ollama" && provider != "openai" && provider != "gpt" {
		return fmt.Errorf("unsupported provider %q", provider)
	}
	if model == "" {
		return fmt.Errorf("model is required")
	}
	a.LLM, a.Model = llm.NewClient(provider), model
	return nil
}

func (a *BaseAgent) processQueue() {
	for request := range a.queryQueue {
		a.runQuery(request.query, request.out)
		close(request.out)
	}
}

// handleEvents now includes colorized output for direct agent prints (step and response)
func (a *BaseAgent) handleEvents() {
	for {
		select {
		case step := <-a.stepCh:
			// Step events are internal telemetry. The request owner renders
			// user-facing progress so it cannot interleave with the prompt.
			logging.AppLogger.Info("Step update", zap.Any("step", step))
		case <-a.responseCh:
			// Response chunks are rendered by the caller that owns the stream.
			// Keep this channel drained for internal reasoning streams.
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
func (a *BaseAgent) createRoughPlan(ctx context.Context, query string) (plan map[string]interface{}) {
	defer func() {
		if r := recover(); r != nil {
			logging.ErrorLogger.Error("Planning failure", zap.Any("recover", r))
			plan = map[string]interface{}{"error": fmt.Sprint(r)}
		}
	}()

	// Get lightweight action summaries (name + description) from runtime registry
	actionSummaries := a.dataActions.ListActions()
	memoryContext, memoryErr := a.dataActions.MemoryContext(query, 6)
	if memoryErr != nil {
		logging.AppLogger.Warn("memory retrieval unavailable; continuing without durable context", zap.Error(memoryErr))
		memoryContext = "Memory retrieval failed; rely on current workspace evidence."
	}

	// Prompts are assembled only in the prompts package.
	systemPrompt := prompts.PlanningSystem(
		prompts.DefaultProfile.Name,
		prompts.DefaultProfile.Role,
		jsonutils.ToJSON(a.getHistory()),
		memoryContext,
		prompts.ActionCatalog(actionSummaries),
		prompts.PlanSchema,
		a.WorkspaceRoot,
	)

	currentDateStr := time.Now().Format("January 2, 2006")
	datePreamble := fmt.Sprintf("Today's date is: %s.\n\n", currentDateStr)

	user_message := datePreamble + prompts.PlanningUser(query)

	req := llm.ChatRequest{
		Model: a.Model,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: user_message},
		},
		Stream: false,
	}

	resp, err := a.LLM.Run(ctx, req)
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
func (a *BaseAgent) generateNextExecutionPlan(ctx context.Context, roughPlan map[string]interface{}, stepIndex int, results any) (plan map[string]interface{}) {
	defer func() {
		if r := recover(); r != nil {
			logging.ErrorLogger.Error("generateNextExecutionPlan failure", zap.Any("recover", r))
			plan = map[string]interface{}{"error": fmt.Sprint(r)}
		}
	}()

	fullActions := a.dataActions.ListActions()
	actionCatalog := prompts.ActionCatalog(fullActions)
	if docs := a.activatedActionDocs(); len(docs) > 0 {
		actionCatalog += "\n\n<activated_action_docs>\n" + prompts.ActivatedActionDocumentation(docs) + "\n</activated_action_docs>"
	}

	systemPrompt := prompts.ExecutionSystem(
		jsonutils.ToJSON(roughPlan),
		jsonutils.ToJSON(results),
		actionCatalog,
		prompts.ExecutionSchema,
		a.WorkspaceRoot,
	)

	currentDateStr := time.Now().Format("January 2, 2006")
	datePreamble := fmt.Sprintf("Today's date is: %s.\n\n", currentDateStr)

	userPrompt := datePreamble + prompts.ExecutionUser()

	req := llm.ChatRequest{
		Model: a.Model,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream: false,
	}

	resp, err := a.LLM.Run(ctx, req)
	if err != nil {
		panic(fmt.Errorf("failed to create plan: %w", err))
	}

	respJSON := jsonutils.ExtractJSON(resp)
	if err := json.Unmarshal([]byte(respJSON), &plan); err != nil {
		panic(fmt.Errorf("invalid plan format: %w", err))
	}
	a.ExecutionPlans = append(a.ExecutionPlans, plan)
	return plan
}

// ProcessQuery queues work and returns immediately. A single worker preserves
// agent state ordering while allowing the CLI/UI to keep accepting input.
func (a *BaseAgent) ProcessQuery(query string) <-chan string {
	ch := make(chan string, 32)
	select {
	case a.queryQueue <- queuedQuery{query: query, out: ch}:
	case <-time.After(250 * time.Millisecond):
		ch <- a.formatEvent("error", map[string]interface{}{"message": "Astra's request queue is full; try again shortly."})
		close(ch)
	}
	return ch
}

// ClearPending discards requests that have not reached the single worker yet.
// Call Stop first when the active request should also be cancelled.
func (a *BaseAgent) ClearPending() int {
	cleared := 0
	for {
		select {
		case request := <-a.queryQueue:
			close(request.out)
			cleared++
		default:
			return cleared
		}
	}
}

func (a *BaseAgent) runQuery(query string, ch chan string) {
	ctx, cancel := a.beginRun()
	defer cancel()
	defer a.endRun()
	if !a.checkpoint(ctx, ch) {
		a.stopped(ch)
		return
	}
	// Execution plans are per request. Keeping prior plans here makes later
	// requests look as if they already performed actions they never performed.
	a.ExecutionPlans = nil
	a.activatedActions = make(map[string]bool)
	a.storeState("user_query", query)
	// Step 1: Create the rough plan
	a.status(ch, "Understanding your request")
	roughPlan := a.createRoughPlan(ctx, query)
	if ctx.Err() != nil {
		a.stopped(ch)
		return
	}
	if roughPlan["error"] != nil {
		ch <- a.formatEvent("error", map[string]interface{}{
			"message": fmt.Sprint(roughPlan["error"]),
		})
		return
	}
	a.RoughPlan = roughPlan
	ch <- a.formatEvent("plan", roughPlan)
	if mode, _ := roughPlan["mode"].(string); mode == "conversation" {
		a.status(ch, "Preparing answer")
		a.streamResponse(ctx, query, map[string]interface{}{"steps": []map[string]interface{}{}}, ch)
		return
	}
	a.status(ch, "Plan ready")
	results := []map[string]interface{}{}
	stepIndex := 1
	// Keep requests bounded while leaving enough room for real workflows that
	// orient, scaffold, validate, and document a project.
	const maxExecutionSteps = 48
	for stepIndex <= maxExecutionSteps {
		if !a.checkpoint(ctx, ch) {
			a.stopped(ch)
			return
		}
		a.status(ch, fmt.Sprintf("Planning next action (%d)", stepIndex))
		expanded := a.generateNextExecutionPlan(ctx, a.RoughPlan, stepIndex, results)
		if ctx.Err() != nil {
			a.stopped(ch)
			return
		}
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
		var planToExec map[string]interface{} = expanded
		step, ok := expanded["next_step"].(map[string]interface{})
		if !ok {
			ch <- a.formatEvent("error", map[string]interface{}{"message": "invalid execution plan: next_step is required"})
			return
		}
		actionName, _ := step["action"].(string)
		if actionName == "" {
			ch <- a.formatEvent("error", map[string]interface{}{"message": "The planner returned no action."})
			return
		}
		if !a.checkpoint(ctx, ch) {
			a.stopped(ch)
			return
		}
		displayStep := cloneActionStep(step)
		ch <- a.formatEvent("action_plan", map[string]interface{}{
			"index":          stepIndex,
			"workspace_root": a.WorkspaceRoot,
			"step":           displayStep,
		})
		a.status(ch, fmt.Sprintf("Running %s", actionName))
		// The model is instructed to activate documentation explicitly. If it
		// skips that step, hydrate the contract automatically so a malformed
		// bookmark-only call cannot cause an avoidable failure. The activation is
		// recorded in the event stream and the docs are included on the next turn.
		if actionName != "activate_actions" && actionName != "think_aloud_reasoning" && actionName != "read_image_with_vision" {
			if a.ensureActionActivated(actionName, ch) == false {
				return
			}
		}
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
			finalThought := a.thinkAloud(ctx, map[string]interface{}{"steps": results}, contextInfo, goal)
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

			a.status(ch, "Reading image(s)")

			visionResults := a.readImageWithVision(ctx, params)

			results = append(results, map[string]interface{}{
				"step_index":    stepIndex,
				"executed_plan": planToExec,
				"result": map[string]interface{}{
					"vision_results": visionResults,
				},
			})

			continue
		}
		// A clarification is a terminal state for this request. Do not feed it
		// back into the planner: that creates an ask-again forever loop.
		if actionName == "ask_follow_up_questions" {
			execRes := a.executePlan(planToExec)
			questions := followUpQuestions(execRes)
			a.storeState("pending_questions", questions)
			results = append(results, map[string]interface{}{
				"step_index":    stepIndex,
				"executed_plan": planToExec,
				"result":        execRes,
			})
			ch <- a.formatEvent("needs_input", map[string]interface{}{
				"message":   "I need one clarification before I continue.",
				"questions": questions,
			})
			return
		}
		execRes := a.executePlan(planToExec)
		actionSummary := summarizeExecutionResult(execRes)
		stepID, _ := step["step_id"].(string)
		a.dataActions.RecordSessionEvent("action_execution", map[string]interface{}{
			"step_index": stepIndex,
			"action":     actionName,
			"step_id":    stepID,
			"params":     sanitizeActionParams(actionParamsFromStep(step)),
			"result":     actionSummary,
		})
		ch <- a.formatEvent("action_result", map[string]interface{}{
			"index":  stepIndex,
			"action": actionName,
			"result": actionSummary,
		})
		results = append(results, map[string]interface{}{
			"step_index":    stepIndex,
			"executed_plan": planToExec,
			"result":        execRes,
		})
		stepIndex++
	}
	if stepIndex > maxExecutionSteps {
		message := fmt.Sprintf("execution stopped after the %d-action safety limit; no completion claim was made", maxExecutionSteps)
		a.storeState("execution_blocked", map[string]interface{}{"reason": message, "steps": len(a.ExecutionPlans)})
		ch <- a.formatEvent("error", map[string]interface{}{"message": message, "next": "Ask Astra to continue from the recorded evidence."})
		return
	}
	fullPlan := map[string]interface{}{
		"rough_plan":      a.RoughPlan,
		"execution_plans": a.ExecutionPlans,
	}
	a.storeState("full_plan", fullPlan)
	// Generate the user-facing response only after execution evidence is ready.
	a.status(ch, "Preparing answer")
	a.streamResponse(ctx, query, map[string]interface{}{"steps": results}, ch)
}

func (a *BaseAgent) streamResponse(ctx context.Context, query string, results map[string]interface{}, ch chan<- string) {
	respReq := a.buildResponseReq(results, query)
	respCh, err := a.LLM.RunStream(ctx, respReq)
	if err != nil {
		ch <- a.formatEvent("error", map[string]interface{}{
			"message": "failed to stream response", "error": err.Error(),
		})
		return
	}
	resp := ""
	var utf8Buf string

	for chunk := range respCh {
		if !a.checkpoint(ctx, ch) {
			a.stopped(ch)
			return
		}
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
	ch <- a.formatEvent("completed", map[string]interface{}{
		"message": "Process completed successfully",
		"steps":   len(a.ExecutionPlans),
	})
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
	out, err := a.dataActions.ExecuteAction(actionName, params)
	if err != nil {
		results["action_results"].(map[string]interface{})[stepID] = map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		}
		return
	}
	if actionName == "activate_actions" {
		if report, ok := out.Diagnostics.(actions.ActivationReport); ok {
			for _, doc := range report.Activated {
				a.activatedActions[doc.Name] = true
			}
		}
	}
	results["action_results"].(map[string]interface{})[stepID] = map[string]interface{}{
		"status": "ok",
		"output": out,
	}
	return results
}

func (a *BaseAgent) activatedActionDocs() []actions.ActionDocumentation {
	names := make([]string, 0, len(a.activatedActions))
	for name := range a.activatedActions {
		names = append(names, name)
	}
	sort.Strings(names)
	docs, _ := a.dataActions.ActionDocumentation(names)
	return docs
}

func (a *BaseAgent) ensureActionActivated(name string, ch chan<- string) bool {
	if a.activatedActions[name] {
		return true
	}
	docs, notFound := a.dataActions.ActionDocumentation([]string{name})
	if len(docs) == 0 {
		ch <- a.formatEvent("error", map[string]interface{}{"message": fmt.Sprintf("action %q is not registered; choose a name from the action bookmarks", name), "not_found": notFound})
		return false
	}
	a.activatedActions[name] = true
	ch <- a.formatEvent("action_activation", map[string]interface{}{
		"actions": []string{name},
		"mode":    "automatic_safety_fallback",
		"message": "Full action documentation was loaded before execution because the planner skipped explicit activation.",
	})
	return true
}

func cloneActionStep(step map[string]interface{}) map[string]interface{} {
	copyStep := make(map[string]interface{}, len(step))
	for key, value := range step {
		copyStep[key] = value
	}
	copyStep["action_params"] = sanitizeActionParams(actionParamsFromStep(step))
	return copyStep
}

func actionParamsFromStep(step map[string]interface{}) map[string]interface{} {
	if params, ok := step["action_params"].(map[string]interface{}); ok {
		return params
	}
	params := map[string]interface{}{}
	if step["action_params"] != nil {
		encoded, _ := json.Marshal(step["action_params"])
		_ = json.Unmarshal(encoded, &params)
	}
	return params
}

// sanitizeActionParams keeps the action audit useful without echoing secrets or
// flooding the terminal with an entire source file or artifact body.
func sanitizeActionParams(params map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(params))
	for key, value := range params {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "api_key") {
			result[key] = "[redacted]"
			continue
		}
		if text, ok := value.(string); ok && len(text) > 1200 {
			result[key] = fmt.Sprintf("%s… (%d chars)", text[:1200], len(text))
			continue
		}
		result[key] = value
	}
	return result
}

func summarizeExecutionResult(execRes map[string]interface{}) map[string]interface{} {
	summary := map[string]interface{}{}
	results, ok := execRes["action_results"].(map[string]interface{})
	if !ok {
		return summary
	}
	for stepID, raw := range results {
		entry, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		out, ok := entry["output"].(*actions.ActionResult)
		if !ok || out == nil {
			if errText, exists := entry["error"]; exists {
				summary[stepID] = map[string]interface{}{"success": false, "error": errText}
			}
			continue
		}
		actionSummary := map[string]interface{}{
			"success":           out.Success,
			"summary":           out.Summary,
			"error":             out.Error,
			"working_directory": out.WorkingDirectory,
			"exit_code":         out.ExitCode,
			"stdout":            clipAuditText(out.Stdout),
			"stderr":            clipAuditText(out.Stderr),
			"files_read":        out.FilesRead,
			"files_written":     out.FilesWritten,
			"artifacts":         out.Artifacts,
			"duration":          out.Duration.String(),
		}
		if commandResults, ok := out.Diagnostics.([]workspace.RunCommandResult); ok {
			steps := make([]map[string]interface{}, 0, len(commandResults))
			for _, command := range commandResults {
				steps = append(steps, map[string]interface{}{
					"command":           command.Command,
					"working_directory": command.WorkingDirectory,
					"exit_code":         command.ExitCode,
					"stdout":            clipAuditText(command.Stdout),
					"stderr":            clipAuditText(command.Stderr),
					"error":             command.Error,
					"duration":          command.Duration.String(),
				})
			}
			actionSummary["commands"] = steps
		}
		summary[stepID] = actionSummary
	}
	return summary
}

func clipAuditText(text string) string {
	text = strings.TrimSpace(text)
	if len(text) <= 800 {
		return text
	}
	return text[:800] + fmt.Sprintf("… (%d chars)", len(text))
}

func (a *BaseAgent) buildResponseReq(results map[string]interface{}, query string) llm.ChatRequest {
	systemPrompt := prompts.ResponseSystem(query, jsonutils.ToJSON(results), a.WorkspaceRoot)
	userMessage := prompts.ResponseUser(query)
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

func (a *BaseAgent) thinkAloud(ctx context.Context, results map[string]interface{}, contextInfo, goal string) string {
	a.stepCh <- map[string]interface{}{
		"message": "Starting internal thought process",
		"context": contextInfo,
		"goal":    goal,
	}
	systemPrompt := prompts.ThinkAloudSystem(contextInfo, goal, jsonutils.ToJSON(a.RoughPlan), jsonutils.ToJSON(results))
	currentDateStr := time.Now().Format("January 2, 2006")
	datePreamble := fmt.Sprintf("Today's date is: %s.\n\n", currentDateStr)
	userPrompt := datePreamble + "Review the next action now."
	req := llm.ChatRequest{
		Model: a.Model,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream: true,
	}
	respCh, err := a.LLM.RunStream(ctx, req)
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
	ctx context.Context,
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

		systemPrompt := prompts.VisionSystem()

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

		resp, err := a.LLM.Run(ctx, req)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.VisionDescription = resp
		}

		results = append(results, res)
	}

	return results
}

// status emits concise progress to the request stream. Internal planner
// telemetry remains in logs and never corrupts the user's input line.
func (a *BaseAgent) status(ch chan<- string, message string) {
	ch <- a.formatEvent("status", map[string]interface{}{"message": message})
}

func (a *BaseAgent) stopped(ch chan<- string) {
	ch <- a.formatEvent("stopped", map[string]interface{}{"message": "Request stopped."})
}

func followUpQuestions(result map[string]interface{}) []string {
	actionResults, ok := result["action_results"].(map[string]interface{})
	if !ok {
		return nil
	}
	for _, raw := range actionResults {
		entry, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		out, ok := entry["output"].(*actions.ActionResult)
		if !ok || out == nil {
			continue
		}
		if details, ok := out.Diagnostics.(actions.AskFollowUpQuestionsResult); ok {
			questions := make([]string, 0, len(details.FollowUps))
			for _, item := range details.FollowUps {
				questions = append(questions, item.Question)
			}
			return questions
		}
	}
	return nil
}
