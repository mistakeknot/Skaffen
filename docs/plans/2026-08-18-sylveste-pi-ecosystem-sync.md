---
artifact_type: plan
bead: none
stage: design
requirements:
  - F1: Clavain runtime and host installs are synchronized and healthy
  - F2: Intercore runtime is provenance-stamped and safely migrated to schema 39
  - F3: Skaffen consumes Intercore provenance, agency, and producer contracts read-only
  - F4: Third-party upstream decisions are evidence-backed and exactly pinned when adopted
---
# Sylveste Pi Ecosystem Sync Implementation Plan

> **For executors:** Use the executing-plans and test-driven-development disciplines task-by-task.

**Intercore portfolio run:** `l39wvala` (`niya78he` Clavain, `vh82qo8n` Intercore, `1odse4u8` Skaffen)

**Goal:** Synchronize the Pi-native Skaffen harness with current Clavain and Intercore contracts while preserving Intercore authority, safe migrations, exact pins, and reversible evidence-backed adoption decisions.

**Architecture:** Pi remains the universal agent loop, Skaffen remains a read-only policy/presentation adapter, Clavain supplies workflow discipline, and Intercore remains canonical for durable state and authority. Host/runtime updates happen before database migration; database migration happens before new contract consumption. Third-party systems contribute audited patterns or exact pins, never competing durable workflow truth.

**Tech Stack:** Pi 0.84.1, TypeScript/Typebox/Vitest, Clavain shell/Python installers, Go Intercore CLI, SQLite WAL, npm.

**Alignment:** Advances orchestration quality and evidence while preserving the Pi/Skaffen/Intercore layer boundary.

**Conflict/Risk:** Cross-repository and live-database changes are high impact; mitigate with clean/isolated checkouts, exact commit provenance, byte-consistent backups, red-green tests, and serial execution.

**Prior Learnings:**
- `Sylveste/docs/solutions/patterns/intercore-schema-upgrade-deployment-20260218.md`: binary/schema DDL travel together; update binary before migration.
- `Sylveste/docs/solutions/patterns/intercore-bridge-subprocess-lifecycle-20260311.md`: use bounded subprocess calls and explicit degraded states.
- `Sylveste/docs/solutions/2026-07-25-unattended-work-needs-a-stopped-signal.md`: every detached workflow needs observable terminal state.
- `Skaffen/docs/research/assess-pi-harness-extensions-2026-08-11.md`: exact-pin companions and avoid competing workflow truth.

---

## Must-Haves

**Truths**
- Clavain project, Codex skills, and Claude plugin resolve to the same current release and pass their doctors.
- The installed `ic` reports a non-dirty source commit, the live database is schema 39, and the pre-migration schema-37 backup remains valid.
- `/harness`, `/situation`, and `observe_situation` remain bounded and read-only while surfacing Intercore build provenance and new situation ownership fields.
- Existing schema-37 snapshots remain accepted; malformed optional v39 fields fail safely without inventing authority.
- No Superpowers, Compound Engineering, Beads, or other package is installed as a competing controller.

**Artifacts**
- `pi-package/src/types.ts` defines validated provenance, agency, and producer shapes.
- `pi-package/src/intercore.ts` runs only `health`, `version`, and `situation snapshot` reads.
- `pi-package/src/presentation.ts` renders bounded provenance/ownership summaries.
- `pi-package/test/*.test.ts` proves compatibility, degradation, read-only behavior, and output bounds.
- `docs/research/2026-08-18-sylveste-ecosystem-sync-execution-receipt.md` records execution state and verification.
- `docs/research/2026-08-18-intercore-schema39-migration-proof.json` durably embeds pre/post outputs, per-table preservation digests, backup evidence, and the last rollback-eligible boundary.
- `docs/research/2026-08-18-intercore-schema39-rollback-transition.json` durably records the first v39 application write and roll-forward-only transition.
- `docs/research/2026-08-18-sylveste-ecosystem-sync-decisions.md` records adopt/defer/ignore decisions and exact evidence.

**Key Links**
- The installed `ic` binary is upgraded and verified before `ic init` migrates the live DB.
- Skaffen parses canonical `ic version --json` output independently from DB health; the database schema is never treated as binary provenance.
- Intercore phases remain canonical; `oodarc_role`, producer, and agency are annotations, not a second state machine.

**Approved immutable sources:**
- Clavain `43db9fcf83d9736c3439d2ea1911c39e7123b275` (`0.6.299`). Any newer remote head requires a new diff review before installation.
- Intercore `ac2dc66f59fc57a901c214080f99056a96de670f` (`0.3.5`, binary maximum schema 39). Any newer remote head requires a new diff review before build or migration.
- tldr-swinton `6f6a4ec5f1319e89149da6d79d0d9bb27952879b` (`0.8.4`) as the Clavain context-gateway dependency.

**Execution evidence:** `docs/research/2026-08-18-sylveste-ecosystem-sync-execution-receipt.md` summarizes approved SHAs, paths/hashes, preflight results, timestamps, rollback state, validation, portfolio, and landing state. `docs/research/2026-08-18-intercore-schema39-migration-proof.json` contains the complete durable machine-readable migration outputs and preservation proof; `docs/research/2026-08-18-intercore-schema39-rollback-transition.json` records the later first-write boundary. No claim depends on ephemeral `/tmp` artifacts after landing.

---

### Task 1: Synchronize and repair Clavain host installations

**Files:**
- Existing checkout: `/Users/arouth/projects/Clavain`
- Global installs: `~/.codex/clavain`, `~/.agents/skills/clavain`, Claude plugin cache/settings

**Steps:**
1. Reconfirm the Clavain worktree is clean and remote main equals approved SHA `43db9fcf83d9736c3439d2ea1911c39e7123b275`; stop if it differs.
2. Fast-forward with `git pull --ff-only`; do not merge or rebase unrelated work.
3. Run the repository’s focused structural tests for orchestration resume and review with explicit ephemeral `pytest` and `pyyaml` dependencies.
4. Install exact tldr-swinton source SHA `6f6a4ec5f1319e89149da6d79d0d9bb27952879b` (`0.8.4`) and validate its packet schema.
5. Run `scripts/install-codex.sh update --source /Users/arouth/projects/Clavain`, then require JSON doctor status `ok` with zero issues.
6. Run `claude plugin update clavain@interagency-marketplace`, verify the resolved `0.6.299` cache target exists and `claude plugin list` reports the same version.
7. Confirm the detached Flux Melange process was not interrupted; completed-idle is acceptable.

<verify>
- run: `git -C /Users/arouth/projects/Clavain status --short --branch`
  expect: contains "main...origin/main"
- run: `bash /Users/arouth/projects/Clavain/scripts/install-codex.sh doctor --source /Users/arouth/projects/Clavain --json`
  expect: exit 0
- run: `claude plugin list`
  expect: contains "Version: 0.6.299"
</verify>

### Task 2: Build and stage Intercore schema-39 runtime

**Files:**
- Isolated clone/worktree under `/tmp`
- Installed binary: `~/.local/bin/ic`
- Live DB: `/Users/arouth/projects/.clavain/intercore.db`

**Steps:**
1. Require remote Intercore main to equal approved SHA `ac2dc66f59fc57a901c214080f99056a96de670f`; create an isolated clean checkout at that exact SHA and capture the v39 situation/producer/agency contract files as test evidence.
2. Run focused DB migration, observation, goal, autonomy, receipt, and CLI tests plus `go test ./...`.
3. Build `ic` with repository-supported provenance stamping; reject empty commit, dirty build, wrong architecture, unsupported schema, or bytes not matching the staged hash.
4. Back up the current installed binary and test schema 37→39 on a fresh SQLite-backup-API copy. Record deterministic pre/post table counts and schema-object queries.
5. Confirm copied core table counts, integrity, zero foreign-key rows, `lanes.intent`, goals, agencies, and situation output.
6. Establish a maintenance fence: require no active dispatches/queue writes, no live `ic` process, and no open DB/WAL/SHM handles. Do not invoke other Intercore tools between the final snapshot and migration.
7. Create a transactionally consistent live backup with SQLite `.backup`; independently open it, require schema 37/integrity `ok`, record hash, and record rollback eligibility.
8. Atomically install the verified binary, require path/hash/provenance equality, then run the new `ic init` and verify schema 39 before any v39 write.
9. Rollback criterion: if migration or pre-write verification fails, fence writers, restore the old binary and schema-37 backup as a pair, then reverify. After any successful v39 write, mark rollback ineligible and roll forward only.
10. Preserve every material migration command result, path, hash, timestamp, per-table common-column digest, and rollback-state transition in the tracked machine-readable migration proof; summarize it in the execution receipt.

<verify>
- run: `cd /Users/arouth/projects && test "$(command -v ic)" = "$HOME/.local/bin/ic" && ic version --json | jq -e '.schema == 39 and .commit == "ac2dc66f59fc57a901c214080f99056a96de670f" and .dirty == false and .source != "unknown"'`
  expect: exit 0
- run: `cd /Users/arouth/projects && ic health`
  expect: contains "ok"
- run: `sqlite3 /Users/arouth/projects/.clavain/intercore.db 'PRAGMA integrity_check; PRAGMA foreign_key_check;'`
  expect: contains "ok"
</verify>

### Task 3: Add Intercore provenance inspection to Skaffen using TDD

**Files:**
- Modify: `pi-package/src/types.ts`
- Modify: `pi-package/src/intercore.ts`
- Modify: `pi-package/src/presentation.ts`
- Modify: `pi-package/test/intercore.test.ts`
- Modify: `pi-package/test/presentation.test.ts`
- Modify: `pi-package/test/extension.test.ts`

**Steps:**
1. Add failing tests for `ic --json version`: valid stamped build, unstamped/dirty build, malformed JSON, timeout, and schema/runtime distinction.
2. Run focused tests and confirm failures are caused by missing provenance support.
3. Add minimal validated types/parser and execute `version --json` concurrently with existing read calls, preserving bounded timeout/cancellation.
4. Define policy: health+situation determine operational health; unavailable/malformed version visibly degrades provenance but never writes/migrates.
5. Add failing presentation tests for bounded version/commit/source output.
6. Implement minimal presentation and rerun focused/full tests.

<verify>
- run: `npm --prefix pi-package test -- --run test/intercore.test.ts test/presentation.test.ts`
  expect: exit 0
- run: `npm --prefix pi-package run typecheck`
  expect: exit 0
</verify>

### Task 4: Consume agency and producer situation fields using TDD

**Files:**
- Modify: `pi-package/src/types.ts`
- Modify: `pi-package/src/intercore.ts`
- Modify: `pi-package/src/presentation.ts`
- Modify: `pi-package/test/intercore.test.ts`
- Modify: `pi-package/test/presentation.test.ts`

**Steps:**
1. Pin golden schema-39 fixtures and field semantics from approved Intercore SHA `ac2dc66f59fc57a901c214080f99056a96de670f` (`contracts/cli/situation-snapshot.json`, `internal/observation/observation.go`, and live copied-DB output), then add failing compatibility tests for agencies and producer metadata, legacy schema-37 snapshots, malformed optional fields, and bounded collection rendering.
2. Confirm red failures.
3. Implement strict optional shapes while preserving unknown-field forward compatibility.
4. Render producer ownership on runs and a bounded agency summary; do not infer owner or authority when absent.
5. Run focused and full tests.

<verify>
- run: `npm --prefix pi-package test`
  expect: exit 0
- run: `npm --prefix pi-package run typecheck`
  expect: exit 0
</verify>

### Task 5: Audit exact upstream pins and record decisions

**Files:**
- Create: `docs/research/2026-08-18-sylveste-ecosystem-sync-decisions.md`
- Optional global package change: `@steipete/oracle@0.17.3`

**Steps:**
1. Record official release evidence for Superpowers 6.3.0, Oracle 0.17.3, Beads 1.2.2, Compound Engineering 3.22.4, Superpowers Lab, and CanonGraph.
2. Record decisions: adopt Superpowers controller patterns only; defer Beads major upgrade; do not install Compound/Superpowers controllers; defer unrelated CanonGraph/Lab changes.
3. Download and inspect Oracle 0.17.3 tarball metadata, lifecycle scripts, dependency tree, license, integrity, and npm audit in isolation.
4. If audit passes, install the exact Oracle pin and verify CLI/version; otherwise leave 0.9.0 installed, record the blocker and unchanged installed version, and treat the task as a safe defer rather than a failure.
5. Leave Clavain upstream issue #17 open unless its formal decision-record/PR gate is satisfied; do not fabricate a PR-number record.

<verify>
- run: `test "$(oracle --version)" = "0.17.3" || rg -n 'Oracle.*defer.*0.9.0|Oracle.*0.9.0.*defer' docs/research/2026-08-18-sylveste-ecosystem-sync-decisions.md`
  expect: exit 0
- run: `rg -n 'adopt|defer|ignore' docs/research/2026-08-18-sylveste-ecosystem-sync-decisions.md`
  expect: exit 0
</verify>

### Task 6: End-to-end validation, review, and landing

**Files:**
- Modify: `README.md`
- Modify: `pi-package/README.md`
- Modify: `docs/research/assess-pi-harness-extensions-2026-08-11.md`
- Modify: this plan and execution receipt as needed

**Steps:**
1. Update Skaffen docs with the read-only provenance/ownership contract and exact companion state.
2. Run package tests, typecheck, npm pack inspection, npm audit, `ic health`, situation snapshot, and a newly loaded Pi TUI smoke for `/harness` and `/situation`.
3. Dispatch fresh spec and code-quality reviewers; resolve blocking findings and rerun affected checks.
4. Commit and push Skaffen only after verification; do not commit unrelated files.
5. Add verified artifacts to the three named child runs and run gate checks only. Do not advance or close the portfolio or children in this execution; durable phase transitions require a separately enumerated transition after evidence review.
6. Verify exact HEAD/upstream equality separately for Clavain and Skaffen; report the untouched Intercore feature branch separately from the isolated runtime source. Preserve rollback receipts.

<verify>
- run: `npm --prefix pi-package test && npm --prefix pi-package run typecheck`
  expect: exit 0
- run: `cd /Users/arouth/projects && ic health && ic situation snapshot --json`
  expect: exit 0
- run: `git status --short --branch`
  expect: contains "main...origin/main"
</verify>
