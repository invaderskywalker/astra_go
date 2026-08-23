// Package promptstore stores user-authored instruction and personality
// profiles as inspectable Markdown files. Profiles enrich behavior; they do
// not override Astra's compiled policy, authority checks, or tool contracts.
package promptstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"astra/astra/config"
)

type Profile struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	File        string    `json:"file"`
	Enabled     bool      `json:"enabled"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type index struct {
	Version  int       `json:"version"`
	Profiles []Profile `json:"profiles"`
}

type Store struct{ root string }

func New(root string) Store { return Store{root: root} }

func Default() Store { return New(filepath.Join(config.LoadConfig().AstraRoot, "prompts")) }

func (s Store) Root() string { return s.root }

func (s Store) List() ([]Profile, error) {
	stored, err := s.readIndex()
	if err != nil {
		return nil, err
	}
	sort.Slice(stored.Profiles, func(i, j int) bool { return stored.Profiles[i].Name < stored.Profiles[j].Name })
	return stored.Profiles, nil
}

func (s Store) Save(name, description, content string, enabled bool) (Profile, error) {
	name = strings.TrimSpace(name)
	content = strings.TrimSpace(content)
	if name == "" || content == "" {
		return Profile{}, fmt.Errorf("prompt profile name and content are required")
	}
	fileName := safeName(name) + ".md"
	if fileName == ".md" {
		return Profile{}, fmt.Errorf("prompt profile name must contain letters or numbers")
	}
	stored, err := s.readIndex()
	if err != nil {
		return Profile{}, err
	}
	now := time.Now().UTC()
	profile := Profile{ID: "prompt_" + safeName(name), Name: name, Description: strings.TrimSpace(description), File: fileName, Enabled: enabled, UpdatedAt: now}
	replaced := false
	for i := range stored.Profiles {
		if stored.Profiles[i].ID == profile.ID {
			profile.File = stored.Profiles[i].File
			profile.UpdatedAt = now
			stored.Profiles[i] = profile
			replaced = true
			break
		}
	}
	if !replaced {
		stored.Profiles = append(stored.Profiles, profile)
	}
	if err := os.MkdirAll(s.root, 0700); err != nil {
		return Profile{}, err
	}
	if err := os.WriteFile(filepath.Join(s.root, profile.File), []byte(render(profile, content)), 0600); err != nil {
		return Profile{}, err
	}
	if err := s.writeIndex(stored); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func (s Store) SetEnabled(identifier string, enabled bool) (Profile, error) {
	stored, err := s.readIndex()
	if err != nil {
		return Profile{}, err
	}
	for i := range stored.Profiles {
		if stored.Profiles[i].ID == identifier || stored.Profiles[i].Name == identifier || stored.Profiles[i].File == identifier {
			stored.Profiles[i].Enabled = enabled
			stored.Profiles[i].UpdatedAt = time.Now().UTC()
			if err := s.writeIndex(stored); err != nil {
				return Profile{}, err
			}
			return stored.Profiles[i], nil
		}
	}
	return Profile{}, fmt.Errorf("prompt profile %q was not found", identifier)
}

func (s Store) Context(maxChars int) (string, error) {
	profiles, err := s.List()
	if err != nil {
		return "", err
	}
	if maxChars <= 0 {
		maxChars = 12000
	}
	var builder strings.Builder
	for _, profile := range profiles {
		if !profile.Enabled {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.root, profile.File))
		if err != nil {
			continue
		}
		content := strings.TrimSpace(stripHeader(string(data)))
		if content == "" {
			continue
		}
		block := fmt.Sprintf("\n<profile name=%q description=%q>\n%s\n</profile>\n", profile.Name, profile.Description, content)
		if builder.Len()+len(block) > maxChars {
			break
		}
		builder.WriteString(block)
	}
	if builder.Len() == 0 {
		return "No user-authored prompt profiles are enabled.", nil
	}
	return strings.TrimSpace(builder.String()), nil
}

func (s Store) readIndex() (index, error) {
	data, err := os.ReadFile(filepath.Join(s.root, "index.json"))
	if os.IsNotExist(err) {
		return index{Version: 1, Profiles: []Profile{}}, nil
	}
	if err != nil {
		return index{}, err
	}
	var stored index
	if err := json.Unmarshal(data, &stored); err != nil {
		return index{}, fmt.Errorf("read prompt profile index: %w", err)
	}
	return stored, nil
}

func (s Store) writeIndex(stored index) error {
	if err := os.MkdirAll(s.root, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.root, "index.json"), append(data, '\n'), 0600)
}

func render(profile Profile, content string) string {
	return fmt.Sprintf("# %s\n\n<!-- Astra profile: %s -->\n\n%s\n", profile.Name, profile.ID, strings.TrimSpace(content))
}

func stripHeader(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "# ") {
		lines = lines[1:]
	}
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "<!-- Astra profile:") {
		lines = lines[1:]
	}
	return strings.Join(lines, "\n")
}

func safeName(value string) string {
	var builder strings.Builder
	lastDash := false
	for _, char := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			builder.WriteRune(char)
			lastDash = false
		} else if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}
