// Package actions defines the standard ActionResult returned from every action in the system.
package actions

import (
	"time"
)

// ActionResult is the unified return type for all actions executed by Astra.
type ActionResult struct {
	Success          bool          `json:"success"`           // Whether the action succeeded (true/false)
	Summary          string        `json:"summary,omitempty"` // Brief human-readable summary
	ExitCode         int           `json:"exit_code,omitempty"`
	WorkingDirectory string        `json:"working_directory,omitempty"`
	Stdout           string        `json:"stdout,omitempty"`
	Stderr           string        `json:"stderr,omitempty"`
	Diagnostics      interface{}   `json:"diagnostics,omitempty"` // Can hold parsed error details (see Milestone 3)
	FilesRead        []string      `json:"files_read,omitempty"`
	FilesWritten     []string      `json:"files_written,omitempty"`
	Artifacts        []string      `json:"artifacts,omitempty"`
	Duration         time.Duration `json:"duration,omitempty"` // Execution duration
	Warnings         []string      `json:"warnings,omitempty"`
	Error            string        `json:"error,omitempty"`
}
