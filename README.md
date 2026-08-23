# Astra

Astra is a Go-based local-first engineering agent with a professional terminal
cockpit. It can operate with a local Ollama model or an OpenAI model.

## Requirements

- Go 1.25+
- A private local filesystem for the profile, Mind Palace, sessions, and artifacts
- Ollama for local inference, or an `OPENAI_API_KEY` for OpenAI inference

## CLI model selection

Check the installed CLI version:

```sh
astra --version
# Astra CLI v0.6.0
```

Show locally installed Ollama models plus the supported OpenAI options:

```sh
astra models
```

Run Astra with the installed local model:

```sh
astra connect --provider ollama --model qwen3:14b
```

Run Astra with OpenAI's efficient GPT-5.6 model:

```sh
export OPENAI_API_KEY="..."
astra connect --provider openai --model gpt-5.6-luna
```

Other supported cloud choices are `gpt-5.6-terra` (balanced) and `gpt-5.6-sol` (highest capability).

## Private local identity

Astra is single-user and file-backed. Before using `connect`, `models`,
`eval`, or improvement commands, create or unlock the private local profile:

```sh
astra signup                         # interactive; stored under ~/.astra/identity
astra login                          # unlock the existing profile
astra whoami
astra logout
```

The profile and login marker are written with owner-only permissions. The
connected directory never becomes an account, and no user row is created in a
database. The Mind Palace is also local and private; external synchronization
is disabled unless a future explicit export command is used.

On a real terminal, `connect` opens Astra's full-screen cockpit. It keeps the
conversation, plan/tool transcript, input composer, and project views in
separate regions so long prompts and streaming output cannot overwrite each
other. The left rail contains Chat, Dashboard, Workspace, Mind Palace,
Sessions, and Sync. Use `Ctrl+1` through `Ctrl+6` for direct navigation, or
`Tab`/`Shift+Tab` to move between views.

The Chat composer grows for multiline prompts. Press `Enter` to send and
`Ctrl-J` to insert a newline. `Ctrl+P` pauses at a safe checkpoint, `Ctrl+R`
resumes, `Ctrl+X` requests cancellation, and `Ctrl+Q` exits. Local commands
such as `:model openai gpt-5.6-luna`, `:dashboard`, and `:attach <path>` work
inside the cockpit as well.

For pipes, CI, transcript capture, or the original stream-oriented experience,
use `--plain`:

```sh
astra connect --plain --provider ollama --model qwen3:14b
```

The plain CLI remains interactive while Astra works. Requests are queued in
order and streamed as they complete. Its local controls are:

In a real terminal, the input bar supports editing. Press `Enter` to submit,
`Backspace` to delete a character, `←`/`→` to move the cursor, `↑`/`↓` to
navigate commands from this session, `Home`/`End` to jump within a draft,
`Ctrl-J` to add a new line, `Ctrl-W` to delete the previous word, `Ctrl-U` to
clear the draft, and `Ctrl-C` to cancel the draft.

```text
:help                         Show controls
:pwd                          Show the connected workspace
:ls [path]                    List files and sizes
:tree [path]                  Recursively navigate files
:model                        Show the active provider/model
:model ollama qwen3:14b       Switch models when idle
:model openai gpt-5.6-luna    Switch to Luna for future requests
:chat                         Return to the normal conversation view
:dashboard                    Show workspace, session, memory, artifact, and sync stats
:workspace                    Browse the connected project workspace
:mindpalace                   Browse the durable user Mind Palace
:sessions                     Browse session evidence files
:sync                         Show managed-file synchronization configuration/status
:agents                       Show worker-agent branches and their status
:scopes                       Show approved filesystem scopes
:prompts                      Show global instruction/personality profiles
:pause                       Pause at the next safe agent checkpoint
:resume                      Resume a paused agent
:stop                        Cancel the active request safely
:clear                       Cancel active work and discard all queued requests
:attach /path/to/file         Safely attach an outside file
:paste                        Start a multiline paste; finish with :endpaste
```

Terminals that support bracketed paste can accept a multiline prompt directly:
paste it into the input bar, then press Enter once. Astra keeps the entire
paste as one request. `:paste` remains available as a manual fallback.

During work, Astra displays each selected action, sanitized parameters, and a
concise result. Those action audits are also recorded in the session evidence
JSONL file. Commands can target nested project directories with
`working_directory`; use `run_commands` when several related commands should
run in order.

Each request now uses one adaptive living task state rather than a separate
rough-plan phase. After every action Astra updates the goal, evidence,
completed work, remaining work, blockers, verification status, and next action;
the next tool choice is based on that current state.

Execution is progress-aware: there is no small normal action cap. Astra keeps
working while meaningful state or evidence changes, and checkpoints instead of
looping after six unchanged turns or three identical failures. A last-resort
emergency ceiling defaults to 256 actions; advanced users can set
`ASTRA_MAX_ACTION_STEPS` (1–2048) when a workflow genuinely needs a different
ceiling.

For unfamiliar or potentially large files, Astra uses a progressive evidence
loop. `analyze_files` streams file size, line count, language, hash, headings,
symbols, imports, query matches, and recommended line ranges without returning
the source body. It then uses `search_code` and bounded `read_files` ranges,
iterating only when new evidence requires another part of the file. This keeps
large repositories useful without filling the model context window.

Explicitly attached files and very large single-line pastes are copied into the
private Astra data directory for the current project/session, outside the
repository, and passed to Astra by reference. This keeps source trees clean
while preserving traceability.

## Approved filesystem scopes

The connected project is automatically approved. Additional directories are
never silently assumed; explicitly grant a scope from the shell:

```sh
astra scope add /Users/me/another-project all
astra scope add /Users/me/read-only-project read,execute
astra scope list
astra scope revoke scope_<id>
```

The agent can run commands in an approved scope by providing its exact
`working_directory`. Every command re-checks the scope and permission at
execution time. Scope approval is Astra's authority record; it does not elevate
the operating-system user's permissions.

## Worker agents and profiles

Astra has a bounded supervisor/worker runtime. The main agent can spawn focused
workers for research, implementation, testing, or documentation, wait for their
results, and reconcile their evidence. Each branch has its own session ID,
goal, model, personality, workspace scope, event trail, and lifecycle status.
The cockpit exposes these branches through the Agents view and `:agents`.

User-authored instruction and personality profiles are global Markdown files
under `~/.astra/prompts/` (or `$ASTRA_DATA_DIR/prompts/`). The agent can create
them with the `write_prompt_profile` action, and they can be inspected with:

```sh
astra prompt list
astra prompt enable prompt_<name>
astra prompt disable prompt_<name>
```

Enabled profiles are preferences loaded into planning and execution context;
they cannot override Astra's compiled policy, evidence rules, permissions, or
tool contracts.

Set persistent CLI defaults with:

```sh
export ASTRA_LLM_PROVIDER=ollama
export ASTRA_LLM_MODEL=qwen3:14b
# Optional when Ollama is not local:
export OLLAMA_BASE_URL=http://host:11434/api
```

## Development

```sh
go test ./...
go run ./astra/cmd models
go run ./astra/cmd connect --provider ollama --model qwen3:14b
```

## Build once, use the CLI anywhere

From the repository root, build the CLI into a directory on your `PATH`:

```sh
mkdir -p "$HOME/.local/bin"
go build -o "$HOME/.local/bin/astra" ./astra/cmd
export PATH="$HOME/.local/bin:$PATH"
```

For a system-wide installation on macOS/Linux, this is also valid:

```sh
go build -o ./astra_cli ./astra/cmd
sudo install -m 0755 ./astra_cli /usr/local/bin/astra
rm ./astra_cli
```

Use `install` instead of `mv` so the final binary has predictable executable
permissions. You may need `/usr/local/bin` on your `PATH`; on Apple Silicon,
`/opt/homebrew/bin` is another common choice.

To keep the command available in future terminal sessions, add the `export`
line to `~/.zshrc` (macOS) or `~/.bashrc` (Linux). Verify the installation:

```sh
which astra
astra models
```

Then connect from any project directory. The directory where you run `astra
connect` becomes the default local workspace Astra can inspect and modify. Astra
can also answer general questions, research the web, create file artifacts,
retrieve linked memory, and run explicit commands inside that workspace:

```sh
cd /path/to/any/project
astra connect --provider ollama --model qwen3:14b
```

Use `astra connect --provider openai --model gpt-5.6-luna` when you want Luna.
The CLI does not require PostgreSQL or MinIO. Ollama or an OpenAI API key
provides the selected model.

## How Astra skills, prompts, and tools work

See the living [capability map](docs/capabilities.md) for the current ability
matrix, authority rules, evidence contracts, and roadmap. The terminal layout,
keyboard contract, and plain-mode boundary are documented in the [CLI cockpit
guide](docs/cli-cockpit.md).

Astra does not currently have a separate plugin-style skill registry. Its
capabilities are assembled in layers:

| Layer | What it does | How long it lasts |
| --- | --- | --- |
| Prompt policy | Sets behavior such as evidence-first edits and truthful reporting | One model call; defined in `astra/agents/prompts/prompts.go` |
| Execution prompt | Maintains task state and chooses the next single tool call using intent, evidence, and results | One execution decision, repeated while progress is meaningful |
| Action bookmarks | Compact typed-tool choices shown to the executor; full contracts are loaded lazily | Included in execution calls |
| Action activation | Loads complete schema, examples, return shape, side effects, and recovery rules for up to five selected tools | One activation step, then retained for the current request |
| Agent bookmarks | Groups related tools into routing roles such as repository operator or verifier | Execution context only; does not grant hidden permissions |
| Action handler | Real Go code that reads, writes, searches, tests, or saves memory | Executes only when selected |
| Mind Palace | Persists verified knowledge in local linked files | Durable until changed or removed |

So a prompt does not permanently reprogram Astra. The execution prompt changes
the next action decision, and a tool changes the workspace. Only an explicit `save_memory` action creates a
durable learning. The current action catalog is registered in
`astra/agents/actions`; the implementation is in Go, not YAML.

The action registry now has two views: compact bookmarks for discovery and
full activation documentation for the tools selected next. This keeps prompts
reviewable and powerful without injecting every parameter schema into every
model call. When current workspace context already answers a scope question,
Astra answers directly; otherwise it performs a focused inspection. Agent
bookmarks provide role-oriented routing, while the runtime action registry and
authority policy remain the source of truth for permission.

## File-backed memory

Learning is stored outside any database. Astra writes a per-user mind palace under
`$ASTRA_MIND_PALACE_DIR` (default: `~/.astra/mind-palace`) so it remains available
across project directories and sessions. It is local and owner-private; normal
agent work never contacts a remote mirror.

```text
users/{user_id}/
  sessions/{session_id}/events.jsonl     # immutable session evidence
  memory/{kind}/{memory_id}.md           # curated, linked memory blocks
  memory/index.json                       # retrieval index
```

The connected project remains the local source-of-truth workspace. Astra-owned
session state is stored outside it under
`$ASTRA_DATA_DIR/projects/{project_id}/` (default: `~/.astra/projects/`). This
keeps repositories free of Astra metadata while the CLI still shows the
project, session, artifacts, and attachments through its views. Set
`ASTRA_DATA_DIR` to relocate all Astra-owned state. Arbitrary source files are
never uploaded implicitly.

The agent uses `save_memory`, `search_memory`, `list_memory`, and `link_memory`.
Use Markdown for learnings and decisions, JSONL for append-only events, JSON for indexes, and CSV only for tabular artifacts.

Agent prompts, behavioral instructions, and output schemas are code-owned in
`astra/agents/prompts/prompts.go`; Astra does not load agent YAML instruction files.
For user-facing deliverables it has a validated `write_artifact` action. Artifacts
are written below the external project data directory in Markdown, JSON, JSONL,
CSV, or text.

## Controlled self-improvement

Improvement proposals are private Markdown files under the external project data
directory. The scout can observe and propose; it cannot modify code. Luna can
review a proposal; only you can approve or reject it.

```sh
# Qwen observes the workspace and creates one review-ready proposal.
astra improve scan --provider ollama --model qwen3:14b

# Read proposals, then ask Luna for an evidence/risk review.
astra improve list
astra improve review imp_... --provider openai --model gpt-5.6-luna

# Human decision. Approval does not execute a change.
astra improve approve imp_... --reason "Reviewed tests and scope"
astra improve reject imp_... --reason "Not valuable now"
```

Every proposal includes evidence, a bounded action list, validation steps, risk, and a mandatory human-approval flag. The controlled branch/PR executor is intentionally a later phase.

## Automated capability evaluations

Astra includes a deterministic local evaluation gate and a reusable scenario
catalog for provider-backed Luna/Qwen comparisons:

```sh
astra eval list
astra eval local
```

The evaluator uses a temporary workspace by default and checks action contracts,
all supported artifact formats, malformed-artifact rejection, nested workspace
operations, command evidence, linked file-backed memory, and prompt catalogs.
See [docs/evaluations.md](docs/evaluations.md) for the scenario contract and
the quality dimensions for technical, architecture, code-structure, data, and
memory artifacts. Model runs should score action traces and filesystem evidence
instead of judging fluent responses alone.
