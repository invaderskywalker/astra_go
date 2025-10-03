package core

import (
	"astra/astra/agents/actions"
	"astra/astra/agents/configs"
	"astra/astra/agents/getters"
	"astra/astra/services/llm"
	"astra/astra/utils/jsonutils"
	"astra/astra/utils/logging"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
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
	cfg := configs.LoadConfig()
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

	systemPrompt := fmt.Sprintf(
		`
			You are %s. Analyze query: %s
			Available sources: %v
			Available actions: %v
			Decision process: %s
			Output JSON: %s
		`,
		a.Config.AgentName,
		query,
		a.Config.AvailableDataSources,
		a.Config.AvailableActions,
		a.Config.DecisionProcess,
		a.Config.PlanOutputStructure,
	)

	req := llm.ChatRequest{
		Model:    DefaultModel,
		Messages: []llm.Message{{Role: "system", Content: systemPrompt}, {Role: "user", Content: query}},
		Stream:   false,
	}

	resp, err := a.LLM.Run(context.Background(), req)
	if err != nil {
		panic(fmt.Errorf("failed to create plan: %w", err))
	}
	fmt.Println("resp", resp)
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
		"data_sources_results": map[string]interface{}{},
		"action_results":       map[string]interface{}{},
	}

	// // Safe extraction
	// if ds, ok := plan["data_sources_to_trigger_with_source_params"].(map[string]interface{}); ok {
	// 	for name, params := range ds {
	// 		if pmap, ok := params.(map[string]interface{}); ok {
	// 			result := a.runDataSource(name, pmap)
	// 			results["data_sources_results"].(map[string]interface{})[name] = result
	// 		}
	// 	}
	// }

	// if acts, ok := plan["actions_to_trigger_with_action_params"].(map[string]interface{}); ok {
	// 	for name, params := range acts {
	// 		if pmap, ok := params.(map[string]interface{}); ok {
	// 			result := a.runAction(name, pmap)
	// 			results["action_results"].(map[string]interface{})[name] = result
	// 		}
	// 	}
	// }

	return results
}

func (a *BaseAgent) runDataSource(name string, params map[string]interface{}) map[string]interface{} {
	switch name {
	case "fetch_user_context":
		return map[string]interface{}{"status": "fetched", "context": "TBD"}
	case "fetch_repo_context":
		repo, _ := params["repo"].(string)
		branch, _ := params["branch"].(string)
		cmd := exec.Command("git", "-C", repo, "rev-parse", "--verify", branch)
		if err := cmd.Run(); err != nil {
			return map[string]interface{}{"error": "branch not found"}
		}
		return map[string]interface{}{"status": "valid", "repo": repo, "branch": branch}
	case "simulate_code_execution":
		repo, _ := params["repo"].(string)
		file, _ := params["file"].(string)
		if strings.Contains(repo, "frontend") {
			cmd := exec.Command("npm", "run", "start", "--prefix", repo)
			output, err := cmd.CombinedOutput()
			if err != nil {
				return map[string]interface{}{"error": string(output)}
			}
			return map[string]interface{}{"status": "success", "output": string(output)}
		}
		cmd := exec.Command("go", "run", file)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return map[string]interface{}{"error": string(output)}
		}
		return map[string]interface{}{"status": "success", "output": string(output)}
	default:
		return map[string]interface{}{"error": "unknown data source"}
	}
}

func (a *BaseAgent) runAction(name string, params map[string]interface{}) map[string]interface{} {
	switch name {
	// case "apply_code_edits":
	// 	return a.dataActions.ApplyCodeEdits(params)
	case "switch_git_branch":
		repo, _ := params["repo"].(string)
		branch, _ := params["branch"].(string)
		cmd := exec.Command("git", "-C", repo, "checkout", branch)
		if err := cmd.Run(); err != nil {
			cmd = exec.Command("git", "-C", repo, "checkout", "-b", branch)
			if err := cmd.Run(); err != nil {
				return map[string]interface{}{"error": "failed to switch/create branch"}
			}
		}
		return map[string]interface{}{"status": "switched", "branch": branch}
	case "replicate_db_for_branch":
		branch, _ := params["branch"].(string)
		sourceDB, _ := params["source_db"].(string)
		newDB := fmt.Sprintf("%s_%s", sourceDB, branch)
		cmd := exec.Command("psql", "-c", fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s;", newDB, sourceDB))
		if err := cmd.Run(); err != nil {
			return map[string]interface{}{"error": "failed to replicate DB"}
		}
		return map[string]interface{}{"status": "created", "db": newDB}
	case "update_db_connection":
		dbName, _ := params["db_name"].(string)
		return map[string]interface{}{"status": "updated", "db_name": dbName}
	case "create_new_repo":
		repoPath, _ := params["repo_path"].(string)
		cmd := exec.Command("git", "init", repoPath)
		if err := cmd.Run(); err != nil {
			return map[string]interface{}{"error": "failed to create repo"}
		}
		return map[string]interface{}{"status": "created", "repo": repoPath}
	case "store_user_context":
		return map[string]interface{}{"status": "stored"}
	case "store_user_interaction":
		// query, _ := params["query"].(string)
		// a.dataActions.SaveInteraction(a.UserID, a.SessionID, query)
		return map[string]interface{}{"status": "stored"}
	default:
		return map[string]interface{}{"error": "unknown action"}
	}
}

func (a *BaseAgent) buildResponseReq(results map[string]interface{}, query string) llm.ChatRequest {
	system := fmt.Sprintf(`
You are %s in %s mode. Respond based on results: %v
Instructions: %s
`, a.Config.AgentName, a.Config.AgentRole, results, a.Config.UserCTAInstructions)

	return llm.ChatRequest{
		Model: DefaultModel,
		Messages: []llm.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: fmt.Sprintf("Query: %s\nResults: %+v", query, results)},
		},
		Stream: true,
	}
}

func (a *BaseAgent) storeState(key string, value interface{}) {
	// Store state in DB (stub for now)
	fmt.Println("storeState ", key, value)
}
