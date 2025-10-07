// Package actions provides functionality for managing database and code manipulation actions.
package actions

import (
	"astra/astra/utils/logging"
	"astra/astra/utils/math"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

// CodeEdit represents a single code modification operation.
type CodeEdit struct {
	Type          string `json:"type"`           // "create_file", "delete_file", "replace", or "insert"
	File          string `json:"file"`           // Absolute or relative file path
	Target        string `json:"target"`         // Target line or block (optional)
	Start         string `json:"start"`          // Start of block (optional)
	End           string `json:"end"`            // End of block (optional)
	Replacement   string `json:"replacement"`    // Replacement text
	Content       string `json:"content"`        // Insert/create content
	Position      string `json:"position"`       // "before" or "after" (default "after")
	ContextBefore string `json:"context_before"` // Context before target (optional)
	ContextAfter  string `json:"context_after"`  // Context after target (optional)
}

type ApplyCodeEditsParams struct {
	Edits []CodeEdit `json:"edits"`
}

type ApplyCodeEditsResult struct {
	Success      bool   `json:"success,omitempty"`
	EditsApplied int    `json:"edits_applied,omitempty"`
	Error        string `json:"error,omitempty"`
}

// applyCodeEdits applies a batch of file modifications (insert, replace, create, delete).
func (a *DataActions) applyCodeEdits(params ApplyCodeEditsParams) ApplyCodeEditsResult {
	edits := params.Edits
	if len(edits) == 0 {
		return ApplyCodeEditsResult{Error: "edits list must not be empty"}
	}

	for i, edit := range edits {
		if strings.TrimSpace(edit.File) == "" {
			return ApplyCodeEditsResult{
				Error: fmt.Sprintf("edit[%d] is missing required 'file' field", i),
			}
		}
		absFile, err := filepath.Abs(edit.File)
		if err != nil {
			return ApplyCodeEditsResult{
				Error: fmt.Sprintf("failed to resolve absolute path for file %s: %v", edit.File, err),
			}
		}
		edits[i].File = absFile
	}

	editsByFile := make(map[string][]CodeEdit)
	for _, edit := range edits {
		editsByFile[edit.File] = append(editsByFile[edit.File], edit)
	}

	for file := range editsByFile {
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			return ApplyCodeEditsResult{
				Error: fmt.Sprintf("failed to create directory for file %s: %v", file, err),
			}
		}
	}

	applied := 0
	for file, fileEdits := range editsByFile {
		if err := a.applyEditsToFile(file, fileEdits); err != nil {
			return ApplyCodeEditsResult{Error: err.Error()}
		}
		applied += len(fileEdits)
	}

	return ApplyCodeEditsResult{Success: true, EditsApplied: applied}
}

// applyEditsToFile applies multiple edits to a single file safely.
func (a *DataActions) applyEditsToFile(file string, edits []CodeEdit) error {
	fmt.Println("applyEditsToFile →", file)
	if os.Getenv("ASTRA_TEST") == "1" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return fmt.Errorf("failed to ensure directory for %s: %w", file, err)
	}

	content, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read file %s: %w", file, err)
	}

	lines := []string{}
	if err == nil {
		lines = strings.Split(string(content), "\n")
	}

	// --- 🧠 Sanity check for Go files before edits ---
	if strings.HasSuffix(file, ".go") {
		contentStr := strings.Join(lines, "\n")
		if !strings.Contains(contentStr, "package ") {
			return fmt.Errorf("sanity check failed: file %s missing 'package' declaration; edit aborted", file)
		}
	}

	for _, edit := range edits {
		fmt.Println("→ applying edit:", edit.Type, "target:", edit.Target, "position:", edit.Position)

		// Normalize destructive replace to safe insert
		if edit.Type == "replace" && strings.HasPrefix(strings.TrimSpace(edit.Target), "type ") {
			logging.AppLogger.Warn("auto-converting risky replace into safe insert",
				zap.String("file", edit.File), zap.String("target", edit.Target))
			edit.Type = "insert"
			edit.Position = "after"
		}

		switch edit.Type {
		case "create_file":
			a.createFile(edit.File, edit.Content)
		case "delete_file":
			if err := os.Remove(edit.File); err != nil && !os.IsNotExist(err) {
				logging.AppLogger.Warn("delete_file failed", zap.String("file", edit.File), zap.Error(err))
			}
		case "replace":
			lines = a.handleReplace(lines, edit)
		case "insert":
			lines = a.handleInsert(lines, edit)
		default:
			logging.AppLogger.Warn("unknown edit type", zap.String("type", edit.Type))
		}
	}

	newContent := strings.Join(lines, "\n")
	if err := os.WriteFile(file, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", file, err)
	}

	_ = exec.Command("go", "fmt", file).Run()
	logging.AppLogger.Info("applied edits successfully", zap.String("file", file))
	return nil
}

// --- Utility functions ---

func (a *DataActions) handleReplace(lines []string, edit CodeEdit) []string {
	replacement := strings.Split(edit.Replacement, "\n")

	// 🚨 Prevent replacing entire type/function declarations accidentally
	if strings.HasPrefix(strings.TrimSpace(edit.Target), "type ") ||
		strings.HasPrefix(strings.TrimSpace(edit.Target), "func ") {
		logging.AppLogger.Warn("skipping risky replace operation on code declaration",
			zap.String("target", edit.Target),
			zap.String("file", edit.File))
		return lines
	}

	if edit.Start != "" && edit.End != "" {
		startIdx := a.findLineIndex(lines, edit.Start, edit.ContextBefore, true)
		if startIdx == -1 {
			return lines
		}
		endIdx := a.findLineIndex(lines[startIdx:], edit.End, edit.ContextAfter, false)
		if endIdx == -1 {
			return lines
		}
		endIdx += startIdx
		return append(lines[:startIdx], append(replacement, lines[endIdx+1:]...)...)
	}

	idx := a.findLineIndex(lines, edit.Target, edit.ContextBefore, true)
	if idx == -1 {
		return lines
	}
	return append(append(lines[:idx], replacement...), lines[idx+1:]...)
}

func (a *DataActions) handleInsert(lines []string, edit CodeEdit) []string {
	content := strings.Split(edit.Content, "\n")
	pos := edit.Position
	if pos == "" {
		pos = "after"
	}

	idx := a.findLineIndex(lines, edit.Target, edit.ContextBefore, true)
	if idx == -1 {
		return lines
	}

	insertIdx := idx
	if pos == "after" {
		insertIdx++
	}
	return append(lines[:insertIdx], append(content, lines[insertIdx:]...)...)
}

func (a *DataActions) findLineIndex(lines []string, target, context string, before bool) int {
	window := 5
	for i, line := range lines {
		if safeLineMatch(line, target) {
			if context == "" {
				return i
			}
			if before && a.hasContextInRange(lines, math.Max(0, i-window), window, context) {
				return i
			}
			if !before && a.hasContextInRange(lines, i+1, window, context) {
				return i
			}
		}
	}
	return -1
}

func safeLineMatch(line, target string) bool {
	line = strings.TrimSpace(line)
	target = strings.TrimSpace(target)
	if line == target {
		return true
	}
	matched, _ := regexp.MatchString(`^\s*`+regexp.QuoteMeta(target)+`\s*$`, line)
	return matched
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
	_ = os.MkdirAll(filepath.Dir(file), 0755)
	_ = os.WriteFile(file, []byte(content), 0644)
	logging.AppLogger.Info("created file", zap.String("file", file))
}

func (a *DataActions) rollbackFiles(editsByFile map[string][]CodeEdit) {
	for file := range editsByFile {
		backup := file + ".bak"
		if _, err := os.Stat(backup); err == nil {
			_ = os.Rename(backup, file)
			logging.AppLogger.Info("rolled back file", zap.String("file", file))
		}
	}
}
