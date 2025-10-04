// Package actions provides functionality for managing database and code manipulation actions.
package actions

import (
	"astra/astra/utils/logging"
	"astra/astra/utils/math"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// CodeEdit represents a single code modification operation.
type CodeEdit struct {
	Type          string `json:"type"`           // "create_file", "delete_file", "replace", or "insert"
	File          string `json:"file"`           // Path to file
	Target        string `json:"target"`         // Target line or block (optional)
	Start         string `json:"start"`          // Start of block (optional)
	End           string `json:"end"`            // End of block (optional)
	Replacement   string `json:"replacement"`    // Replacement text
	Content       string `json:"content"`        // Insert/create content
	Position      string `json:"position"`       // "before" or "after" (default "after")
	ContextBefore string `json:"context_before"` // Context before target (optional)
	ContextAfter  string `json:"context_after"`  // Context after target (optional)
}

// ApplyCodeEditsParams defines parameters for the applyCodeEdits function.
type ApplyCodeEditsParams struct {
	Edits []CodeEdit `json:"edits"` // List of code edits
}

// ApplyCodeEditsResult defines the result of the applyCodeEdits function.
type ApplyCodeEditsResult struct {
	Success      bool   `json:"success,omitempty"`
	EditsApplied int    `json:"edits_applied,omitempty"`
	Error        string `json:"error,omitempty"`
}

// applyCodeEdits applies edits and validates syntax.
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

func (a *DataActions) applyEditsToFile(file string, edits []CodeEdit) error {
	if os.Getenv("ASTRA_TEST") == "1" {
		return nil
	}

	if _, err := os.Stat(file); err == nil {
		backup := file + ".bak"
		src, err := os.Open(file)
		if err != nil {
			return err
		}
		defer src.Close()
		dst, err := os.Create(backup)
		if err != nil {
			return err
		}
		defer dst.Close()
		if _, err := io.Copy(dst, src); err != nil {
			return err
		}
	}

	content, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lines := strings.Split(string(content), "\n")

	for _, edit := range edits {
		switch edit.Type {
		case "create_file":
			a.createFile(edit.File, edit.Content)
		case "delete_file":
			os.Remove(edit.File)
		case "replace":
			lines = a.handleReplace(lines, edit)
		case "insert":
			lines = a.handleInsert(lines, edit)
		}
	}

	tmpFile := file + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return err
	}
	return os.Rename(tmpFile, file)
}

func (a *DataActions) handleReplace(lines []string, edit CodeEdit) []string {
	replacementLines := strings.Split(edit.Replacement, "\n")
	contextBefore := edit.ContextBefore
	contextAfter := edit.ContextAfter

	if edit.Start != "" && edit.End != "" {
		startIdx := a.findLineIndex(lines, edit.Start, contextBefore, true)
		if startIdx == -1 {
			return lines
		}
		endIdx := a.findLineIndex(lines[startIdx:], edit.End, contextAfter, false)
		if endIdx == -1 {
			return lines
		}
		endIdx += startIdx
		return append(lines[:startIdx], append(replacementLines, lines[endIdx+1:]...)...)
	} else {
		idx := a.findLineIndex(lines, edit.Target, contextBefore, true)
		if idx == -1 {
			return lines
		}
		if contextAfter != "" && !a.hasContextInRange(lines, idx+1, 5, contextAfter) {
			return lines
		}
		return append(lines[:idx], append(replacementLines, lines[idx+1:]...)...)
	}
}

func (a *DataActions) handleInsert(lines []string, edit CodeEdit) []string {
	contentLines := strings.Split(edit.Content, "\n")
	position := edit.Position
	if position == "" {
		position = "after"
	}

	idx := a.findLineIndex(lines, edit.Target, edit.ContextBefore, true)
	if idx == -1 {
		return lines
	}

	if edit.ContextAfter != "" && !a.hasContextInRange(lines, idx+1, 5, edit.ContextAfter) {
		return lines
	}

	insertIdx := idx
	if position == "after" {
		insertIdx++
	}

	return append(lines[:insertIdx], append(contentLines, lines[insertIdx:]...)...)
}

func (a *DataActions) findLineIndex(lines []string, target, context string, checkBefore bool) int {
	window := 5
	for i, line := range lines {
		if strings.Contains(line, target) {
			if context == "" || (checkBefore && a.hasContextInRange(lines, math.Max(0, i-window), window, context)) ||
				(!checkBefore && a.hasContextInRange(lines, i+1, window, context)) {
				return i
			}
		}
	}
	return -1
}

func (a *DataActions) hasContextInRange(lines []string, start, window int, context string) bool {
	for i := start; i < math.Min(len(lines), start+window); i++ {
		if strings.Contains(lines[i], context) {
			return true
		}
	}
	return false
}

func (a *DataActions) createFile(file, content string) {
	os.MkdirAll(filepath.Dir(file), 0755)
	os.WriteFile(file, []byte(content), 0644)
	logging.AppLogger.Info("Created file", zap.String("file", file))
}

func (a *DataActions) rollbackFiles(editsByFile map[string][]CodeEdit) {
	for file := range editsByFile {
		backup := file + ".bak"
		if _, err := os.Stat(backup); err == nil {
			os.Rename(backup, file)
		}
	}
}
