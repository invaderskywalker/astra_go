package actions

import (
	"fmt"
	"strings"
)

type SearchCodeParams struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

func (a *DataActions) SearchCode(params SearchCodeParams) ActionResult {
	if strings.TrimSpace(params.Query) == "" {
		return ActionResult{Success: false, Error: "query is required"}
	}
	matches, err := a.workspace.SearchText(params.Query, params.Limit)
	if err != nil {
		return ActionResult{Success: false, Error: err.Error()}
	}
	return ActionResult{Success: true, Summary: fmt.Sprintf("Found %d match(es) for %q", len(matches), params.Query), Diagnostics: matches}
}
