# Skaffen

Sovereign AI agent runtime implementing the OODARC (Observe-Orient-Decide-Act-Reflect-Compound) workflow. Go application, part of the Sylveste monorepo (`os/Skaffen/`).

## Quick Reference

- **Build:** `go build ./cmd/skaffen`
- **Test:** `go test ./... -count=1`
- **Vet:** `go vet ./...`
- **Run TUI:** `go run ./cmd/skaffen`
- **Run headless:** `echo "prompt" | go run ./cmd/skaffen --mode print`
- **Binary:** `cmd/skaffen/main.go` — single entry point for both TUI and print modes

## Beads Tracking

Skaffen uses the **Sylveste monorepo beads tracker** at `/home/mk/projects/Sylveste/.beads/` (prefix `Sylveste-`). Run `bd` commands from the monorepo root, not from `os/Skaffen/`.

## Git

Skaffen has its own git repo (`os/Skaffen/.git`), separate from the Sylveste monorepo. Commit Skaffen changes from `os/Skaffen/`, not from the monorepo root.

## Structure

```
cmd/skaffen/         CLI entry point (TUI + print modes)
internal/
  agent/             OODARC workflow engine (phase FSM, gated registry)
  agentloop/         Universal Decide→Act loop (phase-agnostic)
  evidence/          Structured event emission (JSONL + intercore bridge)
  git/               Git operations for auto-commit
  intercore/         Intercore CLI bridge (ic binary detection)
  mcp/               MCP stdio client (Interverse plugin tools)
  provider/          LLM provider abstraction
  provider/anthropic Anthropic Messages API (SSE streaming)
  provider/claudecode Claude Code RPC proxy (subprocess)
  router/            Per-turn model selection (phase defaults, budget, complexity)
  session/           JSONL session persistence + context management
  tool/              Tool registry with phase gating + 7 built-in tools
  trust/             Smart trust evaluator for tool approval
  tui/               Bubble Tea TUI (chat viewport, composer, status bar)
```

## Key Patterns

- **Adapter pattern:** `agent/` bridges phase-typed interfaces to phase-agnostic `agentloop/` via adapters (routerAdapter, sessionAdapter, emitterAdapter)
- **Provider registration:** Providers register via `init()` in their packages; imported as blank imports in `main.go`
- **Tool registration:** Built-in tools via `tool.RegisterBuiltins()`, MCP tools via `mcp.Manager.LoadAll()` → `registry.RegisterForPhases()`
- **Graceful degradation:** Missing `ic` binary, failed MCP plugins, and unavailable providers all degrade silently

## Dependencies

- **Masaq** (`../../masaq`): Shared Bubble Tea component library (local replace directive)
- **MCP Go SDK** (`modelcontextprotocol/go-sdk`): MCP stdio client transport
- **Bubble Tea ecosystem**: TUI framework (bubbletea, bubbles, lipgloss)
- **BurntSushi/toml**: Plugin config parsing

## See Also

- `AGENTS.md` — architecture deep-dive, testing patterns, module relationships
- `PHILOSOPHY.md` — design principles and tradeoffs
- Sylveste PRD: `docs/prds/2026-03-11-skaffen-go-rewrite.md` (at monorepo root)
