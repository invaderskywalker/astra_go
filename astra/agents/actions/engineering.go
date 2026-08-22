package actions

import (
	"astra/astra/agents/workspace"
	"fmt"
	"strings"
)

type RunCommandActionParams struct {
	Command          string   `json:"command"`
	Args             []string `json:"args,omitempty"`
	WorkingDirectory string   `json:"working_directory,omitempty"`
	TimeoutSeconds   int      `json:"timeout_seconds,omitempty"`
}
type RunTestsParams struct {
	Package        string `json:"package,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func (a *DataActions) RunCommand(params RunCommandActionParams) ActionResult {
	if strings.TrimSpace(params.Command) == "" {
		return ActionResult{Success: false, Error: "command is required"}
	}
	result := a.workspace.RunCommand(workspace.RunCommandParams{Cmd: params.Command, Args: params.Args, Cwd: params.WorkingDirectory, TimeoutSec: params.TimeoutSeconds})
	return commandResult(params.Command, result)
}

func (a *DataActions) BuildProject(_ struct{}) ActionResult {
	return commandResult("go build ./...", a.workspace.RunCommand(workspace.RunCommandParams{Cmd: "go", Args: []string{"build", "./..."}, TimeoutSec: 120}))
}

func (a *DataActions) RunTests(params RunTestsParams) ActionResult {
	target := params.Package
	if target == "" {
		target = "./..."
	}
	return commandResult("go test "+target, a.workspace.RunCommand(workspace.RunCommandParams{Cmd: "go", Args: []string{"test", target}, TimeoutSec: params.TimeoutSeconds}))
}

func (a *DataActions) GitStatus(_ struct{}) ActionResult {
	return commandResult("git status --short", a.workspace.RunCommand(workspace.RunCommandParams{Cmd: "git", Args: []string{"status", "--short"}, TimeoutSec: 30}))
}

func commandResult(command string, result workspace.RunCommandResult) ActionResult {
	diagnostics := ParseGoDiagnostics(result.Stdout + "\n" + result.Stderr)
	if result.Error != "" {
		return ActionResult{Success: false, Summary: command + " failed", ExitCode: result.ExitCode, Stdout: result.Stdout, Stderr: result.Stderr, Diagnostics: diagnostics, Error: result.Error, Duration: result.Duration}
	}
	return ActionResult{Success: true, Summary: fmt.Sprintf("%s completed", command), ExitCode: result.ExitCode, Stdout: result.Stdout, Stderr: result.Stderr, Diagnostics: diagnostics, Duration: result.Duration}
}
