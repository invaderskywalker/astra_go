// Package actions provides functionality for managing database and code manipulation actions.
package actions

import (
	"astra/astra/utils/logging"
	"astra/astra/utils/math"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	// if strings.HasSuffix(file, ".go") {
	// 	contentStr := strings.Join(lines, "\n")
	// 	if !strings.Contains(contentStr, "package ") {
	// 		return fmt.Errorf("sanity check failed: file %s missing 'package' declaration; edit aborted", file)
	// 	}
	// }
	// --- 🧠 Sanity check for Go files before edits ---
	// Skip sanity check if file is being created in this batch
	if strings.HasSuffix(file, ".go") {
		creating := false
		for _, e := range edits {
			if e.Type == "create_file" {
				creating = true
				break
			}
		}
		if !creating {
			contentStr := strings.Join(lines, "\n")
			if !strings.Contains(contentStr, "package ") {
				return fmt.Errorf("sanity check failed: file %s missing 'package' declaration; edit aborted", file)
			}
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

	// 🚫 Skip rewriting if the file was just created in this batch
	created := false
	for _, e := range edits {
		if e.Type == "create_file" {
			created = true
			break
		}
	}
	if !created {
		newContent := strings.Join(lines, "\n")
		if err := os.WriteFile(file, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", file, err)
		}
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

	fmt.Println("debug ", edit.Start != "", edit.End != "")

	// --- 🧩 Multiline region replace mode ---
	if edit.Start != "" && edit.End != "" {
		startIdx := a.findLineIndex(lines, edit.Start, edit.ContextBefore, true)
		if startIdx == -1 {
			fmt.Println("⚠️  start marker not found:", edit.Start)
			return lines
		}

		// find the end marker — trim spaces/tabs before comparing
		endIdx := -1
		for i := startIdx + 1; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if strings.Contains(line, strings.TrimSpace(edit.End)) {
				endIdx = i
				break
			}
		}
		if endIdx == -1 {
			fmt.Println("⚠️  end marker not found:", edit.End)
			return lines
		}

		fmt.Printf("🧠 Replacing lines %d–%d in %s\n", startIdx, endIdx, edit.File)
		return append(lines[:startIdx], append(replacement, lines[endIdx+1:]...)...)
	}

	fmt.Println("debug 2 ", edit.Target, edit.ContextBefore)

	// --- 🧩 Single-line replace fallback ---
	idx := a.findLineIndex(lines, edit.Target, edit.ContextBefore, true)
	if idx == -1 {
		fmt.Println("⚠️  target not found:", edit.Target)
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

	// 🚀 Special markers for file start and end
	if edit.Target == "__BOF__" { // Beginning of File
		return append(content, lines...)
	}
	if edit.Target == "__EOF__" { // End of File
		return append(lines, content...)
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
	window := 200
	for i, line := range lines {
		if safeLineMatch(line, target) {
			fmt.Println(" ---> ", safeLineMatch(line, target), i, line)
			if context == "" {
				fmt.Println(" ---> 1")
				return i
			}
			if before && a.hasContextInRange(lines, math.Max(0, i-window), window, context) {
				fmt.Println(" ---> 2")
				return i
			}
			if !before && a.hasContextInRange(lines, i+1, window, context) {
				fmt.Println(" ---> 3")
				return i
			}
		}
	}
	return -1
}

func safeLineMatch(line, target string) bool {
	line = strings.ToLower(strings.TrimSpace(strings.Split(line, "//")[0]))
	target = strings.ToLower(strings.TrimSpace(target))

	// normalize both
	line = strings.ReplaceAll(line, "`", "")
	target = strings.ReplaceAll(target, "`", "")

	// tolerate any whitespace mismatch
	line = strings.Join(strings.Fields(line), " ")
	target = strings.Join(strings.Fields(target), " ")

	return strings.Contains(line, target)
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

func (a *DataActions) FmtVetBuild() (map[string]interface{}, error) {
	cmds := [][]string{
		{"go", "fmt", "./..."},
		{"go", "vet", "./..."},
		{"go", "build", "./..."},
	}

	outputs := []string{}

	for _, cmdArgs := range cmds {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Dir = "." // project root
		out, err := cmd.CombinedOutput()
		if len(out) > 0 {
			outputs = append(outputs, fmt.Sprintf("🔍 %s output:\n%s\n", strings.Join(cmdArgs, " "), string(out)))
		}
		if err != nil {
			failMsg := fmt.Sprintf("❌ %s failed: %v", strings.Join(cmdArgs, " "), err)
			outputs = append(outputs, failMsg)
			return map[string]interface{}{
				"success": false,
				"output":  strings.Join(outputs, "\n"),
			}, err
		}
	}

	successMsg := "✅ Go code formatted, vetted, and compiled successfully."
	outputs = append(outputs, successMsg)

	return map[string]interface{}{
		"success": true,
		"output":  strings.Join(outputs, "\n"),
	}, nil
}
