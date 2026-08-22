package actions

import (
	"fmt"
	"path/filepath"
	"strings"
)

type ListFilesParams struct {
	Path      string `json:"path,omitempty"`
	Recursive bool   `json:"recursive,omitempty"`
}
type InspectFileParams struct {
	Path string `json:"path"`
}

func (a *DataActions) ListFiles(params ListFilesParams) ActionResult {
	files, err := a.workspace.ListFiles(params.Path, params.Recursive)
	if err != nil {
		return ActionResult{Success: false, Error: err.Error()}
	}
	return ActionResult{Success: true, Summary: fmt.Sprintf("Listed %d entries", len(files)), Diagnostics: files}
}

func (a *DataActions) InspectFile(params InspectFileParams) ActionResult {
	if strings.TrimSpace(params.Path) == "" {
		return ActionResult{Success: false, Error: "path is required"}
	}
	if filepath.Ext(params.Path) != ".go" {
		return ActionResult{Success: false, Error: "inspect_file currently supports Go source files only"}
	}
	inspection, err := a.workspace.InspectGoFile(params.Path)
	if err != nil {
		return ActionResult{Success: false, Error: err.Error()}
	}
	return ActionResult{Success: true, Summary: "Inspected " + params.Path, FilesRead: []string{params.Path}, Diagnostics: inspection}
}
