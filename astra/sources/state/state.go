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

	"astra/astra/config"
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

// ProjectDataRoot is the Astra-owned storage area for a connected project.
// It intentionally lives outside the repository so connecting to many
// directories does not scatter Astra metadata through source trees.
func ProjectDataRoot(root string) string { return config.ProjectDataRoot(root) }

func projectManifestPath(root string) string {
	return filepath.Join(ProjectDataRoot(root), "project.json")
}

func SessionRoot(root, sessionID string) string {
	return filepath.Join(ProjectDataRoot(root), "sessions", safe(sessionID))
}

func SessionManifestPath(root, sessionID string) string {
	return filepath.Join(SessionRoot(root, sessionID), "manifest.json")
}

func SessionHistoryPath(root, sessionID string) string {
	return filepath.Join(SessionRoot(root, sessionID), "chat.jsonl")
}

func SessionArtifactsRoot(root, sessionID string) string {
	return filepath.Join(ProjectDataRoot(root), "artifacts", safe(sessionID))
}

func SessionAttachmentsRoot(root, sessionID string) string {
	return filepath.Join(SessionRoot(root, sessionID), "attachments")
}

func SessionSyncRoot(root, sessionID string) string {
	return filepath.Join(SessionRoot(root, sessionID), "sync")
}

func ProjectID(root string) string {
	digest := sha256.Sum256([]byte(filepath.Clean(root)))
	return "project_" + hex.EncodeToString(digest[:])[:16]
}

func EnsureProject(root string) (ProjectManifest, error) {
	dataRoot := ProjectDataRoot(root)
	if err := migrateLegacyProject(filepath.Join(root, ".astra"), dataRoot); err != nil {
		return ProjectManifest{}, err
	}
	path := filepath.Join(dataRoot, "project.json")
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

// migrateLegacyProject makes the storage move non-destructive for projects
// created by older Astra versions. It copies only missing managed files and
// leaves the original .astra tree recoverable for the user to remove later.
func migrateLegacyProject(legacyRoot, targetRoot string) error {
	if _, err := os.Stat(legacyRoot); os.IsNotExist(err) {
		return nil
	}
	marker := filepath.Join(targetRoot, ".legacy-migrated")
	if _, err := os.Stat(marker); err == nil {
		return nil
	}
	for _, name := range []string{"project.json", "sessions", "artifacts", "attachments", "sync"} {
		if err := copyMissing(filepath.Join(legacyRoot, name), filepath.Join(targetRoot, name)); err != nil {
			return err
		}
	}
	return writeJSON(marker, map[string]any{"migrated_at": time.Now().UTC(), "source": legacyRoot})
}

func copyMissing(source, target string) error {
	_, err := os.Stat(source)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return filepath.Walk(source, func(path string, fileInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := target
		if rel != "." {
			destination = filepath.Join(target, rel)
		}
		if fileInfo.IsDir() {
			return os.MkdirAll(destination, 0700)
		}
		if _, err := os.Stat(destination); err == nil {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0600)
	})
}

func EnsureSession(root string, userID int, sessionID, provider, model string) (SessionManifest, error) {
	project, err := EnsureProject(root)
	if err != nil {
		return SessionManifest{}, err
	}
	path := SessionManifestPath(root, sessionID)
	var manifest SessionManifest
	data, readErr := os.ReadFile(path)
	if readErr == nil {
		_ = json.Unmarshal(data, &manifest)
	}
	now := time.Now().UTC()
	if manifest.SessionID == "" {
		manifest = SessionManifest{SessionID: sessionID, UserID: userID, ProjectID: project.ProjectID, WorkspaceRoot: root, ArtifactsRoot: SessionArtifactsRoot(root, sessionID), AttachmentsRoot: SessionAttachmentsRoot(root, sessionID), StartedAt: now, Status: "active", Provider: provider, Model: model}
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
	path := SessionManifestPath(root, sessionID)
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
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
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
