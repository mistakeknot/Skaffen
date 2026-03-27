# Skaffen Design Philosophy

## Core Bet: Go for Agent Runtimes

Go compiles in 1-5 seconds (vs 3+ minutes for Rust), produces single binaries, and is already Sylveste's systems language (14+ modules). The tradeoff: no algebraic types, less compile-time safety. We compensate with interface-heavy design and comprehensive tests (355+).

## Design Principles

### Sovereignty Over Convenience
Skaffen owns its inference pipeline end-to-end. No cloud-hosted agent platforms, no vendor lock-in. Direct Anthropic API calls or Claude Code proxy — both controlled locally.

### Phase-Gated Safety
OODARC phases aren't just workflow labels — they're hard security boundaries. A brainstorm phase literally cannot write files. The tool registry enforces this at the type level, not by convention.

### Graceful Degradation Everywhere
Every optional dependency (intercore, MCP plugins, Claude Code binary) degrades silently. Skaffen runs on a fresh machine with just an API key.

### Two-Layer Architecture
The universal `agentloop` (Decide→Act) is deliberately separated from the OODARC workflow in `agent`. This lets the core loop be reused without importing the phase system — important for future multi-agent scenarios where not every agent needs OODARC.

### Interface Injection, Not Configuration
Core components (Provider, Router, Session, Emitter, Tool) are interfaces injected at construction. No global state, no service locators. Every component is independently testable with mocks.

## Tradeoffs We Accept

- **No official Anthropic Go SDK.** We implement the Messages API directly (~300 lines). More control, less drift risk from SDK changes.
- **Subprocess for intercore.** The `ic` CLI bridge adds ~5ms per event vs native Go integration. Acceptable until intercore ships `pkg/client`.
- **MCP tools are slower than built-ins.** JSON-RPC over stdio adds latency. The tradeoff is plugin compatibility with the entire Interverse ecosystem.
- **TUI depends on Masaq.** Shared component library means coordinated releases. The benefit is consistency across all Sylveste TUI tools.

## Non-Goals

- **Multi-agent orchestration.** Skaffen is single-agent. Autarch handles fleet coordination.
- **OpenAI/Gemini providers.** Anthropic-first. Other providers in v0.2 only if needed.
- **Extension sandbox.** MCP plugins via Interverse replace the need for in-process scripting.
