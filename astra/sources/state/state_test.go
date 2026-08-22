package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureProjectAndSessionManifests(t *testing.T) {
	root := t.TempDir()
	project, err := EnsureProject(root)
	if err != nil || project.ProjectID == "" {
		t.Fatalf("project manifest failed: %#v / %v", project, err)
	}
	session, err := EnsureSession(root, 7, "cli-test", "openai", "gpt-5.6-luna")
	if err != nil || session.ProjectID != project.ProjectID || session.Status != "active" {
		t.Fatalf("session manifest failed: %#v / %v", session, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".astra", "project.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".astra", "sessions", safe("cli-test"), "manifest.json")); err != nil {
		t.Fatal(err)
	}
	if err := CloseSession(root, "cli-test"); err != nil {
		t.Fatal(err)
	}
}
