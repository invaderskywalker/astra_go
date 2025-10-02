// astra/agents/actions/actions.go
package actions

import (
	"astra/astra/utils/logging"
	"os"
	"os/exec"
	"path/filepath"

	"gorm.io/gorm"
)

type DataActions struct {
	fnMaps map[string]func(map[string]interface{}) interface{}
	db     *gorm.DB // Inject your PSQL DB
}

func NewDataActions(db *gorm.DB) *DataActions {
	a := &DataActions{fnMaps: make(map[string]func(map[string]interface{}) interface{}), db: db}
	a.fnMaps["replicate_db_for_branch"] = a.replicateDBForBranch
	a.fnMaps["apply_code_edits"] = a.applyCodeEdits
	return a
}

func (a *DataActions) replicateDBForBranch(params map[string]interface{}) interface{} {
	branch := params["branch"].(string)
	baseDB := params["base_db"].(string)
	newDB := "project_" + branch
	cmd := exec.Command("psql", "-c", "CREATE DATABASE "+newDB+" TEMPLATE "+baseDB+";")
	if err := cmd.Run(); err != nil {
		logging.Logger.Error("replicate_db error", "error", err)
		return map[string]interface{}{"error": err.Error()}
	}
	// Update config/env for agent to use newDB
	return map[string]interface{}{"new_db": newDB}
}

func (a *DataActions) applyCodeEdits(params map[string]interface{}) interface{} {
	edits := params["edits"].([]interface{}) // List of maps: {"type": "replace", "target": "...", ...}
	for _, editI := range edits {
		edit := editI.(map[string]interface{})
		typ := edit["type"].(string)
		file := edit["file"].(string)
		switch typ {
		case "replace":
			a.handleReplace(file, edit)
		case "insert":
			a.handleInsert(file, edit)
		case "create_file":
			a.createFile(file, edit["content"].(string))
		case "delete_file":
			os.Remove(file)
		}
	}
	// Syntax check: Run `go vet` or `go build` on affected files
	if err := exec.Command("go", "vet", "./...").Run(); err != nil {
		// Rollback logic: Restore from .bak
		return map[string]interface{}{"error": "Syntax error: " + err.Error()}
	}
	return map[string]interface{}{"success": true}
}

func (a *DataActions) handleReplace(file string, edit map[string]interface{}) {
	// Read lines, find target/start-end, replace (use strings.Split for lines)
	// Similar to Python: search with context_before/after
	// Write back, backup .bak
	// Omitted full impl for brevity; mirror Python's handle_replace
}

func (a *DataActions) handleInsert(file string, edit map[string]interface{}) {
	// Similar to above
}

func (a *DataActions) createFile(file, content string) {
	dir := filepath.Dir(file)
	os.MkdirAll(dir, 0755)
	os.WriteFile(file, []byte(content), 0644)
}
