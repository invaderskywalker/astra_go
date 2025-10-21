// Package actions provides functionality for managing database and code manipulation actions.
package actions

import (
	"astra/astra/utils/logging"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// CodeEdit represents a single code modification operation.
type CodeEdit struct {
	Type string `json:"type"` // "create_file", "delete_file", "replace", or "insert"
	// New edit type - replace_file: replaces the entire file contents in one step
	// Example: {"type": "replace_file", "file": "foo.go", "replacement": "new contents..." }
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

		fmt.Println("applyCodeEdits basepath for edit 4", absFile, err)

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

	if strings.HasSuffix(file, ".go") {
		creating := false
		for _, e := range edits {
			if e.Type == "create_file" {
				creating = true
				break
			}
		}
		if !creating {
			// contentStr := strings.Join(lines, "\n")
			// if !strings.Contains(contentStr, "package ") {
			// 	return fmt.Errorf("sanity check failed: file %s missing 'package' declaration; edit aborted", file)
			// }
		}
	}

	for _, edit := range edits {
		fmt.Println("â applying edit:", edit.Type, "target:", edit.Target, "position:", edit.Position)
		switch edit.Type {
		case "create_file":
			a.createFile(edit.File, edit.Content)
		case "delete_file":
			if err := os.Remove(edit.File); err != nil && !os.IsNotExist(err) {
				logging.AppLogger.Warn("delete_file failed", zap.String("file", edit.File), zap.Error(err))
			}
		case "replace_file":
			// Replace entire file contents with provided replacement
			if err := os.WriteFile(edit.File, []byte(edit.Replacement), 0644); err != nil {
				return fmt.Errorf("failed to replace entire file %s: %w", edit.File, err)
			}
			lines = strings.Split(edit.Replacement, "\n")
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
	pos := edit.Position
	if pos == "" {
		pos = "after"
	}

	// Determine context direction
	var ctxBefore bool
	if strings.TrimSpace(edit.ContextBefore) != "" {
		ctxBefore = true
	} else if strings.TrimSpace(edit.ContextAfter) != "" {
		ctxBefore = false
	} else {
		ctxBefore = (pos == "before")
	}

	fmt.Println("debug handleReplace ", ctxBefore, edit.Start != "", edit.End != "")

	// --- Multiline region replace mode (improved) ---
	if edit.Start != "" && edit.End != "" {
		startIdx := a.findLineIndex(lines, edit.Start, edit.ContextBefore, true)
		if startIdx == -1 {
			fmt.Println("⚠️  start marker not found:", edit.Start)
			return lines
		}

		// Support multi-line end marker: split into parts and search for the sequence
		endParts := strings.Split(strings.TrimSpace(edit.End), "\n")
		for i := range endParts {
			endParts[i] = strings.TrimSpace(endParts[i])
		}

		endIdx := -1
		// scan forward, try to match the sequence of endParts
		for i := startIdx; i < len(lines); i++ {
			// if endParts length is 1, do the old fast path
			if len(endParts) == 1 {
				if strings.Contains(strings.TrimSpace(lines[i]), endParts[0]) {
					endIdx = i
					break
				}
				continue
			}

			// for multi-line endParts, ensure we have enough lines left
			if i+len(endParts)-1 >= len(lines) {
				break
			}

			matched := true
			for k := 0; k < len(endParts); k++ {
				if !strings.Contains(strings.TrimSpace(lines[i+k]), endParts[k]) {
					matched = false
					break
				}
			}
			if matched {
				endIdx = i + len(endParts) - 1
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
	return lines
}

func (a *DataActions) handleInsert(lines []string, edit CodeEdit) []string {
	content := strings.Split(edit.Content, "\n")
	pos := edit.Position
	if pos == "" {
		pos = "after"
	}
	var ctxBefore bool
	if strings.TrimSpace(edit.ContextBefore) != "" {
		ctxBefore = true
	} else if strings.TrimSpace(edit.ContextAfter) != "" {
		ctxBefore = false
	} else {
		ctxBefore = (pos == "before")
	}

	// 🚀 Special markers for file start and end
	if edit.Target == "__BOF__" { // Beginning of File
		return append(content, lines...)
	}
	if edit.Target == "__EOF__" { // End of File
		return append(lines, content...)
	}

	idx := a.findLineIndex(lines, edit.Target, edit.ContextBefore, ctxBefore)
	if idx == -1 {
		fmt.Println("⚠️  insert target not found:", edit.Target)
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
	fmt.Println("findLineIndex target  ", target)
	for i, line := range lines {
		// fmt.Println("match line  ", safeLineMatch(line, target), i, line)
		if safeLineMatch(line, target) {
			fmt.Print("matched but now ", before, i,
				a.hasContextInRange(lines, i, window, context, true),
				a.hasContextInRange(lines, i, window, context, false))

			if context == "" {
				fmt.Println(" ---> 1")
				return i
			}
			if before && a.hasContextInRange(lines, i, window, context, true) {
				fmt.Println(" ---> 2 (context before)")
				return i
			}
			if !before && a.hasContextInRange(lines, i, window, context, false) {
				fmt.Println(" ---> 3 (context after)")
				return i
			}
		}
	}
	return -1
}

func safeLineMatch(line, target string) bool {
	normalizeLine := func(s string) string {
		s = strings.ToLower(s)
		s = strings.ReplaceAll(s, "\t", " ")
		// drop inline comments for the file line
		// s = strings.Split(s, "//")[0]
		s = strings.TrimSpace(s)
		s = strings.Join(strings.Fields(s), " ")
		return s
	}
	normalizeTarget := func(s string) string {
		s = strings.ToLower(s)
		s = strings.ReplaceAll(s, "\t", " ")
		s = strings.TrimSpace(s)
		s = strings.Join(strings.Fields(s), " ")
		return s
	}

	lineNorm := normalizeLine(line)
	targetNorm := normalizeTarget(target)

	// fmt.Printf("safeLineMatch: line - %s, target - %s\n", lineNorm, targetNorm)

	if targetNorm == "" {
		return false
	}
	match := lineNorm == targetNorm || strings.HasPrefix(lineNorm, targetNorm+" ")
	if match {
		fmt.Printf("✅ normalized strict match: [%s] in [%s]\n", targetNorm, line)
	}
	return match
}

func (a *DataActions) hasContextInRange(lines []string, start int, window int, context string, searchUp bool) bool {
	clean := strings.ToLower(strings.TrimSpace(context))
	fmt.Println("\nclean - ", searchUp, clean)

	if clean == "" {
		return false
	}

	if searchUp {
		// search backwards from the line *before* start
		for i := start - 1; i >= 0 && i >= start-window; i-- {
			if strings.Contains(strings.ToLower(lines[i]), clean) {
				return true
			}
		}
	} else {
		// search forward from the line *after* start
		for i := start + 1; i < len(lines) && i <= start+window; i++ {
			if strings.Contains(strings.ToLower(lines[i]), clean) {
				return true
			}
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
		{"goimports", "-w", "./"},
		{"go", "fmt", "./..."},
		{"go", "vet", "./..."},
		{"go", "mod", "tidy"},
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

func (a *DataActions) FrontendBuild() (map[string]interface{}, error) {
	cmds := [][]string{
		{"npm", "run", "build"},
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

	successMsg := "Build output"
	outputs = append(outputs, successMsg)

	return map[string]interface{}{
		"success": true,
		"output":  strings.Join(outputs, "\n"),
	}, nil
}
