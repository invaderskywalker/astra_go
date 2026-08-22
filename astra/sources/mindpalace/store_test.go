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
	linkedData, err := os.ReadFile(filepath.Join(store.userRoot(), "memory", "project-note", second.ID+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(linkedData), first.ID) {
		t.Fatal("reciprocal memory link was not rendered")
	}
	if _, err := os.Stat(store.indexPath()); err != nil {
		t.Fatalf("index was not created: %v", err)
	}
}

func TestStoreRefinesStableMemoryIdentity(t *testing.T) {
	store := New(t.TempDir(), 42, "session-1", nil)
	first, _, err := store.Save(context.Background(), Record{Kind: "decision", Title: "Backend", Content: "Start with Go.", Importance: 2})
	if err != nil {
		t.Fatal(err)
	}
	updated, _, err := store.Save(context.Background(), Record{Kind: "decision", Title: "Backend", Content: "Use Go with explicit interfaces.", Importance: 5, Confidence: "confirmed"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != updated.ID {
		t.Fatalf("expected refinement to retain ID: %s != %s", first.ID, updated.ID)
	}
	records, err := store.List("decision")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Content != updated.Content || records[0].Importance != 5 {
		t.Fatalf("unexpected refined record: %#v", records)
	}
}

func TestStoreSearchRanksTitleAndImportance(t *testing.T) {
	store := New(t.TempDir(), 42, "session-1", nil)
	_, _, err := store.Save(context.Background(), Record{Kind: "note", Title: "General note", Content: "Astra works with Go.", Importance: 1})
	if err != nil {
		t.Fatal(err)
	}
	preferred, _, err := store.Save(context.Background(), Record{Kind: "decision", Title: "Go architecture", Content: "Use Go for backend.", Importance: 5})
	if err != nil {
		t.Fatal(err)
	}
	matches, err := store.Search("go", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 || matches[0].ID != preferred.ID {
		t.Fatalf("expected ranked result first, got %#v", matches)
	}
}

func TestStoreContextIncludesRelatedMemory(t *testing.T) {
	store := New(t.TempDir(), 42, "session-1", nil)
	decision, _, err := store.Save(context.Background(), Record{Kind: "decision", Title: "Use Go", Summary: "Backend language", Content: "Use Go for the backend.", Confidence: "confirmed"})
	if err != nil {
		t.Fatal(err)
	}
	constraint, _, err := store.Save(context.Background(), Record{Kind: "constraint", Title: "Local first", Summary: "Keep data portable", Content: "Durable learning belongs in files.", Importance: 4})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Link(context.Background(), decision.ID, constraint.ID); err != nil {
		t.Fatal(err)
	}
	pack, err := store.Context("backend language", 3)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pack, decision.ID) || !strings.Contains(pack, constraint.ID) {
		t.Fatalf("context omitted linked memory: %s", pack)
	}
}

func TestStoreDoesNotRecallArchivedMemory(t *testing.T) {
	store := New(t.TempDir(), 42, "session-1", nil)
	_, _, err := store.Save(context.Background(), Record{Kind: "decision", Title: "Old choice", Content: "Use the old approach.", Status: "archived"})
	if err != nil {
		t.Fatal(err)
	}
	matches, err := store.Search("old choice", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("archived memory was recalled: %#v", matches)
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
