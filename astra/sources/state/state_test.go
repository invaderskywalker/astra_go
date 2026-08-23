package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureProjectAndSessionManifests(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ASTRA_DATA_DIR", filepath.Join(root, "astra-data"))
	project, err := EnsureProject(root)
	if err != nil || project.ProjectID == "" {
		t.Fatalf("project manifest failed: %#v / %v", project, err)
	}
	session, err := EnsureSession(root, 7, "cli-test", "openai", "gpt-5.6-luna")
	if err != nil || session.ProjectID != project.ProjectID || session.Status != "active" {
		t.Fatalf("session manifest failed: %#v / %v", session, err)
	}
	if _, err := os.Stat(filepath.Join(ProjectDataRoot(root), "project.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(SessionManifestPath(root, "cli-test")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".astra")); !os.IsNotExist(err) {
		t.Fatalf("Astra metadata should not be created in the project: %v", err)
	}
	if err := CloseSession(root, "cli-test"); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureSessionMigratesLegacyArtifactAndSyncRoots(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ASTRA_DATA_DIR", filepath.Join(root, "astra-data"))
	sessionID := "cli-legacy-test"
	legacyArtifact := filepath.Join(ProjectDataRoot(root), "artifacts", legacySessionName(sessionID), "report.md")
	legacySync := filepath.Join(ProjectDataRoot(root), "sessions", legacySessionName(sessionID), "sync", "report.json")
	if err := os.MkdirAll(filepath.Dir(legacyArtifact), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(legacySync), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyArtifact, []byte("# legacy"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacySync, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureSession(root, 1, sessionID, "openai", "gpt-5.6-luna"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(SessionArtifactsRoot(root, sessionID), "report.md")); err != nil {
		t.Fatalf("legacy artifact was not migrated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(SessionSyncRoot(root, sessionID), "report.json")); err != nil {
		t.Fatalf("legacy sync record was not migrated: %v", err)
	}
}
