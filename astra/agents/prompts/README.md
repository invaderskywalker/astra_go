# Astra prompt architecture

Astra uses layered, code-owned prompts. The goal is not to make one enormous
prompt; it is to give each model call the smallest complete contract for its
job while sharing one consistent operating policy.

## Layers

1. `EngineeringPolicy` — identity, authority, evidence, workspace, memory,
   artifacts, recovery, and completion rules shared by every call.
2. Living task state — records the interpreted request, interaction mode,
   applicable skills, success criteria, evidence, completed and remaining work,
   blockers, verification, and next action.
3. Execution prompt — reassesses that living state after every result and acts
   as a state machine. It selects one registered action at a time, records
   expected evidence, handles failure, and stops at a clear condition.
4. Response prompt — converts execution evidence into a truthful user handoff.
5. `skills.go` — reusable competencies. Skills guide judgment; they do not grant
   tools or permissions.
6. Action bookmarks and activation — the executor sees compact capability
   bookmarks. Before a first tool call, `activate_actions` loads the selected
   action's complete prose contract, parameter schema, examples, return shape,
   side effects, approval rule, and failure recovery guidance. Activation is
   limited to five actions per call. The runtime also auto-hydrates a skipped
   activation as a safety fallback and records that event.
7. Agent bookmarks — role-oriented groupings such as `repository_operator`,
   `artifact_author`, and `verification_engineer`. These are routing hints,
   not separate permission systems or hidden sub-agents.

## Decision quality contract

Prompt behavior is outcome-driven rather than repository-specific:

- Explicit user acceptance criteria are binding. The executor identifies the
  evidence that proves each criterion and selects the smallest sufficient set
  of actions.
- Negative claims require negative evidence. A listing establishes structure,
  a read establishes contents, and a command establishes executable behavior;
  none of these are interchangeable.
- A task does not complete from an assumption when evidence is required. The
  executor must orient or inspect first, unless the necessary evidence was
  already supplied.
- Clarification is a last-mile decision gate, not a substitute for safe
  inspection. Astra asks only when a material choice cannot be resolved from
  the request, evidence, conventions, or a reversible default.
- Once acceptance criteria are satisfied, Astra stops. It does not explore
  unrelated files or create extra reports merely because more tools exist.

## Context priority

Current user request → current workspace evidence → conversation commitments →
file-backed mind-palace memory → explicit assumptions. Memory guides retrieval;
it does not override current evidence.

## Prompt changes

When adding a rule, decide which layer owns it and state it once. Prefer an
outcome, success criteria, constraints, evidence requirements, and stop rule
over a brittle hard-coded call sequence. Update `PromptVersion`, add a prompt
test for the behavioral contract, and run `go test ./...`.

The CLI may display plans and action summaries, but raw prompts, credentials,
and unprocessed telemetry must never be sent to the user.

## Tool documentation contract

Every registered action has two code-owned views:

- `ActionBookmark`: small enough to include on every planning turn (`name`,
  category, purpose, when to use, risk, and related actions).
- `ActionDocumentation`: loaded on demand and generated from the same action
  registration (`parameters`, examples, return contract, side effects,
  approval, and bounded failure recovery).

This keeps prompts substantial and explicit without paying the context cost of
every schema on every call. It also makes the action surface reviewable in Go,
without a second YAML instruction language.
