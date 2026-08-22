package actions

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
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
	return ActionResult{Success: true, Summary: "Artifact written: " + path, FilesWritten: []string{path}, Artifacts: []string{path}}
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
