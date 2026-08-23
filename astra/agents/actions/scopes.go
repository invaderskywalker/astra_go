package actions

import (
	"fmt"
	"path/filepath"
	"strings"

	"astra/astra/sources/scope"
)

func (a *DataActions) ListScopes(_ struct{}) ActionResult {
	entries, err := a.scopes.List()
	if err != nil {
		return ActionResult{Success: false, Error: err.Error()}
	}
	return ActionResult{Success: true, Summary: fmt.Sprintf("Listed %d approved scope(s)", len(entries)), Diagnostics: entries}
}

func (a *DataActions) commandDirectory(requested string) (string, error) {
	return a.commandDirectoryWithPermission(requested, scope.Execute)
}

func (a *DataActions) commandDirectoryWithPermission(requested, permission string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		requested = a.workspace.Root
	} else if !filepath.IsAbs(requested) {
		requested = filepath.Join(a.workspace.Root, requested)
	}
	permission = strings.ToLower(strings.TrimSpace(permission))
	if permission == "" {
		permission = scope.Execute
	}
	if permission != scope.Execute && permission != scope.Write && permission != scope.Read {
		return "", fmt.Errorf("unsupported required permission %q; use read, execute, or write", permission)
	}
	entry, err := a.scopes.Authorize(requested, permission)
	if err != nil {
		return "", fmt.Errorf("working directory %q is not approved; add it with `astra scope add %s all` (%w)", requested, requested, err)
	}
	_ = entry
	return filepath.Clean(requested), nil
}

func (a *DataActions) scopeSummary() string {
	entries, err := a.scopes.List()
	if err != nil || len(entries) == 0 {
		return "No additional filesystem scopes are approved."
	}
	var builder strings.Builder
	builder.WriteString("Approved filesystem scopes (use only when relevant and authorized):\n")
	for _, entry := range entries {
		fmt.Fprintf(&builder, "- %s [%s] %s\n", entry.Path, strings.Join(entry.Permissions, ","), entry.ID)
	}
	return strings.TrimSpace(builder.String())
}

// ScopeContext is a compact, planner-facing view of approved roots. It is
// descriptive only; every command still re-checks authorization at execution.
func (a *DataActions) ScopeContext() string { return a.scopeSummary() }
