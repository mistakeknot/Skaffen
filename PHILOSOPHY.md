# Skaffen Philosophy

Skaffen inherits Demarch's three core principles and applies them to the agent runtime itself:

1. **Every action produces evidence.** The agent loop emits structured events at every turn — tool calls, model selections, phase transitions, steering decisions. Receipts are not optional; they're architectural.
2. **Evidence earns authority.** Skaffen's autonomy level is a dial, calibrated by Interspect from accumulated evidence. More evidence → better routing → cheaper tokens → more autonomy. The flywheel runs inside the loop, not around it.
3. **Authority is scoped and composed.** Phase gates are hard constraints, not hints. In brainstorm phase, write tools are structurally unavailable. In review phase, the model may differ from build phase. The loop enforces scope; the LLM operates within it.

## Why a Sovereign Runtime

Clavain proves that discipline infrastructure (phase gates, review agents, evidence pipelines) delivers real value. But bolting discipline onto an opaque host runtime hits a ceiling:

- **The loop is not ours.** Host agents decide when to compact, which model to use, how to handle overflow. We can hook around decisions but not change them.
- **Evidence is aftermarket.** Scraping events from hooks and logs is fragile. Native emission from the loop is reliable.
- **Model routing is host-dependent.** The cheapest-model-that-clears-the-bar philosophy requires mid-session model switching. Host agents don't support this.

Skaffen removes the ceiling by owning the loop. The discipline becomes structural, not aspirational.

## OODARC in the Loop

Every turn implements the full cycle:

- **Observe** — Tool results, LLM response, evidence from prior turns
- **Orient** — Phase context, model selection, tool availability
- **Decide** — LLM call with oriented context
- **Act** — Tool execution
- **Reflect** — Structured evidence emission (what happened vs. what was expected)
- **Compound** — Persist learnings that change future behavior (routing overrides, calibration)

Reflect without Compound is journaling. Compound without Reflect is cargo-culting.

## Design Bets

1. **Performance matters for autonomy.** Sub-100ms startup and 12x lower memory than Node means Skaffen can run more sessions, longer sessions, and parallel sessions. Capacity enables the flywheel.
2. **Safety is structural.** `#![forbid(unsafe_code)]`, capability-gated extensions, hard phase gates. Trust the type system over the prompt.
3. **Fork, don't rewrite.** Pi_agent_rust is battle-tested (89/89 feature parity, 3,857+ tests). Rewriting a coding agent from scratch is accidental complexity. Fork, diverge where it matters, keep compatibility where it doesn't.
