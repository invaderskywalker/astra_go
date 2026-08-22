# Astra prompt architecture

Astra uses layered, code-owned prompts. The goal is not to make one enormous
prompt; it is to give each model call the smallest complete contract for its
job while sharing one consistent operating policy.

## Layers

1. `EngineeringPolicy` — identity, authority, evidence, workspace, memory,
   artifacts, recovery, and completion rules shared by every call.
2. Planning prompt — interprets the request, preserves conversation continuity,
   selects the interaction mode and applicable skills, defines success criteria,
   and builds an inspectable mind map.
3. Execution prompt — acts as a state machine. It selects one registered action
   at a time, records expected evidence, handles failure, and stops at a clear
   condition.
4. Response prompt — converts execution evidence into a truthful user handoff.
5. `skills.go` — reusable competencies. Skills guide judgment; they do not grant
   tools or permissions.

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
