package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTUIPlanFormattingIsReadable(t *testing.T) {
	text := formatTUIPlan(map[string]interface{}{
		"status":                             "in_progress",
		"mode":                               "task",
		"goal":                               "Inspect the repository and report evidence",
		"selected_skills":                    []interface{}{"repository_intelligence"},
		"mind_map_steps_in_natural_language": []interface{}{"List the root", "Detect the stack"},
		"success_criteria":                   []interface{}{"No unsupported claims"},
		"remaining_work":                     []interface{}{"Run validation"},
		"next_action":                        "Run validation",
	})
	for _, want := range []string{"Status: in_progress", "Mode: task", "Goal: Inspect", "Mind map:", "1. List the root", "No unsupported claims", "Remaining: Run validation", "Next: Run validation"} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted plan missing %q: %s", want, text)
		}
	}
}

func TestTUICollectFilesSkipsCachesAndSorts(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".astra"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "z.md"), []byte("z"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "ignored.js"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".astra", "managed.json"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	files := collectTUIFiles(root)
	if len(files) != 2 || !strings.Contains(files[0], "a.md") || !strings.Contains(files[1], "z.md") {
		t.Fatalf("unexpected file view: %#v", files)
	}
}

func TestTUIWrapNeverEmitsBlankLeadingLine(t *testing.T) {
	wrapped := tuiWrap("one two three", 7)
	if strings.HasPrefix(wrapped, "\n") || !strings.Contains(wrapped, "\n") {
		t.Fatalf("expected wrapped text, got %q", wrapped)
	}
}
