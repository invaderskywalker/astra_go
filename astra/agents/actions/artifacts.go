package actions

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// WriteArtifactParams deliberately accepts content rather than a path. Astra owns
// the destination, preventing a planning model from scattering files across a repo.
type WriteArtifactParams struct {
	Title   string `json:"title"`
	Format  string `json:"format"` // markdown, json, jsonl, csv, text
	Content string `json:"content"`
}

func (a *DataActions) WriteArtifact(params WriteArtifactParams) ActionResult {
	if strings.TrimSpace(params.Title) == "" || strings.TrimSpace(params.Content) == "" {
		return ActionResult{Success: false, Error: "title and content are required"}
	}
	format := strings.ToLower(strings.TrimSpace(params.Format))
	if format == "" {
		format = "markdown"
	}
	extension, err := validateArtifact(format, params.Content)
	if err != nil {
		return ActionResult{Success: false, Error: err.Error()}
	}
	name := artifactName(params.Title)
	if name == "" {
		return ActionResult{Success: false, Error: "title must contain letters or numbers"}
	}
	path := filepath.ToSlash(filepath.Join(".astra", "artifacts", safeSessionName(a.memorySessionID()), name+extension))
	if err := a.workspace.CreateFile(path, []byte(params.Content)); err != nil {
		return ActionResult{Success: false, Error: fmt.Sprintf("write artifact: %v", err)}
	}
	warnings := a.syncManagedFile(path, []byte(params.Content), artifactContentType(format))
	return ActionResult{Success: true, Summary: "Artifact written: " + path, FilesWritten: []string{path}, Artifacts: []string{path}, Warnings: warnings}
}

func artifactContentType(format string) string {
	switch format {
	case "json", "jsonl":
		return "application/json"
	case "csv":
		return "text/csv"
	case "text", "txt":
		return "text/plain"
	default:
		return "text/markdown"
	}
}

// syncManagedFile mirrors Astra-owned artifacts only. Source repositories are
// never uploaded implicitly; a future explicit workspace-sync action can add
// an approval boundary for that larger operation.
func (a *DataActions) syncManagedFile(path string, data []byte, contentType string) []string {
	key := filepath.ToSlash(filepath.Join("users", fmt.Sprintf("%d", a.UserID), "projects", safeSessionName(filepath.Base(a.workspace.Root)), "sessions", safeSessionName(a.memorySessionID()), strings.TrimPrefix(path, ".astra/")))
	if a.mirror == nil {
		a.writeSyncRecord(path, key, "local_only", "MinIO is not configured")
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.mirror.PutObject(ctx, key, data, contentType); err != nil {
		a.writeSyncRecord(path, key, "pending", err.Error())
		return []string{"MinIO sync pending: " + err.Error()}
	}
	a.writeSyncRecord(path, key, "synced", "")
	return nil
}

func (a *DataActions) writeSyncRecord(path, key, status, message string) {
	record := map[string]any{"path": path, "object_key": key, "status": status, "message": message, "updated_at": time.Now().UTC()}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return
	}
	rel := filepath.ToSlash(filepath.Join(".astra", "sync", safeSessionName(a.memorySessionID()), artifactName(filepath.Base(path))+".json"))
	absolute := filepath.Join(a.workspace.Root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(absolute), 0755); err != nil {
		return
	}
	_ = os.WriteFile(absolute, append(data, '\n'), 0644)
}

func (a *DataActions) memorySessionID() string { return a.memory.SessionID() }

func validateArtifact(format, content string) (string, error) {
	switch format {
	case "markdown", "md":
		return ".md", nil
	case "text", "txt":
		return ".txt", nil
	case "json":
		var value any
		if err := json.Unmarshal([]byte(content), &value); err != nil {
			return "", fmt.Errorf("invalid JSON artifact: %w", err)
		}
		return ".json", nil
	case "jsonl":
		for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
			var value any
			if err := json.Unmarshal([]byte(line), &value); err != nil {
				return "", fmt.Errorf("invalid JSONL artifact: %w", err)
			}
		}
		return ".jsonl", nil
	case "csv":
		if _, err := csv.NewReader(strings.NewReader(content)).ReadAll(); err != nil {
			return "", fmt.Errorf("invalid CSV artifact: %w", err)
		}
		return ".csv", nil
	default:
		return "", fmt.Errorf("unsupported artifact format %q; use markdown, json, jsonl, csv, or text", format)
	}
}

func artifactName(title string) string {
	var out strings.Builder
	lastDash := false
	for _, char := range strings.ToLower(strings.TrimSpace(title)) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			out.WriteRune(char)
			lastDash = false
		} else if !lastDash {
			out.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

func safeSessionName(value string) string {
	if name := artifactName(value); name != "" {
		return name
	}
	return "unsessioned"
}
