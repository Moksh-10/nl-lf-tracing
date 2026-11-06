# nl-lf tracing (Go)

Compare **Langfuse Cloud** and **Neatlogs** on the same local **Ollama** agent run.

Hexagonal layout: add or swap trace providers under `internal/adapters/tracing/` without touching the agent loop.

## Prerequisites

- Go **1.25+**
- Ollama running (`ollama serve`), model pulled e.g. `ollama pull qwen3:1.7b`
- Langfuse Cloud keys (optional for compare)
- Neatlogs project API key (optional for compare)
- Tavily key for `WebSearch` / `WebFetch` (optional)

## Setup

```bash
cd "nl-lf tracing"
cp .env.example .env
# set NEATLOGS_API_KEY, LANGFUSE_* , optional TAVILY_API_KEY
go run ./cmd/nl-lf
```

One-shot:

```bash
go run ./cmd/nl-lf "Summarize this repo in one paragraph"
```

## Neatlogs

- SDK: `github.com/neatlogs/neatlogs-go`
- Init once in `internal/bootstrap/bootstrap.go` with workflow name `local-ollama-agent`
- Entry-point workflow roots: `nl-lf-agent-oneshot`, `nl-lf-agent-repl`, `nl-lf-subagent`
- US ingest: `NEATLOGS_ENDPOINT=https://ingest.neatlogs.com` (default)
- Session grouping: `Identify(session_id)` on each run

Verify SDK (from project root after `go mod tidy`):

```bash
go list -m -f '{{.Version}}' github.com/neatlogs/neatlogs-go
go install github.com/neatlogs/neatlogs-go/cmd/neatlogs@$(go list -m -f '{{.Version}}' github.com/neatlogs/neatlogs-go)
neatlogs doctor --local --json
export NEATLOGS_API_KEY=...   # your key, not committed
neatlogs doctor --probe --json
```

## Langfuse Cloud

OTLP HTTP to `{LANGFUSE_BASE_URL}/api/public/otel/v1/traces` with Basic auth — see `internal/adapters/tracing/langfuse.go`.

## Tools

`Read`, `Write`, `Edit`, `Glob`, `Grep`, `Bash`, `WebSearch`, `WebFetch`, `Task`, `TodoWrite` — scoped to `AGENT_WORKSPACE`.

## Architecture

```
cmd/nl-lf/           CLI entry (one-shot + REPL)
internal/
  ports/             LLM + Tracer interfaces
  application/       agent loop
  adapters/
    llm/             Ollama (OpenAI-compatible)
    tracing/         neatlogs, langfuse, composite
    tools/           filesystem, shell, tavily HTTP
  bootstrap/         wiring
```

Dual tracing uses nested `Composite` so the agent body runs **once** while both backends record spans.

Sources: [Neatlogs Go SDK](https://docs.neatlogs.com/sdk/go), [Langfuse OTEL](https://langfuse.com/integrations/native/opentelemetry).
