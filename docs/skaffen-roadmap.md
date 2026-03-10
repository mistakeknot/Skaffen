---
artifact_type: roadmap
bead: Demarch-6qb
stage: design
---

# Skaffen: Roadmap

> **Reading context.** Milestone-based, not time-boxed. Skaffen is built alongside ongoing Clavain, Interspect, and Intercore work; pacing depends on what else is happening across Demarch. Milestones are ordered and have explicit dependencies. See also: [vision](skaffen-vision.md), [PRD](prds/2026-03-10-skaffen-sovereign-agent.md).

## Milestones

### v0.1 -- Fork and Stabilize

**Bead:** Demarch-rp5

Fork pi_agent_rust, rebrand to Skaffen, get CI green. The bar: all tests pass AND `skaffen` completes a real read-edit-test workflow interactively. This is the foundation everything else builds on; if the fork is harder than expected (undocumented assumptions, tight coupling, Rust-specific complexity), this is where that surfaces.

**What gets done:**
- Fork pi_agent_rust, rename binary/crate/config dirs to `skaffen`
- Fix any tests that break from rebranding (skip none)
- Set up GitHub Actions CI (build + test on every push)
- Verify interactive mode works with all 7 tools
- Verify RPC mode and single-shot mode
- Update README, CLAUDE.md, AGENTS.md for Skaffen identity
- Run a real workflow: read a file, edit it, run `cargo test`, report results

**Dependencies:** None. This is the starting line.

**Blocks:** Everything else.

---

### v0.2 -- OODARC Loop + Phase Gates

**Bead:** Demarch-92j

Modify the agent loop to implement OODARC natively and hard-gate tools by phase. The Reflect/Compound interface gets specced as a trait; the policy (every turn vs phase boundaries vs hybrid) is an experiment, not a commitment. Mid-session model switching goes in here too.

**What gets done:**
- Implement OODARC turn cycle in `src/agent.rs`: observe, orient, decide, act, reflect, compound
- Add Phase enum (Brainstorm, Plan, Build, Review, Ship)
- Implement `tools_for_phase()` with hard gating (type-system enforced)
- Structured error on out-of-phase tool calls
- Explicit phase transitions (command-driven or steering-driven)
- Reflect/Compound trait with at least one concrete policy
- `select_model(phase, routing, budget)` for mid-session model switching
- Test coverage for phase gate enforcement and OODARC cycle

**Dependencies:**
- v0.1 complete (stable fork with passing tests)

**Parallel Demarch work that affects this:**
- Clavain improvements continue independently; discipline content that Skaffen will eventually load is being refined in parallel
- Interspect routing overrides schema should be stabilizing; v0.2 reads overrides but doesn't depend on the full evidence pipeline

---

### v0.3 -- Intercore Bridge + Interspect Evidence

**Bead:** Demarch-j2f

Connect Skaffen to the L1 kernel and evidence pipeline. This is where Skaffen stops being a standalone binary and becomes a Demarch citizen. The bridge is thin (CLI calls to `ic` and structured event emission); native SQLite integration comes later.

**What gets done:**
- `ic` CLI bridge: dispatch events, run state, session registration
- Every Skaffen session registers as an Intercore run
- Structured event emission from the agent loop (tool calls, model selections, phase transitions, steering decisions)
- Events flow into Interspect evidence pipeline
- Interspect routing overrides read and applied to model selection per phase
- Bead correlation: Skaffen sessions link to beads for cost attribution
- Standalone mode: graceful degradation when Intercore/Interspect unavailable

**Dependencies:**
- v0.2 complete (OODARC loop and phase gates working)
- **Intercore:** `ic` CLI stable enough for session registration and event dispatch. Specifically: `ic run create`, `ic event emit`, `ic state get/set` need to work reliably.
- **Interspect:** Event schema finalized enough to emit structured events. Routing overrides schema readable by Skaffen's `select_model()`.

**If deps aren't ready:** v0.3 can still ship with standalone mode (no Intercore/Interspect), but the evidence flywheel doesn't turn until the bridges are live. This is acceptable as a staging strategy, not as a permanent state.

---

### v0.4 -- Self-Building

**Bead:** Demarch-22q

Skaffen develops Skaffen features. This happens in two stages: first, Skaffen is viable for routine Demarch development (bug fixes, small features) without falling back to Claude Code. Then, Skaffen ships at least one real feature end-to-end with no human code edits.

**What gets done:**
- Skaffen reads its own source, edits, tests, and commits
- Phase gates apply to Skaffen's own development (brainstorm phase is read-only even for its own code)
- Daily-driver viable: routine Demarch development tasks work without Claude Code fallback
- At least one feature shipped entirely by Skaffen (brainstorm through commit, no human code edits)
- Evidence from self-building sessions calibrates routing
- Side-by-side comparison: same task done by Skaffen and by Clavain-rigged Claude Code, measured on quality/cost/time

**Dependencies:**
- v0.3 complete (Intercore bridge and evidence emission working)
- Interspect calibration pipeline operational (so evidence from self-building actually feeds back into routing)

---

### v1.0 -- Production Parity

Daily-driver quality. Measurable improvement over Clavain-rigged Claude Code for Demarch development. The trust ladder from PHILOSOPHY.md applied to Skaffen itself: autonomy earned through demonstrated competence, not assumed.

**What gets done:**
- Skaffen is the primary coding agent for Demarch development
- Evidence flywheel is measurably compounding (Interspect data shows routing improvement over time)
- Performance, stability, and reliability comparable to or better than host agents for Demarch workflows
- Clavain discipline content loads cleanly into Skaffen

**Dependencies:**
- v0.4 complete
- Enough self-building sessions to have meaningful evidence for calibration

## Cross-Project Dependencies

The roadmap depends on work happening in parallel across Demarch. These are the specific gates:

| Skaffen milestone | External dependency | What's needed | Status |
|-------------------|-------------------|---------------|--------|
| v0.2 | Interspect routing overrides | Schema readable by `select_model()` | In progress (routing overrides schema exists but still evolving) |
| v0.3 | Intercore `ic` CLI | `ic run create`, `ic event emit`, `ic state get/set` stable | In progress (Intercore stabilization ongoing) |
| v0.3 | Interspect event schema | Structured event format for tool calls, model selections, phase transitions | In progress (event schema defined but not finalized) |
| v0.4 | Interspect calibration pipeline | Evidence from sessions feeds back into routing overrides | Not yet started |

## What's Not on This Roadmap

- **MCP server support.** Skaffen connecting to MCP servers (like Claude Code does) would add tool extensibility but is out of scope for v0.1-v0.4.
- **Multi-agent orchestration.** Skaffen spawning sub-Skaffens. Orchestration belongs in Autarch/Intercore, not in the agent runtime.
- **FrankenTUI migration.** Planned but not scheduled. Trigger: when inline mode becomes critical for Autarch multi-agent log multiplexing, WASM dashboards, or when charmed_rust hits a capability ceiling.
- **External adoption.** Skaffen is built for Demarch. Others can fork it, but the roadmap doesn't optimize for onboarding or documentation aimed at external users.
