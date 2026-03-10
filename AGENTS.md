# Skaffen — Agent Development Guide

Demarch's sovereign agent runtime. Forked from pi_agent_rust, modified with OODARC-native loop, phase gates, evidence emission, and Intercore integration.

## Key Differences from pi_agent_rust

1. **Phase-aware tool gating** — tools available change by phase (brainstorm=read-only, build=full access)
2. **Native evidence emission** — every turn emits structured events to Interspect pipeline
3. **OODARC loop** — Observe, Orient, Decide, Act, Reflect, Compound built into turn cycle
4. **Model routing** — Interspect routing overrides drive model selection per turn
5. **Intercore integration** — dispatch, events, and run state flow through the kernel

## Naming

Skaffen-Amtiskaw: Culture drone operating with full autonomy within its authority scope. Earned authority, not assumed.

## Git Workflow

Trunk-based development — commit directly to `main`.
