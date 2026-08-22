package workspace

import "time"

type Workspace struct {
	Root string
}

type WorkspaceState struct {
	Root string

	LastCommand string

	LastBuildSucceeded bool

	LastBuildTime time.Time

	FilesModified []string

	LastDiagnostics []Diagnostic

	GitDirty bool
}

type Diagnostic struct {
	File string

	Line int

	Column int

	Severity string

	Message string

	Tool string
}
