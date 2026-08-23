package actions

import (
	"fmt"

	"astra/astra/sources/promptstore"
)

type WritePromptProfileParams struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content"`
	Enabled     bool   `json:"enabled,omitempty"`
}

func (a *DataActions) WritePromptProfile(params WritePromptProfileParams) ActionResult {
	profile, err := a.prompts.Save(params.Name, params.Description, params.Content, params.Enabled)
	if err != nil {
		return ActionResult{Success: false, Error: err.Error()}
	}
	return ActionResult{Success: true, Summary: fmt.Sprintf("Prompt profile saved: %s", profile.Name), FilesWritten: []string{profile.File}, Diagnostics: profile}
}

func (a *DataActions) PromptContext() string {
	context, err := a.prompts.Context(12000)
	if err != nil {
		return "Prompt profile retrieval failed; use compiled Astra policy only."
	}
	return context
}

func (a *DataActions) ListPromptProfiles() ([]promptstore.Profile, error) { return a.prompts.List() }
