package promptstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptProfilesPersistAndContextOnlyUsesEnabled(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "prompts"))
	if _, err := store.Save("Reviewer", "Review code carefully", "Never skip tests.", true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save("Disabled", "Not active", "Ignore this.", false); err != nil {
		t.Fatal(err)
	}
	context, err := store.Context(1000)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(context, "Never skip tests") || strings.Contains(context, "Ignore this") {
		t.Fatalf("unexpected context: %s", context)
	}
	if _, err := os.Stat(filepath.Join(store.Root(), "reviewer.md")); err != nil {
		t.Fatal(err)
	}
}
