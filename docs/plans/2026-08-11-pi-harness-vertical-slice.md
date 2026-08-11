---
artifact_type: plan
bead: sylveste-xghz
stage: design
requirements:
  - F1: Read-only Intercore observation
  - F2: Visible degraded mode
  - F3: Pi-native command and tool surface
  - F4: Pinned companion extensions
---
# Pi-Hosted Skaffen Harness Vertical Slice Implementation Plan

> **For Pi:** Execute with test-driven development, one task at a time. Do not migrate or write to Intercore.

**Bead:** sylveste-xghz

**Goal:** Make Skaffen load as a Pi package that safely observes Intercore, exposes `/harness`, `/situation`, and `observe_situation`, and visibly degrades when the installed `ic` binary and database schema disagree.

**Architecture:** Pi remains the universal agent loop. `pi-package/src/intercore.ts` is a pure, injected CLI adapter over `pi.exec`; `presentation.ts` converts typed state into bounded human/model views; `index.ts` only registers Pi lifecycle, command, and tool surfaces. Intercore remains canonical and the slice performs no writes.

**Tech Stack:** TypeScript, Pi 0.84.1 extension API, TypeBox, Vitest, `ic` CLI.

**Prior Learnings:**
- `docs/solutions/patterns/intercore-bridge-subprocess-lifecycle-20260311.md`: bound every `ic` subprocess and degrade visibly when optional dependencies fail.
- `docs/solutions/patterns/hook-system-adapter-pattern-20260312.md`: isolate host API types behind an adapter and define fail-open/fail-closed per capability.
- `docs/solutions/patterns/intercore-schema-upgrade-deployment-20260218.md`: binary schema and embedded migrations travel together; never run migration from startup integration code.
- `docs/solutions/patterns/activation-sprint-last-mile-gap-20260307.md`: finish with an actual package-load smoke test, not unit tests alone.

---

## Must-Haves

**Truths**
- A Pi session can start even when `ic` is missing, times out, emits malformed JSON, or reports a schema mismatch.
- `/harness` distinguishes healthy, degraded, and unavailable states and explains why.
- `/situation` and `observe_situation` report active runs without presenting an unrelated run as the current project run.
- The footer shows a compact Intercore/run/OODARC status.
- The extension never invokes `ic init` or any write subcommand.
- Companion extensions are pinned to audited versions and independently removable.

**Artifacts**
- `pi-package/src/intercore.ts` exports `inspectIntercore`, `parseSituation`, and typed state.
- `pi-package/src/presentation.ts` exports status and detailed renderers.
- `pi-package/src/index.ts` exports the Pi extension factory.
- `pi-package/test/*.test.ts` cover the specified failure matrix and extension registration.
- `docs/research/assess-pi-harness-extensions-2026-08-11.md` records adoption verdicts and integrity hashes.

**Key Links**
- `index.ts` adapts `pi.exec("ic", ...)` into the pure `inspectIntercore` runner.
- Every CLI call uses `ctx.cwd` and a finite timeout.
- Human commands and the agent tool call the same refresh function and render the same canonical state.
- Footer state is derived from the same state returned by `/harness` and `/situation`.

---

### Task 1: Package and test harness

**Files:**
- Create: `pi-package/package.json`
- Create: `pi-package/tsconfig.json`
- Create: `pi-package/README.md`
- Create: `pi-package/src/types.ts`

**Step 1:** Create a Pi package manifest named `@mistakeknot/pi-skaffen`, tagged `pi-package`, with `pi.extensions: ["./src/index.ts"]` and Pi/TypeBox peer dependencies.

**Step 2:** Add Vitest, TypeScript, and Pi 0.84.1 packages as development dependencies. Add `test`, `typecheck`, and `check` scripts.

**Step 3:** Define only shared data types in `src/types.ts`; no runtime behavior yet.

**Step 4:** Install dependencies and verify the empty test suite and type checker execute.

<verify>
- run: `cd pi-package && npm install`
  expect: exit 0
- run: `cd pi-package && npm run typecheck`
  expect: exit 0
</verify>

### Task 2: Read-only Intercore adapter

**Files:**
- Create: `pi-package/test/intercore.test.ts`
- Create: `pi-package/src/intercore.ts`

**Step 1: Write failing tests**

Cover:
1. `health` exit 0 plus valid snapshot → `healthy`.
2. health schema mismatch plus valid snapshot → `degraded` with snapshot retained.
3. both commands unavailable → `unavailable`.
4. killed command → `unavailable` or `degraded`, never healthy.
5. malformed snapshot JSON → degraded with no snapshot.
6. malformed shape (missing arrays/summaries) → degraded.
7. runner rejection → unavailable without throwing.
8. current-run matching uses the deepest enclosing `project_dir`; unrelated runs remain global only.
9. only the read commands `health --json` and `situation snapshot --json` are invoked, both with a finite timeout and the supplied cwd.

**Step 2: Run tests and verify RED**

Run: `cd pi-package && npm test -- test/intercore.test.ts`
Expected: FAIL because `src/intercore.ts` does not exist.

**Step 3: Implement the minimum adapter**

Use an injected runner:

```ts
export type IcRunner = (
  args: readonly string[],
  options: { cwd: string; timeout: number },
) => Promise<ExecResult>;

export async function inspectIntercore(
  runner: IcRunner,
  cwd: string,
  timeout = 250,
): Promise<HarnessState>;
```

Run health and snapshot concurrently. Parse health code/stdout/stderr separately from situation JSON. Extract a structured error message from JSON log lines when available. Never throw past `inspectIntercore`.

**Step 4: Run tests and verify GREEN**

<verify>
- run: `cd pi-package && npm test -- test/intercore.test.ts`
  expect: exit 0
</verify>

### Task 3: Bounded presentation

**Files:**
- Create: `pi-package/test/presentation.test.ts`
- Create: `pi-package/src/presentation.ts`

**Step 1: Write failing tests**

Cover compact statuses for:
- healthy current run: phase + OODARC role;
- healthy idle with global active count;
- degraded current run;
- unavailable `ic`;
- unknown OODARC role omitted rather than invented.

Cover detailed rendering for:
- health reason and snapshot timestamp;
- current run before unrelated runs;
- dispatch and queue counts;
- bounded error text with terminal control characters removed.

**Step 2: Run tests and verify RED**

Run: `cd pi-package && npm test -- test/presentation.test.ts`
Expected: FAIL because presentation exports do not exist.

**Step 3: Implement status/detail renderers**

Keep footer output under 80 visible characters. Keep command/tool output deterministic Markdown/plain text and cap external error excerpts.

**Step 4: Run tests and verify GREEN**

<verify>
- run: `cd pi-package && npm test -- test/presentation.test.ts`
  expect: exit 0
</verify>

### Task 4: Pi extension integration

**Files:**
- Create: `pi-package/test/extension.test.ts`
- Create: `pi-package/src/index.ts`

**Step 1: Write failing tests with a fake `ExtensionAPI`**

Assert registration of:
- `session_start` refresh;
- `/harness` and `/situation`;
- `observe_situation` read-only tool;
- finite-timeout `pi.exec` adaptation;
- compact footer status update;
- no Intercore write command anywhere in registered behavior.

Invoke captured handlers to prove degraded state does not throw and repeated commands refresh instead of serving stale data.

**Step 2: Run tests and verify RED**

Run: `cd pi-package && npm test -- test/extension.test.ts`
Expected: FAIL because the extension factory does not exist.

**Step 3: Implement the minimum extension**

- Refresh once on `session_start`.
- Use `ctx.ui.setStatus("skaffen", ...)` for the compact state.
- `/harness` and `/situation` append durable, TUI-only status-card entries and notify in headless-safe form.
- `observe_situation` returns text plus typed details and performs no mutation.
- Do not inject situation into every model prompt in this slice.

**Step 4: Run tests and verify GREEN**

<verify>
- run: `cd pi-package && npm test -- test/extension.test.ts`
  expect: exit 0
- run: `cd pi-package && npm run typecheck`
  expect: exit 0
</verify>

### Task 5: Companion extension pinning, documentation, and activation

**Files:**
- Create: `docs/research/assess-pi-harness-extensions-2026-08-11.md`
- Modify: `README.md`
- Modify: Pi user settings via `pi install` commands

**Step 1:** Record the package source, version, integrity, sensitive surfaces, and verdict for:
- `@narumitw/pi-plan-mode@0.49.3` — adopt as Observe/Orient/Decide frontend.
- `@juicesharp/rpiv-ask-user-question@2.4.0` — adopt as structured uncertainty gate.
- Plannotator, goal, task, audit, and subagent packages — optional or pattern-only, not first-slice dependencies.

**Step 2:** Add a README section explaining the Pi-hosted direction, local development commands, degraded behavior, and that the Go runtime remains available during migration.

**Step 3:** Run full package checks.

**Step 4:** Smoke-load only the local package through Pi without a model turn.

**Step 5:** Install the local package and the two companion packages with exact version pins. Verify with `pi list`.

<verify>
- run: `cd pi-package && npm run check`
  expect: exit 0
- run: `pi --no-extensions -e ./pi-package --version`
  expect: contains "0.84.1"
- run: `pi list`
  expect: contains "@narumitw/pi-plan-mode@0.49.3"
- run: `pi list`
  expect: contains "@juicesharp/rpiv-ask-user-question@2.4.0"
- run: `pi list`
  expect: contains "/Users/arouth/projects/Skaffen/pi-package"
</verify>

## Final Verification

```bash
cd /Users/arouth/projects/Skaffen/pi-package
npm run check
cd ..
git diff --check
git status --short
pi list
```

Manual TUI smoke:

```text
/harness
/situation
```

Expected in the current environment: visible **degraded** health because installed `ic` expects schema 37 while the umbrella DB reports schema 30, but `/situation` still renders the valid snapshot. No migration occurs.
