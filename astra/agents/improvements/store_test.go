package improvements

import "testing"

func TestProposalLifecycle(t *testing.T) {
	store := New(t.TempDir())
	proposal, err := store.SaveProposal(Proposal{Title: "Add tests", Objective: "Improve safety", Evidence: []string{"tests missing"}, Risk: "low"})
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Status != ReviewReady || !proposal.RequiresApproval {
		t.Fatalf("unexpected proposal: %#v", proposal)
	}
	approved, err := store.SetStatus(proposal.ID, Approved)
	if err != nil {
		t.Fatal(err)
	}
	if approved.Status != Approved {
		t.Fatal("proposal was not approved")
	}
	if err := store.SaveReview(Review{ProposalID: proposal.ID, Model: "gpt-5.6-luna", Recommendation: "approve", Rationale: "safe"}); err != nil {
		t.Fatal(err)
	}
	listed, err := store.List()
	if err != nil || len(listed) != 1 || listed[0].Status != Approved {
		t.Fatalf("unexpected list: %#v %v", listed, err)
	}
}
