package actions

import (
	"context"
	"fmt"
	"strings"
)

// AgentSummary is the stable cross-package view used by the planner and CLI.
// It intentionally contains status and evidence metadata, not private model
// reasoning.
type AgentSummary struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Personality   string `json:"personality,omitempty"`
	Goal          string `json:"goal"`
	WorkspaceRoot string `json:"workspace_root"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	Status        string `json:"status"`
	StartedAt     string `json:"started_at"`
	CompletedAt   string `json:"completed_at,omitempty"`
	Events        int    `json:"events"`
	Error         string `json:"error,omitempty"`
}

type SpawnAgentRequest struct {
	Name          string
	Personality   string
	Goal          string
	WorkspaceRoot string
	Provider      string
	Model         string
}

type SpawnAgentResult struct {
	Summary AgentSummary `json:"summary"`
}

type WaitAgentsResult struct {
	Summary AgentSummary `json:"summary"`
	Output  string       `json:"output,omitempty"`
}

// AgentSpawner is implemented by the supervisor in the core package. Keeping
// this interface here avoids a package cycle while allowing actions to remain
// planner-callable.
type AgentSpawner interface {
	SpawnAgent(context.Context, SpawnAgentRequest) (SpawnAgentResult, error)
	ListAgents() []AgentSummary
	WaitAgents(context.Context, []string) ([]WaitAgentsResult, error)
	StopAgent(string) bool
}

type SpawnAgentParams struct {
	Name          string `json:"name,omitempty"`
	Personality   string `json:"personality,omitempty"`
	Goal          string `json:"goal"`
	WorkspaceRoot string `json:"workspace_root,omitempty"`
	Provider      string `json:"provider,omitempty"`
	Model         string `json:"model,omitempty"`
}

type WaitAgentsParams struct {
	AgentIDs []string `json:"agent_ids,omitempty"`
}

func (a *DataActions) SpawnAgent(params SpawnAgentParams) ActionResult {
	if a.spawner == nil {
		return ActionResult{Success: false, Error: "agent supervisor is not available in this runtime"}
	}
	if strings.TrimSpace(params.Goal) == "" {
		return ActionResult{Success: false, Error: "goal is required"}
	}
	result, err := a.spawner.SpawnAgent(context.Background(), SpawnAgentRequest{Name: params.Name, Personality: params.Personality, Goal: params.Goal, WorkspaceRoot: params.WorkspaceRoot, Provider: params.Provider, Model: params.Model})
	if err != nil {
		return ActionResult{Success: false, Error: err.Error()}
	}
	return ActionResult{Success: true, Summary: fmt.Sprintf("Spawned agent %s", result.Summary.ID), Diagnostics: result.Summary}
}

func (a *DataActions) ListAgents(_ struct{}) ActionResult {
	if a.spawner == nil {
		return ActionResult{Success: false, Error: "agent supervisor is not available in this runtime"}
	}
	return ActionResult{Success: true, Summary: fmt.Sprintf("Listed %d agent branch(es)", len(a.spawner.ListAgents())), Diagnostics: a.spawner.ListAgents()}
}

func (a *DataActions) WaitAgents(params WaitAgentsParams) ActionResult {
	if a.spawner == nil {
		return ActionResult{Success: false, Error: "agent supervisor is not available in this runtime"}
	}
	results, err := a.spawner.WaitAgents(context.Background(), params.AgentIDs)
	if err != nil {
		return ActionResult{Success: false, Error: err.Error(), Diagnostics: results}
	}
	return ActionResult{Success: true, Summary: fmt.Sprintf("Joined %d agent branch(es)", len(results)), Diagnostics: results}
}

func (a *DataActions) registerAgentActions() {
	a.register(ActionSpec{Name: "spawn_agent", Description: "Starts a bounded worker agent with an explicit goal, personality, model, and approved workspace scope.", Guidance: "Use for genuinely parallel research, implementation, verification, or documentation work. Give each worker one non-overlapping goal and a concrete success condition. Spawn only when parallel work reduces time or improves coverage; the supervisor must wait for and reconcile results.", Params: SpawnAgentParams{}, Category: "orchestration", Approval: "Worker creation is local and reviewable; scope expansion and destructive work still require the normal authority boundary.", handler: decodeHandler(a.SpawnAgent)})
	a.register(ActionSpec{Name: "list_agents", Description: "Lists worker branches and their lifecycle status.", Guidance: "Use to inspect active, completed, failed, or stopped workers before deciding whether to wait or report.", Params: struct{}{}, Category: "orchestration", handler: decodeHandler(a.ListAgents)})
	a.register(ActionSpec{Name: "wait_agents", Description: "Waits for selected worker branches and returns their outcomes.", Guidance: "Use after spawning workers. Do not claim a parallel task is complete until the required branches have returned and their evidence has been reconciled.", Params: WaitAgentsParams{}, Category: "orchestration", handler: decodeHandler(a.WaitAgents)})
}
