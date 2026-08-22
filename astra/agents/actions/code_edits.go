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
	if strings.TrimSpace(edit.File) == "" {
		return workspace.CodeEdit{}, fmt.Errorf("file is required")
	}
	result := workspace.CodeEdit{File: edit.File, Operation: edit.Operation, Anchor: edit.Anchor, Match: edit.Match, NewCode: edit.NewCode}
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
