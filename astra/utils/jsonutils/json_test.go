package jsonutils

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestExtractJSON_UTF8AndComments(t *testing.T) {
	// 🔹 YOU will define `input` here
	input := `json
		{
		"thought_process": "The src/Footer.tsx file was located and its code read. It contains a simple footer with a single text span. According to the plan, the next step is to modify this file to add a small icon. Since the user did not specify the icon, and there is an SVG (react.svg) present in src/assets, I will use this as a placeholder icon. I will update the Footer.tsx to import and display this SVG next to the current footer text while preserving existing structure and styles.",
		"should_continue": true,
		"next_step": {
			"step_id": "add_icon_to_footer",
			"action": "apply_code_edits",
			"action_params": {
			"edits": [
				{
				"type": "update_file_content",
				"file": "src/Footer.tsx",
				"replacement": "import React from \"react\";\nimport reactLogo from \"./assets/react.svg\";\n\nfunction Footer() {\n  return (\n    <footer className=\"site-footer\">\n      <div className=\"footer-content\" style={{ display: 'flex', alignItems: 'center', gap: '0.5em' }}>\n        <img src={reactLogo} alt=\"Footer Icon\" style={{ width: '22px', height: '22px', verticalAlign: 'middle', opacity: 0.75 }} />\n        <span>© 2025 Kalariyo. All rights reserved.</span>\n      </div>\n    </footer>\n  );\n}\n\nexport default Footer;\n"
				}
			]
			}
		}
		}
	`

	output := ExtractJSON(input)
	fmt.Println(output)

	// 1️⃣ Output must be valid UTF-8
	if !utf8.ValidString(output) {
		t.Fatalf("ExtractJSON output is NOT valid UTF-8:\n%q", output)
	}

	// 2️⃣ Must not contain mojibake indicators
	badSequences := []string{"Ã", "Â", "ð"}
	for _, bad := range badSequences {
		if strings.Contains(output, bad) {
			t.Fatalf("Detected mojibake sequence %q in output:\n%s", bad, output)
		}
	}

	// 3️⃣ Inline comments must be removed
	if strings.Contains(output, "//") {
		t.Fatalf("Inline comments were NOT removed:\n%s", output)
	}

	// 4️⃣ Output must still look like JSON
	trimmed := strings.TrimSpace(output)
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		t.Fatalf("Output does not look like valid JSON:\n%s", output)
	}
}

func TestExtractJSONUsesFirstCompleteValueWhenModelEmitsMultipleObjects(t *testing.T) {
	input := `Reasoning before output
{"mode":"task","goal":"inspect"}
{"mode":"conversation","goal":"wrong second object"}`
	got := ExtractJSON(input)
	if got != `{"mode":"task","goal":"inspect"}` {
		t.Fatalf("expected first complete JSON object, got %q", got)
	}
}

func TestExtractJSONHandlesNestedBracesAndTrailingText(t *testing.T) {
	input := "```json\n{\"mode\":\"task\",\"constraints\":[\"keep { braces }\"]}\n```\nextra"
	got := ExtractJSON(input)
	var value map[string]interface{}
	if err := json.Unmarshal([]byte(got), &value); err != nil {
		t.Fatalf("extracted JSON is invalid: %v (%q)", err, got)
	}
	if value["mode"] != "task" {
		t.Fatalf("unexpected extracted value: %#v", value)
	}
}
