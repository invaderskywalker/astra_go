package actions

import (
	"encoding/json"
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

func TestActionBookmarksAndActivationContracts(t *testing.T) {
	a := setupTestEnv(t)
	bookmarks := a.ListActionBookmarks()
	if len(bookmarks) < 10 {
		t.Fatalf("expected a complete bookmark catalog, got %d entries", len(bookmarks))
	}
	var found bool
	for _, bookmark := range bookmarks {
		if bookmark.Name == "run_commands" {
			found = true
			if bookmark.Category == "" || bookmark.Purpose == "" || bookmark.UseWhen == "" {
				t.Fatalf("run_commands bookmark is incomplete: %#v", bookmark)
			}
		}
	}
	if !found {
		t.Fatal("run_commands bookmark missing")
	}
	docs, notFound := a.ActionDocumentation([]string{"run_commands", "not_registered"})
	if len(docs) != 1 || len(notFound) != 1 || docs[0].Name != "run_commands" {
		t.Fatalf("unexpected activation lookup: docs=%#v notFound=%#v", docs, notFound)
	}
	result, err := a.ExecuteAction("activate_actions", map[string]interface{}{"action_names": []string{"run_commands"}})
	if err != nil || !result.Success {
		t.Fatalf("activation action failed: %#v / %v", result, err)
	}
	if report, ok := result.Diagnostics.(ActivationReport); !ok || len(report.Activated) != 1 {
		t.Fatalf("activation report missing full docs: %#v", result.Diagnostics)
	}
}

func TestActivationRejectsMoreThanFiveActions(t *testing.T) {
	a := setupTestEnv(t)
	result, err := a.ExecuteAction("activate_actions", map[string]interface{}{"action_names": []string{"a", "b", "c", "d", "e", "f"}})
	if err != nil || result.Success || result.Error == "" {
		t.Fatalf("expected bounded activation failure: %#v / %v", result, err)
	}
}

func TestEveryRegisteredActionHasAFullUsageContract(t *testing.T) {
	a := setupTestEnv(t)
	for _, spec := range a.ListActions() {
		docs, missing := a.ActionDocumentation([]string{spec.Name})
		if len(missing) != 0 || len(docs) != 1 {
			t.Fatalf("missing documentation for %s: docs=%#v missing=%#v", spec.Name, docs, missing)
		}
		doc := docs[0]
		for field, value := range map[string]string{"purpose": doc.Purpose, "when_to_use": doc.WhenToUse, "never_use_when": doc.NeverUseWhen, "returns": doc.Returns, "side_effects": doc.SideEffects, "approval": doc.Approval, "failure_recovery": doc.FailureRecovery} {
			if value == "" {
				t.Fatalf("action %s missing %s usage guidance", spec.Name, field)
			}
		}
		if doc.Parameters == nil || len(doc.Examples) == 0 {
			t.Fatalf("action %s missing schema or examples", spec.Name)
		}
	}
}

func TestReadFilesAcceptsPathShorthand(t *testing.T) {
	var params ReadFilesParams
	if err := json.Unmarshal([]byte(`{"files":["README.md",{"path":"go.mod","start_line":2}]}`), &params); err != nil {
		t.Fatalf("shorthand read files failed: %v", err)
	}
	if len(params.Files) != 2 || params.Files[0].Path != "README.md" || params.Files[1].Path != "go.mod" || params.Files[1].StartLine != 2 {
		t.Fatalf("unexpected normalized files: %#v", params.Files)
	}
}

func TestCodeEditAcceptsNaturalAliases(t *testing.T) {
	edit, err := toWorkspaceEdit(CodeEdit{Path: "docs/note.md", Find: "old", Replace: "new", Operation: "replace"})
	if err != nil {
		t.Fatalf("alias edit failed: %v", err)
	}
	if edit.File != "docs/note.md" || edit.Match != "old" || edit.NewCode != "new" {
		t.Fatalf("unexpected normalized edit: %#v", edit)
	}
}

func TestCommandInvocationNormalizesSimpleWhitespaceForm(t *testing.T) {
	command, args, normalized := normalizeCommandInvocation("git diff -- docs/api.md", nil)
	if !normalized || command != "git" || len(args) != 3 || args[0] != "diff" || args[2] != "docs/api.md" {
		t.Fatalf("unexpected normalization: %q %#v %v", command, args, normalized)
	}
}

func TestCommandValidationRejectsShellOperators(t *testing.T) {
	if err := validateCommandName("git"); err != nil {
		t.Fatal(err)
	}
	if err := validateCommandName("git && rm"); err == nil {
		t.Fatal("shell operator was accepted")
	}
}
