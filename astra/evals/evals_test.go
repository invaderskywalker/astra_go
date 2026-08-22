package evals

import (
	"testing"
	"time"
)

func TestScenarioCatalogCoversCoreCapabilityFamilies(t *testing.T) {
	required := map[string]bool{
		"reasoning":    false,
		"artifact":     false,
		"memory":       false,
		"verification": false,
	}
	for _, scenario := range BuiltinScenarios {
		required[scenario.Category] = true
		if scenario.ID == "" || scenario.Name == "" || scenario.Prompt == "" || len(scenario.RequiredActions) == 0 {
			t.Fatalf("incomplete scenario: %#v", scenario)
		}
	}
	for category, present := range required {
		if !present {
			t.Fatalf("scenario catalog missing category %q", category)
		}
	}
}

func TestRunLocalEvaluation(t *testing.T) {
	root := t.TempDir()
	report := RunLocal(root)
	if report.FinishedAt.Before(report.StartedAt) || report.Passed == 0 {
		t.Fatalf("invalid local evaluation report: %#v", report)
	}
	if report.Failed != 0 {
		for _, check := range report.Checks {
			if !check.Passed {
				t.Logf("failed check %s: %s (%s)", check.ID, check.Summary, check.Evidence)
			}
		}
		t.Fatalf("local evaluation had failures: %#v", report)
	}
	if time.Since(report.FinishedAt) < 0 {
		t.Fatal("evaluation finish time is in the future")
	}
}
