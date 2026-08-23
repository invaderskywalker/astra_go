// Package actions exposes the safe, typed tools available to the Astra agent.
package actions

import (
	"astra/astra/agents/workspace"
	"astra/astra/config"
	"astra/astra/sources/mindpalace"
	"astra/astra/sources/promptstore"
	"astra/astra/sources/scope"
	"astra/astra/sources/state"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ActionHandler func(map[string]any) ActionResult

// DataActions is a thin adapter between the planner and local repository,
// command, artifact, and Mind Palace tools.
type DataActions struct {
	actions     map[string]ActionSpec
	UserID      int
	memory      *mindpalace.Store
	workspace   *workspace.Workspace
	managedRoot string
	scopes      scope.Store
	prompts     promptstore.Store
	spawner     AgentSpawner
}

type ActionSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ActionExample is a small, concrete example shown when an action is
// activated. Examples are deliberately code-owned so they are reviewed with
// the handler and cannot silently drift through a YAML/config edit.
type ActionExample struct {
	Request string `json:"request"`
	Params  any    `json:"params"`
	Outcome string `json:"outcome"`
}

// ActionDocumentation is the full contract loaded on demand by the
// activate_actions action. The compact bookmark is enough to choose a tool;
// this contract is what the model needs to call it correctly.
type ActionDocumentation struct {
	Name            string            `json:"name"`
	Category        string            `json:"category"`
	Purpose         string            `json:"purpose"`
	WhenToUse       string            `json:"when_to_use"`
	NeverUseWhen    string            `json:"never_use_when"`
	Parameters      any               `json:"parameters"`
	ParameterNotes  map[string]string `json:"parameter_notes,omitempty"`
	Examples        []ActionExample   `json:"examples,omitempty"`
	Returns         string            `json:"returns"`
	SideEffects     string            `json:"side_effects"`
	Approval        string            `json:"approval"`
	FailureRecovery string            `json:"failure_recovery"`
	RelatedActions  []string          `json:"related_actions,omitempty"`
}

// ActionBookmark is the deliberately small registry entry injected into
// every plan. It helps the model choose a capability without spending context
// on schemas for tools it will not use.
type ActionBookmark struct {
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Purpose  string   `json:"purpose"`
	UseWhen  string   `json:"use_when"`
	Risk     string   `json:"risk"`
	Related  []string `json:"related,omitempty"`
}

// ActionSpec is planner-facing metadata. Guidance is intentionally prose: it is
// read by the model, unlike a terse configuration file.
type ActionSpec struct {
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Guidance        string            `json:"guidance"`
	Params          any               `json:"params"`
	Category        string            `json:"category,omitempty"`
	WhenToUse       string            `json:"when_to_use,omitempty"`
	NeverUseWhen    string            `json:"never_use_when,omitempty"`
	Returns         string            `json:"returns,omitempty"`
	SideEffects     string            `json:"side_effects,omitempty"`
	Approval        string            `json:"approval,omitempty"`
	FailureRecovery string            `json:"failure_recovery,omitempty"`
	RelatedActions  []string          `json:"related_actions,omitempty"`
	Examples        []ActionExample   `json:"examples,omitempty"`
	ParameterNotes  map[string]string `json:"parameter_notes,omitempty"`
	handler         ActionHandler
}

// The first argument is retained as an ignored compatibility slot for older
// callers. Astra's active runtime no longer opens or uses a database.
func NewDataActions(_ any, userID int) *DataActions {
	return NewDataActionsForSessionAt(nil, userID, "", "")
}

func NewDataActionsForSession(_ any, userID int, sessionID string) *DataActions {
	return NewDataActionsForSessionAt(nil, userID, sessionID, "")
}

// NewDataActionsForSessionAt binds every filesystem and command action to the
// same explicit root that the caller showed to the user. Keeping this root in
// the action registry prevents the planner and executor from silently drifting
// to a different process working directory.
func NewDataActionsForSessionAt(_ any, userID int, sessionID, workspaceRoot string, spawners ...AgentSpawner) *DataActions {
	ws, err := workspace.NewWorkspace(workspaceRoot)
	if err != nil {
		panic(fmt.Sprintf("initialize workspace: %v", err))
	}
	cfg := config.LoadConfig()
	// Preserve existing project-local memory when upgrading to the global
	// user-owned Mind Palace root. The migration never overwrites newer global
	// files and leaves the legacy copy recoverable.
	legacyMemoryRoot := filepath.Join(ws.Root, ".astra", "mind-palace")
	if legacyMemoryRoot != cfg.MindPalaceRoot {
		_ = mindpalace.MigrateLegacyRoot(legacyMemoryRoot, cfg.MindPalaceRoot)
	}
	scopeStore := scope.Default()
	_, _ = scopeStore.Add(ws.Root, "connected workspace", []string{scope.Read, scope.Write, scope.Execute})
	var spawner AgentSpawner
	if len(spawners) > 0 {
		spawner = spawners[0]
	}
	a := &DataActions{actions: make(map[string]ActionSpec), UserID: userID, workspace: ws, managedRoot: filepath.Join(cfg.AstraRoot, "projects", configProjectID(ws.Root)), memory: mindpalace.New(cfg.MindPalaceRoot, userID, sessionID), scopes: scopeStore, prompts: promptstore.Default(), spawner: spawner}
	a.registerCoreActions()
	a.registerKnowledgeActions()
	a.registerAgentActions()
	return a
}

func configProjectID(root string) string {
	return filepath.Base(config.ProjectDataRoot(root))
}

func (a *DataActions) managedSessionRoot() string {
	return state.SessionRoot(a.workspace.Root, a.memorySessionID())
}

func (a *DataActions) register(spec ActionSpec) {
	applyDocumentationDefaults(&spec)
	a.actions[spec.Name] = spec
}

func (a *DataActions) registerCoreActions() {
	a.register(ActionSpec{Name: "write_artifact", Description: "Writes a durable user-facing artifact in a validated format.", Guidance: "Use this for useful deliverables from an interaction: Markdown for plans/notes/reports, JSON for structured state, JSONL for append-only events, CSV for tables, and text for simple output. Write a concise, complete artifact once the content is supported by evidence. Astra chooses a private external project/session destination; do not use code-edit tools for user artifacts.", Params: WriteArtifactParams{}, handler: decodeHandler(a.WriteArtifact)})
	a.register(ActionSpec{Name: "apply_code_edits", Description: "Preview or apply atomic, precise code edits.", Guidance: "Inspect the target first. Prefer replace, insert_before, insert_after, or delete with a unique match/anchor. Run dry_run first for risky edits. Do not use replace_file unless rewriting a complete file. Canonical fields are file, match, and new_code; path, find, and replace are accepted aliases.", Params: ApplyCodeEditsParams{}, handler: decodeHandler(a.applyCodeEdits)})
	a.register(ActionSpec{Name: "list_files", Description: "Lists repository files and metadata without reading their contents.", Guidance: "Use this to orient yourself. Search before reading unrelated files.", Params: ListFilesParams{}, handler: decodeHandler(a.ListFiles)})
	a.register(ActionSpec{Name: "create_directory", Description: "Creates a directory tree inside the connected workspace.", Guidance: "Use this when bootstrapping a project or preparing a documented folder structure. Paths are relative to the connected workspace and parent directories are created safely.", Params: CreateDirectoryParams{}, handler: decodeHandler(a.CreateDirectory)})
	a.register(ActionSpec{Name: "read_files", Description: "Reads connected-workspace files, explicitly attached files, or Astra-owned project/session records.", Guidance: "Read only files supported by search results, diagnostics, an explicit attachment, or the current task artifact/session record. Prefer line ranges for large files. Absolute paths are accepted only inside Astra's managed project data root; arbitrary machine paths remain denied.", Params: ReadFilesParams{}, handler: decodeHandler(a.ReadFilesInRepo)})
	a.register(ActionSpec{Name: "search_code", Description: "Searches repository text and reports file, line, and snippet.", Guidance: "Use this before an edit to find the smallest relevant context. Search compiler symbols and error text first.", Params: SearchCodeParams{}, handler: decodeHandler(a.SearchCode)})
	a.register(ActionSpec{Name: "inspect_file", Description: "Summarizes a Go source file: package, imports, declarations, and exported symbols.", Guidance: "Use this when you need structure, not full source text.", Params: InspectFileParams{}, handler: decodeHandler(a.InspectFile)})
	a.register(ActionSpec{Name: "analyze_files", Description: "Builds compact metadata and structural evidence for files or directories without returning full source bodies.", Guidance: "Use this before reading an unfamiliar or large file. It streams line counts, hashes, headings, symbols, imports, query matches, and bounded recommended line ranges; generated caches, dependency trees, and binary artifacts are excluded automatically. Then iterate with read_files on only the ranges needed.", Params: AnalyzeFilesParams{}, Category: "repository", WhenToUse: "Use before broad reads, for files that may be large, or when you need orientation across a directory.", NeverUseWhen: "Do not use it instead of read_files when exact source text is already known and small.", Returns: "ActionResult whose diagnostics contain one compact FileAnalysis per selected file.", SideEffects: "Read-only; source bodies are not returned or persisted.", Approval: "No approval required for the connected workspace.", FailureRecovery: "If a path is invalid, correct it. If a scan warning appears, narrow the path or query and continue with bounded reads.", RelatedActions: []string{"list_files", "search_code", "read_files"}, ParameterNotes: map[string]string{"paths": "Files or directories relative to the connected root; omit to analyze the root.", "query": "Optional intent/symbol/error text; matching lines become recommended read ranges.", "recursive": "Recurse into selected directories only when needed.", "limit": "Maximum files to profile; defaults to 64 and is capped at 128; generated caches and binary artifacts do not consume this budget."}, handler: decodeHandler(a.AnalyzeFiles)})
	a.register(ActionSpec{Name: "detect_repository_stack", Description: "Detects languages, ecosystems, frameworks, modules, test signals, and likely validation commands across a polyglot repository.", Guidance: "Use this as the first orientation step for an unfamiliar repository or monorepo. It reads bounded project markers and file metadata, skips generated/dependency/binary content, and reports evidence rather than claiming that commands passed.", Params: DetectRepositoryStackParams{}, Category: "repository", WhenToUse: "Use before selecting language-specific inspection, build, or test actions when the stack is unknown or mixed.", NeverUseWhen: "Do not use it as proof that a build, test suite, service, or external dependency works; run the reported validation command when execution is requested.", Returns: "RepositoryStackReport with detected languages, ecosystems, frameworks, manifests, test signals, and suggested validation commands.", SideEffects: "Read-only; source bodies are not returned or persisted.", Approval: "No approval required for the connected workspace.", FailureRecovery: "If the scan reaches its file budget, narrow path to a module or rerun with a deliberate max_files value; if a marker is malformed, keep the remaining evidence and report the warning.", RelatedActions: []string{"list_files", "analyze_files", "search_code", "run_command"}, ParameterNotes: map[string]string{"path": "Relative repository or module directory from the connected root; omit for the root.", "max_files": "Bounded orientation file budget; defaults to 2000 and is capped at 10000."}, handler: decodeHandler(a.DetectRepositoryStack)})
	a.register(ActionSpec{Name: "run_command", Description: "Runs an explicit command in the connected workspace or an approved filesystem scope and captures stdout, stderr, exit code, and duration.", Guidance: "Use focused commands such as pwd, go test ./path, go build ./..., npm test, or git status. Prefer command plus separate argv-style args; simple whitespace-separated commands are normalized for compatibility. Shell operators are not accepted. A working_directory outside the connected workspace must be in an approved execute scope. Set required_permission to write when the command is expected to modify files; a read-only scope must never be used for writes. Always use output as evidence for the next step.", Params: RunCommandActionParams{}, handler: decodeHandler(a.RunCommand)})
	a.register(ActionSpec{Name: "run_commands", Description: "Runs an ordered sequence of explicit commands, each with its own working directory, permission, and timeout.", Guidance: "Use this for a small, related workflow such as inspect a directory, create a project, and run its validator. Prefer separate argv-style commands over shell strings. Set required_permission to write for mutating steps and allow_failure when a non-zero result is an expected check. The sequence stops on the first unexpected failure unless continue_on_error is true.", Params: RunCommandsParams{}, handler: decodeHandler(a.RunCommands)})
	a.register(ActionSpec{Name: "build_project", Description: "Builds the Go workspace and returns compiler diagnostics.", Guidance: "Run after code edits. Repair the reported file and line rather than exploring unrelated code.", Params: struct{}{}, handler: decodeHandler(a.BuildProject)})
	a.register(ActionSpec{Name: "run_tests", Description: "Runs the Go test suite and returns failures with diagnostics.", Guidance: "Run after a change. Use a focused package test when the failing package is known.", Params: RunTestsParams{}, handler: decodeHandler(a.RunTests)})
	a.register(ActionSpec{Name: "git_status", Description: "Reports changed, staged, and untracked files.", Guidance: "Check this before broad edits and when preparing a summary.", Params: struct{}{}, handler: decodeHandler(a.GitStatus)})
	a.register(ActionSpec{Name: "ask_follow_up_questions", Description: "Returns concise questions that need an answer before work can continue.", Guidance: "Use only when a necessary choice cannot be safely inferred.", Params: AskFollowUpQuestionsParams{}, handler: decodeHandler(a.AskFollowUpQuestions)})
	a.register(ActionSpec{Name: "scrape_urls", Description: "Fetches readable content from specified web pages.", Guidance: "Use only when external web content is necessary to answer the request.", Params: ScrapeURLsParams{}, handler: decodeHandler(a.ScrapeURLs)})
	a.register(ActionSpec{Name: "query_web", Description: "Searches the web for current external information.", Guidance: "Use only for information not present in the workspace or conversation.", Params: QueryWebParams{}, handler: decodeHandler(a.QueryWeb)})
	a.register(ActionSpec{Name: "list_scopes", Description: "Lists directories explicitly approved for Astra access.", Guidance: "Use before targeting a directory outside the connected workspace. A scope grants path authority only; it does not grant operating-system privileges.", Params: struct{}{}, handler: decodeHandler(a.ListScopes)})
	a.register(ActionSpec{Name: "write_prompt_profile", Description: "Creates or updates a user-authored global instruction or personality profile.", Guidance: "Use only when the user explicitly asks to create or change Astra behavior. Store durable, reviewable Markdown instructions. Profiles are preferences below Astra's compiled policy; they cannot grant tools, permissions, or override evidence and safety rules.", Params: WritePromptProfileParams{}, handler: decodeHandler(a.WritePromptProfile)})
	a.register(ActionSpec{Name: "activate_actions", Description: "Loads full documentation and parameter schemas for up to five actions.", Guidance: "Activate only the actions you plan to use next. This is a context-loading operation, not a filesystem or network side effect.", Params: ActivateActionsParams{}, Category: "orchestration", WhenToUse: "Use immediately before a tool's first call when its full schema, examples, or recovery rules are needed.", NeverUseWhen: "Do not activate every action speculatively; choose only the smallest relevant set.", Returns: "ActivationReport containing activated full documentation and any unknown names.", SideEffects: "No project state is changed; only the agent's working context is enriched.", Approval: "No approval required.", FailureRecovery: "If a name is not found, select a valid bookmark name and continue.", handler: decodeHandler(a.ActivateActions)})
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

// ListActionBookmarks returns the compact, always-available registry view.
func (a *DataActions) ListActionBookmarks() []ActionBookmark {
	specs := a.ListActions()
	result := make([]ActionBookmark, 0, len(specs))
	for _, spec := range specs {
		result = append(result, ActionBookmark{
			Name: spec.Name, Category: spec.Category, Purpose: spec.Description,
			UseWhen: spec.WhenToUse, Risk: spec.Approval, Related: spec.RelatedActions,
		})
	}
	return result
}

// ActionDocumentation returns full contracts for the requested names. An
// empty request intentionally returns no docs; callers must opt into the
// actions they need instead of accidentally expanding every prompt.
func (a *DataActions) ActionDocumentation(names []string) ([]ActionDocumentation, []string) {
	docs := make([]ActionDocumentation, 0, len(names))
	notFound := make([]string, 0)
	seen := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		spec, ok := a.actions[name]
		if !ok {
			notFound = append(notFound, name)
			continue
		}
		docs = append(docs, documentationFor(spec))
	}
	return docs, notFound
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
