package scope

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestScopeAddAuthorizeAndRevoke(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	store := New(filepath.Join(t.TempDir(), "scopes.json"))
	entry, err := store.Add(root, "workspace", []string{Read, Execute})
	if err != nil {
		t.Fatal(err)
	}
	if entry.ID == "" || len(entry.Permissions) != 2 {
		t.Fatalf("unexpected entry: %#v", entry)
	}
	if _, err := store.Authorize(child, Read); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authorize(child, Write); !errors.Is(err, ErrNoScope) {
		t.Fatalf("expected write denial, got %v", err)
	}
	if _, err := store.Revoke(entry.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authorize(root, Read); !errors.Is(err, ErrNoScope) {
		t.Fatalf("expected revoked scope denial, got %v", err)
	}
}

func TestScopeNeverTreatsSiblingAsChild(t *testing.T) {
	parent := t.TempDir()
	sibling := parent + "-sibling"
	if err := os.Mkdir(sibling, 0700); err != nil {
		t.Fatal(err)
	}
	store := New(filepath.Join(t.TempDir(), "scopes.json"))
	if _, err := store.Add(parent, "parent", []string{Read}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authorize(sibling, Read); !errors.Is(err, ErrNoScope) {
		t.Fatalf("sibling path was incorrectly authorized: %v", err)
	}
}
