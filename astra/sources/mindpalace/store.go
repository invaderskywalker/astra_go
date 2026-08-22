// Package mindpalace stores agent memory as portable, linked files.
package mindpalace

import (
	"astra/astra/sources/storage"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Record struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Summary   string    `json:"summary"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags,omitempty"`
	Related   []string  `json:"related,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type Index struct {
	Version int      `json:"version"`
	Records []Record `json:"records"`
}
type Store struct {
	root      string
	userID    int
	sessionID string
	mirror    *storage.MinIOClient
}

func New(root string, userID int, sessionID string, mirror *storage.MinIOClient) *Store {
	return &Store{root: root, userID: userID, sessionID: sessionID, mirror: mirror}
}
func (s *Store) userRoot() string  { return filepath.Join(s.root, "users", fmt.Sprintf("%d", s.userID)) }
func (s *Store) indexPath() string { return filepath.Join(s.userRoot(), "memory", "index.json") }
func (s *Store) sessionPath() string {
	return filepath.Join(s.userRoot(), "sessions", safeName(s.sessionID), "events.jsonl")
}
func (s *Store) objectKey(path string) string {
	rel, _ := filepath.Rel(s.root, path)
	return filepath.ToSlash(filepath.Join("mind-palace", rel))
}

func (s *Store) Save(ctx context.Context, record Record) (Record, []string, error) {
	if strings.TrimSpace(record.Kind) == "" || strings.TrimSpace(record.Title) == "" || strings.TrimSpace(record.Content) == "" {
		return Record{}, nil, fmt.Errorf("kind, title, and content are required")
	}
	if record.ID == "" {
		record.ID = memoryID(record.Kind, record.Title, record.Content)
	}
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	if record.SessionID == "" {
		record.SessionID = s.sessionID
	}
	index, err := s.loadIndex()
	if err != nil {
		return Record{}, nil, err
	}
	replaced := false
	for i := range index.Records {
		if index.Records[i].ID == record.ID {
			record.CreatedAt = index.Records[i].CreatedAt
			index.Records[i] = record
			replaced = true
			break
		}
	}
	if !replaced {
		index.Records = append(index.Records, record)
	}
	sort.Slice(index.Records, func(i, j int) bool { return index.Records[i].UpdatedAt.After(index.Records[j].UpdatedAt) })
	path := filepath.Join(s.userRoot(), "memory", safeName(record.Kind), record.ID+".md")
	if err := writeAtomic(path, []byte(render(record, index))); err != nil {
		return Record{}, nil, err
	}
	indexData, _ := json.MarshalIndent(index, "", "  ")
	if err := writeAtomic(s.indexPath(), indexData); err != nil {
		return Record{}, nil, err
	}
	warnings := s.sync(ctx, path, s.indexPath())
	return record, warnings, nil
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
	query = strings.ToLower(query)
	result := []Record{}
	for _, record := range records {
		if strings.Contains(strings.ToLower(record.Title+" "+record.Summary+" "+record.Content+" "+strings.Join(record.Tags, " ")), query) {
			result = append(result, record)
			if len(result) == limit {
				break
			}
		}
	}
	return result, nil
}
func (s *Store) Link(ctx context.Context, fromID, toID string) ([]string, error) {
	index, err := s.loadIndex()
	if err != nil {
		return nil, err
	}
	var from *Record
	foundTo := false
	for i := range index.Records {
		if index.Records[i].ID == fromID {
			from = &index.Records[i]
		}
		if index.Records[i].ID == toID {
			foundTo = true
		}
	}
	if from == nil || !foundTo {
		return nil, fmt.Errorf("both memory ids must exist")
	}
	for _, related := range from.Related {
		if related == toID {
			return nil, nil
		}
	}
	from.Related = append(from.Related, toID)
	from.UpdatedAt = time.Now().UTC()
	path := filepath.Join(s.userRoot(), "memory", safeName(from.Kind), from.ID+".md")
	if err := writeAtomic(path, []byte(render(*from, index))); err != nil {
		return nil, err
	}
	data, _ := json.MarshalIndent(index, "", "  ")
	if err := writeAtomic(s.indexPath(), data); err != nil {
		return nil, err
	}
	return s.sync(ctx, path, s.indexPath()), nil
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
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
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
	if s.mirror == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	warnings := []string{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			contentType := "text/markdown"
			if filepath.Ext(path) == ".json" {
				contentType = "application/json"
			}
			if err := s.mirror.PutObject(ctx, s.objectKey(path), data, contentType); err != nil {
				warnings = append(warnings, "MinIO sync pending: "+err.Error())
			}
		}
	}
	return warnings
}
func memoryID(kind, title, content string) string {
	hash := sha256.Sum256([]byte(kind + "\x00" + title + "\x00" + content))
	return fmt.Sprintf("mem_%x", hash[:8])
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
	metadata, _ := json.MarshalIndent(map[string]any{"id": record.ID, "kind": record.Kind, "summary": record.Summary, "tags": record.Tags, "related": record.Related, "session_id": record.SessionID, "created_at": record.CreatedAt, "updated_at": record.UpdatedAt}, "", "  ")
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
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0644); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
