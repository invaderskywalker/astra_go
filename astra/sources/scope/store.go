// Package scope manages the user's explicitly approved filesystem roots.
// Scopes are local, owner-private files; they are not an operating-system
// sandbox and do not elevate process privileges. They are Astra's authority
// ledger for deciding which paths its tools may target.
package scope

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"astra/astra/config"
)

const (
	Read    = "read"
	Write   = "write"
	Execute = "execute"
)

var ErrNoScope = errors.New("path is outside Astra's approved scopes")

type Entry struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	Label       string    `json:"label,omitempty"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	RevokedAt   time.Time `json:"revoked_at,omitempty"`
}

type file struct {
	Version int     `json:"version"`
	Scopes  []Entry `json:"scopes"`
}

type Store struct{ path string }

func New(path string) Store { return Store{path: path} }

func Default() Store { return New(filepath.Join(config.LoadConfig().AstraRoot, "scopes.json")) }

func (s Store) Path() string { return s.path }

func (s Store) List() ([]Entry, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	var stored file
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("read scope registry: %w", err)
	}
	active := make([]Entry, 0, len(stored.Scopes))
	for _, entry := range stored.Scopes {
		if entry.RevokedAt.IsZero() {
			active = append(active, entry)
		}
	}
	sort.Slice(active, func(i, j int) bool { return active[i].Path < active[j].Path })
	return active, nil
}

func (s Store) Add(path, label string, permissions []string) (Entry, error) {
	canonical, err := canonicalDirectory(path)
	if err != nil {
		return Entry{}, err
	}
	perms, err := normalizePermissions(permissions)
	if err != nil {
		return Entry{}, err
	}
	entries, err := s.readAll()
	if err != nil {
		return Entry{}, err
	}
	now := time.Now().UTC()
	for i := range entries {
		if entries[i].Path == canonical {
			entries[i].Label = strings.TrimSpace(label)
			entries[i].Permissions = mergePermissions(entries[i].Permissions, perms)
			entries[i].UpdatedAt = now
			entries[i].RevokedAt = time.Time{}
			if err := s.write(entries); err != nil {
				return Entry{}, err
			}
			return entries[i], nil
		}
	}
	digest := sha256.Sum256([]byte(canonical))
	entry := Entry{ID: "scope_" + hex.EncodeToString(digest[:])[:16], Path: canonical, Label: strings.TrimSpace(label), Permissions: perms, CreatedAt: now, UpdatedAt: now}
	entries = append(entries, entry)
	if err := s.write(entries); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

func (s Store) Revoke(identifier string) (Entry, error) {
	entries, err := s.readAll()
	if err != nil {
		return Entry{}, err
	}
	identifier = strings.TrimSpace(identifier)
	for i := range entries {
		if entries[i].ID == identifier || entries[i].Path == identifier {
			entries[i].RevokedAt = time.Now().UTC()
			entries[i].UpdatedAt = entries[i].RevokedAt
			if err := s.write(entries); err != nil {
				return Entry{}, err
			}
			return entries[i], nil
		}
	}
	return Entry{}, fmt.Errorf("scope %q was not found", identifier)
}

func (s Store) Authorize(path, permission string) (Entry, error) {
	candidate, err := canonicalPath(path)
	if err != nil {
		return Entry{}, err
	}
	entries, err := s.List()
	if err != nil {
		return Entry{}, err
	}
	var best Entry
	found := false
	for _, entry := range entries {
		if !hasPermission(entry.Permissions, permission) || !within(entry.Path, candidate) {
			continue
		}
		if !found || len(entry.Path) > len(best.Path) {
			best, found = entry, true
		}
	}
	if !found {
		return Entry{}, ErrNoScope
	}
	return best, nil
}

func (s Store) readAll() ([]Entry, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	var stored file
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("read scope registry: %w", err)
	}
	return stored.Scopes, nil
}

func (s Store) write(entries []Entry) error {
	data, err := json.MarshalIndent(file{Version: 1, Scopes: entries}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".scope-")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path)
}

func canonicalDirectory(path string) (string, error) {
	canonical, err := canonicalPath(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("scope path must be a directory: %s", canonical)
	}
	return canonical, nil
}

func canonicalPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("scope path is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = resolved
	}
	return absolute, nil
}

func within(root, candidate string) bool {
	return candidate == root || strings.HasPrefix(candidate, root+string(os.PathSeparator))
}

func normalizePermissions(values []string) ([]string, error) {
	if len(values) == 0 {
		values = []string{Read, Execute}
	}
	allowed := map[string]bool{Read: true, Write: true, Execute: true}
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "all" {
			return []string{Read, Write, Execute}, nil
		}
		if !allowed[value] {
			return nil, fmt.Errorf("unsupported scope permission %q; use read, write, execute, or all", value)
		}
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result, nil
}

func mergePermissions(existing, added []string) []string {
	values := append(append([]string{}, existing...), added...)
	result, _ := normalizePermissions(values)
	return result
}

func hasPermission(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
