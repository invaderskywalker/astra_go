package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type RunCommandParams struct {
	Cmd        string   `json:"cmd"`
	Args       []string `json:"args,omitempty"`
	Cwd        string   `json:"cwd,omitempty"`
	TimeoutSec int      `json:"timeout_sec,omitempty"`
}
type RunCommandResult struct {
	Command          string        `json:"command"`
	WorkingDirectory string        `json:"working_directory"`
	Stdout           string        `json:"stdout,omitempty"`
	Stderr           string        `json:"stderr,omitempty"`
	Error            string        `json:"error,omitempty"`
	ExitCode         int           `json:"exit_code"`
	Duration         time.Duration `json:"duration"`
}

func (w *Workspace) RunCommand(params RunCommandParams) RunCommandResult {
	result := RunCommandResult{Command: strings.Join(append([]string{params.Cmd}, params.Args...), " ")}
	if params.Cmd == "" {
		result.Error = "command is required"
		return result
	}
	cwd, err := w.abs(params.Cwd)
	if err != nil {
		result.Error = "invalid working directory: " + err.Error()
		return result
	}
	result.WorkingDirectory = cwd
	resolvedRoot, err := filepath.EvalSymlinks(w.Root)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	resolvedCWD, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if resolvedCWD != resolvedRoot && !strings.HasPrefix(resolvedCWD, resolvedRoot+string(os.PathSeparator)) {
		result.Error = "working directory is outside the workspace"
		return result
	}
	timeout := time.Duration(params.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	started := time.Now()
	cmd := exec.CommandContext(ctx, params.Cmd, params.Args...)
	cmd.Dir = resolvedCWD
	stdout, stderr := &strings.Builder{}, &strings.Builder{}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err = cmd.Run()
	result.Duration, result.Stdout, result.Stderr = time.Since(started), stdout.String(), stderr.String()
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	if ctx.Err() == context.DeadlineExceeded {
		result.Error = "command timed out"
	} else if err != nil {
		result.Error = err.Error()
	}
	return result
}
