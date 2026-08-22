package actions

import (
	"astra/astra/agents/workspace"
	"fmt"
	"strings"
)

// CodeEdit describes a precise repository edit. Type, Replacement, and Content
// are retained only so existing callers can continue using the former API.
type CodeEdit struct {
	File          string `json:"file"`
	Operation     string `json:"operation,omitempty"`
	Anchor        string `json:"anchor,omitempty"`
	Match         string `json:"match,omitempty"`
	NewCode       string `json:"new_code,omitempty"`
	ContextBefore string `json:"context_before,omitempty"`
	ContextAfter  string `json:"context_after,omitempty"`

	Type        string `json:"type,omitempty"`
	Replacement string `json:"replacement,omitempty"`
	Content     string `json:"content,omitempty"`
	// Natural-language aliases normalized before execution.
	Path    string `json:"path,omitempty"`
	Find    string `json:"find,omitempty"`
	Replace string `json:"replace,omitempty"`
}

type ApplyCodeEditsParams struct {
	Edits  []CodeEdit `json:"edits"`
	DryRun bool       `json:"dry_run,omitempty"`
}

func (a *DataActions) applyCodeEdits(params ApplyCodeEditsParams) ActionResult {
	if len(params.Edits) == 0 {
		return ActionResult{Success: false, Error: "edits must not be empty"}
	}

	edits := make([]workspace.CodeEdit, 0, len(params.Edits))
	for index, edit := range params.Edits {
		converted, err := toWorkspaceEdit(edit)
		if err != nil {
			return ActionResult{Success: false, Error: fmt.Sprintf("edit[%d]: %v", index, err)}
		}
		edits = append(edits, converted)
	}

	tx := workspace.NewEditTransaction(a.workspace, edits)
	if err := tx.DryRun(); err != nil {
		return ActionResult{Success: false, Error: err.Error(), Diagnostics: map[string]any{"diffs": tx.Diffs}}
	}
	if !params.DryRun {
		if err := tx.Commit(); err != nil {
			return ActionResult{Success: false, Error: err.Error(), Diagnostics: map[string]any{"diffs": tx.Diffs}}
		}
	}

	summary := fmt.Sprintf("Prepared %d edit(s)", len(edits))
	if !params.DryRun {
		summary = fmt.Sprintf("Applied %d edit(s)", len(edits))
	}
	return ActionResult{
		Success:      true,
		Summary:      summary,
		FilesWritten: tx.FilesTouched,
		Diagnostics:  map[string]any{"diffs": tx.Diffs, "dry_run": params.DryRun},
	}
}

func toWorkspaceEdit(edit CodeEdit) (workspace.CodeEdit, error) {
	file := edit.File
	if strings.TrimSpace(file) == "" {
		file = edit.Path
	}
	if strings.TrimSpace(file) == "" {
		return workspace.CodeEdit{}, fmt.Errorf("file is required")
	}
	match := edit.Match
	if match == "" {
		match = edit.Find
	}
	newCode := edit.NewCode
	if newCode == "" {
		newCode = edit.Replace
	}
	result := workspace.CodeEdit{File: file, Operation: edit.Operation, Anchor: edit.Anchor, Match: match, NewCode: newCode}
	// Backwards-compatible translations. New callers should use operation.
	if result.Operation == "" {
		switch edit.Type {
		case "create_file":
			result.Operation, result.NewCode = "create_file", edit.Content
		case "delete_file":
			result.Operation = "delete_file"
		case "update_file_content":
			result.Operation, result.NewCode = "replace_file", edit.Replacement
		default:
			return workspace.CodeEdit{}, fmt.Errorf("operation is required")
		}
	}
	return result, nil
}
