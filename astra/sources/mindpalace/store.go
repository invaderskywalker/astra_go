// Package mindpalace stores agent memory as portable, linked files.
package mindpalace

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

type Record struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Title      string    `json:"title"`
	Summary    string    `json:"summary"`
	Content    string    `json:"content"`
	Tags       []string  `json:"tags,omitempty"`
	Related    []string  `json:"related,omitempty"`
	Importance int       `json:"importance,omitempty"`
	Confidence string    `json:"confidence,omitempty"`
	Source     string    `json:"source,omitempty"`
	Status     string    `json:"status,omitempty"`
	Supersedes []string  `json:"supersedes,omitempty"`
	SessionID  string    `json:"session_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
type Index struct {
	Version int      `json:"version"`
	Records []Record `json:"records"`
}
type Store struct {
	root      string
	userID    int
	sessionID string
}

// MigrateLegacyRoot copies the old project-local Mind Palace into the global
// user root without overwriting existing files. It is intentionally a one-way
// compatibility migration; the local source remains recoverable until the
// user removes it.
func MigrateLegacyRoot(legacyRoot, globalRoot string) error {
	legacyRoot = filepath.Clean(legacyRoot)
	globalRoot = filepath.Clean(globalRoot)
	if legacyRoot == globalRoot {
		return nil
	}
	if _, err := os.Stat(legacyRoot); os.IsNotExist(err) {
		return nil
	}
	if err := os.MkdirAll(globalRoot, 0700); err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(legacyRoot))
	marker := filepath.Join(globalRoot, ".migrated-from-project-local-"+hex.EncodeToString(digest[:])[:16])
	if _, err := os.Stat(marker); err == nil {
		return nil
	}
	err := filepath.Walk(legacyRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(legacyRoot, path)
		if err != nil || rel == "." {
			return err
		}
		target := filepath.Join(globalRoot, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if _, statErr := os.Stat(target); statErr == nil {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0600)
	})
	if err != nil {
		return err
	}
	return os.WriteFile(marker, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0600)
}

func New(root string, userID int, sessionID string, _ ...any) *Store {
	return &Store{root: root, userID: userID, sessionID: sessionID}
}
func (s *Store) SessionID() string { return s.sessionID }
func (s *Store) userRoot() string  { return filepath.Join(s.root, "users", fmt.Sprintf("%d", s.userID)) }
func (s *Store) indexPath() string { return filepath.Join(s.userRoot(), "memory", "index.json") }
func (s *Store) lockPath() string  { return filepath.Join(s.userRoot(), "memory", ".index.lock") }
func (s *Store) sessionPath() string {
	return filepath.Join(s.userRoot(), "sessions", safeName(s.sessionID), "events.jsonl")
}
func (s *Store) Save(ctx context.Context, record Record) (Record, []string, error) {
	if strings.TrimSpace(record.Kind) == "" || strings.TrimSpace(record.Title) == "" || strings.TrimSpace(record.Content) == "" {
		return Record{}, nil, fmt.Errorf("kind, title, and content are required")
	}
	if safeName(record.Kind) == "" {
		return Record{}, nil, fmt.Errorf("kind must contain letters or numbers")
	}
	var paths []string
	err := s.withIndexLock(ctx, func() error {
		index, err := s.loadIndex()
		if err != nil {
			return err
		}
		// A caller can use an ID for an explicit update. Otherwise kind + title is
		// the stable human identity, so repeated learnings refine one block instead
		// of creating near-duplicate memories.
		if record.ID == "" {
			for _, existing := range index.Records {
				if sameIdentity(existing, record) {
					record.ID = existing.ID
					break
				}
			}
			if record.ID == "" {
				record.ID = newMemoryID()
			}
		}
		now := time.Now().UTC()
		if record.SessionID == "" {
			record.SessionID = s.sessionID
		}
		if record.Status == "" {
			record.Status = "active"
		}
		if record.Status != "active" && record.Status != "superseded" && record.Status != "archived" {
			return fmt.Errorf("status must be active, superseded, or archived")
		}
		if record.Confidence != "" && record.Confidence != "observed" && record.Confidence != "inferred" && record.Confidence != "confirmed" {
			return fmt.Errorf("confidence must be observed, inferred, or confirmed")
		}
		if record.Importance < 0 || record.Importance > 5 {
			return fmt.Errorf("importance must be between 0 and 5")
		}
		replaced := false
		for i := range index.Records {
			if index.Records[i].ID == record.ID {
				record.CreatedAt = index.Records[i].CreatedAt
				if len(record.Related) == 0 {
					record.Related = index.Records[i].Related
				}
				index.Records[i] = record
				replaced = true
				break
			}
		}
		if !replaced {
			record.CreatedAt = now
			index.Records = append(index.Records, record)
		}
		record.UpdatedAt = now
		for i := range index.Records {
			if index.Records[i].ID == record.ID {
				index.Records[i] = record
			}
		}
		sort.Slice(index.Records, func(i, j int) bool { return index.Records[i].UpdatedAt.After(index.Records[j].UpdatedAt) })
		path := filepath.Join(s.userRoot(), "memory", safeName(record.Kind), record.ID+".md")
		if err := writeAtomic(path, []byte(render(record, index))); err != nil {
			return err
		}
		index.Version = 2
		indexData, err := json.MarshalIndent(index, "", "  ")
		if err != nil {
			return err
		}
		if err := writeAtomic(s.indexPath(), indexData); err != nil {
			return err
		}
		paths = []string{path, s.indexPath()}
		return nil
	})
	if err != nil {
		return Record{}, nil, err
	}
	return record, s.sync(ctx, paths...), nil
}

func (s *Store) List(kind string) ([]Record, error) {
	index, err := s.loadIndex()
	if err != nil {
		return nil, err
	}
	result := make([]Record, 0)
	for _, record := range index.Records {
		if kind == "" || record.Kind == kind {
			result = append(result, record)
		}
	}
	return result, nil
}
func (s *Store) Search(query string, limit int) ([]Record, error) {
	records, err := s.List("")
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 10
	}
	terms := memoryTerms(query)
	type scored struct {
		record Record
		score  int
	}
	matches := []scored{}
	for _, record := range records {
		if record.Status == "superseded" || record.Status == "archived" {
			continue
		}
		title, tags := strings.ToLower(record.Title), strings.ToLower(strings.Join(record.Tags, " "))
		body := strings.ToLower(record.Summary + " " + record.Content)
		score := record.Importance
		if record.Confidence == "confirmed" {
			score += 3
		} else if record.Confidence == "observed" {
			score += 1
		}
		ageDays := time.Since(record.UpdatedAt).Hours() / 24
		if ageDays < 7 {
			score += 2
		} else if ageDays < 30 {
			score++
		}
		matched := false
		for _, term := range terms {
			if strings.Contains(title, term) {
				score += 8
				matched = true
			}
			if strings.Contains(tags, term) {
				score += 5
				matched = true
			}
			if strings.Contains(body, term) {
				score += 2
				matched = true
			}
		}
		if matched {
			matches = append(matches, scored{record, score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score == matches[j].score {
			return matches[i].record.UpdatedAt.After(matches[j].record.UpdatedAt)
		}
		return matches[i].score > matches[j].score
	})
	result := make([]Record, 0, min(limit, len(matches)))
	for _, match := range matches {
		result = append(result, match.record)
		if len(result) == limit {
			break
		}
	}
	return result, nil
}

// Context returns a compact, ranked context pack for planning. It includes
// one-hop neighbors so a single remembered decision can lead the agent to the
// related project convention or constraint without injecting the whole archive.
func (s *Store) Context(query string, limit int) (string, error) {
	if strings.TrimSpace(query) == "" {
		return "No matching memory was requested.", nil
	}
	hits, err := s.Search(query, limit)
	if err != nil {
		return "", err
	}
	if len(hits) == 0 {
		return "No relevant durable memory was found. Do not invent prior knowledge.", nil
	}
	index, err := s.loadIndex()
	if err != nil {
		return "", err
	}
	byID := make(map[string]Record, len(index.Records))
	for _, record := range index.Records {
		byID[record.ID] = record
	}
	seen := map[string]bool{}
	var builder strings.Builder
	builder.WriteString("Relevant durable memory (use as evidence and verify against the workspace when it affects code):\n")
	added := 0
	for _, hit := range hits {
		if seen[hit.ID] {
			continue
		}
		seen[hit.ID] = true
		writeContextRecord(&builder, hit)
		added++
		for _, relatedID := range hit.Related {
			related, ok := byID[relatedID]
			if !ok || seen[related.ID] || related.Status != "active" || added >= limit+2 {
				continue
			}
			seen[related.ID] = true
			builder.WriteString("\nRelated memory:\n")
			writeContextRecord(&builder, related)
			added++
		}
	}
	return builder.String(), nil
}

func writeContextRecord(builder *strings.Builder, record Record) {
	content := strings.TrimSpace(record.Content)
	if len(content) > 1800 {
		content = content[:1800] + "…"
	}
	fmt.Fprintf(builder, "\n- [%s] %s (%s)\n  Confidence: %s | Importance: %d\n  Summary: %s\n  Content: %s\n", record.ID, record.Title, record.Kind, valueOr(record.Confidence, "unspecified"), record.Importance, record.Summary, content)
}
func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func memoryTerms(value string) []string {
	var normalized strings.Builder
	for _, char := range strings.ToLower(value) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			normalized.WriteRune(char)
		} else {
			normalized.WriteByte(' ')
		}
	}
	terms := strings.Fields(normalized.String())
	unique := make([]string, 0, len(terms))
	seen := map[string]bool{}
	for _, term := range terms {
		if len(term) < 2 || seen[term] {
			continue
		}
		seen[term] = true
		unique = append(unique, term)
	}
	return unique
}
func (s *Store) Link(ctx context.Context, fromID, toID string) ([]string, error) {
	if fromID == toID {
		return nil, fmt.Errorf("a memory block cannot link to itself")
	}
	var paths []string
	err := s.withIndexLock(ctx, func() error {
		index, err := s.loadIndex()
		if err != nil {
			return err
		}
		positions := map[string]int{}
		for i := range index.Records {
			positions[index.Records[i].ID] = i
		}
		from, okFrom := positions[fromID]
		to, okTo := positions[toID]
		if !okFrom || !okTo {
			return fmt.Errorf("both memory ids must exist")
		}
		now := time.Now().UTC()
		// Related links are reciprocal. This makes the Mind Palace navigable from
		// either concept and avoids an invisible one-way graph.
		index.Records[from].Related = appendUnique(index.Records[from].Related, toID)
		index.Records[to].Related = appendUnique(index.Records[to].Related, fromID)
		index.Records[from].UpdatedAt, index.Records[to].UpdatedAt = now, now
		for _, pos := range []int{from, to} {
			record := index.Records[pos]
			path := filepath.Join(s.userRoot(), "memory", safeName(record.Kind), record.ID+".md")
			if err := writeAtomic(path, []byte(render(record, index))); err != nil {
				return err
			}
			paths = append(paths, path)
		}
		index.Version = 2
		data, err := json.MarshalIndent(index, "", "  ")
		if err != nil {
			return err
		}
		if err := writeAtomic(s.indexPath(), data); err != nil {
			return err
		}
		paths = append(paths, s.indexPath())
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.sync(ctx, paths...), nil
}

// AppendSessionEvent keeps raw session evidence separate from curated memory.
func (s *Store) AppendSessionEvent(ctx context.Context, eventType string, payload any) ([]string, error) {
	if s.sessionID == "" {
		return nil, nil
	}
	event := map[string]any{"timestamp": time.Now().UTC(), "type": eventType, "payload": payload}
	data, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	path := s.sessionPath()
	if err := s.withIndexLock(ctx, func() error {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer file.Close()
		if _, err := file.Write(append(data, '\n')); err != nil {
			return err
		}
		return file.Sync()
	}); err != nil {
		return nil, err
	}
	return s.sync(ctx, path), nil
}

func (s *Store) loadIndex() (Index, error) {
	data, err := os.ReadFile(s.indexPath())
	if os.IsNotExist(err) {
		return Index{Version: 1, Records: []Record{}}, nil
	}
	if err != nil {
		return Index{}, err
	}
	var index Index
	if err := json.Unmarshal(data, &index); err != nil {
		return Index{}, fmt.Errorf("read memory index: %w", err)
	}
	return index, nil
}
func (s *Store) sync(ctx context.Context, paths ...string) []string {
	// Files are the source of truth. External synchronization is deliberately
	// opt-in and is no longer attempted during memory writes.
	_ = ctx
	_ = paths
	return nil
}
func newMemoryID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("mem_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("mem_%x", bytes)
}
func sameIdentity(a, b Record) bool {
	return safeName(a.Kind) == safeName(b.Kind) && strings.EqualFold(strings.TrimSpace(a.Title), strings.TrimSpace(b.Title))
}
func appendUnique(values []string, value string) []string {
	for _, candidate := range values {
		if candidate == value {
			return values
		}
	}
	return append(values, value)
}

func (s *Store) withIndexLock(ctx context.Context, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(s.lockPath()), 0700); err != nil {
		return err
	}
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	for {
		err := os.Mkdir(s.lockPath(), 0700)
		if err == nil {
			defer os.Remove(s.lockPath())
			return fn()
		}
		if !os.IsExist(err) {
			return err
		}
		if info, statErr := os.Stat(s.lockPath()); statErr == nil && time.Since(info.ModTime()) > time.Minute {
			_ = os.Remove(s.lockPath())
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("memory index is busy")
		case <-time.After(15 * time.Millisecond):
		}
	}
}
func safeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, value)
	return strings.Trim(value, "-")
}
func render(record Record, index Index) string {
	metadata, _ := json.MarshalIndent(map[string]any{"id": record.ID, "kind": record.Kind, "summary": record.Summary, "tags": record.Tags, "related": record.Related, "importance": record.Importance, "confidence": record.Confidence, "source": record.Source, "status": record.Status, "supersedes": record.Supersedes, "session_id": record.SessionID, "created_at": record.CreatedAt, "updated_at": record.UpdatedAt}, "", "  ")
	var links strings.Builder
	for _, related := range record.Related {
		targetKind := "unknown"
		for _, candidate := range index.Records {
			if candidate.ID == related {
				targetKind = safeName(candidate.Kind)
				break
			}
		}
		fmt.Fprintf(&links, "- [%s](../%s/%s.md)\n", related, targetKind, related)
	}
	return fmt.Sprintf("<!-- astra-memory\n%s\n-->\n\n# %s\n\n%s\n\n## Related memory\n%s", metadata, record.Title, record.Content, links.String())
}
func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".astra-write-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Chmod(0600); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}
