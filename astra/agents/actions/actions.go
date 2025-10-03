// Package actions provides functionality for managing database and code manipulation actions.
package actions

import (
	"os/exec"

	"gorm.io/gorm"
)

// DataActions manages a set of actions for database and code manipulation.
type DataActions struct {
	fnMaps map[string]interface{} // Updated to interface{} to support different param types
	db     *gorm.DB
}

// CodeEdit represents a single code modification operation.
type CodeEdit struct {
	Type          string `json:"type"`           // Type of edit: "create_file", "delete_file", "replace", or "insert"
	File          string `json:"file"`           // Path to the file to edit
	Target        string `json:"target"`         // Target line or block to replace or insert relative to (optional)
	Start         string `json:"start"`          // Start of a block to replace (optional, for multi-line replace)
	End           string `json:"end"`            // End of a block to replace (optional, for multi-line replace)
	Replacement   string `json:"replacement"`    // Content to replace the target with (optional, for replace)
	Content       string `json:"content"`        // Content to insert (optional, for insert or create_file)
	Position      string `json:"position"`       // Insertion position: "before" or "after" (optional, defaults to "after")
	ContextBefore string `json:"context_before"` // Context to match before the target (optional)
	ContextAfter  string `json:"context_after"`  // Context to match after the target (optional)
}

// ApplyCodeEditsParams defines the parameters for the applyCodeEdits function.
type ApplyCodeEditsParams struct {
	Edits []CodeEdit `json:"edits"` // List of code edits to apply
}

// ApplyCodeEditsResult defines the result for the applyCodeEdits function.
type ApplyCodeEditsResult struct {
	Success      bool   `json:"success,omitempty"`       // True if edits were applied successfully
	EditsApplied int    `json:"edits_applied,omitempty"` // Number of edits applied
	Error        string `json:"error,omitempty"`         // Error message, if the operation fails
}

// NewDataActions initializes a new DataActions instance with a database connection.
// It sets up the function map for available actions.
//
// Parameters:
//   - db: A pointer to a gorm.DB instance for database operations.
//
// Returns:
//   - A pointer to an initialized DataActions instance.
func NewDataActions(db *gorm.DB) *DataActions {
	a := &DataActions{
		fnMaps: make(map[string]interface{}),
		db:     db,
	}
	a.fnMaps["apply_code_edits"] = a.applyCodeEdits
	return a
}

// applyCodeEdits applies a list of code edits to specified files and performs a syntax check.
// It groups edits by file and applies them, with a rollback mechanism if syntax validation fails.
//
// Parameters:
//   - params: An ApplyCodeEditsParams struct containing the list of code edits.
//
// Returns:
//   - An ApplyCodeEditsResult struct containing the success status, number of edits applied, or an error message.
func (a *DataActions) applyCodeEdits(params ApplyCodeEditsParams) ApplyCodeEditsResult {
	edits := params.Edits
	if len(edits) == 0 {
		return ApplyCodeEditsResult{Error: "edits list must not be empty"}
	}

	editsByFile := make(map[string][]CodeEdit)
	for _, edit := range edits {
		editsByFile[edit.File] = append(editsByFile[edit.File], edit)
	}

	for file, fileEdits := range editsByFile {
		if err := a.applyEditsToFile(file, fileEdits); err != nil {
			return ApplyCodeEditsResult{Error: err.Error()}
		}
	}

	if err := exec.Command("go", "vet", "./...").Run(); err != nil {
		a.rollbackFiles(editsByFile)
		return ApplyCodeEditsResult{Error: "Syntax error: " + err.Error()}
	}

	return ApplyCodeEditsResult{Success: true, EditsApplied: len(edits)}
}
