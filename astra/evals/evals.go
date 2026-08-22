// Package evals contains Astra's repeatable capability-evaluation catalog and
// deterministic local checks. Provider-backed model runs can reuse the same
// scenario definitions without making the normal Go test suite depend on API
// keys, network access, or a particular model's wording.
package evals

import (
	"astra/astra/agents/actions"
	"astra/astra/agents/prompts"
	"astra/astra/agents/workspace"
	"astra/astra/sources/identity"
	"astra/astra/sources/mindpalace"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Scenario struct {
	ID               string
	Name             string
	Category         string
	Prompt           string
	RequiredActions  []string
	ArtifactFormats  []string
	RequiredHeadings []string
	MemoryRequired   bool
	MutatesWorkspace bool
}

// BuiltinScenarios is the durable evaluation contract for the behaviors Astra
// should improve over time. Prompts are intentionally outcome-oriented; the
// model is free to choose a safe equivalent action sequence.
var BuiltinScenarios = []Scenario{
	{ID: "repository-evidence", Name: "Evidence-first repository inspection", Category: "reasoning", Prompt: "Inspect the connected repository, identify its technology evidence, and report only claims supported by tool output. Do not modify files.", RequiredActions: []string{"activate_actions", "list_files", "search_code"}},
	{ID: "technical-document", Name: "Technical requirements document", Category: "artifact", Prompt: "Create a professional Markdown technical requirements document from supplied evidence, with explicit assumptions, interfaces, validation, and open questions.", RequiredActions: []string{"activate_actions", "write_artifact"}, ArtifactFormats: []string{"markdown"}, RequiredHeadings: []string{"Overview", "Requirements", "Validation"}, MutatesWorkspace: true},
	{ID: "architecture-document", Name: "Architecture document", Category: "artifact", Prompt: "Create a Markdown architecture document that records context, components, data flow, boundaries, trade-offs, and verification strategy.", RequiredActions: []string{"activate_actions", "write_artifact"}, ArtifactFormats: []string{"markdown"}, RequiredHeadings: []string{"Context", "Components", "Data flow", "Trade-offs"}, MutatesWorkspace: true},
	{ID: "code-structure-document", Name: "Code structure documentation", Category: "artifact", Prompt: "Inspect a code repository and create a concise Markdown code-structure document grounded in files and symbols, distinguishing observed structure from inference.", RequiredActions: []string{"activate_actions", "list_files", "search_code", "read_files", "write_artifact"}, ArtifactFormats: []string{"markdown"}, RequiredHeadings: []string{"Observed structure", "Entry points", "Data flow"}, MutatesWorkspace: true},
	{ID: "structured-state", Name: "Structured JSON state", Category: "artifact", Prompt: "Create a valid JSON artifact representing the approved structured state and verify that it parses.", RequiredActions: []string{"activate_actions", "write_artifact"}, ArtifactFormats: []string{"json"}, MutatesWorkspace: true},
	{ID: "event-log", Name: "Append-only JSONL log", Category: "artifact", Prompt: "Create a valid JSONL event log with one JSON object per line and verify every line independently.", RequiredActions: []string{"activate_actions", "write_artifact"}, ArtifactFormats: []string{"jsonl"}, MutatesWorkspace: true},
	{ID: "tabular-export", Name: "CSV export", Category: "artifact", Prompt: "Create a rectangular CSV export with a header row, consistent columns, and a validation result.", RequiredActions: []string{"activate_actions", "write_artifact"}, ArtifactFormats: []string{"csv"}, MutatesWorkspace: true},
	{ID: "memory-network", Name: "Linked mind-palace memory", Category: "memory", Prompt: "Save a verified decision as file-backed memory, search it, and link it to a related knowledge block with provenance.", RequiredActions: []string{"activate_actions", "save_memory", "search_memory", "link_memory"}, MemoryRequired: true, MutatesWorkspace: true},
	{ID: "command-evidence", Name: "Ordered command validation", Category: "verification", Prompt: "Run a short ordered set of explicit commands, capture each working directory, stdout, stderr, and exit code, and stop on completion.", RequiredActions: []string{"activate_actions", "run_commands"}},
	{ID: "clarification-boundary", Name: "Necessary clarification only", Category: "reasoning", Prompt: "If the request contains a material unresolved choice, ask one focused question; otherwise proceed with safe inspection instead of asking a generic question.", RequiredActions: []string{"activate_actions"}},
	{ID: "local-identity", Name: "Private single-user identity", Category: "security", Prompt: "Create one private local profile, require login, and retain no directory-derived account state.", RequiredActions: []string{"signup", "login"}, MutatesWorkspace: true},
}

type Check struct {
	ID       string `json:"id"`
	Passed   bool   `json:"passed"`
	Summary  string `json:"summary"`
	Evidence string `json:"evidence,omitempty"`
}

type Report struct {
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Root       string    `json:"root"`
	Checks     []Check   `json:"checks"`
	Passed     int       `json:"passed"`
	Failed     int       `json:"failed"`
}

// RunLocal exercises capabilities without an LLM. It writes only below the
// supplied temporary/evaluation root and uses no external services. This is a
// regression gate for action handlers and file-backed memory.
func RunLocal(root string) Report {
	started := time.Now().UTC()
	report := Report{StartedAt: started, Root: root, Checks: []Check{}}
	add := func(id string, passed bool, summary, evidence string) {
		report.Checks = append(report.Checks, Check{ID: id, Passed: passed, Summary: summary, Evidence: evidence})
		if passed {
			report.Passed++
		} else {
			report.Failed++
		}
	}

	if err := os.MkdirAll(root, 0755); err != nil {
		add("workspace-root", false, "create evaluation root", err.Error())
		report.FinishedAt = time.Now().UTC()
		return report
	}
	previousMemoryRoot, hadMemoryRoot := os.LookupEnv("ASTRA_MIND_PALACE_DIR")
	memoryRoot := filepath.Join(root, ".astra", "mind-palace")
	_ = os.Setenv("ASTRA_MIND_PALACE_DIR", memoryRoot)
	defer func() {
		if hadMemoryRoot {
			_ = os.Setenv("ASTRA_MIND_PALACE_DIR", previousMemoryRoot)
		} else {
			_ = os.Unsetenv("ASTRA_MIND_PALACE_DIR")
		}
	}()

	registry := actions.NewDataActionsForSessionAt(nil, 1, "eval-local", root)
	checkContracts(registry, add)
	checkArtifacts(registry, root, add)
	checkWorkspaceAndCommands(registry, add)
	checkMemory(registry, add)
	checkPromptContract(add)
	checkLocalIdentity(root, add)
	report.FinishedAt = time.Now().UTC()
	return report
}

func checkLocalIdentity(root string, add func(string, bool, string, string)) {
	store := identity.New(filepath.Join(root, ".astra", "identity"))
	profile, signupErr := store.Signup("Evaluation owner", "eval@example.test", "correct-horse-battery")
	_, loggedInErr := store.LoggedIn()
	logoutErr := store.Logout()
	_, afterLogoutErr := store.LoggedIn()
	passed := signupErr == nil && profile.ID != "" && loggedInErr == nil && logoutErr == nil && afterLogoutErr == identity.ErrNotLoggedIn
	add("local-identity", passed, "creates a private profile and enforces explicit login", store.Root())
}

func checkContracts(registry *actions.DataActions, add func(string, bool, string, string)) {
	missing := []string{}
	for _, spec := range registry.ListActions() {
		docs, notFound := registry.ActionDocumentation([]string{spec.Name})
		if len(docs) != 1 || len(notFound) != 0 || docs[0].Purpose == "" || docs[0].WhenToUse == "" || docs[0].Returns == "" || len(docs[0].Examples) == 0 {
			missing = append(missing, spec.Name)
		}
	}
	add("action-contracts", len(missing) == 0, "every registered action has a full usage contract", strings.Join(missing, ", "))
}

func checkArtifacts(registry *actions.DataActions, root string, add func(string, bool, string, string)) {
	cases := []struct {
		format string
		body   string
		ext    string
	}{
		{"markdown", "# Overview\n\n## Requirements\n- verified", ".md"},
		{"json", `{"kind":"decision","approved":true}`, ".json"},
		{"jsonl", "{\"event\":\"created\"}\n{\"event\":\"verified\"}", ".jsonl"},
		{"csv", "name,value\nastra,1\n", ".csv"},
		{"text", "plain evidence\n", ".txt"},
	}
	for _, test := range cases {
		result, _ := registry.ExecuteAction("write_artifact", map[string]any{"title": "eval-" + test.format, "format": test.format, "content": test.body})
		passed := result != nil && result.Success && len(result.FilesWritten) == 1 && strings.HasSuffix(result.FilesWritten[0], test.ext)
		evidence := strings.Join(result.FilesWritten, ", ")
		if passed {
			path := filepath.Join(root, filepath.FromSlash(result.FilesWritten[0]))
			written, readErr := os.ReadFile(path)
			passed = readErr == nil && artifactContentValid(test.format, string(written))
			if readErr != nil {
				evidence += "; read-back failed: " + readErr.Error()
			} else {
				evidence += "; read-back validated"
			}
		}
		add("artifact-"+test.format, passed, "writes, reads back, and validates "+test.format+" artifacts", evidence)
	}
	invalid, _ := registry.ExecuteAction("write_artifact", map[string]any{"title": "invalid-json", "format": "json", "content": "not-json"})
	add("artifact-invalid-rejected", invalid != nil && !invalid.Success, "rejects invalid structured artifacts", invalidError(invalid))
}

func artifactContentValid(format, content string) bool {
	switch format {
	case "json":
		var value any
		return json.Unmarshal([]byte(content), &value) == nil
	case "jsonl":
		for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
			var value any
			if json.Unmarshal([]byte(line), &value) != nil {
				return false
			}
		}
		return true
	case "markdown":
		return strings.Contains(content, "# Overview") && strings.Contains(content, "## Requirements")
	case "csv":
		return strings.HasPrefix(content, "name,value\n")
	case "text":
		return strings.TrimSpace(content) != ""
	default:
		return false
	}
}

func checkWorkspaceAndCommands(registry *actions.DataActions, add func(string, bool, string, string)) {
	directory, _ := registry.ExecuteAction("create_directory", map[string]any{"path": ".astra/eval-workspace/nested"})
	listing, _ := registry.ExecuteAction("list_files", map[string]any{"path": ".astra/eval-workspace", "recursive": true})
	add("workspace-tree", directory != nil && directory.Success && listing != nil && listing.Success, "creates and lists a nested workspace directory", resultSummary(directory, listing))
	commands, _ := registry.ExecuteAction("run_commands", map[string]any{"commands": []any{map[string]any{"command": "pwd", "allow_failure": false}}, "continue_on_error": false})
	commandCount := 0
	if commands != nil {
		if records, ok := commands.Diagnostics.([]workspace.RunCommandResult); ok {
			commandCount = len(records)
		}
	}
	add("command-evidence", commands != nil && commands.Success && commandCount == 1, "captures ordered command evidence", resultSummary(commands))
}

func checkMemory(registry *actions.DataActions, add func(string, bool, string, string)) {
	one, _ := registry.ExecuteAction("save_memory", map[string]any{"kind": "decision", "title": "Evaluation decision", "summary": "Use file-backed memory", "content": "Memory is stored as linked Markdown blocks with provenance.", "confidence": "confirmed", "source": "eval"})
	two, _ := registry.ExecuteAction("save_memory", map[string]any{"kind": "convention", "title": "Evaluation convention", "summary": "Link related knowledge", "content": "Related memory should be linked for retrieval.", "confidence": "observed", "source": "eval"})
	firstID := memoryID(one)
	secondID := memoryID(two)
	link, _ := registry.ExecuteAction("link_memory", map[string]any{"from_id": firstID, "to_id": secondID})
	search, _ := registry.ExecuteAction("search_memory", map[string]any{"query": "file-backed linked memory", "limit": 5})
	passed := one != nil && one.Success && two != nil && two.Success && link != nil && link.Success && search != nil && search.Success
	add("memory-network", passed, "saves, searches, and links file-backed memory", fmt.Sprintf("%s -> %s", firstID, secondID))
}

func checkPromptContract(add func(string, bool, string, string)) {
	valid := prompts.PromptVersion != "" && strings.Contains(prompts.AgentBookmarkCatalog(), "repository_operator") && strings.Contains(prompts.SkillCatalog(), "mind_palace_memory")
	add("prompt-contract", valid, "prompt and agent catalogs are available", prompts.PromptVersion)
}

func resultSummary(results ...*actions.ActionResult) string {
	parts := make([]string, 0, len(results))
	for _, result := range results {
		if result == nil {
			continue
		}
		parts = append(parts, result.Summary)
	}
	return strings.Join(parts, "; ")
}

func invalidError(result *actions.ActionResult) string {
	if result == nil {
		return "nil result"
	}
	return result.Error
}

func memoryID(result *actions.ActionResult) string {
	if result == nil {
		return ""
	}
	if record, ok := result.Diagnostics.(mindpalace.Record); ok {
		return record.ID
	}
	return ""
}

// MarshalReport is used by the CLI and keeps the report format stable for CI.
func MarshalReport(report Report) string {
	data, _ := json.MarshalIndent(report, "", "  ")
	return string(data)
}
