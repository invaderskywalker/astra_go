package actions

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// FetchFileStructureParams defines parameters for fetching a repo structure.
type FetchFileStructureParams struct {
	Path       string   `json:"path"`        // Base path (defaults to current dir if empty)
	IgnoreDirs []string `json:"ignore_dirs"` // Directories to ignore (e.g. ".git", "logs", "node_modules")
}

// FetchFileStructureResult holds the resulting file tree.
type FetchFileStructureResult struct {
	Structure string `json:"structure"`       // Formatted tree output
	Error     string `json:"error,omitempty"` // Error message, if any
}

// FetchFileStructureInRepo runs the `tree` command and captures its output.
// It automatically includes the `-I` flag for ignored directories.
func (a *DataActions) FetchFileStructureInRepo(params FetchFileStructureParams) FetchFileStructureResult {
	path := params.Path
	if path == "" {
		path = "." // default to current directory
	}

	// Construct ignore pattern for `tree -I`
	ignorePattern := ""
	if len(params.IgnoreDirs) > 0 {
		ignorePattern = strings.Join(params.IgnoreDirs, "|")
	}

	var cmd *exec.Cmd
	if ignorePattern != "" {
		cmd = exec.Command("tree", path, "-I", ignorePattern)
	} else {
		cmd = exec.Command("tree", path)
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		return FetchFileStructureResult{Error: fmt.Sprintf("tree command failed: %v\n%s", err, out.String())}
	}

	return FetchFileStructureResult{Structure: out.String()}
}
