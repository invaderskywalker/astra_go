package actions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"astra/astra/sources/state"
	"astra/astra/utils/logging"
)

func TestRunCommandUsesApprovedExternalScope(t *testing.T) {
	project := t.TempDir()
	external := t.TempDir()
	t.Setenv("ASTRA_DATA_DIR", filepath.Join(t.TempDir(), "astra"))
	registry := NewDataActionsForSessionAt(nil, 1, "scope-test", project)
	if _, err := registry.scopes.Add(external, "external", []string{"read", "execute"}); err != nil {
		t.Fatal(err)
	}
	result := registry.RunCommand(RunCommandActionParams{Command: "pwd", WorkingDirectory: external})
	resolvedExternal, _ := filepath.EvalSymlinks(external)
	if !result.Success || result.WorkingDirectory != resolvedExternal {
		t.Fatalf("external command was not authorized: %#v", result)
	}
	if _, err := os.Stat(external); err != nil {
		t.Fatal(err)
	}
}

func TestReadFilesAllowsOnlyCurrentSessionAttachments(t *testing.T) {
	project := t.TempDir()
	t.Setenv("ASTRA_DATA_DIR", filepath.Join(t.TempDir(), "astra"))
	registry := NewDataActionsForSessionAt(nil, 1, "attachment-test", project)
	attachmentRoot := filepath.Join(registry.managedSessionRoot(), "attachments")
	if err := os.MkdirAll(attachmentRoot, 0700); err != nil {
		t.Fatal(err)
	}
	attachment := filepath.Join(attachmentRoot, "input.md")
	if err := os.WriteFile(attachment, []byte("# attached\ncontent"), 0600); err != nil {
		t.Fatal(err)
	}
	result := registry.ReadFilesInRepo(ReadFilesParams{Files: []ReadFileParams{{Path: attachment}}})
	if !result.Success || !strings.Contains(fmt.Sprint(result.Diagnostics), "attached") {
		t.Fatalf("attachment was not readable: %#v", result)
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	denied := registry.ReadFilesInRepo(ReadFilesParams{Files: []ReadFileParams{{Path: outside}}})
	if denied.Success {
		t.Fatal("arbitrary absolute path was readable")
	}
}

func TestReadFilesAllowsCurrentSessionArtifacts(t *testing.T) {
	project := t.TempDir()
	t.Setenv("ASTRA_DATA_DIR", filepath.Join(t.TempDir(), "astra"))
	registry := NewDataActionsForSessionAt(nil, 1, "artifact-test", project)
	written := registry.WriteArtifact(WriteArtifactParams{Title: "assessment", Format: "markdown", Content: "# Assessment\n\nverified"})
	if !written.Success || len(written.Artifacts) != 1 {
		t.Fatalf("artifact write failed: %#v", written)
	}
	path := written.Artifacts[0]
	if filepath.Dir(path) != state.SessionArtifactsRoot(project, "artifact-test") {
		t.Fatalf("artifact path is outside the session manifest root: %s", path)
	}
	result := registry.ReadFilesInRepo(ReadFilesParams{Files: []ReadFileParams{{Path: path}}})
	if !result.Success || !strings.Contains(fmt.Sprint(result.Diagnostics), "Assessment") {
		t.Fatalf("managed artifact was not readable: %#v", result)
	}
}

func TestRunScopedArtifactUsesRunRootAndSyncRecord(t *testing.T) {
	project := t.TempDir()
	t.Setenv("ASTRA_DATA_DIR", filepath.Join(t.TempDir(), "astra"))
	registry := NewDataActionsForSessionAt(nil, 1, "run-artifact-test", project)
	registry.SetRunID("run-one")
	written := registry.WriteArtifact(WriteArtifactParams{Title: "assessment", Format: "markdown", Content: "# Assessment\n\nverified"})
	if !written.Success || len(written.Artifacts) != 1 {
		t.Fatalf("artifact write failed: %#v", written)
	}
	if filepath.Dir(written.Artifacts[0]) != state.RunArtifactsRoot(project, "run-artifact-test", "run-one") {
		t.Fatalf("artifact was not scoped to run root: %s", written.Artifacts[0])
	}
	if _, err := os.Stat(filepath.Join(state.RunSyncRoot(project, "run-artifact-test", "run-one"), "assessment-md.json")); err != nil {
		t.Fatalf("run sync record missing: %v", err)
	}
}

func TestAnalyzeFilesReturnsStructureAndRanges(t *testing.T) {
	project := t.TempDir()
	t.Setenv("ASTRA_DATA_DIR", filepath.Join(t.TempDir(), "astra"))
	if err := os.WriteFile(filepath.Join(project, "service.go"), []byte("package demo\n\nimport \"fmt\"\n\nfunc Run() {\n fmt.Println(\"ready\")\n}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	registry := NewDataActionsForSessionAt(nil, 1, "analysis-test", project)
	result := registry.AnalyzeFiles(AnalyzeFilesParams{Paths: []string{"service.go"}, Query: "ready"})
	if !result.Success {
		t.Fatalf("analysis failed: %#v", result)
	}
	profiles, ok := result.Diagnostics.([]FileAnalysis)
	if !ok || len(profiles) != 1 || profiles[0].Lines != 7 {
		t.Fatalf("unexpected analysis: %#v", result.Diagnostics)
	}
	if len(profiles[0].Symbols) == 0 || len(profiles[0].Matches) != 1 || len(profiles[0].RecommendedRanges) != 1 {
		t.Fatalf("analysis omitted structure or ranges: %#v", profiles[0])
	}
}

func TestAnalyzeFilesSkipsGeneratedCachesAndBinaryArtifacts(t *testing.T) {
	project := t.TempDir()
	t.Setenv("ASTRA_DATA_DIR", filepath.Join(t.TempDir(), "astra"))
	if err := os.MkdirAll(filepath.Join(project, "__pycache__"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(project, ".venv", "lib"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "__pycache__", "bad.pyc"), []byte("needle"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".venv", "lib", "ignored.py"), []byte("ignored"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "main.py"), []byte("def run():\n    return True\n"), 0600); err != nil {
		t.Fatal(err)
	}
	registry := NewDataActionsForSessionAt(nil, 1, "analysis-cache-test", project)
	result := registry.AnalyzeFiles(AnalyzeFilesParams{Paths: []string{"."}, Recursive: true, Limit: 20})
	if !result.Success {
		t.Fatalf("analysis failed: %#v", result)
	}
	profiles, ok := result.Diagnostics.([]FileAnalysis)
	if !ok || len(profiles) != 1 || profiles[0].Path != "main.py" {
		t.Fatalf("generated files were included in analysis: %#v", result.Diagnostics)
	}
	listed := registry.ListFiles(ListFilesParams{Path: ".", Recursive: true})
	if !listed.Success || strings.Contains(fmt.Sprint(listed.Diagnostics), "bad.pyc") {
		t.Fatalf("generated files were included in recursive listing: %#v", listed)
	}
	if err := os.WriteFile(filepath.Join(project, "main.txt"), []byte("needle"), 0600); err != nil {
		t.Fatal(err)
	}
	searched := registry.SearchCode(SearchCodeParams{Query: "needle"})
	if !searched.Success || strings.Contains(fmt.Sprint(searched.Diagnostics), "bad.pyc") {
		t.Fatalf("generated files were included in search: %#v", searched)
	}
}

func TestReadFilesRejectsHugeUnboundedFileBeforeLoading(t *testing.T) {
	project := t.TempDir()
	t.Setenv("ASTRA_DATA_DIR", filepath.Join(t.TempDir(), "astra"))
	file := filepath.Join(project, "huge.txt")
	data := make([]byte, maxReadFileBytes+1)
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	registry := NewDataActionsForSessionAt(nil, 1, "huge-test", project)
	result := registry.ReadFilesInRepo(ReadFilesParams{Files: []ReadFileParams{{Path: "huge.txt"}}})
	if result.Success || !strings.Contains(result.Error, "bounded line ranges") {
		t.Fatalf("expected bounded-read guidance, got %#v", result)
	}
}

// --- Helpers ---
func setupTestEnv(t *testing.T) *DataActions {

	logging.InitLogger() // ensures AppLogger isn’t nil
	return NewDataActions(nil, 1)
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
	analyzeFound := false
	for _, bookmark := range bookmarks {
		if bookmark.Name == "analyze_files" {
			analyzeFound = true
		}
	}
	if !analyzeFound {
		t.Fatal("analyze_files bookmark missing")
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
