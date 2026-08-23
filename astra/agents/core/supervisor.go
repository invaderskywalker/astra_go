package core

import (
	"astra/astra/agents/actions"
	"astra/astra/sources/scope"
	"astra/astra/sources/state"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const maxWorkerBranches = 16

type workerBranch struct {
	mu      sync.Mutex
	summary actions.AgentSummary
	agent   *BaseAgent
	output  strings.Builder
	done    chan struct{}
}

// Supervisor owns bounded worker branches for one Astra session. Workers are
// ordinary BaseAgents with independent session IDs and prompts, not hidden
// goroutines that bypass the action and evidence system.
type Supervisor struct {
	mu       sync.Mutex
	userID   int
	root     string
	provider string
	model    string
	branches map[string]*workerBranch
}

func NewSupervisor(userID int, root, provider, model string) *Supervisor {
	return &Supervisor{userID: userID, root: root, provider: provider, model: model, branches: map[string]*workerBranch{}}
}

func (s *Supervisor) SpawnAgent(ctx context.Context, request actions.SpawnAgentRequest) (actions.SpawnAgentResult, error) {
	if err := ctx.Err(); err != nil {
		return actions.SpawnAgentResult{}, err
	}
	goal := strings.TrimSpace(request.Goal)
	if goal == "" {
		return actions.SpawnAgentResult{}, fmt.Errorf("worker goal is required")
	}
	root := strings.TrimSpace(request.WorkspaceRoot)
	if root == "" {
		root = s.root
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return actions.SpawnAgentResult{}, err
	}
	if _, err := scope.Default().Authorize(root, scope.Execute); err != nil {
		return actions.SpawnAgentResult{}, fmt.Errorf("worker workspace is not approved: %w", err)
	}
	provider, model := request.Provider, request.Model
	if provider == "" {
		provider = s.provider
	}
	if model == "" {
		model = s.model
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		name = "worker"
	}
	personality := strings.TrimSpace(request.Personality)
	if personality == "" {
		personality = "focused, evidence-first worker; report observations, changes, and validation separately"
	}

	s.mu.Lock()
	if len(s.branches) >= maxWorkerBranches {
		s.mu.Unlock()
		return actions.SpawnAgentResult{}, fmt.Errorf("worker branch limit reached (%d)", maxWorkerBranches)
	}
	id := "agent-" + uuid.New().String()
	started := time.Now().UTC()
	sessionID := "branch-" + uuid.New().String()
	agent := NewBaseAgentWithWorkspace(s.userID, sessionID, name, s, provider, model, root)
	agent.Role = personality
	branch := &workerBranch{agent: agent, done: make(chan struct{}), summary: actions.AgentSummary{ID: id, Name: name, Personality: personality, Goal: goal, WorkspaceRoot: root, Provider: provider, Model: model, Status: "running", StartedAt: started.Format(time.RFC3339)}}
	s.branches[id] = branch
	s.mu.Unlock()
	_, _ = state.EnsureSession(root, s.userID, sessionID, provider, model)

	_, output := agent.ProcessQueryWithRun(goal)
	go s.collectBranch(id, output, root, sessionID)
	return actions.SpawnAgentResult{Summary: s.summary(id)}, nil
}

func (s *Supervisor) collectBranch(id string, output <-chan string, root, sessionID string) {
	branch := s.branch(id)
	if branch == nil {
		return
	}
	for raw := range output {
		branch.mu.Lock()
		branch.summary.Events++
		var envelope struct {
			Type    string `json:"type"`
			Payload struct {
				Chunk   string `json:"chunk"`
				Message string `json:"message"`
			} `json:"payload"`
		}
		if json.Unmarshal([]byte(raw), &envelope) == nil {
			if envelope.Type == "response_chunk" {
				branch.output.WriteString(envelope.Payload.Chunk)
			}
			if envelope.Type == "error" {
				branch.summary.Error = envelope.Payload.Message
				branch.summary.Status = "failed"
			}
			if envelope.Type == "stopped" {
				branch.summary.Status = "stopped"
			}
		}
		branch.mu.Unlock()
	}
	branch.mu.Lock()
	if branch.summary.Status == "running" {
		branch.summary.Status = "completed"
	}
	branch.summary.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	branch.mu.Unlock()
	_ = state.CloseSession(root, sessionID)
	close(branch.done)
}

func (s *Supervisor) ListAgents() []actions.AgentSummary {
	s.mu.Lock()
	ids := make([]string, 0, len(s.branches))
	for id := range s.branches {
		ids = append(ids, id)
	}
	s.mu.Unlock()
	summaries := make([]actions.AgentSummary, 0, len(ids))
	for _, id := range ids {
		summaries = append(summaries, s.summary(id))
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].StartedAt < summaries[j].StartedAt })
	return summaries
}

func (s *Supervisor) WaitAgents(ctx context.Context, ids []string) ([]actions.WaitAgentsResult, error) {
	if len(ids) == 0 {
		for _, summary := range s.ListAgents() {
			ids = append(ids, summary.ID)
		}
	}
	for _, id := range ids {
		branch := s.branch(id)
		if branch == nil {
			return nil, fmt.Errorf("agent branch %q was not found", id)
		}
		select {
		case <-branch.done:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	results := make([]actions.WaitAgentsResult, 0, len(ids))
	for _, id := range ids {
		branch := s.branch(id)
		branch.mu.Lock()
		results = append(results, actions.WaitAgentsResult{Summary: branch.summary, Output: strings.TrimSpace(branch.output.String())})
		branch.mu.Unlock()
	}
	return results, nil
}

func (s *Supervisor) StopAgent(id string) bool {
	branch := s.branch(id)
	if branch == nil {
		return false
	}
	return branch.agent.Stop()
}

func (s *Supervisor) branch(id string) *workerBranch {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.branches[id]
}

func (s *Supervisor) summary(id string) actions.AgentSummary {
	branch := s.branch(id)
	if branch == nil {
		return actions.AgentSummary{ID: id, Status: "missing"}
	}
	branch.mu.Lock()
	defer branch.mu.Unlock()
	return branch.summary
}
