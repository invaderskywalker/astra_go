package actions

import (
	"encoding/json"
	"fmt"
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
	for _, request := range params.Files {
		if strings.TrimSpace(request.Path) == "" {
			return ActionResult{Success: false, Error: "file path is required"}
		}
		var data []byte
		var err error
		if request.StartLine > 0 || request.EndLine > 0 {
			lines, readErr := a.workspace.ReadFileLines(request.Path, request.StartLine, request.EndLine)
			err = readErr
			data = []byte(strings.Join(lines, "\n"))
		} else {
			data, err = a.workspace.ReadFile(request.Path)
		}
		if err != nil {
			return ActionResult{Success: false, Error: fmt.Sprintf("read %s: %v", request.Path, err), FilesRead: filesRead}
		}
		if len(data) > 2*1024*1024 {
			return ActionResult{Success: false, Error: fmt.Sprintf("%s is too large to display", request.Path), FilesRead: filesRead}
		}
		contents[request.Path] = string(data)
		filesRead = append(filesRead, request.Path)
	}
	return ActionResult{Success: true, Summary: fmt.Sprintf("Read %d file(s)", len(filesRead)), FilesRead: filesRead, Diagnostics: contents}
}
