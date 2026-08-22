// Package actions exposes the safe, typed tools available to the Astra agent.
package actions

import (
	"astra/astra/agents/workspace"
	"astra/astra/config"
	"astra/astra/sources/mindpalace"
	"astra/astra/sources/storage"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"
)

type ActionHandler func(map[string]any) ActionResult

// DataActions is a thin adapter between the planner and repository/database tools.
type DataActions struct {
	actions   map[string]ActionSpec
	db        *gorm.DB
	UserID    int
	memory    *mindpalace.Store
	workspace *workspace.Workspace
}

type ActionSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ActionSpec is planner-facing metadata. Guidance is intentionally prose: it is
// read by the model, unlike a terse configuration file.
type ActionSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Guidance    string `json:"guidance"`
	Params      any    `json:"params"`
	handler     ActionHandler
}

func NewDataActions(db *gorm.DB, userID int) *DataActions {
	return NewDataActionsForSession(db, userID, "")
}

func NewDataActionsForSession(db *gorm.DB, userID int, sessionID string) *DataActions {
	ws, err := workspace.NewWorkspace("")
	if err != nil {
		panic(fmt.Sprintf("initialize workspace: %v", err))
	}
	cfg := config.LoadConfig()
	mirror, mirrorErr := storage.NewMinIOClient(cfg)
	if mirrorErr != nil {
		mirror = nil
	}
	a := &DataActions{actions: make(map[string]ActionSpec), db: db, UserID: userID, workspace: ws, memory: mindpalace.New(cfg.MindPalaceRoot, userID, sessionID, mirror)}
	a.registerCoreActions()
	a.registerKnowledgeActions()
	return a
}

func (a *DataActions) register(spec ActionSpec) { a.actions[spec.Name] = spec }

func (a *DataActions) registerCoreActions() {
	a.register(ActionSpec{Name: "write_artifact", Description: "Writes a durable user-facing artifact in a validated format.", Guidance: "Use this for useful deliverables from an interaction: Markdown for plans/notes/reports, JSON for structured state, JSONL for append-only events, CSV for tables, and text for simple output. Write a concise, complete artifact once the content is supported by evidence. Astra chooses the safe .astra/artifacts/session destination; do not use code-edit tools for user artifacts.", Params: WriteArtifactParams{}, handler: decodeHandler(a.WriteArtifact)})
	a.register(ActionSpec{Name: "apply_code_edits", Description: "Preview or apply atomic, precise code edits.", Guidance: "Inspect the target first. Prefer replace, insert_before, insert_after, or delete with a unique match/anchor. Run dry_run first for risky edits. Do not use replace_file unless rewriting a complete file.", Params: ApplyCodeEditsParams{}, handler: decodeHandler(a.applyCodeEdits)})
	a.register(ActionSpec{Name: "list_files", Description: "Lists repository files and metadata without reading their contents.", Guidance: "Use this to orient yourself. Search before reading unrelated files.", Params: ListFilesParams{}, handler: decodeHandler(a.ListFiles)})
	a.register(ActionSpec{Name: "read_files", Description: "Reads one or more workspace files.", Guidance: "Read only files supported by search results or diagnostics. Prefer line ranges for large files.", Params: ReadFilesParams{}, handler: decodeHandler(a.ReadFilesInRepo)})
	a.register(ActionSpec{Name: "search_code", Description: "Searches repository text and reports file, line, and snippet.", Guidance: "Use this before an edit to find the smallest relevant context. Search compiler symbols and error text first.", Params: SearchCodeParams{}, handler: decodeHandler(a.SearchCode)})
	a.register(ActionSpec{Name: "inspect_file", Description: "Summarizes a Go source file: package, imports, declarations, and exported symbols.", Guidance: "Use this when you need structure, not full source text.", Params: InspectFileParams{}, handler: decodeHandler(a.InspectFile)})
	a.register(ActionSpec{Name: "run_command", Description: "Runs a vetted command inside the workspace and captures stdout, stderr, exit code, and duration.", Guidance: "Use focused commands such as go test ./path, go build ./..., npm test, or git status. Always use output as evidence for the next step.", Params: RunCommandActionParams{}, handler: decodeHandler(a.RunCommand)})
	a.register(ActionSpec{Name: "build_project", Description: "Builds the Go workspace and returns compiler diagnostics.", Guidance: "Run after code edits. Repair the reported file and line rather than exploring unrelated code.", Params: struct{}{}, handler: decodeHandler(a.BuildProject)})
	a.register(ActionSpec{Name: "run_tests", Description: "Runs the Go test suite and returns failures with diagnostics.", Guidance: "Run after a change. Use a focused package test when the failing package is known.", Params: RunTestsParams{}, handler: decodeHandler(a.RunTests)})
	a.register(ActionSpec{Name: "git_status", Description: "Reports changed, staged, and untracked files.", Guidance: "Check this before broad edits and when preparing a summary.", Params: struct{}{}, handler: decodeHandler(a.GitStatus)})
	a.register(ActionSpec{Name: "ask_follow_up_questions", Description: "Returns concise questions that need an answer before work can continue.", Guidance: "Use only when a necessary choice cannot be safely inferred.", Params: AskFollowUpQuestionsParams{}, handler: decodeHandler(a.AskFollowUpQuestions)})
	a.register(ActionSpec{Name: "scrape_urls", Description: "Fetches readable content from specified web pages.", Guidance: "Use only when external web content is necessary to answer the request.", Params: ScrapeURLsParams{}, handler: decodeHandler(a.ScrapeURLs)})
	a.register(ActionSpec{Name: "query_web", Description: "Searches the web for current external information.", Guidance: "Use only for information not present in the workspace or conversation.", Params: QueryWebParams{}, handler: decodeHandler(a.QueryWeb)})
}

func (a *DataActions) ListActions() []ActionSpec {
	result := make([]ActionSpec, 0, len(a.actions))
	for _, spec := range a.actions {
		result = append(result, spec)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
func (a *DataActions) GetAction(name string) (ActionSpec, bool) {
	spec, ok := a.actions[name]
	return spec, ok
}
func (a *DataActions) ListActionSummaries() []ActionSummary {
	specs := a.ListActions()
	result := make([]ActionSummary, 0, len(specs))
	for _, spec := range specs {
		result = append(result, ActionSummary{Name: spec.Name, Description: spec.Description})
	}
	return result
}

func (a *DataActions) RecordSessionEvent(eventType string, payload any) {
	_, _ = a.memory.AppendSessionEvent(context.Background(), eventType, payload)
}

// ExecuteAction never exposes reflection or arbitrary return values to callers.
func (a *DataActions) ExecuteAction(name string, params map[string]any) (result *ActionResult, err error) {
	spec, ok := a.actions[name]
	if !ok {
		return &ActionResult{Success: false, Error: fmt.Sprintf("action not found: %s", name)}, fmt.Errorf("action not found: %s", name)
	}
	start := time.Now()
	defer func() {
		if recovered := recover(); recovered != nil {
			result = &ActionResult{Success: false, Error: fmt.Sprintf("action panic: %v", recovered)}
		}
		if result != nil {
			result.Duration = time.Since(start)
		}
	}()
	resultValue := spec.handler(params)
	return &resultValue, nil
}

func decodeHandler[T any](fn func(T) ActionResult) ActionHandler {
	return func(raw map[string]any) ActionResult {
		var params T
		data, err := json.Marshal(raw)
		if err != nil {
			return ActionResult{Success: false, Error: "encode action parameters: " + err.Error()}
		}
		if err := json.Unmarshal(data, &params); err != nil {
			return ActionResult{Success: false, Error: "invalid action parameters: " + err.Error()}
		}
		return fn(params)
	}
}
