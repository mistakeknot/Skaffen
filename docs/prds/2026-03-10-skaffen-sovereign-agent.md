---
artifact_type: prd
bead: Demarch-6qb
stage: design
---

# PRD: Skaffen — Sovereign Agent Runtime

## Problem

Demarch's discipline infrastructure (phase gates, review agents, evidence pipelines, model routing) is bolted onto opaque host runtimes (Claude Code, Codex, Gemini CLI). These runtimes control the agent loop — when to compact, which model to use, which tools to expose — and provide no API for changing these decisions. Clavain hooks *around* host decisions but cannot change *how* the loop works.

This ceiling blocks:
- **Mid-session model routing** (cheapest-model-that-clears-the-bar requires switching models per phase)
- **Structural phase gates** (tool availability should be enforced by the type system, not system prompts)
- **Native evidence emission** (scraping events from hooks is fragile; the loop should emit evidence as a first-class primitive)
- **Programmatic steering** (interrupt mid-tool, queue follow-ups, control compaction policy)

## Solution

Fork [pi_agent_rust](https://github.com/Dicklesworthstone/pi_agent_rust) into a standalone Rust binary (`skaffen`) that implements OODARC natively in the agent loop, with hard phase gates, native Interspect evidence emission, and Intercore integration.

## Features

### F1: Fork, Rebrand, Stabilize (v0.1)
**Bead:** Demarch-rp5
**What:** Fork pi_agent_rust, rebrand to Skaffen, get CI green, verify all 3,857+ tests pass.
**Acceptance criteria:**
- [ ] `skaffen` binary builds from `cargo build --release`
- [ ] All pi_agent_rust tests pass under Skaffen branding
- [ ] CI pipeline (GitHub Actions) green on every push
- [ ] Pi-specific branding (binary name, config dirs, help text) replaced
- [ ] `skaffen` runs interactive mode with all 7 tools (read, write, edit, bash, grep, find, ls)
- [ ] `skaffen --mode rpc` runs headless mode
- [ ] `skaffen -p` runs single-shot mode
- [ ] README, CLAUDE.md, AGENTS.md reflect Skaffen identity

### F2: OODARC Loop + Phase-Aware Tool Gating (v0.2)
**Bead:** Demarch-92j
**What:** Modify `src/agent.rs` to implement OODARC turn structure and hard tool gating per phase.
**Acceptance criteria:**
- [ ] Agent loop implements: observe, orient, decide, act, reflect, compound
- [ ] Phase enum: Brainstorm, Plan, Build, Review, Ship
- [ ] `tools_for_phase()` returns only allowed tools per phase:
  - Brainstorm: read, grep, find, ls (read-only)
  - Plan: read, grep, find, ls, write (can write plan docs)
  - Build: all tools (full access)
  - Review: read, grep, find, ls, bash (read + test, no write/edit)
  - Ship: read, bash, grep (commit/push only)
- [ ] Tool calls outside phase gate return structured error (not silent failure)
- [ ] Phase transitions are explicit (command or steering-driven)
- [ ] Lightweight evidence emission every turn (JSON event to interspect)
- [ ] Heavier LLM-based reflection at phase boundaries
- [ ] Mid-session model switching via `select_model(phase, routing, budget)`

### F3: Intercore Bridge + Interspect Evidence (v0.3)
**Bead:** Demarch-j2f
**What:** Connect Skaffen to L1 kernel (Intercore) and evidence pipeline (Interspect) as a first-class agent runtime.
**Acceptance criteria:**
- [ ] Skaffen calls `ic` CLI for dispatch, events, state (thin bridge)
- [ ] Every Skaffen session registers as an Intercore run
- [ ] Agent loop emits structured events: tool calls, model selections, phase transitions, steering decisions
- [ ] Events flow into Interspect evidence pipeline
- [ ] Interspect routing overrides read and applied to model selection
- [ ] Skaffen sessions correlate with beads for cost attribution
- [ ] Graceful degradation when Intercore/Interspect unavailable (standalone mode)

### F4: Self-Building Loop (v0.4)
**Bead:** Demarch-22q
**What:** Skaffen develops Skaffen features. Bootstrap: Clavain-rigged Claude Code builds v0.1-v0.3, then Skaffen builds v0.4+.
**Acceptance criteria:**
- [ ] Skaffen can read its own source, edit files, run tests, commit changes
- [ ] Phase gates applied to Skaffen's own development (brainstorm its own features read-only)
- [ ] Evidence from self-building sessions calibrates its own routing
- [ ] At least one Skaffen feature shipped entirely by Skaffen (no host agent fallback)
- [ ] Measurable comparison: Skaffen-built vs Clavain-built change quality

## Non-Goals

- **Replacing Clavain.** Clavain's 53-plugin ecosystem and host-agent integrations remain production-proven.
- **Rebuilding the LLM abstraction.** Fork pi_agent_rust's provider layer wholesale.
- **General-purpose agent framework.** Skaffen is opinionated for software development with Demarch's OODARC philosophy.
- **Full pi-mono compatibility.** Best-effort extension compatibility, don't gate on it.

## Technical Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Fork base | pi_agent_rust | 4-12x faster than TS, 12x less memory, `#![forbid(unsafe_code)]`, single binary |
| TUI | charmed_rust now, FrankenTUI later | Agent loop is the value; TUI can evolve |
| Intercore integration | CLI bridge first, native SQLite later | Correctness before optimization |
| Phase gates | Hard (type-system enforced) | Structural > prompt-based (PHILOSOPHY.md) |
| Evidence | Hybrid (lightweight every turn, heavy at phase boundaries) | Balance signal density vs overhead |
| Discipline content | Shared docs (not Clavain plugin format) | Portable between runtimes |

## Dependencies

- pi_agent_rust fork (external, MIT license)
- Intercore `ic` CLI (internal, for bridge integration)
- Interspect event schema (internal, for evidence emission)
- Beads `bd` CLI (internal, for work tracking correlation)

## Risks

| Risk | Mitigation |
|------|-----------|
| pi_agent_rust diverges significantly | Fork early, cherry-pick selectively, maintain compatibility where cheap |
| OODARC loop modification destabilizes tests | Gate behind feature flag until stable, run full test suite on every change |
| Intercore schema not stable enough for native bridge | Start with CLI bridge (loose coupling), evolve to native when schema stabilizes |
| Self-building loop bootstrapping is circular | Explicit bootstrap sequence: Claude Code builds v0.1-v0.3, Skaffen takes over at v0.4 |
