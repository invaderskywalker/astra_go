// Package actions provides functionality for managing database and code manipulation actions.
package actions

import (
	"astra/astra/utils/logging"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// applyEditsToFile applies a list of code edits to a single file with backup and atomic writes.
// It supports creating, deleting, replacing, or inserting content in the file.
//
// Parameters:
//   - file: The path to the file to edit.
//   - edits: A slice of CodeEdit structs specifying the edits to apply.
//
// Returns:
//   - An error if any operation fails, nil otherwise.
func (a *DataActions) applyEditsToFile(file string, edits []CodeEdit) error {
	if os.Getenv("ASTRA_TEST") == "1" {
		return nil
	}

	if _, err := os.Stat(file); err == nil {
		backup := file + ".bak"
		src, err := os.Open(file)
		if err != nil {
			return err
		}
		defer src.Close()
		dst, err := os.Create(backup)
		if err != nil {
			return err
		}
		defer dst.Close()
		if _, err := io.Copy(dst, src); err != nil {
			return err
		}
	}

	content, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lines := strings.Split(string(content), "\n")

	for _, edit := range edits {
		switch edit.Type {
		case "create_file":
			a.createFile(edit.File, edit.Content)
		case "delete_file":
			os.Remove(edit.File)
		case "replace":
			lines = a.handleReplace(lines, edit)
		case "insert":
			lines = a.handleInsert(lines, edit)
		}
	}

	tmpFile := file + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return err
	}
	return os.Rename(tmpFile, file)
}

// handleReplace replaces a target line or block in the file content with new content.
// It uses context and start/end markers to locate the replacement target.
//
// Parameters:
//   - lines: The file content as a slice of lines.
//   - edit: A CodeEdit struct specifying the replacement details.
//
// Returns:
//   - The updated slice of lines after applying the replacement.
func (a *DataActions) handleReplace(lines []string, edit CodeEdit) []string {
	replacementLines := strings.Split(edit.Replacement, "\n")
	contextBefore := edit.ContextBefore
	contextAfter := edit.ContextAfter

	if edit.Start != "" && edit.End != "" {
		startIdx := a.findLineIndex(lines, edit.Start, contextBefore, true)
		if startIdx == -1 {
			return lines
		}
		endIdx := a.findLineIndex(lines[startIdx:], edit.End, contextAfter, false)
		if endIdx == -1 {
			return lines
		}
		endIdx += startIdx
		return append(lines[:startIdx], append(replacementLines, lines[endIdx+1:]...)...)
	} else {
		idx := a.findLineIndex(lines, edit.Target, contextBefore, true)
		if idx == -1 {
			return lines
		}
		if contextAfter != "" && !a.hasContextInRange(lines, idx+1, 5, contextAfter) {
			return lines
		}
		return append(lines[:idx], append(replacementLines, lines[idx+1:]...)...)
	}
}

// handleInsert inserts new content before or after a target line in the file content.
// It uses context and position to determine where to insert the content.
//
// Parameters:
//   - lines: The file content as a slice of lines.
//   - edit: A CodeEdit struct specifying the insertion details.
//
// Returns:
//   - The updated slice of lines after applying the insertion.
func (a *DataActions) handleInsert(lines []string, edit CodeEdit) []string {
	contentLines := strings.Split(edit.Content, "\n")
	position := edit.Position
	if position == "" {
		position = "after"
	}

	idx := a.findLineIndex(lines, edit.Target, edit.ContextBefore, true)
	if idx == -1 {
		return lines
	}

	if edit.ContextAfter != "" && !a.hasContextInRange(lines, idx+1, 5, edit.ContextAfter) {
		return lines
	}

	insertIdx := idx
	if position == "after" {
		insertIdx++
	}

	return append(lines[:insertIdx], append(contentLines, lines[insertIdx:]...)...)
}

// findLineIndex locates the index of a target line in the file content, considering context.
// It searches within a window of lines to match the context before or after the target.
//
// Parameters:
//   - lines: The file content as a slice of lines.
//   - target: The target string to search for in the lines.
//   - context: The context string to match before or after the target.
//   - checkBefore: If true, check context before the target; otherwise, check after.
//
// Returns:
//   - The index of the matching line, or -1 if not found.
func (a *DataActions) findLineIndex(lines []string, target, context string, checkBefore bool) int {
	window := 5
	for i, line := range lines {
		if strings.Contains(line, target) {
			if context == "" || (checkBefore && a.hasContextInRange(lines, max(0, i-window), window, context)) ||
				(!checkBefore && a.hasContextInRange(lines, i+1, window, context)) {
				return i
			}
		}
	}
	return -1
}

// hasContextInRange checks if a context string exists within a range of lines.
// It searches within a specified window of lines starting from a given index.
//
// Parameters:
//   - lines: The file content as a slice of lines.
//   - start: The starting index for the search.
//   - window: The number of lines to search within.
//   - context: The context string to search for.
//
// Returns:
//   - True if the context is found within the range, false otherwise.
func (a *DataActions) hasContextInRange(lines []string, start, window int, context string) bool {
	for i := start; i < min(len(lines), start+window); i++ {
		if strings.Contains(lines[i], context) {
			return true
		}
	}
	return false
}

// createFile creates a new file with the specified content, creating directories as needed.
// It logs the creation event for tracking.
//
// Parameters:
//   - file: The path to the file to create.
//   - content: The content to write to the file.
func (a *DataActions) createFile(file, content string) {
	os.MkdirAll(filepath.Dir(file), 0755)
	os.WriteFile(file, []byte(content), 0644)
	logging.AppLogger.Info("Created file", zap.String("file", file))
}

// rollbackFiles restores backup files for all edited files in case of failure.
// It moves .bak files back to their original names.
//
// Parameters:
//   - editsByFile: A map of file paths to their corresponding CodeEdit slices.
func (a *DataActions) rollbackFiles(editsByFile map[string][]CodeEdit) {
	for file := range editsByFile {
		backup := file + ".bak"
		if _, err := os.Stat(backup); err == nil {
			os.Rename(backup, file)
		}
	}
}

// max returns the maximum of two integers.
//
// Parameters:
//   - a: The first integer.
//   - b: The second integer.
//
// Returns:
//   - The larger of the two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the minimum of two integers.
//
// Parameters:
//   - a: The first integer.
//   - b: The second integer.
//
// Returns:
//   - The smaller of the two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
