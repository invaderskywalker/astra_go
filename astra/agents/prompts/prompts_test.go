package prompts

import "testing"

func TestPlanningPromptIncludesMemoryAndArtifactPolicy(t *testing.T) {
	prompt := PlanningSystem("Astra", "engineer", "history", "memory decision", "write_artifact", PlanSchema, "/tmp/astra-project")
	for _, expected := range []string{"<retrieved_memory>", "memory decision", "write_artifact", "current workspace evidence", "<skill_catalog>", "<workspace_context>", "/tmp/astra-project", "success_criteria", "stop_conditions"} {
		if !contains(prompt, expected) {
			t.Fatalf("planning prompt missing %q", expected)
		}
	}
}

func TestWorkspaceContextDoesNotInventMissingRoot(t *testing.T) {
	context := WorkspaceContext("")
	if contains(context, "Connected workspace root: /tmp") {
		t.Fatal("workspace context invented a path")
	}
	if !contains(context, "unavailable") {
		t.Fatal("missing workspace context should be explicit")
	}
}

func TestResponsePromptUsesConnectedRootForScopeQuestions(t *testing.T) {
	prompt := ResponseSystem("Which directory can you work in?", "{}", "/Users/example/project")
	for _, expected := range []string{"/Users/example/project", "exact connected workspace root", "workspace_context"} {
		if !contains(prompt, expected) {
			t.Fatalf("response prompt missing %q", expected)
		}
	}
}

func TestSkillCatalogContainsLayeredCompetencies(t *testing.T) {
	catalog := SkillCatalog()
	for _, expected := range []string{"repository_intelligence", "software_delivery", "verification_and_recovery", "artifact_authoring", "mind_palace_memory"} {
		if !contains(catalog, expected) {
			t.Fatalf("skill catalog missing %q", expected)
		}
	}
}

func contains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
