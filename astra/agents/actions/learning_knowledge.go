package actions

import (
	"astra/astra/sources/mindpalace"
	"context"
	"fmt"
	"strings"
)

type SaveMemoryParams struct {
	ID         string   `json:"id,omitempty"`
	Kind       string   `json:"kind"`
	Title      string   `json:"title"`
	Summary    string   `json:"summary"`
	Content    string   `json:"content"`
	Tags       []string `json:"tags,omitempty"`
	Related    []string `json:"related,omitempty"`
	Importance int      `json:"importance,omitempty"`
	Confidence string   `json:"confidence,omitempty"`
	Source     string   `json:"source,omitempty"`
	Status     string   `json:"status,omitempty"`
	Supersedes []string `json:"supersedes,omitempty"`
}
type SearchMemoryParams struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}
type ListMemoryParams struct {
	Kind string `json:"kind,omitempty"`
}
type LinkMemoryParams struct {
	FromID string `json:"from_id"`
	ToID   string `json:"to_id"`
}

func (a *DataActions) SaveMemory(params SaveMemoryParams) ActionResult {
	if strings.TrimSpace(params.Kind) == "" || strings.TrimSpace(params.Title) == "" || strings.TrimSpace(params.Content) == "" {
		return ActionResult{Success: false, Error: "kind, title, and content are required"}
	}
	record, warnings, err := a.memory.Save(context.Background(), mindpalace.Record{ID: params.ID, Kind: params.Kind, Title: params.Title, Summary: params.Summary, Content: params.Content, Tags: params.Tags, Related: params.Related, Importance: params.Importance, Confidence: params.Confidence, Source: params.Source, Status: params.Status, Supersedes: params.Supersedes})
	if err != nil {
		return ActionResult{Success: false, Error: err.Error()}
	}
	return ActionResult{Success: true, Summary: "Memory saved: " + record.ID, Diagnostics: record, Artifacts: []string{record.ID}, Warnings: warnings}
}

func (a *DataActions) SearchMemory(params SearchMemoryParams) ActionResult {
	if strings.TrimSpace(params.Query) == "" {
		return ActionResult{Success: false, Error: "query is required"}
	}
	records, err := a.memory.Search(params.Query, params.Limit)
	if err != nil {
		return ActionResult{Success: false, Error: err.Error()}
	}
	return ActionResult{Success: true, Summary: fmt.Sprintf("Found %d memory block(s)", len(records)), Diagnostics: records}
}

// MemoryContext is used by the planner automatically; it is intentionally
// separate from search_memory so the model does not have to remember to call a
// retrieval tool before every request.
func (a *DataActions) MemoryContext(query string, limit int) (string, error) {
	return a.memory.Context(query, limit)
}
func (a *DataActions) ListMemory(params ListMemoryParams) ActionResult {
	records, err := a.memory.List(params.Kind)
	if err != nil {
		return ActionResult{Success: false, Error: err.Error()}
	}
	return ActionResult{Success: true, Summary: fmt.Sprintf("Listed %d memory block(s)", len(records)), Diagnostics: records}
}
func (a *DataActions) LinkMemory(params LinkMemoryParams) ActionResult {
	warnings, err := a.memory.Link(context.Background(), params.FromID, params.ToID)
	if err != nil {
		return ActionResult{Success: false, Error: err.Error()}
	}
	return ActionResult{Success: true, Summary: "Memory link created", Warnings: warnings}
}

func (a *DataActions) registerKnowledgeActions() {
	a.register(ActionSpec{Name: "save_memory", Description: "Creates or refines a linked, durable file-backed memory block.", Guidance: "Store one verified, reusable idea as concise Markdown. Classify it as a fact, decision, convention, lesson, project, workflow, artifact, open question, or hub. Reuse kind + title to refine an existing block; use ID only for an explicit update. Set importance 1-5, confidence to observed/inferred/confirmed, source to the evidence location, meaningful tags, and related links. Mark conflicting old knowledge superseded instead of silently deleting it. Do not save raw conversation transcripts or guesses.", Category: "memory", WhenToUse: "Use after a verified decision, reusable convention, project fact, lesson, user preference, or durable artifact relationship is established.", NeverUseWhen: "Do not use it for transient progress, unverified assumptions, secrets, or a complete chat transcript.", Returns: "The saved memory record, Markdown path, stable ID, and any non-fatal warnings.", SideEffects: "Writes or refines user-private Markdown and the file-backed memory index.", Approval: "No approval for routine private memory updates explicitly justified by current evidence.", FailureRecovery: "If validation fails, correct the kind, title, content, confidence, or importance; do not create a duplicate block to bypass identity matching.", RelatedActions: []string{"search_memory", "link_memory"}, Params: SaveMemoryParams{}, handler: decodeHandler(a.SaveMemory)})
	a.register(ActionSpec{Name: "search_memory", Description: "Searches the current user's file-backed mind palace and returns ranked clues.", Guidance: "Search before asking the user for information that may already be remembered. Use intent-rich terms, inspect returned provenance and confidence, follow only relevant links, and verify important claims against current workspace evidence.", Category: "memory", WhenToUse: "Use when prior decisions, conventions, project facts, preferences, or lessons could change the current action.", NeverUseWhen: "Do not treat a search hit as proof when the workspace or user request can provide newer evidence.", Returns: "Ranked active memory records with summaries, content, confidence, provenance, and related IDs.", SideEffects: "Read-only.", Approval: "No approval required.", FailureRecovery: "Broaden or narrow the query deliberately; if no result is found, state that durable memory was unavailable rather than inventing it.", RelatedActions: []string{"list_memory", "link_memory"}, Params: SearchMemoryParams{}, handler: decodeHandler(a.SearchMemory)})
	a.register(ActionSpec{Name: "list_memory", Description: "Lists memory blocks, optionally by kind, for Mind Palace orientation.", Guidance: "Use to inspect a hub or category when search is insufficient. Prefer a focused kind and follow links instead of dumping the entire archive into context.", Category: "memory", WhenToUse: "Use when orienting within a known Mind Palace area or maintaining a hub.", NeverUseWhen: "Do not list the entire archive for an unrelated task.", Returns: "File-backed memory records matching the optional kind filter.", SideEffects: "Read-only.", Approval: "No approval required.", FailureRecovery: "Use a narrower kind or report the index error without fabricating memory.", RelatedActions: []string{"search_memory", "link_memory"}, Params: ListMemoryParams{}, handler: decodeHandler(a.ListMemory)})
	a.register(ActionSpec{Name: "link_memory", Description: "Creates a reciprocal link between two memory blocks.", Guidance: "Use links to connect a hub to details, a decision to its evidence or convention, a lesson to the project where it applies, and an artifact to the knowledge it represents. Both blocks will show the relationship.", Category: "memory", WhenToUse: "Use when two verified blocks should be traversed together during future retrieval.", NeverUseWhen: "Do not add speculative, decorative, or unrelated links.", Returns: "Updated Markdown paths and the reciprocal relationship result.", SideEffects: "Updates both linked Markdown files and the file-backed index.", Approval: "No approval required for explicit knowledge organization.", FailureRecovery: "Verify both IDs from search/list results, then retry with the exact IDs.", RelatedActions: []string{"save_memory", "search_memory"}, Params: LinkMemoryParams{}, handler: decodeHandler(a.LinkMemory)})
}
