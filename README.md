# Astra

Astra is a Go-based engineering agent with a React frontend. It can operate with a local Ollama model or an OpenAI model.

## Requirements

- Go 1.25+
- PostgreSQL and MinIO for the API/server workflow
- Ollama for local inference, or an `OPENAI_API_KEY` for OpenAI inference

## CLI model selection

Check the installed CLI version:

```sh
astra --version
# Astra CLI v0.2.0
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

While connected, the CLI remains interactive while Astra works. Requests are
queued in order and streamed as they complete. Use these local controls:

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
:pause                       Pause at the next safe agent checkpoint
:resume                      Resume a paused agent
:stop                        Cancel the active request safely
:attach /path/to/file         Safely attach an outside file
:paste                        Start a multiline paste; finish with :endpaste
```

Explicitly attached files are copied into `.astra/attachments/`. Very large
single-line pastes are automatically saved there as text attachments and passed
to Astra by reference, keeping the model context usable and traceable.

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
go run astra/cmd/astra.go models
go run astra/cmd/astra.go connect --provider ollama --model qwen3:14b
```

The API server starts with:

```sh
go run astra/main.go
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
connect` becomes the workspace Astra can inspect and modify:

```sh
cd /path/to/any/project
astra connect --provider ollama --model qwen3:14b
```

Use `astra connect --provider openai --model gpt-5.6-luna` when you want Luna.
The CLI still requires the configured PostgreSQL workflow dependencies; Ollama
or an OpenAI API key provides the selected model.

## How Astra skills, prompts, and tools work

Astra does not currently have a separate plugin-style skill registry. Its
capabilities are assembled in layers:

| Layer | What it does | How long it lasts |
| --- | --- | --- |
| Prompt policy | Sets behavior such as evidence-first edits and truthful reporting | One model call; defined in `astra/agents/prompts/prompts.go` |
| Planner prompt | Chooses a sequence of actions for the user request | Planning call only |
| Execution prompt | Chooses the next single tool call using the plan and results | One execution decision |
| Action catalog | Describes the typed tools available to the model | Included in planning/execution calls |
| Action handler | Real Go code that reads, writes, searches, tests, or saves memory | Executes only when selected |
| Mind Palace | Persists verified knowledge in local linked files | Durable until changed or removed |

So a prompt does not permanently reprogram Astra. A planner prompt changes the
planner's decision, an execution prompt changes the next action decision, and a
tool changes the workspace. Only an explicit `save_memory` action creates a
durable learning. The current action catalog is registered in
`astra/agents/actions`; the implementation is in Go, not YAML.

The next architectural milestone should be a first-class capability registry:
each skill would declare its purpose, allowed tools, stage-specific prompt
instructions, input/output artifact contracts, and validators. The first three
should be `repository_navigation`, `file_artifacts`, and `mind_palace`. This
will make Astra's abilities inspectable and composable without turning every
request into one enormous system prompt.

## File-backed memory

Learning is stored outside PostgreSQL. Astra writes a per-user mind palace under
`$ASTRA_MIND_PALACE_DIR` (default: `.astra/mind-palace`) and mirrors it to the configured MinIO bucket.

```text
users/{user_id}/
  sessions/{session_id}/events.jsonl     # immutable session evidence
  memory/{kind}/{memory_id}.md           # curated, linked memory blocks
  memory/index.json                       # retrieval index
```

The agent uses `save_memory`, `search_memory`, `list_memory`, and `link_memory`.
Use Markdown for learnings and decisions, JSONL for append-only events, JSON for indexes, and CSV only for tabular artifacts.

Agent prompts, behavioral instructions, and output schemas are code-owned in
`astra/agents/prompts/prompts.go`; Astra does not load agent YAML instruction files.
For user-facing deliverables it has a validated `write_artifact` action. Artifacts
are written below `.astra/artifacts/{session}/` in Markdown, JSON, JSONL, CSV, or text.

## Controlled self-improvement

Improvement proposals are local Markdown files under `.astra/improvements`. The scout can observe and propose; it cannot modify code. Luna can review a proposal; only you can approve or reject it.

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
