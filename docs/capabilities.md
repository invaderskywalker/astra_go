# Astra capability map

This is the living map of what Astra can do, where the ability is implemented,
what evidence it should produce, and what remains to be built. Update this file
when an action, prompt contract, or user-facing workflow changes.

## Operating model

Astra is a model-directed system with explicit runtime context:

```text
user request
  -> living task state (intent, mode, skills, success criteria, evidence)
  -> executor (one concrete action at a time, reassessed after every result)
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
| Repository intelligence | Detects polyglot stacks, monorepo modules, frameworks, test signals, project manifests, generated content, and likely validation commands before language-specific work | Ecosystems, languages, framework evidence, manifests, test paths, bounded validation command suggestions, scan warnings | `astra/agents/actions/stack_detection.go` |
| Workspace orientation | Lists files, creates directory trees, searches text, analyzes file metadata/structure, reads bounded files, and inspects Go structure | Paths, sizes, hashes, line counts, headings, symbols, matches, recommended ranges, snippets, diagnostics; generated caches and binary artifacts are excluded from recursive analysis | `astra/agents/workspace`, `astra/agents/actions` |
| Code delivery | Applies precise edits, then can build/test | Changed files plus validator output | `astra/agents/actions/edits.go`, `engineering.go` |
| Command execution | Runs one command or a short ordered sequence, each with an explicit working directory, timeout, and captured output; expected non-zero checks can be marked | stdout, stderr, exit code, duration, per-step results | `astra/agents/workspace/commands.go`, `astra/agents/actions/engineering.go` |
| Web research | Searches current external information and scrapes supplied URLs when needed | Source URLs and extracted content | `astra/agents/actions/scraping_action.go` |
| File artifacts | Writes validated Markdown, JSON, JSONL, CSV, or text to the private external project/session artifact area; the same managed root can be read back by `read_files` | Exact artifact path and format, independently readable within Astra's managed scope | `astra/agents/actions/artifacts.go`, `fs.go` |
| Mind Palace | Stores curated linked Markdown memory and append-only session evidence | Memory file, provenance, links | `astra/agents/actions/learning_knowledge.go`, `sources/mindpalace` |
| Local identity | Single private profile with explicit signup/login/logout; no directory-derived users | Owner-only profile and login marker under `~/.astra/identity` | `astra/sources/identity`, `astra/cmd/auth.go` |
| Local storage | Session history, manifests, artifacts, attachments, and Mind Palace are file-backed outside connected repositories; external sync is disabled by default | Portable files with local status records | `astra/sources/state`, `astra/sources/mindpalace` |
| Interactive CLI | Full-screen cockpit with an isolated multiline composer, scrollable transcript, queued requests, model switching, pause/resume/stop/clear, and keyboard navigation; `--plain` remains available for pipes | Terminal events, status, and view state | `astra/cmd/tui.go`, `astra/cmd/input.go` |
| Session/run continuity | Each session contains durable run records. The first top-level message opens a `run_id`; input submitted while it is active is stored as `user_update` evidence and reassessed by the same living task | Run manifest, per-run events, task state, artifacts, and stream envelopes carrying both IDs | `astra/agents/core/base_agent.go`, `astra/sources/state` |
| CLI workspace views | Dashboard, project files, user Mind Palace, session manifests/evidence, and managed sync configuration are navigable from one shell | Read-only panels, metrics, Unicode bars, and bounded file listings | `astra/cmd/tui.go`, `astra/cmd/views.go`, `astra/sources/state` |
| Self-improvement | Qwen can scan for one bounded proposal; Luna can review; human approves | Reviewable proposal file | `astra/agents/improvements` |
| Action bookmarks | Every action has a compact bookmark; the executor activates full documentation for the next one to five tools and records fallback activation when needed | Bookmark catalog, activation report, action activation event | `astra/agents/actions/action_registry.go`, `astra/agents/prompts` |
| Agent bookmarks | Main agent can route a request through role-oriented capability groupings without granting hidden permissions | Agent bookmark catalog in execution context | `astra/agents/prompts/agents.go` |
| Filesystem scopes | Stores explicit read/write/execute approvals for additional directories and re-checks command working directories at runtime | Private scope registry and denied/approved command evidence | `astra/sources/scope`, `astra/agents/actions/scopes.go` |
| Worker agents | Supervisor can spawn bounded goal-oriented branches, wait for results, stop workers, and expose lifecycle metadata | Branch IDs, statuses, goals, models, event counts, and outputs | `astra/agents/core/supervisor.go`, `astra/agents/actions/agents.go` |
| Prompt/personality profiles | Stores enabled user-authored Markdown profiles globally and injects them as lower-priority preferences | Profile files, index, enabled/disabled status | `astra/sources/promptstore`, `astra/agents/actions/prompt_profiles.go` |

## Scope and authority

The connected workspace is the source-code boundary. Astra-owned project and
session files default to `~/.astra/projects/` and can be relocated with
`ASTRA_DATA_DIR`. It is not a
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
- `ExecutionSystem` maintains the living task state, chooses one typed action,
  and updates evidence, completed work, remaining work, blockers, and
  verification after every result.
- `ResponseSystem` turns evidence into a clean handoff.
- `skills.go` contains reusable judgment modules. A skill does not add tools or
  authority; it tells the executor how to use existing tools well.
- `ActionCatalog` is the compact bookmark view. `activate_actions` and
  `ActivatedActionDocumentation` provide full schemas only when needed.
- `agents.go` groups related tools into role bookmarks. These are routing hints,
  not independent agents or authorization grants.
- Planning and execution use a general evidence contract: explicit acceptance
  criteria drive the minimum action set, negative claims require supporting
  checks, task mode cannot finish without required evidence, and clarification
  is reserved for material unresolved decisions.
- Each request has one adaptive living task state rather than a separate rough
  plan. The state is updated after every action with current evidence,
  completed work, remaining work, blockers, verification status, and the next
  action.
- Mind Palace organization follows a Sherlock-style evidence graph: one idea
  per block, explicit confidence and provenance, hubs for stable domains,
  reciprocal links to related decisions and artifacts, and supersession when
  newer evidence contradicts an older block. Retrieval follows the smallest
  relevant chain instead of loading the archive as a transcript.
- Large-file analysis is progressive: `analyze_files` streams metadata and
  structural signals without returning source bodies; `search_code` locates
  evidence; `read_files` streams only requested line ranges. Results are
  context-bounded, so Astra can iterate over different parts of a file without
  exhausting the model window.

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

Execution requests use a progress-aware watchdog rather than a small normal
step limit. Astra may continue through a long inspect → change → verify
workflow while the living task state or evidence changes. Six unchanged turns
or three identical failures checkpoint the task as blocked with a recovery
instruction. A last-resort emergency ceiling defaults to 256 actions and can
be adjusted with `ASTRA_MAX_ACTION_STEPS` (capped at 2048); reaching it never
produces a completion claim.
