# Skaffen

Skaffen is a sovereign Go agent runtime for Sylveste, the platform that orchestrates agents by human/machine comparative advantage.

It owns the agent loop directly: provider selection, tool access, phase state, evidence emission, and terminal UI all run inside a local Go binary. Clavain is the reference Claude Code rig; Skaffen is the runtime path for agents that need their own execution loop instead of living entirely inside a host IDE.

## What this does

Skaffen runs an OODARC workflow — Observe, Orient, Decide, Act, Reflect, Compound — with phase-gated tools. A brainstorm phase cannot write files; a build phase can, subject to trust evaluation. That makes workflow discipline a runtime boundary rather than a prompt convention.

The implementation is split in two layers. `internal/agentloop` is the universal Decide→Act loop that knows about providers, tools, sessions, and emitters. `internal/agent` wraps it with OODARC phase semantics, gated registries, and adapters. This keeps the reusable loop independent from the workflow policy.

## Quick start

```bash
# Build
go build ./cmd/skaffen

# Test
go test ./... -count=1

# Run TUI mode
go run ./cmd/skaffen

# Run print mode
echo "Explain the repo" | go run ./cmd/skaffen --mode print -p "Summarize this project"
```

## Architecture

| Package | Role |
|---|---|
| `cmd/skaffen` | CLI entry point for TUI and print modes |
| `internal/agentloop` | Phase-agnostic Decide→Act core |
| `internal/agent` | OODARC workflow engine and phase FSM |
| `internal/provider` | LLM provider abstraction |
| `internal/router` | Model selection by phase, budget, and complexity |
| `internal/tool` | Built-in tools and phase availability |
| `internal/mcp` | Interverse-compatible MCP tool loading |
| `internal/evidence` | JSONL evidence emission and Intercore bridge |
| `internal/tui` | Bubble Tea terminal interface |

## Role in Sylveste

- **Intercore** records runs, dispatches, gates, and events.
- **Clavain** codifies the Claude Code workflow and multi-agent review discipline.
- **Skaffen** provides a host-independent agent runtime for work that should run as its own process.
- **Interverse** supplies companion tools through MCP and plugin conventions.

## Design principles

- **Sovereignty over convenience:** own the inference pipeline locally.
- **Phase-gated safety:** make the workflow boundary enforceable in code.
- **Graceful degradation:** missing optional dependencies should reduce capability, not crash the runtime.
- **Interface injection:** providers, routers, sessions, emitters, and tools are replaceable interfaces.

See [PHILOSOPHY.md](PHILOSOPHY.md) for the full design doctrine and [AGENTS.md](AGENTS.md) for development guidance.

## License

MIT
