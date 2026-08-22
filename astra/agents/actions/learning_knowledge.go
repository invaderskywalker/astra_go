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
	record, warnings, err := a.memory.Save(context.Background(), mindpalace.Record{ID: params.ID, Kind: params.Kind, Title: params.Title, Summary: params.Summary, Content: params.Content, Tags: params.Tags, Related: params.Related, Importance: params.Importance, Confidence: params.Confidence, Source: params.Source})
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
	a.register(ActionSpec{Name: "save_memory", Description: "Creates or refines a linked, durable file-backed memory block.", Guidance: "Store verified, reusable facts, decisions, project conventions, or learned preferences as concise Markdown. Reuse kind + title to refine an existing block; use ID only for an explicit update. Set importance 1-5 for retrieval priority, confidence to observed/inferred/confirmed, and source to the evidence location. Do not save raw conversation transcripts as memory.", Params: SaveMemoryParams{}, handler: decodeHandler(a.SaveMemory)})
	a.register(ActionSpec{Name: "search_memory", Description: "Searches the current user's file-backed mind palace.", Guidance: "Search before asking the user for information that may already be remembered. Read only returned blocks relevant to the current task.", Params: SearchMemoryParams{}, handler: decodeHandler(a.SearchMemory)})
	a.register(ActionSpec{Name: "list_memory", Description: "Lists memory blocks, optionally by kind.", Guidance: "Use only to orient yourself or when a focused search query is unavailable.", Params: ListMemoryParams{}, handler: decodeHandler(a.ListMemory)})
	a.register(ActionSpec{Name: "link_memory", Description: "Creates a reciprocal link between two memory blocks.", Guidance: "Use links to connect facts, decisions, and project notes that belong in the same reasoning path. Both blocks will show the relationship.", Params: LinkMemoryParams{}, handler: decodeHandler(a.LinkMemory)})
}
