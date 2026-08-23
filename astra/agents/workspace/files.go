// Package workspace encapsulates all repository filesystem operations for Astra.
package workspace

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FileInfo struct {
	Path         string
	Name         string
	Extension    string
	Size         int64
	IsDirectory  bool
	ModifiedTime time.Time
}

func (w *Workspace) abs(path string) (string, error) {
	if path == "" {
		path = "."
	}
	resolved := path
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(w.Root, resolved)
	}
	resolved = filepath.Clean(resolved)
	rel, err := filepath.Rel(w.Root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", os.ErrPermission
	}
	return resolved, nil
}

func (w *Workspace) ListFiles(path string, recursive bool) ([]FileInfo, error) {
	result := []FileInfo{}
	root, err := w.abs(path)
	if err != nil {
		return nil, err
	}
	walkFn := func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(w.Root, p)
		result = append(result, FileInfo{
			Path:         rel,
			Name:         info.Name(),
			Extension:    filepath.Ext(info.Name()),
			Size:         info.Size(),
			IsDirectory:  info.IsDir(),
			ModifiedTime: info.ModTime(),
		})
		if !recursive && info.IsDir() && p != root {
			return filepath.SkipDir
		}
		return nil
	}
	if recursive {
		err := filepath.Walk(root, walkFn)
		return result, err
	} else {
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			result = append(result, FileInfo{
				Path:      filepath.Join(path, entry.Name()),
				Name:      entry.Name(),
				Extension: filepath.Ext(entry.Name()),
				Size: func() int64 {
					info, _ := entry.Info()
					if info == nil {
						return 0
					}
					return info.Size()
				}(),
				IsDirectory: entry.IsDir(),
				ModifiedTime: func() time.Time {
					info, _ := entry.Info()
					if info == nil {
						return time.Time{}
					}
					return info.ModTime()
				}(),
			})
		}
		return result, nil
	}
}

func (w *Workspace) ReadFile(path string) ([]byte, error) {
	absPath, err := w.abs(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(absPath)
}

// ResolvePath validates and resolves a path without exposing the workspace's
// internal path policy to callers that only need metadata analysis.
func (w *Workspace) ResolvePath(path string) (string, error) { return w.abs(path) }

func (w *Workspace) ReadFileLines(path string, start, end int) ([]string, error) {
	absPath, err := w.abs(path)
	if err != nil {
		return nil, err
	}
	if start < 1 {
		start = 1
	}
	if end == 0 {
		end = start + 2000 - 1
	}
	if end < start {
		return nil, os.ErrInvalid
	}
	file, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	lines := make([]string, 0, end-start+1)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		if lineNumber < start {
			continue
		}
		if lineNumber > end {
			break
		}
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(lines) == 0 && lineNumber < start {
		return nil, os.ErrInvalid
	}
	return lines, nil
}

func (w *Workspace) WriteFile(path string, data []byte) error {
	absPath, err := w.abs(path)
	if err != nil {
		return err
	}
	return os.WriteFile(absPath, data, 0644)
}

func (w *Workspace) CreateFile(path string, data []byte) error {
	absPath, err := w.abs(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(absPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

// CreateDirectory creates a directory tree inside the workspace. It is kept
// separate from CreateFile so the planner can express project bootstrapping
// intent directly and the action log can describe it accurately.
func (w *Workspace) CreateDirectory(path string) error {
	absPath, err := w.abs(path)
	if err != nil {
		return err
	}
	return os.MkdirAll(absPath, 0755)
}

func (w *Workspace) DeleteFile(path string) error {
	absPath, err := w.abs(path)
	if err != nil {
		return err
	}
	return os.Remove(absPath)
}

func (w *Workspace) RenameFile(oldPath, newPath string) error {
	oldAbs, err := w.abs(oldPath)
	if err != nil {
		return err
	}
	newAbs, err := w.abs(newPath)
	if err != nil {
		return err
	}
	return os.Rename(oldAbs, newAbs)
}

func (w *Workspace) CopyFile(src, dst string) error {
	from, err := w.ReadFile(src)
	if err != nil {
		return err
	}
	return w.WriteFile(dst, from)
}

func (w *Workspace) MoveFile(src, dst string) error {
	err := w.CopyFile(src, dst)
	if err != nil {
		return err
	}
	return w.DeleteFile(src)
}

func (w *Workspace) Exists(path string) bool {
	absPath, err := w.abs(path)
	if err != nil {
		return false
	}
	_, err = os.Stat(absPath)
	return err == nil
}

// splitLines splits a string into lines. Handles both \n and \r\n.
func splitLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}
