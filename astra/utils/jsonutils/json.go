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

	// Case 1: fenced block. Models sometimes include more than one fenced or
	// raw JSON value; do not concatenate them into an invalid document.
	reFence := regexp.MustCompile("(?s)```json(.*?)```")
	if match := reFence.FindStringSubmatch(input); len(match) > 1 {
		input = strings.TrimSpace(match[1])
	}

	// 🔹 Remove inline // comments safely (but not in strings)
	input = removeJSONComments(input)
	if value := firstValidJSONValue(input); value != "" {
		return value
	}

	input = CleanJSON(input)
	return input
}

// firstValidJSONValue finds the first complete JSON object/array embedded in
// model text. A greedy regular expression is unsafe here: if a model emits a
// reasoning object followed by the final plan, it creates `}{` and the caller
// receives the misleading "after top-level value" error.
func firstValidJSONValue(input string) string {
	for offset, char := range input {
		if char != '{' && char != '[' {
			continue
		}
		decoder := json.NewDecoder(strings.NewReader(input[offset:]))
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			continue
		}
		if len(value) == 0 {
			continue
		}
		return strings.TrimSpace(string(value))
	}
	return ""
}

// ToJSON serializes a Go value to a JSON string with indentation.
// func ToJSON(v interface{}) string {
// 	bytes, err := json.MarshalIndent(v, "", "  ")
// 	if err != nil {
// 		return ""
// 	}
// 	return strings.TrimSpace(string(bytes))
// }

func ToJSON(v interface{}) string {
	var buf strings.Builder
	enc := json.NewEncoder(&buf)

	// 🔥 THIS IS THE CRITICAL LINE
	enc.SetEscapeHTML(false)

	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return ""
	}

	// Important: trim trailing newline added by Encoder
	return strings.TrimSpace(buf.String())
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
// func removeJSONComments(input string) string {
// 	var sb strings.Builder
// 	inString := false
// 	escaped := false

// 	lines := strings.Split(input, "\n")
// 	for _, line := range lines {
// 		cleanLine := ""
// 		for i := 0; i < len(line); i++ {
// 			ch := line[i]

// 			// Handle escape in strings
// 			if ch == '\\' && inString {
// 				escaped = !escaped
// 				cleanLine += string(ch)
// 				continue
// 			}

// 			if ch == '"' && !escaped {
// 				inString = !inString
// 				cleanLine += string(ch)
// 				continue
// 			}

// 			// Detect // when not inside string
// 			if !inString && i+1 < len(line) && ch == '/' && line[i+1] == '/' {
// 				// stop reading this line at comment start
// 				break
// 			}

// 			cleanLine += string(ch)
// 			escaped = false
// 		}
// 		sb.WriteString(strings.TrimRight(cleanLine, " \t"))
// 		sb.WriteByte('\n')
// 	}

// 	return sb.String()
// }

func removeJSONComments(input string) string {
	var sb strings.Builder
	inString := false
	escaped := false

	for _, line := range strings.Split(input, "\n") {
		runes := []rune(line)
		for i := 0; i < len(runes); i++ {
			ch := runes[i]

			// Handle escape inside string
			if ch == '\\' && inString {
				escaped = !escaped
				sb.WriteRune(ch)
				continue
			}

			if ch == '"' && !escaped {
				inString = !inString
				sb.WriteRune(ch)
				continue
			}

			// Detect // comment ONLY outside strings
			if !inString && ch == '/' && i+1 < len(runes) && runes[i+1] == '/' {
				break // stop line
			}

			sb.WriteRune(ch)
			escaped = false
		}
		sb.WriteByte('\n')
	}

	return sb.String()
}
