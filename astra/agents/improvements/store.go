// Package improvements manages the human-approved self-improvement queue.
package improvements

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Status string

const (
	ReviewReady Status = "review-ready"
	Approved    Status = "approved"
	Rejected    Status = "rejected"
	Completed   Status = "completed"
)

type Proposal struct {
	ID               string    `json:"id"`
	Status           Status    `json:"status"`
	Title            string    `json:"title"`
	Objective        string    `json:"objective"`
	Evidence         []string  `json:"evidence"`
	ProposedActions  []string  `json:"proposed_actions"`
	Validation       []string  `json:"validation"`
	Risk             string    `json:"risk"`
	CreatedAt        time.Time `json:"created_at"`
	Model            string    `json:"model"`
	Workspace        string    `json:"workspace"`
	RequiresApproval bool      `json:"requires_approval"`
}
type Review struct {
	ProposalID      string    `json:"proposal_id"`
	Model           string    `json:"model"`
	Recommendation  string    `json:"recommendation"`
	Rationale       string    `json:"rationale"`
	MissingEvidence []string  `json:"missing_evidence"`
	CreatedAt       time.Time `json:"created_at"`
}
type Store struct{ Root string }

func New(root string) *Store { return &Store{Root: root} }
func (s *Store) proposalPath(status Status, id string) string {
	return filepath.Join(s.Root, string(status), id+".md")
}
func (s *Store) reviewPath(id string) string { return filepath.Join(s.Root, "reviews", id+".json") }

func (s *Store) SaveProposal(proposal Proposal) (Proposal, error) {
	if proposal.ID == "" {
		proposal.ID = fmt.Sprintf("imp_%d", time.Now().UTC().UnixNano())
	}
	if proposal.Status == "" {
		proposal.Status = ReviewReady
	}
	if proposal.CreatedAt.IsZero() {
		proposal.CreatedAt = time.Now().UTC()
	}
	proposal.RequiresApproval = true
	if proposal.Title == "" || proposal.Objective == "" {
		return Proposal{}, fmt.Errorf("proposal title and objective are required")
	}
	data, err := json.MarshalIndent(proposal, "", "  ")
	if err != nil {
		return Proposal{}, err
	}
	body := fmt.Sprintf("<!-- astra-improvement\n%s\n-->\n\n# %s\n\n## Objective\n\n%s\n\n## Evidence\n%s\n\n## Proposed actions\n%s\n\n## Validation\n%s\n\n## Risk\n\n%s\n", data, proposal.Title, proposal.Objective, bullets(proposal.Evidence), bullets(proposal.ProposedActions), bullets(proposal.Validation), proposal.Risk)
	if err := writeAtomic(s.proposalPath(proposal.Status, proposal.ID), []byte(body)); err != nil {
		return Proposal{}, err
	}
	return proposal, nil
}

func (s *Store) List() ([]Proposal, error) {
	result := []Proposal{}
	for _, status := range []Status{ReviewReady, Approved, Rejected, Completed} {
		entries, err := os.ReadDir(filepath.Join(s.Root, string(status)))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
				continue
			}
			proposal, err := readProposal(filepath.Join(s.Root, string(status), entry.Name()))
			if err != nil {
				return nil, err
			}
			result = append(result, proposal)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result, nil
}
func (s *Store) Get(id string) (Proposal, error) {
	for _, status := range []Status{ReviewReady, Approved, Rejected, Completed} {
		path := s.proposalPath(status, id)
		if _, err := os.Stat(path); err == nil {
			return readProposal(path)
		}
	}
	return Proposal{}, fmt.Errorf("proposal %s not found", id)
}
func (s *Store) SetStatus(id string, status Status) (Proposal, error) {
	proposal, err := s.Get(id)
	if err != nil {
		return Proposal{}, err
	}
	oldPath := s.proposalPath(proposal.Status, id)
	proposal.Status = status
	proposal, err = s.SaveProposal(proposal)
	if err != nil {
		return Proposal{}, err
	}
	if oldPath != s.proposalPath(status, id) {
		_ = os.Remove(oldPath)
	}
	return proposal, nil
}
func (s *Store) SaveReview(review Review) error {
	if review.ProposalID == "" || review.Recommendation == "" {
		return fmt.Errorf("proposal id and recommendation are required")
	}
	review.CreatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(s.reviewPath(review.ProposalID), data)
}

func readProposal(path string) (Proposal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Proposal{}, err
	}
	text := string(data)
	const start = "<!-- astra-improvement\n"
	const end = "\n-->"
	startAt := strings.Index(text, start)
	endAt := strings.Index(text, end)
	if startAt != 0 || endAt < len(start) {
		return Proposal{}, fmt.Errorf("invalid proposal file %s", path)
	}
	var proposal Proposal
	if err := json.Unmarshal([]byte(text[len(start):endAt]), &proposal); err != nil {
		return Proposal{}, err
	}
	return proposal, nil
}
func bullets(items []string) string {
	if len(items) == 0 {
		return "- None"
	}
	return "- " + strings.Join(items, "\n- ")
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
