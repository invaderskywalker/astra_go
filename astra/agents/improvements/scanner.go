package improvements

import (
	"astra/astra/services/llm"
	"astra/astra/utils/jsonutils"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Scan is observation-only: it cannot edit source, create branches, or run arbitrary commands.
func Scan(ctx context.Context, client llm.LLMClient, model, workspace string) (Proposal, error) {
	evidence := collectEvidence(ctx, workspace)
	prompt := fmt.Sprintf(`You are the local Astra improvement scout. Analyze only the supplied evidence. Propose ONE small, measurable improvement to Astra itself. Do not suggest edits outside the repository. Do not claim tests passed unless evidence says so. Return JSON only with title, objective, evidence, proposed_actions, validation, and risk. Every action must be reviewable and require human approval.

Evidence:
%s`, evidence)
	response, err := client.Run(ctx, llm.ChatRequest{Model: model, Messages: []llm.Message{{Role: "system", Content: "You are an evidence-first software quality analyst. You never edit code."}, {Role: "user", Content: prompt}}})
	if err != nil {
		return Proposal{}, err
	}
	var proposal Proposal
	if err := json.Unmarshal([]byte(jsonutils.ExtractJSON(response)), &proposal); err != nil {
		return Proposal{}, fmt.Errorf("Qwen returned invalid proposal JSON: %w", err)
	}
	proposal.Model, proposal.Workspace, proposal.Status, proposal.RequiresApproval = model, workspace, ReviewReady, true
	return proposal, nil
}

func ReviewProposal(ctx context.Context, client llm.LLMClient, model string, proposal Proposal) (Review, error) {
	data, _ := json.MarshalIndent(proposal, "", "  ")
	prompt := fmt.Sprintf(`You are Astra's master reviewer. Review this proposed self-improvement. Return JSON only with recommendation (approve, reject, or needs_evidence), rationale, and missing_evidence. Reject broad, unsafe, unmeasurable, or unsupported changes. Approval means it is safe to ask the human for permission, not permission to execute.

Proposal:
%s`, data)
	response, err := client.Run(ctx, llm.ChatRequest{Model: model, Messages: []llm.Message{{Role: "system", Content: "You are a cautious engineering reviewer. Evidence and testability matter more than novelty."}, {Role: "user", Content: prompt}}})
	if err != nil {
		return Review{}, err
	}
	var review Review
	if err := json.Unmarshal([]byte(jsonutils.ExtractJSON(response)), &review); err != nil {
		return Review{}, fmt.Errorf("review model returned invalid JSON: %w", err)
	}
	review.ProposalID, review.Model = proposal.ID, model
	return review, nil
}

func collectEvidence(ctx context.Context, workspace string) string {
	commands := [][]string{{"git", "status", "--short"}, {"go", "test", "./..."}}
	parts := []string{}
	for _, command := range commands {
		cmd := exec.CommandContext(ctx, command[0], command[1:]...)
		cmd.Dir = workspace
		output, err := cmd.CombinedOutput()
		text := string(output)
		if err != nil {
			text += "\ncommand error: " + err.Error()
		}
		if len(text) > 12000 {
			text = text[:12000] + "\n[truncated]"
		}
		parts = append(parts, "$ "+strings.Join(command, " ")+"\n"+text)
	}
	return strings.Join(parts, "\n\n")
}
