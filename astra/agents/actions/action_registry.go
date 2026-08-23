package actions

import "fmt"

// ActivateActionsParams is intentionally tiny: activation only loads docs and
// never performs the underlying action.
type ActivateActionsParams struct {
	ActionNames []string `json:"action_names"`
}

type ActivationReport struct {
	Activated []ActionDocumentation `json:"activated"`
	NotFound  []string              `json:"not_found,omitempty"`
	Message   string                `json:"message"`
}

// ActivateActions exposes the Tango-style documentation gate as a normal
// action. The BaseAgent also uses ActionDocumentation directly as a safety
// fallback when a model skips this explicit step.
func (a *DataActions) ActivateActions(params ActivateActionsParams) ActionResult {
	if len(params.ActionNames) == 0 {
		return ActionResult{Success: false, Summary: "No actions were requested for activation.", Error: "action_names must contain one to five registered action names"}
	}
	if len(params.ActionNames) > 5 {
		return ActionResult{Success: false, Summary: "Too many actions were requested for activation.", Error: "activate_actions accepts at most five action names per call"}
	}
	docs, notFound := a.ActionDocumentation(params.ActionNames)
	if len(docs) == 0 {
		return ActionResult{Success: false, Summary: "No registered actions were activated.", Error: "none of the requested action names exist", Diagnostics: ActivationReport{Activated: docs, NotFound: notFound, Message: "Choose action names from the compact bookmark catalog."}}
	}
	message := fmt.Sprintf("Activated %d action documentation entr%s.", len(docs), "y")
	if len(docs) != 1 {
		message = fmt.Sprintf("Activated %d action documentation entries.", len(docs))
	}
	return ActionResult{Success: true, Summary: message, Diagnostics: ActivationReport{Activated: docs, NotFound: notFound, Message: "Full parameter schemas, examples, return shape, and recovery rules are now available."}}
}

func documentationFor(spec ActionSpec) ActionDocumentation {
	return ActionDocumentation{
		Name: spec.Name, Category: spec.Category, Purpose: spec.Description,
		WhenToUse: spec.WhenToUse, NeverUseWhen: spec.NeverUseWhen, Parameters: spec.Params,
		ParameterNotes: spec.ParameterNotes, Examples: spec.Examples, Returns: spec.Returns, SideEffects: spec.SideEffects,
		Approval: spec.Approval, FailureRecovery: spec.FailureRecovery, RelatedActions: spec.RelatedActions,
	}
}

// Defaults ensure every existing action has a complete contract while keeping
// the registration sites readable. High-risk and unusual actions override
// these values in action registration itself.
func applyDocumentationDefaults(spec *ActionSpec) {
	if spec.Category == "" {
		switch spec.Name {
		case "spawn_agent", "list_agents", "wait_agents", "activate_actions":
			spec.Category = "orchestration"
		case "run_command", "run_commands", "build_project", "run_tests", "git_status":
			spec.Category = "engineering"
		case "read_files", "list_files", "search_code", "inspect_file", "analyze_files", "create_directory":
			spec.Category = "repository"
		case "write_artifact", "apply_code_edits":
			spec.Category = "delivery"
		case "save_memory", "search_memory", "list_memory", "link_memory":
			spec.Category = "memory"
		case "write_prompt_profile":
			spec.Category = "configuration"
		case "scrape_urls", "query_web":
			spec.Category = "research"
		default:
			spec.Category = "conversation"
		}
	}
	if spec.WhenToUse == "" {
		spec.WhenToUse = spec.Guidance
	}
	if spec.NeverUseWhen == "" {
		spec.NeverUseWhen = "Do not use it when a narrower action or a direct answer is sufficient."
	}
	if spec.Returns == "" {
		spec.Returns = "ActionResult with success, summary, diagnostics, affected files, warnings, and an error when unsuccessful."
	}
	if spec.SideEffects == "" {
		spec.SideEffects = "Read-only unless the action name and guidance explicitly describe a file, memory, or command mutation."
	}
	if spec.Approval == "" {
		spec.Approval = "No approval for routine in-scope work; follow the authority and destructive-action rules in the system policy."
	}
	if spec.FailureRecovery == "" {
		spec.FailureRecovery = "Read the returned error and diagnostics, correct the smallest parameter or environment issue, and retry at most twice."
	}
	if len(spec.Examples) == 0 {
		spec.Examples = []ActionExample{{
			Request: "Use this action for the purpose described above.",
			Params:  spec.Params,
			Outcome: "A typed ActionResult is returned; verify success before continuing.",
		}}
	}
	if len(spec.ParameterNotes) == 0 {
		spec.ParameterNotes = map[string]string{}
		switch spec.Name {
		case "write_artifact":
			spec.ParameterNotes = map[string]string{"title": "Human-readable artifact title.", "format": "markdown, json, jsonl, csv, or text.", "content": "Complete file content; do not put a plan here when a deliverable is requested."}
		case "apply_code_edits":
			spec.ParameterNotes = map[string]string{"edits": "Ordered precise edits; inspect first and use unique anchors.", "dry_run": "Preview edits without writing when the change is risky or uncertain."}
		case "list_files":
			spec.ParameterNotes = map[string]string{"path": "Relative path from the connected root; use . for the root.", "recursive": "Use true only when a recursive inventory is necessary."}
		case "create_directory":
			spec.ParameterNotes = map[string]string{"path": "Relative directory path; parent directories are created safely."}
		case "read_files":
			spec.ParameterNotes = map[string]string{"files": "Array of {path, start_line, end_line}; shorthand string paths are accepted. Use an attached session path only when the user explicitly supplied it."}
		case "search_code":
			spec.ParameterNotes = map[string]string{"query": "Literal or supported text pattern to find.", "limit": "Maximum focused matches; keep it bounded."}
		case "inspect_file":
			spec.ParameterNotes = map[string]string{"path": "Relative Go source path to summarize."}
		case "analyze_files":
			spec.ParameterNotes = map[string]string{"paths": "Files or directories to profile; omit for the connected root.", "query": "Optional text or symbol to locate and turn into bounded read ranges.", "recursive": "Recurse into directories only when needed.", "limit": "Maximum files; defaults to 64 and is capped at 128."}
		case "run_command":
			spec.ParameterNotes = map[string]string{"command": "Executable only, or a simple whitespace-separated command for compatibility; no shell operators.", "args": "Separate argv-style arguments.", "working_directory": "Relative directory from the connected root or an explicitly approved absolute scope.", "required_permission": "execute for read/check commands; write when the command is expected to modify files.", "timeout_seconds": "Bounded timeout; use the smallest useful value."}
		case "run_commands":
			spec.ParameterNotes = map[string]string{"commands": "Ordered array of command, args, working_directory, timeout_seconds, and allow_failure.", "continue_on_error": "Use only when later diagnostics remain useful after an expected failure."}
		case "run_tests":
			spec.ParameterNotes = map[string]string{"package": "Focused Go package or ./... for the full suite.", "timeout_seconds": "Bounded test timeout."}
		case "ask_follow_up_questions":
			spec.ParameterNotes = map[string]string{"questions": "One or more concise questions for a material missing decision."}
		case "scrape_urls":
			spec.ParameterNotes = map[string]string{"urls": "Specific pages to fetch.", "word_limit": "Optional per-page extraction bound."}
		case "query_web":
			spec.ParameterNotes = map[string]string{"queries": "Focused current-information searches.", "result_limit": "Bounded result count."}
		case "spawn_agent":
			spec.ParameterNotes = map[string]string{"goal": "One bounded, non-overlapping worker objective.", "personality": "Optional role/personality instructions; they cannot override Astra policy.", "workspace_root": "Current workspace or an approved scope.", "provider": "Optional provider override.", "model": "Optional model override."}
		case "wait_agents":
			spec.ParameterNotes = map[string]string{"agent_ids": "Worker branch IDs; omit to wait for all known branches."}
		case "write_prompt_profile":
			spec.ParameterNotes = map[string]string{"name": "Stable human-readable profile name.", "description": "Short purpose statement.", "content": "Markdown instructions or personality guidance; compiled policy and authority remain higher priority.", "enabled": "Whether to include the profile in future planning and execution prompts."}
		case "save_memory":
			spec.ParameterNotes = map[string]string{"kind": "Knowledge category such as decision, convention, lesson, or fact.", "title": "Stable concise title used for refinement.", "summary": "Short retrieval-friendly summary.", "content": "Verified reusable knowledge with provenance.", "related": "IDs or paths of related memory blocks.", "confidence": "observed, inferred, or confirmed.", "source": "Evidence path or session reference."}
		case "search_memory":
			spec.ParameterNotes = map[string]string{"query": "Intent-rich terms to retrieve linked knowledge.", "limit": "Small result bound."}
		case "list_memory":
			spec.ParameterNotes = map[string]string{"kind": "Optional category filter."}
		case "link_memory":
			spec.ParameterNotes = map[string]string{"from_id": "Source memory block ID.", "to_id": "Related target memory block ID."}
		case "activate_actions":
			spec.ParameterNotes = map[string]string{"action_names": "One to five exact names from the compact bookmark catalog."}
		}
	}
}
