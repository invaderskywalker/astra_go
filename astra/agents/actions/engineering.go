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
type CommandRequest struct {
	Command          string   `json:"command"`
	Args             []string `json:"args,omitempty"`
	WorkingDirectory string   `json:"working_directory,omitempty"`
	TimeoutSeconds   int      `json:"timeout_seconds,omitempty"`
	AllowFailure     bool     `json:"allow_failure,omitempty"`
}
type RunCommandsParams struct {
	Commands        []CommandRequest `json:"commands"`
	ContinueOnError bool             `json:"continue_on_error,omitempty"`
}
type RunTestsParams struct {
	Package        string `json:"package,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func (a *DataActions) RunCommand(params RunCommandActionParams) ActionResult {
	if strings.TrimSpace(params.Command) == "" {
		return ActionResult{Success: false, Error: "command is required"}
	}
	command, args, normalized := normalizeCommandInvocation(params.Command, params.Args)
	params.Command, params.Args = command, args
	if err := validateCommandName(params.Command); err != nil {
		return ActionResult{Success: false, Error: err.Error()}
	}
	result := a.workspace.RunCommand(workspace.RunCommandParams{Cmd: params.Command, Args: params.Args, Cwd: params.WorkingDirectory, TimeoutSec: params.TimeoutSeconds})
	actionResult := commandResult(params.Command, result)
	if normalized {
		actionResult.Warnings = append(actionResult.Warnings, "normalized a whitespace-separated command into executable and arguments")
	}
	return actionResult
}

// RunCommands executes a deliberate sequence without forcing the model to
// encode shell syntax. Each command keeps its own working directory and
// timeout, and the action records every result in order.
func (a *DataActions) RunCommands(params RunCommandsParams) ActionResult {
	if len(params.Commands) == 0 {
		return ActionResult{Success: false, Error: "commands must not be empty"}
	}
	results := make([]workspace.RunCommandResult, 0, len(params.Commands))
	succeeded := 0
	for index, command := range params.Commands {
		if strings.TrimSpace(command.Command) == "" {
			return ActionResult{Success: false, Error: fmt.Sprintf("commands[%d].command is required", index), Diagnostics: results}
		}
		executable, args, _ := normalizeCommandInvocation(command.Command, command.Args)
		if err := validateCommandName(executable); err != nil {
			return ActionResult{Success: false, Error: fmt.Sprintf("commands[%d]: %v", index, err), Diagnostics: results}
		}
		result := a.workspace.RunCommand(workspace.RunCommandParams{
			Cmd: executable, Args: args, Cwd: command.WorkingDirectory, TimeoutSec: command.TimeoutSeconds,
		})
		results = append(results, result)
		if result.Error != "" {
			if command.AllowFailure {
				continue
			}
			if !params.ContinueOnError {
				return ActionResult{Success: false, Summary: fmt.Sprintf("Stopped after command %d failed", index+1), Diagnostics: results, Error: result.Error}
			}
			continue
		}
		succeeded++
	}
	return ActionResult{Success: succeeded == len(results), Summary: fmt.Sprintf("Completed %d/%d command(s)", succeeded, len(params.Commands)), Diagnostics: results}
}

func validateCommandName(command string) error {
	if strings.ContainsAny(command, "\t\r\n;&|><") {
		return fmt.Errorf("command must contain only the executable name; put arguments in args or use run_commands")
	}
	return nil
}

func normalizeCommandInvocation(command string, args []string) (string, []string, bool) {
	if len(args) > 0 || !strings.ContainsAny(command, " \t") {
		return command, args, false
	}
	parts := strings.Fields(command)
	if len(parts) < 2 {
		return command, args, false
	}
	return parts[0], parts[1:], true
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
		return ActionResult{Success: false, Summary: command + " failed", ExitCode: result.ExitCode, WorkingDirectory: result.WorkingDirectory, Stdout: result.Stdout, Stderr: result.Stderr, Diagnostics: diagnostics, Error: result.Error, Duration: result.Duration}
	}
	return ActionResult{Success: true, Summary: fmt.Sprintf("%s completed", command), ExitCode: result.ExitCode, WorkingDirectory: result.WorkingDirectory, Stdout: result.Stdout, Stderr: result.Stderr, Diagnostics: diagnostics, Duration: result.Duration}
}
