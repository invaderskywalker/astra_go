package actions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ReadFileParams struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}
type ReadFilesParams struct {
	Files []ReadFileParams `json:"files"`
}

const (
	maxReadFileBytes  = 96 * 1024  // roughly 20–30k tokens for source text
	maxReadBatchBytes = 192 * 1024 // keep one read action bounded as a whole
)

// UnmarshalJSON accepts both the canonical object form and the convenient
// shorthand {"files":["README.md"]}. Models often produce the latter when
// reading several known files; normalize it here instead of forcing a retry.
func (p *ReadFilesParams) UnmarshalJSON(data []byte) error {
	var raw struct {
		Files []json.RawMessage `json:"files"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.Files = make([]ReadFileParams, 0, len(raw.Files))
	for _, item := range raw.Files {
		var object ReadFileParams
		if err := json.Unmarshal(item, &object); err == nil && object.Path != "" {
			p.Files = append(p.Files, object)
			continue
		}
		var path string
		if err := json.Unmarshal(item, &path); err != nil {
			return fmt.Errorf("files entry must be a path string or object: %w", err)
		}
		p.Files = append(p.Files, ReadFileParams{Path: path})
	}
	return nil
}

func (a *DataActions) ReadFilesInRepo(params ReadFilesParams) ActionResult {
	if len(params.Files) == 0 {
		return ActionResult{Success: false, Error: "files must not be empty"}
	}
	contents := make(map[string]any, len(params.Files))
	filesRead := make([]string, 0, len(params.Files))
	totalBytes := 0
	for _, request := range params.Files {
		if strings.TrimSpace(request.Path) == "" {
			return ActionResult{Success: false, Error: "file path is required"}
		}
		var data []byte
		var err error
		data, err = a.readWorkspaceOrManaged(request.Path, request.StartLine, request.EndLine)
		if err != nil {
			return ActionResult{Success: false, Error: fmt.Sprintf("read %s: %v", request.Path, err), FilesRead: filesRead}
		}
		if len(data) > maxReadFileBytes {
			return ActionResult{Success: false, Error: fmt.Sprintf("%s is %d bytes and exceeds the %d-byte read budget; use search_code or read bounded line ranges", request.Path, len(data), maxReadFileBytes), FilesRead: filesRead}
		}
		if totalBytes+len(data) > maxReadBatchBytes {
			return ActionResult{Success: false, Error: fmt.Sprintf("read batch exceeds the %d-byte context budget; split it into smaller targeted reads", maxReadBatchBytes), FilesRead: filesRead}
		}
		totalBytes += len(data)
		contents[request.Path] = string(data)
		filesRead = append(filesRead, request.Path)
	}
	return ActionResult{Success: true, Summary: fmt.Sprintf("Read %d file(s)", len(filesRead)), FilesRead: filesRead, Diagnostics: contents}
}

// readWorkspaceOrManaged keeps normal reads inside the connected project,
// while allowing explicitly attached files and Astra-owned project/session
// records to be read back by their absolute paths. Arbitrary machine paths
// remain denied.
func (a *DataActions) readWorkspaceOrManaged(path string, start, end int) ([]byte, error) {
	if !filepath.IsAbs(path) {
		if start > 0 || end > 0 {
			lines, err := a.workspace.ReadFileLines(path, start, end)
			return []byte(strings.Join(lines, "\n")), err
		}
		absolute, err := a.workspace.ResolvePath(path)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(absolute)
		if err != nil {
			return nil, err
		}
		if info.Size() > maxReadFileBytes {
			return nil, fmt.Errorf("file is %d bytes; use bounded line ranges or analyze_files first", info.Size())
		}
		return os.ReadFile(absolute)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	managedRoot, _ := filepath.EvalSymlinks(a.managedRoot)
	candidate, _ := filepath.EvalSymlinks(absolute)
	if managedRoot == "" || candidate == "" || (candidate != managedRoot && !strings.HasPrefix(candidate, managedRoot+string(os.PathSeparator))) {
		return nil, fmt.Errorf("absolute reads are allowed only for Astra-managed project/session files")
	}
	if start > 0 || end > 0 {
		data, err := os.ReadFile(candidate)
		if err != nil {
			return nil, err
		}
		lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		if start < 1 {
			start = 1
		}
		if end < 0 || end > len(lines) {
			end = len(lines)
		}
		if start > end {
			return nil, os.ErrInvalid
		}
		return []byte(strings.Join(lines[start-1:end], "\n")), nil
	}
	info, err := os.Stat(candidate)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxReadFileBytes {
		return nil, fmt.Errorf("file is %d bytes; use bounded line ranges or analyze_files first", info.Size())
	}
	return os.ReadFile(candidate)
}
