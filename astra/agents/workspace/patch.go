package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CodeEdit represents a small, exact source change. Anchors must resolve once.
type CodeEdit struct{ File, Operation, Anchor, Match, NewCode string }

type EditTransaction struct {
	workspace    *Workspace
	Edits        []CodeEdit
	Originals    map[string][]byte
	Patched      map[string][]byte
	Diffs        map[string]string
	FilesTouched []string
	Deletes      map[string]bool
}

func NewEditTransaction(w *Workspace, edits []CodeEdit) *EditTransaction {
	return &EditTransaction{workspace: w, Edits: edits, Originals: map[string][]byte{}, Patched: map[string][]byte{}, Diffs: map[string]string{}, Deletes: map[string]bool{}}
}

func (tx *EditTransaction) DryRun() error {
	grouped := map[string][]CodeEdit{}
	for _, edit := range tx.Edits {
		grouped[edit.File] = append(grouped[edit.File], edit)
	}
	files := make([]string, 0, len(grouped))
	for file := range grouped {
		files = append(files, file)
	}
	sort.Strings(files)
	for _, file := range files {
		original, err := tx.workspace.ReadFile(file)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("read %s: %w", file, err)
		}
		missing := os.IsNotExist(err)
		current := string(original)
		for _, edit := range grouped[file] {
			if edit.Operation == "delete_file" {
				if missing {
					return fmt.Errorf("%s: file does not exist", file)
				}
				tx.Deletes[file] = true
				current = ""
				continue
			}
			if tx.Deletes[file] {
				return fmt.Errorf("%s: cannot edit a file after deleting it", file)
			}
			current, err = applyEdit(current, edit, missing)
			if err != nil {
				return fmt.Errorf("%s: %w", file, err)
			}
		}
		tx.Originals[file], tx.Patched[file] = original, []byte(current)
		tx.Diffs[file] = unifiedDiff(file, string(original), current)
		tx.FilesTouched = append(tx.FilesTouched, file)
	}
	return nil
}

func (tx *EditTransaction) Commit() error {
	for _, file := range tx.FilesTouched {
		if tx.Deletes[file] {
			abs, err := tx.workspace.abs(file)
			if err != nil {
				return err
			}
			if err := os.Remove(abs); err != nil {
				return err
			}
			continue
		}
		data := tx.Patched[file]
		if data == nil {
			continue
		}
		abs, err := tx.workspace.abs(file)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(abs, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

func applyEdit(input string, edit CodeEdit, missing bool) (string, error) {
	switch edit.Operation {
	case "create_file":
		if !missing {
			return "", fmt.Errorf("file already exists")
		}
		return edit.NewCode, nil
	case "replace_file":
		return edit.NewCode, nil // compatibility operation; precise edits are preferred.
	case "replace":
		target := edit.Match
		if target == "" {
			target = edit.Anchor
		}
		if target == "" {
			return "", fmt.Errorf("match or anchor is required")
		}
		if strings.Count(input, target) != 1 {
			return "", fmt.Errorf("target must occur exactly once")
		}
		return strings.Replace(input, target, edit.NewCode, 1), nil
	case "insert_before", "insert_after":
		if edit.Anchor == "" || strings.Count(input, edit.Anchor) != 1 {
			return "", fmt.Errorf("anchor must occur exactly once")
		}
		if edit.Operation == "insert_before" {
			return strings.Replace(input, edit.Anchor, edit.NewCode+edit.Anchor, 1), nil
		}
		return strings.Replace(input, edit.Anchor, edit.Anchor+edit.NewCode, 1), nil
	case "delete":
		target := edit.Match
		if target == "" {
			target = edit.Anchor
		}
		if target == "" || strings.Count(input, target) != 1 {
			return "", fmt.Errorf("target must occur exactly once")
		}
		return strings.Replace(input, target, "", 1), nil
	default:
		return "", fmt.Errorf("unsupported operation %q", edit.Operation)
	}
}

func unifiedDiff(file, before, after string) string {
	if before == after {
		return ""
	}
	return fmt.Sprintf("--- %s\n+++ %s\n- %s\n+ %s\n", file, file, strings.ReplaceAll(before, "\n", "\n- "), strings.ReplaceAll(after, "\n", "\n+ "))
}
