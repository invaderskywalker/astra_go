// astra/agents/core/base_agent.go
package core

import (
	"astra/astra/agents/actions"
	"astra/astra/agents/prompts"
	"astra/astra/services/llm"
	"astra/astra/sources/psql/dao"
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
	ExecutionPlans []map[string]interface{}
	RoughPlan      map[string]interface{}
	SessionID      string
	LogInfo        map[string]interface{}
	dataActions    *actions.DataActions
	stepCh         chan map[string]interface{}
	responseCh     chan string
	queryQueue     chan queuedQuery
	mu             sync.Mutex
	controlMu      sync.Mutex
	controlCond    *sync.Cond
	paused         bool
	running        bool
	cancelCurrent  context.CancelFunc
	chatDAO        *dao.ChatMessageDAO
	summaryDAO     *dao.SessionSummaryDAO
	DB             *gorm.DB
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
	chatDAO := dao.NewChatMessageDAO(db)
	summaryDAO := dao.NewSessionSummaryDAO(db)

	agent := &BaseAgent{
		Name:        agentName,
		TenantID:    userID,
		UserID:      userID,
		LLM:         llm.NewClient(provider),
		Model:       model,
		SessionID:   sessionID,
		LogInfo:     map[string]interface{}{"tenant_id": userID, "user_id": userID, "session_id": sessionID},
		stepCh:      make(chan map[string]interface{}, 10),
		responseCh:  make(chan string, 10),
		queryQueue:  make(chan queuedQuery, 32),
		dataActions: actions.NewDataActionsForSession(db, userID, sessionID),
		chatDAO:     chatDAO,
		summaryDAO:  summaryDAO,
		DB:          db,
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

	systemPrompt := prompts.ExecutionSystem(
		jsonutils.ToJSON(roughPlan),
		jsonutils.ToJSON(results),
		prompts.ActionCatalog(fullActions),
		prompts.ExecutionSchema,
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
	const maxExecutionSteps = 12
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
		ch <- a.formatEvent("action_plan", map[string]interface{}{
			"index": stepIndex,
			"step":  step,
		})
		a.status(ch, fmt.Sprintf("Running %s", actionName))
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
	results["action_results"].(map[string]interface{})[stepID] = map[string]interface{}{
		"status": "ok",
		"output": out,
	}
	return results
}

func (a *BaseAgent) buildResponseReq(results map[string]interface{}, query string) llm.ChatRequest {
	systemPrompt := prompts.ResponseSystem(query, jsonutils.ToJSON(results))
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
