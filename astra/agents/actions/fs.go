package actions

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("⚠️ Recovered from panic in FetchFileStructureInRepo: %v\n", r)
		}
	}()

	path := params.Path
	if path == "" {
		path = "."
	}

	// Defensive: if IgnoreDirs is nil, initialize it
	if params.IgnoreDirs == nil {
		params.IgnoreDirs = []string{}
	}

	// Construct ignore pattern for `tree -I`
	ignoreArr := "venv|node_modules"
	ignorePattern := ignoreArr
	if len(params.IgnoreDirs) > 0 {
		ignorePattern = fmt.Sprintf("%s|%s", ignoreArr, strings.Join(params.IgnoreDirs, "|"))
	}

	// Verify that the tree binary exists
	treePath, err := exec.LookPath("tree")
	if err != nil {
		return FetchFileStructureResult{Error: "the 'tree' command is not installed or not in PATH"}
	}

	var cmd *exec.Cmd
	if ignorePattern != "" {
		cmd = exec.Command(treePath, path, "-I", ignorePattern)
	} else {
		cmd = exec.Command(treePath, path)
	}

	fmt.Println("cmd:", cmd.String())

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	// Safely run command
	if err := cmd.Run(); err != nil {
		return FetchFileStructureResult{
			Error: fmt.Sprintf("tree command failed: %v\n%s", err, out.String()),
		}
	}

	// Safety: Limit very large output
	output := out.String()
	if len(output) > 500000 { // ~500KB limit
		output = output[:500000] + "\n... (output truncated)"
	}

	return FetchFileStructureResult{
		Structure: output,
	}
}

// ReadFileParams defines parameters for reading a specific file in the repo.
type ReadFileParams struct {
	Path string `json:"path"` // Full or relative path to the file
}

// ReadFileResult defines the output containing the file’s contents.
type ReadFileResult struct {
	Path    string `json:"path"`              // Path of the file read
	Content string `json:"content,omitempty"` // File content (if read successfully)
	Error   string `json:"error,omitempty"`   // Error message, if any
}

type ReadFilesParams struct {
	Paths []string `json:"paths"` // List of files to read
}

type ReadFilesResult struct {
	Results []ReadFileResult `json:"results"`
}

// ReadFileInRepo reads the contents of a file within the repository.
func (a *DataActions) ReadFileInRepo(params ReadFileParams) ReadFileResult {
	if params.Path == "" {
		return ReadFileResult{Error: "file path is required"}
	}

	// Resolve absolute path
	absPath, err := filepath.Abs(params.Path)
	if err != nil {
		return ReadFileResult{Error: fmt.Sprintf("failed to resolve path: %v", err)}
	}

	// Ensure it's within repo root
	repoRoot, err := os.Getwd()
	if err != nil {
		return ReadFileResult{Error: fmt.Sprintf("failed to get working directory: %v", err)}
	}

	if !strings.HasPrefix(absPath, repoRoot) {
		return ReadFileResult{Error: "access denied: file outside repository root"}
	}

	// Read file using modern API
	data, err := os.ReadFile(absPath)
	if err != nil {
		return ReadFileResult{
			Path:  absPath,
			Error: fmt.Sprintf("failed to read file: %v", err),
		}
	}

	// Safety: limit file size
	if len(data) > 2*1024*1024 { // 2MB
		return ReadFileResult{
			Path:  absPath,
			Error: "file too large to display (>2MB)",
		}
	}

	return ReadFileResult{
		Path:    absPath,
		Content: string(data),
	}
}

func (a *DataActions) ReadFilesInRepo(params ReadFilesParams) ReadFilesResult {
	if len(params.Paths) == 0 {
		return ReadFilesResult{
			Results: []ReadFileResult{{Error: "no paths provided"}},
		}
	}

	results := make([]ReadFileResult, 0, len(params.Paths))
	for _, p := range params.Paths {
		res := a.ReadFileInRepo(ReadFileParams{Path: p})
		results = append(results, res)
	}

	return ReadFilesResult{Results: results}
}

func (a *DataActions) GetPWD() (map[string]interface{}, error) {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting current directory: %v", err)
	}

	fmt.Println("GetPWD_GetPWD ", dir)

	return map[string]interface{}{
		"success": true,
		"result":  dir,
	}, nil
}
