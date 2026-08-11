# Skaffen

Skaffen is the OODARC policy layer for Sylveste, the platform that orchestrates agents by human/machine comparative advantage.

Current development targets a Pi-hosted package: Pi owns the universal provider/tool/session loop, Skaffen supplies OODARC workflow policy, and Intercore remains the durable run/event/gate kernel. The existing sovereign Go runtime remains available as a reference and compatibility path while the Pi adapter earns parity.

## Pi-hosted harness (in development)

The first vertical slice lives in `pi-package/`. It observes Intercore without writing to it and adds `/harness`, `/situation`, `observe_situation`, and compact run/OODARC status to Pi.

```bash
cd pi-package
npm install
npm run check

# Temporary load
pi --no-extensions -e .

# Persistent local-path install
pi install /absolute/path/to/Skaffen/pi-package
```

A missing `ic` binary, timeout, malformed snapshot, or schema mismatch is displayed as degraded state; startup never runs `ic init` or another migration/write command.

## Existing Go runtime

The Go implementation owns its agent loop directly: provider selection, tool access, phase state, evidence emission, and terminal UI all run inside a local binary. It runs an OODARC workflow — Observe, Orient, Decide, Act, Reflect, Compound — with phase-gated tools.

The implementation is split in two layers. `internal/agentloop` is the universal Decide→Act loop that knows about providers, tools, sessions, and emitters. `internal/agent` wraps it with OODARC phase semantics, gated registries, and adapters. This separation now also defines the migration boundary: Pi replaces the universal loop while Skaffen's policy moves into the package.

## Go runtime quick start

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
| `pi-package` | Pi-native OODARC policy adapter and Intercore observation surface |
| `cmd/skaffen` | Go CLI entry point for TUI and print modes |
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
