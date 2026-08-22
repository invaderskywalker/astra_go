package prompts

import "testing"

func TestPlanningPromptIncludesMemoryAndArtifactPolicy(t *testing.T) {
	prompt := PlanningSystem("Astra", "engineer", "history", "memory decision", "write_artifact", PlanSchema)
	for _, expected := range []string{"<retrieved_memory>", "memory decision", "write_artifact", "current workspace evidence"} {
		if !contains(prompt, expected) {
			t.Fatalf("planning prompt missing %q", expected)
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
