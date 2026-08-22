package actions

import (
	"os"
	"strings"
	"testing"
)

func TestApplyCodeEditsDryRunDoesNotWrite(t *testing.T) {
	a := setupTestEnv(t)
	file := "test_apply_dry_run.go"
	defer os.Remove(file)
	if err := os.WriteFile(file, []byte("package sample\n// old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := a.ExecuteAction("apply_code_edits", map[string]interface{}{"dry_run": true, "edits": []map[string]interface{}{{"file": file, "operation": "replace", "match": "// old", "new_code": "// new"}}})
	if err != nil || !result.Success {
		t.Fatalf("dry run failed: %#v, %v", result, err)
	}
	contents, _ := os.ReadFile(file)
	if !strings.Contains(string(contents), "// old") {
		t.Fatal("dry run changed the file")
	}
	diagnostics := result.Diagnostics.(map[string]interface{})
	if _, ok := diagnostics["diffs"]; !ok {
		t.Fatal("dry run did not return diffs")
	}
}

func TestApplyCodeEditsAtomicValidation(t *testing.T) {
	a := setupTestEnv(t)
	first, second := "test_apply_atomic_one.txt", "test_apply_atomic_two.txt"
	defer os.Remove(first)
	defer os.Remove(second)
	os.WriteFile(first, []byte("one"), 0644)
	os.WriteFile(second, []byte("two"), 0644)
	result, err := a.ExecuteAction("apply_code_edits", map[string]interface{}{"edits": []map[string]interface{}{
		{"file": first, "operation": "replace", "match": "one", "new_code": "updated"},
		{"file": second, "operation": "replace", "match": "missing", "new_code": "updated"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatalf("expected validation failure: %#v", result)
	}
	contents, _ := os.ReadFile(first)
	if string(contents) != "one" {
		t.Fatal("transaction wrote before all edits validated")
	}
}

func TestApplyCodeEditsSupportsLegacyWholeFileUpdate(t *testing.T) {
	a := setupTestEnv(t)
	file := "test_apply_legacy.txt"
	defer os.Remove(file)
	os.WriteFile(file, []byte("before"), 0644)
	result, err := a.ExecuteAction("apply_code_edits", map[string]interface{}{"edits": []map[string]interface{}{{"type": "update_file_content", "file": file, "replacement": "after"}}})
	if err != nil || !result.Success {
		t.Fatalf("legacy update failed: %#v, %v", result, err)
	}
	contents, _ := os.ReadFile(file)
	if string(contents) != "after" {
		t.Fatalf("got %q", contents)
	}
}

func TestApplyCodeEditsSupportsLegacyDelete(t *testing.T) {
	a := setupTestEnv(t)
	file := "test_apply_delete.txt"
	defer os.Remove(file)
	if err := os.WriteFile(file, []byte("remove me"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := a.ExecuteAction("apply_code_edits", map[string]interface{}{"edits": []map[string]interface{}{{"type": "delete_file", "file": file}}})
	if err != nil || !result.Success {
		t.Fatalf("legacy delete failed: %#v, %v", result, err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("file still exists: %v", err)
	}
}
