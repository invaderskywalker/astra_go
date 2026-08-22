package mindpalace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreSavesSearchesAndLinksMemory(t *testing.T) {
	store := New(t.TempDir(), 42, "session-1", nil)
	first, _, err := store.Save(context.Background(), Record{Kind: "decision", Title: "Backend", Summary: "Use Go", Content: "Build the backend in Go.", Tags: []string{"backend", "go"}})
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := store.Save(context.Background(), Record{Kind: "project-note", Title: "Frontend", Summary: "Use React", Content: "Build the frontend in React.", Tags: []string{"frontend", "react"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Link(context.Background(), first.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	matches, err := store.Search("backend", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].ID != first.ID {
		t.Fatalf("unexpected matches: %#v", matches)
	}
	data, err := os.ReadFile(filepath.Join(store.userRoot(), "memory", "decision", first.ID+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), second.ID) {
		t.Fatal("related memory link was not rendered")
	}
	if _, err := os.Stat(store.indexPath()); err != nil {
		t.Fatalf("index was not created: %v", err)
	}
}

func TestStoreAppendsSessionEvidence(t *testing.T) {
	store := New(t.TempDir(), 7, "session-42", nil)
	if _, err := store.AppendSessionEvent(context.Background(), "plan", map[string]string{"goal": "test"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(store.sessionPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"type":"plan"`) {
		t.Fatalf("event was not written: %s", data)
	}
}
