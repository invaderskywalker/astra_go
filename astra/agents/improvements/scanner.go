package improvements

import (
	"astra/astra/agents/prompts"
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
	response, err := client.Run(ctx, llm.ChatRequest{Model: model, Messages: []llm.Message{{Role: "system", Content: prompts.ImprovementScoutSystem()}, {Role: "user", Content: prompts.ImprovementScoutUser(evidence)}}})
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
	response, err := client.Run(ctx, llm.ChatRequest{Model: model, Messages: []llm.Message{{Role: "system", Content: prompts.ImprovementReviewerSystem()}, {Role: "user", Content: prompts.ImprovementReviewerUser(string(data))}}})
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
