package prompts

import (
	"strings"
)

// AgentBookmark is a lightweight role map for the main agent. It documents
// which capabilities normally belong together without spawning an untracked
// sub-agent or granting permissions outside the runtime action registry.
type AgentBookmark struct {
	Name       string
	Purpose    string
	Tools      []string
	Activation string
}

var BuiltinAgentBookmarks = []AgentBookmark{
	{Name: "repository_operator", Purpose: "Orient in an unfamiliar repository, analyze large files, inspect evidence, make precise edits, and navigate project structure.", Tools: []string{"list_files", "analyze_files", "search_code", "read_files", "inspect_file", "create_directory", "apply_code_edits"}, Activation: "Use for repository questions, debugging, scaffolding, and code changes. Analyze before broad reads and iterate through bounded ranges."},
	{Name: "verification_engineer", Purpose: "Run focused commands, builds, tests, and git checks; classify failures and prove outcomes.", Tools: []string{"run_command", "run_commands", "build_project", "run_tests", "git_status"}, Activation: "Use after changes or whenever a claim depends on executable evidence."},
	{Name: "artifact_author", Purpose: "Create durable Markdown, JSON, JSONL, CSV, and text deliverables with correct structure.", Tools: []string{"write_artifact", "read_files", "apply_code_edits"}, Activation: "Use when the user requests a file, report, specification, export, or project documentation."},
	{Name: "memory_curator", Purpose: "Save, retrieve, list, and link concise file-backed mind-palace knowledge with provenance.", Tools: []string{"save_memory", "search_memory", "list_memory", "link_memory"}, Activation: "Use for durable decisions, conventions, constraints, lessons, and related knowledge links."},
	{Name: "researcher", Purpose: "Retrieve current external evidence and synthesize it with source identity and dates.", Tools: []string{"query_web", "scrape_urls", "write_artifact"}, Activation: "Use only when external or changing information is required."},
}

func AgentBookmarkCatalog() string {
	var builder strings.Builder
	for _, agent := range BuiltinAgentBookmarks {
		builder.WriteString("- ")
		builder.WriteString(agent.Name)
		builder.WriteString("\n  Purpose: ")
		builder.WriteString(agent.Purpose)
		builder.WriteString("\n  Typical tools: ")
		builder.WriteString(strings.Join(agent.Tools, ", "))
		builder.WriteString("\n  Use when: ")
		builder.WriteString(agent.Activation)
		builder.WriteByte('\n')
	}
	return strings.TrimSpace(builder.String())
}
