// astra/agents/actions/actions.go (update)
package actions

import (
	"astra/astra/utils/logging"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type DataActions struct {
	fnMaps map[string]func(map[string]interface{}) interface{}
	db     *gorm.DB
	// agentDAO *dao.AgentDAO // For agent-specific ops
}

func NewDataActions(db *gorm.DB) *DataActions {
	a := &DataActions{
		fnMaps: make(map[string]func(map[string]interface{}) interface{}),
		db:     db,
		// agentDAO: agentDAO,
	}
	a.fnMaps["replicate_db_for_branch"] = a.replicateDBForBranch
	a.fnMaps["apply_code_edits"] = a.applyCodeEdits
	a.fnMaps["switch_git_branch"] = a.switchGitBranch
	a.fnMaps["run_server_on_port"] = a.runServerOnPort
	a.fnMaps["crawl_web_search"] = a.crawlWebSearch
	// a.fnMaps["create_agent_table"] = a.createAgentTable
	return a
}

func (a *DataActions) replicateDBForBranch(params map[string]interface{}) interface{} {
	branch := params["branch"].(string)
	baseDB := params["base_db"].(string)
	newDB := "project_" + branch
	cmd := exec.Command("psql", "-c", "CREATE DATABASE "+newDB+" TEMPLATE "+baseDB+";")
	if err := cmd.Run(); err != nil {
		logging.ErrorLogger.Error("Created file", zap.Error(err))
		// logging.Logger.Error("replicate_db error", "error", err)
		return map[string]interface{}{"error": err.Error()}
	}
	// Update config/env for agent to use newDB
	return map[string]interface{}{"new_db": newDB}
}

type CodeEdit struct {
	Type          string `json:"type"`
	File          string `json:"file"`
	Target        string `json:"target,omitempty"`
	Start         string `json:"start,omitempty"`
	End           string `json:"end,omitempty"`
	Replacement   string `json:"replacement,omitempty"`
	Content       string `json:"content,omitempty"`
	Position      string `json:"position,omitempty"` // "before", "after"
	ContextBefore string `json:"context_before,omitempty"`
	ContextAfter  string `json:"context_after,omitempty"`
}

func (a *DataActions) applyCodeEdits(params map[string]interface{}) interface{} {
	editsI, ok := params["edits"].([]interface{})
	if !ok {
		return map[string]interface{}{"error": "Invalid edits format"}
	}

	var edits []CodeEdit
	for _, editI := range editsI {
		editBytes, _ := json.Marshal(editI)
		var edit CodeEdit
		if err := json.Unmarshal(editBytes, &edit); err != nil {
			return map[string]interface{}{"error": "Failed to parse edit: " + err.Error()}
		}
		edits = append(edits, edit)
	}

	// Group by file
	editsByFile := make(map[string][]CodeEdit)
	for _, edit := range edits {
		editsByFile[edit.File] = append(editsByFile[edit.File], edit)
	}

	for file, fileEdits := range editsByFile {
		if err := a.applyEditsToFile(file, fileEdits); err != nil {
			return map[string]interface{}{"error": err.Error()}
		}
	}

	// Global syntax check
	if err := exec.Command("go", "vet", "./...").Run(); err != nil {
		a.rollbackFiles(editsByFile) // Restore .bak
		return map[string]interface{}{"error": "Syntax error: " + err.Error()}
	}

	return map[string]interface{}{"success": true, "edits_applied": len(edits)}
}

func (a *DataActions) applyEditsToFile(file string, edits []CodeEdit) error {
	if os.Getenv("ASTRA_TEST") == "1" { // Skip writes in tests
		return nil
	}

	// Backup
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

	// Apply in order
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

	// Atomic write
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
		// Multi-line block
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
		// Single line
		idx := a.findLineIndex(lines, edit.Target, contextBefore, true)
		if idx == -1 {
			return lines
		}
		// Check after context
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

	// Check after context
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
	window := 5 // Context window
	for i, line := range lines {
		if strings.Contains(line, target) {
			if context == "" || (checkBefore && a.hasContextInRange(lines, max(0, i-window), window, context)) ||
				(!checkBefore && a.hasContextInRange(lines, i+1, window, context)) {
				return i
			}
		}
	}
	return -1
}

func (a *DataActions) hasContextInRange(lines []string, start, window int, context string) bool {
	for i := start; i < min(len(lines), start+window); i++ {
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

// New Actions
func (a *DataActions) switchGitBranch(params map[string]interface{}) interface{} {
	branch := params["branch"].(string)
	cmd := exec.Command("git", "checkout", branch)
	if err := cmd.Run(); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	// Replicate DB post-switch if flagged
	if replicate, ok := params["replicate_db"]; ok && replicate.(bool) {
		a.replicateDBForBranch(map[string]interface{}{"branch": branch, "base_db": "main_db"})
	}
	return map[string]interface{}{"switched_to": branch}
}

func (a *DataActions) runServerOnPort(params map[string]interface{}) interface{} {
	port := params["port"].(string)
	cmd := exec.Command("PORT="+port, "go", "run", "main.go")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"pid": cmd.Process.Pid, "port": port}
}

func (a *DataActions) crawlWebSearch(params map[string]interface{}) interface{} {
	// Stub: Later integrate Google API or scraper
	query := params["query"].(string)
	return map[string]interface{}{"results": []string{"Mock: Searched '" + query + "' - Real-time data incoming!"}} // Placeholder
}

// func (a *DataActions) createAgentTable(params map[string]interface{}) interface{} {
// 	// Run migration for AgentFile
// 	err := a.db.AutoMigrate(&models.AgentFile{})
// 	if err != nil {
// 		return map[string]interface{}{"error": err.Error()}
// 	}
// 	return map[string]interface{}{"success": true}
// }

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
