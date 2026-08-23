package prompts

import (
	"testing"

	"astra/astra/agents/actions"
)

func TestPromptIncludesEvidenceAndClarificationGates(t *testing.T) {
	execution := ExecutionSystem("{\"status\":\"understanding\"}", "{}", "list_files", ExecutionSchema, "/tmp/project", "retrieved memory")
	for _, expected := range []string{"negative claim requires", "smallest set of actions", "only when an essential decision", "Task completion gate", "living task state", "complete updated task_state", "<retrieved_memory>", "retrieved memory", "Sherlock method", "navigable knowledge graph"} {
		if !contains(execution, expected) {
			t.Fatalf("execution prompt missing decision rule %q", expected)
		}
	}
}

func TestActionCatalogIsCompactAndActivationIsDetailed(t *testing.T) {
	a := setupTestActions(t)
	compact := ActionCatalog(a.ListActions())
	if contains(compact, "Parameters:") || !contains(compact, "run_commands") || !contains(compact, "Approval/risk:") {
		t.Fatalf("action catalog should be compact bookmarks: %s", compact)
	}
	docs, _ := a.ActionDocumentation([]string{"run_commands"})
	detailed := ActivatedActionDocumentation(docs)
	for _, expected := range []string{"Parameters/schema:", "Examples:", "Returns:", "Failure recovery:", "run_commands"} {
		if !contains(detailed, expected) {
			t.Fatalf("activation docs missing %q: %s", expected, detailed)
		}
	}
}

// prompts tests use the same in-memory action registry as the runtime without
// coupling the prompt package to a database implementation.
func setupTestActions(t *testing.T) *actions.DataActions {
	t.Helper()
	return actions.NewDataActions(nil, 1)
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
