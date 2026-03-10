---
artifact_type: brainstorm
stage: discover
---

# Skaffen — Initial Goals

**Date:** 2026-03-10
**Epic:** Demarch-6qb

## Month 1 Goals (Priority Order)

### G1: Fork and Stabilize
Fork pi_agent_rust, rebrand to Skaffen, get CI green, verify all 3,857+ tests pass. Strip pi-specific branding. Establish `cargo build && cargo test` as the quality gate.

### G2: OODARC Loop
Modify the agent loop (`src/agent.rs`) to implement phase-aware tool gating and the OODARC turn structure. Hard gate tools by phase (brainstorm=read-only, build=full access, review=read+test). Each turn: observe → orient → decide → act → reflect → compound.

### G3: Intercore Bridge
Connect Skaffen to Intercore via CLI bridge (`ic` binary). Agent loop emits dispatch events, receives run state. Skaffen participates in the kernel as a first-class agent runtime.

### G4: Evidence Pipeline
Native Interspect evidence emission from the agent loop. Every turn emits structured events (tool calls, model selections, phase transitions, steering decisions). Events flow into the Interspect pipeline for routing calibration.

## Success Criteria

- **v0.1:** Fork built, CI green, `skaffen` binary runs interactive mode with all 7 tools
- **v0.2:** OODARC loop, phase-aware tool gating, mid-session model switching
- **v0.3:** Intercore bridge, evidence emission, routing overrides drive model selection
- **v0.4:** Self-building (Skaffen develops Skaffen features)
