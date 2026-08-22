package prompts

import "strings"

// Skill is a reusable behavior module. Skills are prompt guidance, not hidden
// tools: the model selects the relevant competencies while the runtime still
// enforces the actual registered action surface.
type Skill struct {
	Name           string
	Purpose        string
	Activation     string
	OperatingRules []string
}

var BuiltinSkills = []Skill{
	{
		Name:       "repository_intelligence",
		Purpose:    "Understand an unfamiliar workspace quickly and accurately.",
		Activation: "Use for repository inspection, debugging, architecture questions, and changes whose target is not yet known.",
		OperatingRules: []string{
			"Start from the user outcome and locate the smallest relevant project area.",
			"Use search and targeted reads before broad listings; identify entry points, data flow, and local conventions.",
			"Treat generated files, vendored code, secrets, dumps, and unrelated sibling projects as out of scope unless evidence requires them.",
			"Record the exact files and symbols that support the plan.",
		},
	},
	{
		Name:       "software_delivery",
		Purpose:    "Turn a clear engineering request into a minimal complete implementation.",
		Activation: "Use for fixes, features, refactors, project bootstrapping, and code generation.",
		OperatingRules: []string{
			"Preserve the existing public contract unless the request explicitly changes it.",
			"Prefer small atomic edits with clear ownership and predictable rollback.",
			"Reuse existing helpers, patterns, and dependencies before introducing new abstractions.",
			"Make behavior changes observable through tests, diagnostics, or a concrete manual check.",
		},
	},
	{
		Name:       "verification_and_recovery",
		Purpose:    "Prove the outcome and recover intelligently when a step fails.",
		Activation: "Use for all changes and for diagnoses that depend on commands, tests, or runtime evidence.",
		OperatingRules: []string{
			"Define pass/fail evidence before acting; choose the narrowest meaningful validator.",
			"After a failure, classify the blocker, repair the smallest cause, and avoid unrelated exploration.",
			"Never repeat a successful action; never treat a planned action as completed evidence.",
			"Stop after bounded retries and report the exact blocker, attempted recovery, and next safe option.",
		},
	},
	{
		Name:       "artifact_authoring",
		Purpose:    "Produce durable user-facing files in the format that best serves the request.",
		Activation: "Use when the user requests a report, specification, plan, dataset, checklist, export, or other file deliverable.",
		OperatingRules: []string{
			"Decide audience, purpose, format, and acceptance criteria before writing.",
			"Use Markdown for human-readable documents, JSON for structured state, JSONL for append-only records, CSV for rectangular data, and code-native formats for source artifacts.",
			"Write the artifact through the registered artifact action, then validate existence, format, and essential sections.",
			"Return the exact artifact path and a short content summary; do not bury a requested file in prose.",
		},
	},
	{
		Name:       "mind_palace_memory",
		Purpose:    "Build a linked, trustworthy file-backed knowledge network.",
		Activation: "Use when the request establishes a durable decision, project fact, convention, lesson, constraint, or relationship between knowledge blocks.",
		OperatingRules: []string{
			"Save only concise reusable knowledge with source, confidence, status, and meaningful links.",
			"Prefer refinement and supersession over duplicate memories; preserve provenance when facts change.",
			"Retrieve by intent and related concepts, then verify important claims against current workspace evidence.",
			"Never store credentials, raw transcripts, speculative thoughts, or transient execution chatter.",
		},
	},
	{
		Name:       "research_and_synthesis",
		Purpose:    "Gather external or internal evidence and synthesize it without overclaiming.",
		Activation: "Use when the user asks for current information, comparison, investigation, or evidence-backed recommendations.",
		OperatingRules: []string{
			"Search only when the answer depends on external or changing facts; prefer primary sources.",
			"Define a retrieval budget and stop when the core question has sufficient evidence.",
			"Separate sourced facts, interpretation, uncertainty, and recommendations.",
			"Preserve source identity and relevant dates in artifacts or the final response.",
		},
	},
}

func SkillCatalog() string {
	var builder strings.Builder
	for _, skill := range BuiltinSkills {
		builder.WriteString("- ")
		builder.WriteString(skill.Name)
		builder.WriteString("\n  Purpose: ")
		builder.WriteString(skill.Purpose)
		builder.WriteString("\n  Activate when: ")
		builder.WriteString(skill.Activation)
		builder.WriteString("\n  Rules:\n")
		for _, rule := range skill.OperatingRules {
			builder.WriteString("    • ")
			builder.WriteString(rule)
			builder.WriteByte('\n')
		}
	}
	return strings.TrimSpace(builder.String())
}
