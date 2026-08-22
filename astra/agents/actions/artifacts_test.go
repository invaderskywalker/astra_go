package actions

import "testing"

func TestValidateArtifactFormats(t *testing.T) {
	tests := []struct{ format, content, extension string }{
		{"markdown", "# Decision", ".md"},
		{"json", `{"ok":true}`, ".json"},
		{"jsonl", "{\"one\":1}\n{\"two\":2}", ".jsonl"},
		{"csv", "name,value\nastra,1", ".csv"},
	}
	for _, test := range tests {
		got, err := validateArtifact(test.format, test.content)
		if err != nil || got != test.extension {
			t.Fatalf("%s: got %q, %v", test.format, got, err)
		}
	}
	if _, err := validateArtifact("json", "not json"); err == nil {
		t.Fatal("invalid JSON was accepted")
	}
}

func TestArtifactName(t *testing.T) {
	if got := artifactName("  Architecture / Decision #1 "); got != "architecture-decision-1" {
		t.Fatalf("unexpected name: %q", got)
	}
}
