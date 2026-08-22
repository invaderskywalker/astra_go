# Astra capability map

This is the living map of what Astra can do, where the ability is implemented,
what evidence it should produce, and what remains to be built. Update this file
when an action, prompt contract, or user-facing workflow changes.

## Operating model

Astra is a model-directed system with explicit runtime context:

```text
user request
  -> planner (intent, mode, skills, success criteria)
  -> executor (one concrete action at a time)
  -> typed action (local, web, memory, or artifact capability)
  -> evidence
  -> response writer (truthful handoff)
```

Prompts guide judgment for one model call. Go action handlers provide the real
capability. Runtime context—workspace root, model, session, history, memory,
and available actions—is injected into the prompts instead of being guessed.

## Capability matrix

| Capability | Current behavior | Evidence/output | Primary implementation |
| --- | --- | --- | --- |
| Conversation | Answers questions and explains concepts without pretending tools ran | User-facing response | `astra/agents/prompts` and `BaseAgent` |
| Workspace orientation | Lists files, creates directory trees, searches text, reads bounded files, and inspects Go structure | Paths, line ranges, snippets, diagnostics | `astra/agents/workspace`, `astra/agents/actions` |
| Code delivery | Applies precise edits, then can build/test | Changed files plus validator output | `astra/agents/actions/edits.go`, `engineering.go` |
| Command execution | Runs one command or a short ordered sequence, each with an explicit working directory, timeout, and captured output; expected non-zero checks can be marked | stdout, stderr, exit code, duration, per-step results | `astra/agents/workspace/commands.go`, `astra/agents/actions/engineering.go` |
| Web research | Searches current external information and scrapes supplied URLs when needed | Source URLs and extracted content | `astra/agents/actions/scraping_action.go` |
| File artifacts | Writes validated Markdown, JSON, JSONL, CSV, or text to the session artifact area | Exact artifact path and format | `astra/agents/actions/artifacts.go` |
| Mind Palace | Stores curated linked Markdown memory and append-only session evidence | Memory file, provenance, links | `astra/agents/actions/learning_knowledge.go`, `sources/mindpalace` |
| Interactive CLI | Supports multiline editing, bracketed paste, history, queued requests, model switching, pause/resume/stop/clear | Terminal events and status | `astra/cmd` |
| Self-improvement | Qwen can scan for one bounded proposal; Luna can review; human approves | Reviewable proposal file | `astra/agents/improvements` |
| Action bookmarks | Every action has a compact bookmark; the executor activates full documentation for the next one to five tools and records fallback activation when needed | Bookmark catalog, activation report, action activation event | `astra/agents/actions/action_registry.go`, `astra/agents/prompts` |
| Agent bookmarks | Main agent can route a request through role-oriented capability groupings without granting hidden permissions | Agent bookmark catalog in planning context | `astra/agents/prompts/agents.go` |

## Scope and authority

The connected workspace is the default local project boundary. It is not a
claim that Astra can only converse about that directory. Astra can also answer
general questions, use current web research, create user-facing artifacts,
retrieve linked memory, and run explicit local commands. Every action still has
an authority rule:

- read-only inspection is allowed when it is relevant to the request;
- requested implementation may edit in-scope files and validate them;
- commands are executed with a bounded working directory and timeout;
- command parameters and concise results are shown in the CLI action log;
- external or destructive side effects require an explicit user decision;
- tools are capabilities, not permission to use them indiscriminately.

When a user asks which directory Astra can work in, the response must use the
exact `Connected workspace root` injected at runtime. When a repository task is
vague but safely orientable, Astra should perform a focused inspection rather
than asking the user to repeat information already available from runtime
context.

## Prompt and skill architecture

- `EngineeringPolicy` is the shared behavioral contract.
- `PlanningSystem` chooses mode, skills, evidence, and acceptance checks.
- `ExecutionSystem` chooses one typed action and tracks results.
- `ResponseSystem` turns evidence into a clean handoff.
- `skills.go` contains reusable judgment modules. A skill does not add tools or
  authority; it tells the planner how to use existing tools well.
- `ActionCatalog` is the compact bookmark view. `activate_actions` and
  `ActivatedActionDocumentation` provide full schemas only when needed.
- `agents.go` groups related tools into role bookmarks. These are routing hints,
  not independent agents or authorization grants.
- Planning and execution use a general evidence contract: explicit acceptance
  criteria drive the minimum action set, negative claims require supporting
  checks, task mode cannot finish without required evidence, and clarification
  is reserved for material unresolved decisions.

This layered design keeps prompts substantial and reviewable without hiding
behavior in small YAML fragments or silently changing the agent at runtime.

## Roadmap

1. Add per-capability validators and
   artifact contracts.
2. Add attachment metadata and explicit external-file import workflows.
3. Add richer repository maps and dependency graphs to the file-backed mind
   palace.
4. Add resumable execution checkpoints and a human approval UI for risky plans.
5. Add evaluation scenarios for truthful scope answers, command safety, file
   format correctness, and memory retrieval quality.

## Recovery controls

If a large paste was accidentally submitted as many requests, use `:clear`.
It requests cancellation of the active model call and drains all queued work.
On terminals with bracketed-paste support, multiline clipboard content is now
inserted into one draft and submitted only when the user presses Enter once.
For an older binary, use `:stop`, exit Astra, rebuild, and restart. To unload a
local Ollama model manually, inspect it with `ollama ps` and stop the named
model with `ollama stop <model>`.

Execution requests are bounded to 48 action turns per request. Reaching that
bound is now reported as a blocker and does not produce a success/completion
claim. This leaves room for real scaffold-and-verify workflows while preventing
an accidental planner loop from running forever.
