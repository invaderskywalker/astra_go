# Astra

Astra is a Go-based engineering agent with a React frontend. It can operate with a local Ollama model or an OpenAI model.

## Requirements

- Go 1.25+
- PostgreSQL and MinIO for the API/server workflow
- Ollama for local inference, or an `OPENAI_API_KEY` for OpenAI inference

## CLI model selection

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
