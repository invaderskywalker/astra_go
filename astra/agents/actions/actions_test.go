package actions

import (
	"testing"

	"astra/astra/utils/logging"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// --- Helpers ---
func setupTestEnv(t *testing.T) *DataActions {

	logging.InitLogger() // ensures AppLogger isn’t nil
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	return NewDataActions(db, 1)
}

func TestExecuteActionUsesUnifiedResult(t *testing.T) {
	a := setupTestEnv(t)
	result, err := a.ExecuteAction("list_files", map[string]interface{}{"path": "."})
	if err != nil {
		t.Fatalf("pwd action failed: %v", err)
	}
	if !result.Success || result.Diagnostics == nil {
		t.Fatalf("unexpected unified result: %#v", result)
	}
}

func TestExecuteActionRejectsUnknownAction(t *testing.T) {
	a := setupTestEnv(t)
	result, err := a.ExecuteAction("not_a_real_action", map[string]interface{}{})
	if err == nil || result.Success || result.Error == "" {
		t.Fatalf("expected an actionable unknown-action error, got %#v / %v", result, err)
	}
}

func TestActionCatalogHasNoLegacyYAMLTools(t *testing.T) {
	a := setupTestEnv(t)
	for _, spec := range a.ListActions() {
		if spec.Name == "fetch_file_structure_in_this_repo" || spec.Name == "read_files_in_this_repo" {
			t.Fatalf("legacy action remained registered: %s", spec.Name)
		}
		if spec.Guidance == "" {
			t.Fatalf("action %s is missing planner guidance", spec.Name)
		}
	}
}
