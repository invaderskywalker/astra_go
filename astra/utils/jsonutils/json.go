package jsonutils

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ExtractJSON tries to extract a JSON block from LLM output.
// Priority:
// 1. Triple-backtick fenced ```json ... ```
// 2. Any {...} JSON object
func ExtractJSON(input string) string {
	// Case 1: fenced block
	reFence := regexp.MustCompile("(?s)```json(.*?)```")
	if match := reFence.FindStringSubmatch(input); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}

	// Case 2: raw object (greedy match from first { to last })
	reObj := regexp.MustCompile(`(?s)\{.*\}`)
	if match := reObj.FindString(input); match != "" {
		return strings.TrimSpace(match)
	}

	// Nothing found
	return ""
}

// ToJSON serializes a Go value to a JSON string with indentation.
// Returns an empty string if serialization fails.
func ToJSON(v interface{}) string {
	// Use json.MarshalIndent for pretty-printed JSON with 2-space indentation
	bytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		// Return empty string on error, consistent with ExtractJSON's fallback
		return ""
	}
	return strings.TrimSpace(string(bytes))
}
