---
artifact_type: prd
bead: Demarch-6qb
stage: design
---

# PRD: Skaffen -- Sovereign Agent Runtime

> **Reading context.** This is the canonical PRD for the Skaffen project. It lives in the [Skaffen repo](https://github.com/mistakeknot/Skaffen) and is symlinked from the Demarch monorepo at `docs/prds/`. See also: [vision](../skaffen-vision.md), [brainstorm](../brainstorms/2026-03-10-skaffen-initial-goals.md).

## Problem

Demarch's discipline infrastructure (phase gates, review agents, evidence pipelines, model routing) is bolted onto opaque host runtimes (Claude Code, Codex, Gemini CLI). These runtimes control the agent loop and provide no API for changing those decisions. Clavain hooks *around* host decisions but cannot change *how* the loop works.

This blocks four things today:

1. **Mid-session model routing.** The cheapest-model-that-clears-the-bar philosophy requires switching models per phase. No host agent supports this.
2. **Structural phase gates.** Tool availability should be enforced by the type system, not by system prompt suggestions.
3. **Native evidence emission.** Scraping events from hooks is fragile; the loop should emit evidence as part of the turn cycle.
4. **Programmatic steering.** Interrupt mid-tool, queue follow-ups, control compaction policy. These require owning the loop.

## Solution

Fork [pi_agent_rust](https://github.com/Dicklesworthstone/pi_agent_rust) into a standalone Rust binary (`skaffen`) that implements OODARC natively in the agent loop, with hard phase gates, native Interspect evidence emission, and Intercore integration.

## Features

### F1: Fork, Rebrand, Stabilize (v0.1)

**Bead:** Demarch-rp5

Fork pi_agent_rust, rebrand to Skaffen, get CI green. The bar is twofold: all tests pass AND `skaffen` completes a real read-edit-test workflow interactively.

**Acceptance criteria:**
- [ ] `skaffen` binary builds from `cargo build --release`
- [ ] All pi_agent_rust tests pass under Skaffen branding (fix any that fail from rebranding, skip none)
- [ ] CI pipeline (GitHub Actions) green on every push
- [ ] Pi-specific branding (binary name, config dirs, help text) replaced with Skaffen equivalents
- [ ] `skaffen` runs interactive mode with all 7 tools (read, write, edit, bash, grep, find, ls)
- [ ] `skaffen` completes a simple workflow: read a file, edit it, run `cargo test`, report results
- [ ] `skaffen --mode rpc` runs headless mode
- [ ] `skaffen -p` runs single-shot mode
- [ ] README, CLAUDE.md, AGENTS.md reflect Skaffen identity

### F2: OODARC Loop + Phase-Aware Tool Gating (v0.2)

**Bead:** Demarch-92j

Modify `src/agent.rs` to implement OODARC turn structure and hard tool gating per phase. The Reflect/Compound interface should be specced; the policy (every turn vs phase boundaries vs hybrid) is a design question that needs experimentation during v0.2, not a locked-in decision.

**Acceptance criteria:**
- [ ] Agent loop implements the OODARC cycle: observe, orient, decide, act, reflect, compound
- [ ] Phase enum: Brainstorm, Plan, Build, Review, Ship
- [ ] `tools_for_phase()` returns only allowed tools per phase:
  - Brainstorm: read, grep, find, ls (read-only)
  - Plan: read, grep, find, ls, write (can write plan docs)
  - Build: all tools (full access)
  - Review: read, grep, find, ls, bash (read + test, no write/edit)
  - Ship: read, bash, grep (commit/push only)
- [ ] Tool calls outside phase gate return structured error, not silent failure
- [ ] Phase transitions are explicit (command-driven or steering-driven)
- [ ] Reflect and Compound have a stable interface (trait/API) that supports multiple policies
- [ ] At least one Reflect/Compound policy implemented and tested
- [ ] Mid-session model switching via `select_model(phase, routing, budget)`

### F3: Intercore Bridge + Interspect Evidence (v0.3)

**Bead:** Demarch-j2f

Connect Skaffen to L1 kernel (Intercore) and evidence pipeline (Interspect). Skaffen talks to Intercore via `ic` CLI (thin bridge); native SQLite comes later when the schema stabilizes.

**Acceptance criteria:**
- [ ] Skaffen calls `ic` CLI for dispatch, events, state
- [ ] Every Skaffen session registers as an Intercore run
- [ ] Agent loop emits structured events: tool calls, model selections, phase transitions, steering decisions
- [ ] Events flow into Interspect evidence pipeline
- [ ] Interspect routing overrides are read and applied to model selection
- [ ] Skaffen sessions correlate with beads for cost attribution
- [ ] Graceful degradation when Intercore/Interspect unavailable (standalone mode still works)

### F4: Self-Building Loop (v0.4)

**Bead:** Demarch-22q

Skaffen develops Skaffen features. The bootstrap: Clavain-rigged Claude Code builds v0.1-v0.3, then Skaffen builds v0.4+.

Self-building means two things, in order: first, Skaffen is viable for routine Demarch development tasks (bug fixes, small features) without falling back to Claude Code. Then, Skaffen ships at least one real feature end-to-end with no human code edits.

**Acceptance criteria:**
- [ ] Skaffen can read its own source, edit files, run tests, commit changes
- [ ] Phase gates applied to Skaffen's own development (brainstorm phase is read-only even for its own code)
- [ ] Skaffen is usable for routine Demarch development (bug fixes, small features) without Claude Code fallback
- [ ] At least one Skaffen feature shipped entirely by Skaffen (no human code edits)
- [ ] Evidence from self-building sessions calibrates its own routing
- [ ] Measurable comparison: Skaffen-built vs Clavain-built change quality/cost/time

## Non-Goals

- **Replacing Clavain.** Clavain's 53-plugin ecosystem and host-agent integrations are production-proven and permanent.
- **Rebuilding the LLM abstraction.** Fork pi_agent_rust's provider layer wholesale.
- **General-purpose agent framework.** Skaffen is opinionated for software development with Demarch's OODARC philosophy.
- **Full pi-mono compatibility.** Best-effort extension compatibility; don't gate on it.

## Technical Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Fork base | pi_agent_rust | 4-12x faster than TS, 12x less memory, `#![forbid(unsafe_code)]`, single binary |
| TUI | charmed_rust now, FrankenTUI later | The agent loop is the value; the TUI can evolve |
| Intercore integration | CLI bridge first, native SQLite later | Correctness before optimization |
| Phase gates | Hard (type-system enforced) | Structural beats prompt-based |
| Evidence Reflect/Compound | Stable interface, experimental policy | Spec the trait, try multiple policies during v0.2 |
| Discipline content | Shared docs, not Clavain plugin format | Portable between runtimes |

## Dependencies

- pi_agent_rust fork (external, MIT license)
- Intercore `ic` CLI (internal, for bridge integration)
- Interspect event schema (internal, for evidence emission)
- Beads `bd` CLI (internal, for work tracking correlation)

## Risks

| Risk | Mitigation |
|------|-----------|
| **The fork is harder than expected.** Pi_agent_rust may have undocumented assumptions, tight coupling, or Rust-specific complexity that makes modification harder than reading the code suggests. | Budget extra time for v0.1. Treat the fork as a learning exercise, not just a rename. If the codebase resists modification after a serious attempt, reassess the fork-vs-rewrite decision. |
| **Pi_agent_rust diverges significantly.** The upstream repo moves fast; our fork may fall behind. | Fork early, cherry-pick selectively, maintain compatibility where cheap. Accept divergence where Skaffen's needs differ. |
| **OODARC loop modification destabilizes tests.** Changing the core agent loop touches everything. | Gate behind feature flag until stable. Run full test suite on every change. |
| **Intercore schema not stable enough for native bridge.** | Start with CLI bridge (loose coupling). Evolve to native SQLite when the schema stabilizes. |
| **Self-building bootstrap is circular.** Skaffen can't build itself until it works, but it doesn't fully work until v0.3. | Explicit bootstrap: Claude Code builds v0.1-v0.3, Skaffen takes over at v0.4. |
