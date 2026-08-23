package core

import "testing"

func TestMergeTaskStateKeepsPriorEvidenceAndAppliesUpdates(t *testing.T) {
	current := map[string]interface{}{
		"goal":               "Inspect repository",
		"evidence_collected": []interface{}{"root listed"},
		"remaining_work":     []interface{}{"search stack"},
	}
	updated := map[string]interface{}{
		"evidence_collected": []interface{}{"root listed", "package.json found"},
		"next_action":        "read package.json",
	}
	merged := mergeTaskState(current, updated)
	if merged["goal"] != "Inspect repository" {
		t.Fatalf("merge dropped prior goal: %#v", merged)
	}
	if merged["next_action"] != "read package.json" {
		t.Fatalf("merge did not apply next action: %#v", merged)
	}
	if len(merged["evidence_collected"].([]interface{})) != 2 {
		t.Fatalf("merge did not apply updated evidence: %#v", merged)
	}
}

func TestEmergencyActionBudgetDefaultsAndCaps(t *testing.T) {
	t.Setenv("ASTRA_MAX_ACTION_STEPS", "")
	if got := emergencyActionBudget(); got != defaultEmergencyActionBudget {
		t.Fatalf("default budget = %d, want %d", got, defaultEmergencyActionBudget)
	}
	t.Setenv("ASTRA_MAX_ACTION_STEPS", "9999")
	if got := emergencyActionBudget(); got != maxEmergencyActionBudget {
		t.Fatalf("capped budget = %d, want %d", got, maxEmergencyActionBudget)
	}
	t.Setenv("ASTRA_MAX_ACTION_STEPS", "not-a-number")
	if got := emergencyActionBudget(); got != defaultEmergencyActionBudget {
		t.Fatalf("invalid budget = %d, want %d", got, defaultEmergencyActionBudget)
	}
}

func TestRepeatedFailureSignatureTracksStableFailure(t *testing.T) {
	result := map[string]interface{}{
		"run_commands": map[string]interface{}{
			"success": true,
		},
		"write_file": map[string]interface{}{
			"success": false,
			"error":   "permission denied",
		},
	}
	if got := repeatedFailureSignature("write_file", result); got != "write_file|permission denied|" {
		t.Fatalf("failure signature = %q", got)
	}
	if got := repeatedFailureSignature("write_file", map[string]interface{}{"write_file": map[string]interface{}{"success": true}}); got != "" {
		t.Fatalf("successful result produced failure signature %q", got)
	}
}

func TestProgressSignatureIgnoresVolatileResultFields(t *testing.T) {
	state := map[string]interface{}{"status": "running", "next_action": "inspect"}
	first := map[string]interface{}{"list_files": map[string]interface{}{
		"success":           true,
		"summary":           "listed files",
		"duration_ms":       10,
		"working_directory": "/tmp/a",
	}}
	second := map[string]interface{}{"list_files": map[string]interface{}{
		"success":           true,
		"summary":           "listed files",
		"duration_ms":       200,
		"working_directory": "/tmp/b",
	}}
	if progressSignature(state, "list_files", first) != progressSignature(state, "list_files", second) {
		t.Fatal("volatile result fields changed the progress signature")
	}
}
