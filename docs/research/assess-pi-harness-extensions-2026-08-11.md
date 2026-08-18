---
artifact_type: assessment
stage: research
bead: sylveste-xghz
subject: Pi harness companion extensions
verdict: selective-adopt
---
# Pi Harness Companion Extension Assessment

**Date:** 2026-08-11
**Scope:** Published package metadata and unpacked npm tarballs. This is a targeted adoption review, not a complete security audit of every transitive dependency.

## Verdict

Adopt two narrow, independently removable companions with exact top-level version pins:

1. `@narumitw/pi-plan-mode@0.49.3` — Observe/Orient/Decide frontend.
2. `@juicesharp/rpiv-ask-user-question@2.4.0` — structured uncertainty and authority questions outside plan mode.

Do not adopt another goal/task lifecycle as Skaffen's durable control plane. Intercore remains canonical.

## `@narumitw/pi-plan-mode@0.49.3`

**Verdict:** Adopt, pinned.

- License: MIT
- Repository: `narumiruna/pi-extensions`, `extensions/pi-plan-mode`
- Release commit observed: `4c2c2e8` (`v0.49.3`)
- npm SHA-512 integrity: `sha512-6yJoJ2nXsvnXAaEzr1iQlpUMqOFnDX+rMYVRXE/uV6OVmeB1AxFr9jm0H8N+1jr7Q0WebjtVSNE6dTOwaBGurA==`
- Published contents: 30 files, approximately 185 KB unpacked
- Pi compatibility stated by package: Pi 0.80.6+
- Local Pi: 0.84.1
- Lifecycle/install scripts: none in the published package

### Relevant behavior

- Fail-closed read-only tool policy during planning.
- Structured `plan_mode_question` and terminating `plan_mode_complete` tools.
- Exact accepted-plan persistence across compaction and resume.
- Current-session or fresh-linked-session implementation handoff.
- Explicit export writes using `writeFile(..., { flag: "wx" })`, so existing targets are not overwritten.
- Settings writes use temporary-file-plus-rename semantics.

### Sensitive surfaces reviewed

- `src/plan-export.ts`: creates parent directories and writes only after explicit `/plan export` or UI action.
- `src/settings.ts`: reads/writes Pi plan-mode settings after explicit settings changes.
- No network or child-process API use found in the published TypeScript.
- Direct dependency `@narumitw/pi-tui-kit` is declared as `^0.49.1`; the exact resolved transitive version must be retained in Pi's installed package lock/cache when reproducibility matters.

### Integration boundary

Skaffen should observe `plan_mode_complete` and register the accepted plan as an Intercore artifact in a later write-enabled phase. Plan mode does not become the canonical lifecycle store.

## `@juicesharp/rpiv-ask-user-question@2.4.0`

**Verdict:** Adopt, pinned.

- License: MIT
- Repository: `juicesharp/rpiv-mono`, `packages/rpiv-ask-user-question`
- Release commit observed: `a1531ed` (`v2.4.0`)
- npm SHA-512 integrity: `sha512-JVWc6CrePUgAEP3g6C3vrJBqZ6JSS56HQy7+2QZBdwwtcxqCNCHdPgXMJ43sexeJvusSJKTadeDJcc/4kuJEGA==`
- Published contents: 50 files, approximately 105 KB unpacked
- Lifecycle/install scripts: none in the published package
- Makes no model calls of its own according to its published documentation and reviewed source.

### Relevant behavior

- One `ask_user_question` tool with structured options, previews, notes, multi-select, and free-form answers.
- TUI, RPC, and ACP handling; tool is removed in non-interactive mode instead of failing.
- Useful for high-impact ambiguity, gate overrides, and authority escalation.

### Sensitive surfaces reviewed

- `state/external-editor.ts` spawns the user's configured editor only after the explicit external-editor key flow.
- That flow writes a temporary file, reads the edited result, and removes the temporary directory.
- No general shell execution or network surface found in the published package.
- Direct dependencies are `@juicesharp/rpiv-config@^2.4.0` and `typebox@^1.1.24`; exact resolved transitives should be retained by the installed lock/cache.

## Pattern-only or optional packages

| Package | Verdict | Reason |
|---|---|---|
| `@plannotator/pi-extension` | Optional frontend | Its `executionMode: external` and `plannotator:plan-approved` event are excellent future Intercore handoff seams, but it adds a browser workflow and should not run beside another plan-state owner by default. |
| `@narumitw/pi-goal` | Pattern only | Strong completion, continuation, budget, and stale-goal guards, but it owns goal state that would compete with Intercore. |
| `pi-goal-list-loop-audit` | Pattern only | Detached independent auditor and raw-evidence shield are valuable designs; its own goal/list/loop state machine overlaps Skaffen. |
| `@mjasnikovs/pi-task` | Pattern only | Crash-safe deterministic spec pipeline, but `.pi-tasks` would create another lifecycle source of truth. |
| `pi-subagents@0.48.0` | Adopt as bounded execution backend | Exact pin audited and live-tested. It supplies process-isolated async workflows, status/transcript/steer/stop controls, worktrees, and terminal lifecycle receipts while Intercore remains canonical. Schedules and missions are disabled; ordinary children cannot become nested orchestrators. |
| `pi-background-tasks` | Defer | Potential dispatch backend, but must not become a second durable scheduler beside Intercore. |
| `@gotgenes/pi-permission-system` | Study | Permission composition and failure semantics need review against Skaffen's phase/trust policy before installation. |
| `@juicesharp/rpiv-todo` | Do not adopt as canonical | Beads and Intercore already own work state. Its overlay is a UI pattern only. |

## Installed resolution

Pi installed both exact top-level pins successfully on 2026-08-11; npm reported zero known vulnerabilities. The shared Pi package lock resolved:

- `@narumitw/pi-plan-mode` 0.49.3
- `@narumitw/pi-tui-kit` 0.49.3
- `highlight.js` 11.11.1
- `@juicesharp/rpiv-ask-user-question` 2.4.0
- `@juicesharp/rpiv-config` 2.4.0
- `typebox` 1.3.12

The lock is runtime installation state under `~/.pi/agent/npm/package-lock.json`; the exact top-level source specs are durable in Pi settings.

## Installation and rollback

```bash
pi install npm:@narumitw/pi-plan-mode@0.49.3
pi install npm:@juicesharp/rpiv-ask-user-question@2.4.0

# Roll back independently
pi remove npm:@narumitw/pi-plan-mode
pi remove npm:@juicesharp/rpiv-ask-user-question
```

Pi packages execute with full user access. Re-review source and integrity before moving any pin.

## `pi-subagents@0.48.0` bounded adoption

Installed after a source/package audit and a live acceptance test. The trial proved a detached reviewer could run while foreground Pi executed `/harness` and completed an independent model turn. The child completed with durable status/events artifacts and no Skaffen worktree changes.

Conservative user configuration lives at `~/.pi/agent/extensions/subagent/config.json`:

- two active async workflows per session;
- concurrency three, six tasks maximum, depth one;
- twelve spawns per run and twenty-four per session;
- schedules and package missions disabled;
- destructive cleanup and budget grants require confirmation;
- artifacts remain session-scoped rather than canonical project truth.

This package is an execution substrate only. Skaffen may bridge terminal receipts and Intercore dispatch identities later, but package missions, schedules, or result files must not become a second durable authority store.
