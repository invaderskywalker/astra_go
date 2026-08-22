# Astra evaluation system

Astra now separates deterministic capability checks from provider-backed model
evaluations.

## Run the local gate

The local evaluator never calls an LLM and never writes into the current
project unless an explicit `--root` is supplied. By default it creates a
temporary evaluation workspace:

```sh
astra eval list
astra eval local
```

The gate checks:

- full usage contracts for every registered action;
- Markdown, JSON, JSONL, CSV, and text artifact writing;
- rejection of malformed structured artifacts;
- nested workspace directory creation and listing;
- command evidence with working directory and result capture;
- file-backed memory save, search, and reciprocal linking;
- prompt, skill, and agent-bookmark catalogs.

## Scenario catalog

`astra/evals/evals.go` contains the reusable scenario contract. Scenarios are
organized by behavior rather than by model:

- repository evidence and negative-claim discipline;
- technical requirements, architecture, and code-structure documents;
- structured JSON state, JSONL event logs, CSV exports, and text artifacts;
- linked mind-palace memory;
- ordered command validation;
- focused clarification boundaries.

Each scenario states its intended outcome, required action families, artifact
formats, required headings, and whether mutation or memory is expected. The
prompt is deliberately outcome-oriented so a model may choose an equivalent
safe action sequence.

## Provider-backed evaluations

The local gate proves handlers and contracts. Luna and Qwen evaluation runs
should use the same scenario IDs and record:

1. the model/provider and prompt version;
2. selected actions and activation events;
3. sanitized action parameters and results;
4. created artifact paths and format/heading checks;
5. memory IDs, links, and retrieval evidence;
6. unsupported claims, unnecessary actions, retries, and failures;
7. latency, token/cost data when available, and final pass/fail reasons.

Provider runs must use an isolated temporary workspace and a disposable memory
root. They should never treat fluent prose as a pass: acceptance comes from
the action trace and filesystem evidence.

The interactive CLI exposes the same state through `:dashboard`, `:workspace`,
`:mindpalace`, `:sessions`, and `:sync`. These views are read-only and are
intended to make evaluation artifacts and synchronization state inspectable by
the user rather than hidden in logs.

## File-writing quality dimensions

For document scenarios, evaluate structure as well as syntax:

- technical requirements: scope, actors, functional/non-functional
  requirements, assumptions, acceptance criteria, and open questions;
- architecture: context, components, boundaries, data flow, decisions,
  trade-offs, failure modes, and verification;
- code structure: observed files/symbols, entry points, dependencies, data
  flow, and a clear distinction between observation and inference;
- JSON/JSONL/CSV: parseability, schema/headers, deterministic fields, and
  consistent records;
- memory: concise reusable content, provenance, confidence, status, and links
  that lead to related blocks without duplicating the archive.

This document is the evaluation contract. When Astra gains a capability, add a
scenario and a deterministic check before relying on provider demos.
