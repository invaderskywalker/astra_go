// Package actions: diagnostics.go
// Implements structured diagnostics parsing for Go build output
package actions

import (
	"regexp"
	"strings"
)

type GoDiagnostic struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// ParseGoDiagnostics parses Go compiler or build output to structured diagnostics.
func ParseGoDiagnostics(output string) []GoDiagnostic {
	lines := strings.Split(output, "\n")
	diagnostics := []GoDiagnostic{}
	// Example Go error: ./src/file.go:18:4: cannot find module
	re := regexp.MustCompile(`(?P<File>.+\.go):(\d+):(\d+):\s(?P<Message>.+)\b`)
	for _, l := range lines {
		match := re.FindStringSubmatch(l)
		if len(match) == 5 {
			file := match[1]
			line := atoi(match[2])
			col := atoi(match[3])
			msg := match[4]
			d := GoDiagnostic{
				File:     file,
				Line:     line,
				Column:   col,
				Severity: "error",
				Message:  msg,
			}
			diagnostics = append(diagnostics, d)
		}
	}
	return diagnostics
}

// Helper atoi with safe fallback
func atoi(s string) int {
	res := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		res = res*10 + int(c-'0')
	}
	return res
}
