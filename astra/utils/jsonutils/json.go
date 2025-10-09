package jsonutils

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ExtractJSON tries to extract and sanitize a JSON block from LLM output.
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

	// 🔹 Remove inline // comments safely (but not in strings)
	input = removeJSONComments(input)

	input = CleanJSON(input)
	return input
}

// ToJSON serializes a Go value to a JSON string with indentation.
func ToJSON(v interface{}) string {
	bytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(bytes))
}

// CleanJSON trims junk before/after braces and code fences.
func CleanJSON(input string) string {
	input = strings.TrimSpace(strings.Trim(input, "`"))

	re := regexp.MustCompile(`\{[\s\S]*\}`)
	if match := re.FindString(input); match != "" {
		input = match
	}

	if lastIdx := strings.LastIndex(input, "}"); lastIdx != -1 {
		input = input[:lastIdx+1]
	}

	return input
}

// --- NEW FUNCTION ---
// removeJSONComments removes // comments that are not inside string literals.
func removeJSONComments(input string) string {
	var sb strings.Builder
	inString := false
	escaped := false

	lines := strings.Split(input, "\n")
	for _, line := range lines {
		cleanLine := ""
		for i := 0; i < len(line); i++ {
			ch := line[i]

			// Handle escape in strings
			if ch == '\\' && inString {
				escaped = !escaped
				cleanLine += string(ch)
				continue
			}

			if ch == '"' && !escaped {
				inString = !inString
				cleanLine += string(ch)
				continue
			}

			// Detect // when not inside string
			if !inString && i+1 < len(line) && ch == '/' && line[i+1] == '/' {
				// stop reading this line at comment start
				break
			}

			cleanLine += string(ch)
			escaped = false
		}
		sb.WriteString(strings.TrimRight(cleanLine, " \t"))
		sb.WriteByte('\n')
	}

	return sb.String()
}
