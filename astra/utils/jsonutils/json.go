package jsonutils

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ExtractJSON tries to extract a JSON block from LLM output.
//
// Priority:
// 1. Triple-backtick fenced ```json ... ```
// 2. Any {...} JSON object
//
// It also sanitizes common LLM formatting issues like escaped quotes,
// double backslashes, stray commas, and invisible Unicode characters.
func ExtractJSON(input string) string {
	// Remove BOMs and invisible control characters
	input = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r == '\uFEFF' || r == '\u200B' || r == '\u200C' || r == '\u200D' {
			return -1 // skip
		}
		return r
	}, input))

	// Case 1: fenced block
	reFence := regexp.MustCompile("(?s)```json(.*?)```")
	if match := reFence.FindStringSubmatch(input); len(match) > 1 {
		input = strings.TrimSpace(match[1])
	} else {
		// Case 2: raw object (greedy match from first { to last })
		reObj := regexp.MustCompile(`(?s)\{.*\}`)
		if match := reObj.FindString(input); match != "" {
			input = strings.TrimSpace(match)
		}
	}

	// ---- SANITIZATION ----

	// Unescape double backslashes and escaped quotes (from model output)
	input = strings.ReplaceAll(input, `\\`, `\`)
	input = strings.ReplaceAll(input, `\"`, `"`)

	// Remove any trailing commas before closing braces/brackets
	reTrailingComma := regexp.MustCompile(`,(\s*[}\]])`)
	input = reTrailingComma.ReplaceAllString(input, "$1")

	// Clean up unnecessary newlines / indentation artifacts
	input = strings.TrimSpace(input)

	return input
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
