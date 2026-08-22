// Package state stores small, portable manifests that make Astra's project,
// session, and managed-file views discoverable without reading the entire
// workspace or memory archive.
package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type ProjectManifest struct {
	ProjectID  string    `json:"project_id"`
	Name       string    `json:"name"`
	Root       string    `json:"root"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

type SessionManifest struct {
	SessionID       string    `json:"session_id"`
	UserID          int       `json:"user_id"`
	ProjectID       string    `json:"project_id"`
	WorkspaceRoot   string    `json:"workspace_root"`
	ArtifactsRoot   string    `json:"artifacts_root"`
	AttachmentsRoot string    `json:"attachments_root"`
	StartedAt       time.Time `json:"started_at"`
	LastSeenAt      time.Time `json:"last_seen_at"`
	Status          string    `json:"status"`
	Provider        string    `json:"provider"`
	Model           string    `json:"model"`
}

func ProjectID(root string) string {
	digest := sha256.Sum256([]byte(root))
	return "project_" + hex.EncodeToString(digest[:])[:16]
}

func EnsureProject(root string) (ProjectManifest, error) {
	path := filepath.Join(root, ".astra", "project.json")
	var manifest ProjectManifest
	data, err := os.ReadFile(path)
	if err == nil {
		if json.Unmarshal(data, &manifest) == nil && manifest.ProjectID != "" {
			manifest.LastSeenAt = time.Now().UTC()
			return manifest, writeJSON(path, manifest)
		}
	}
	now := time.Now().UTC()
	manifest = ProjectManifest{ProjectID: ProjectID(root), Name: filepath.Base(root), Root: root, CreatedAt: now, LastSeenAt: now}
	return manifest, writeJSON(path, manifest)
}

func EnsureSession(root string, userID int, sessionID, provider, model string) (SessionManifest, error) {
	project, err := EnsureProject(root)
	if err != nil {
		return SessionManifest{}, err
	}
	sessionRoot := filepath.Join(root, ".astra", "sessions", safe(sessionID))
	path := filepath.Join(sessionRoot, "manifest.json")
	var manifest SessionManifest
	data, readErr := os.ReadFile(path)
	if readErr == nil {
		_ = json.Unmarshal(data, &manifest)
	}
	now := time.Now().UTC()
	if manifest.SessionID == "" {
		manifest = SessionManifest{SessionID: sessionID, UserID: userID, ProjectID: project.ProjectID, WorkspaceRoot: root, ArtifactsRoot: filepath.Join(root, ".astra", "artifacts", safe(sessionID)), AttachmentsRoot: filepath.Join(root, ".astra", "attachments"), StartedAt: now, Status: "active", Provider: provider, Model: model}
	} else {
		manifest.LastSeenAt = now
		manifest.Provider, manifest.Model, manifest.Status = provider, model, "active"
	}
	if err := writeJSON(path, manifest); err != nil {
		return SessionManifest{}, err
	}
	return manifest, nil
}

func CloseSession(root string, sessionID string) error {
	path := filepath.Join(root, ".astra", "sessions", safe(sessionID), "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var manifest SessionManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return err
	}
	manifest.Status = "closed"
	manifest.LastSeenAt = time.Now().UTC()
	return writeJSON(path, manifest)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".astra-state-")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}

func safe(value string) string {
	if value == "" {
		return "unknown"
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(value)))[:24]
}
