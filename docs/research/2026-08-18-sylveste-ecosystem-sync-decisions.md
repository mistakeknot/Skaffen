---
title: "Sylveste ecosystem sync decisions for the Pi harness"
artifact_type: research
run: l39wvala
date: 2026-08-18
status: accepted
---

# Sylveste Ecosystem Sync Decisions

## Decision rule

Adopt updates that improve recovery, provenance, ownership, or evidence without introducing another durable workflow controller. Exact-pin third-party executables after inspecting artifacts, lifecycle scripts, dependencies, licenses, registry signatures, and vulnerabilities. Defer major state-store upgrades until their migration path is separately rehearsed.

## Internal ecosystem

| Component | Evidence | Decision | Rationale |
|---|---|---|---|
| Clavain 0.6.299 | [`43db9fc`](https://github.com/mistakeknot/Clavain/commit/43db9fcf83d9736c3439d2ea1911c39e7123b275) | **adopt now** | Kill-safe orchestration journal/`--resume`, guard cleanup, bounded independent review, and one-page artifact support directly improve detached execution. Project, Codex source/skills, and Claude plugin were synchronized to this exact SHA/version. |
| tldr-swinton 0.8.4 | [`6f6a4ec`](https://github.com/mistakeknot/tldr-swinton/commit/6f6a4ec5f1319e89149da6d79d0d9bb27952879b) | **adopt now** | Required by Clavain’s context gateway packet schema. Exact clean local/remote source installed; machine packet schema v1 and Clavain Codex doctor passed. |
| Intercore schema 39 | [`ac2dc66`](https://github.com/mistakeknot/intercore/commit/ac2dc66f59fc57a901c214080f99056a96de670f) | **adopt now** | Adds build provenance, durable goals, agency/producer observation, delegation rulings, and stronger witness semantics. Binary-first migration was rehearsed and live schema 37→39 preserved all existing table counts. |
| Skaffen provenance/ownership bridge | Intercore v39 contract plus local TDD | **adopt now** | Consumes canonical facts read-only and keeps Pi/Skaffen subordinate to Intercore authority. |

## Tracked upstreams

| Upstream | Observed current | Decision | Rationale |
|---|---:|---|---|
| [obra/superpowers](https://github.com/obra/superpowers/releases/tag/v6.3.0) | v6.3.0 | **adopt patterns only** | Event-driven waits and non-orchestrating ordinary children already exist in `pi-subagents`. Retain the additional patterns: batch same-shaped microtasks, record conflict checks, attach explicit specs, and let controllers rule on reversible ambiguity. Do not install it as a competing planning/workflow controller. |
| [steipete/oracle](https://github.com/steipete/oracle/releases/tag/v0.17.3) | v0.17.3 | **adopt exact pin** | Browser answer preservation, reattach cookie recovery, explicit headless behavior, and localized effort-selection fixes benefit cross-AI review without owning durable task state. Installed exact npm pin after audit below. |
| [steveyegge/beads](https://github.com/steveyegge/beads/releases/tag/v1.2.2) | v1.2.2; installed 0.57.0 | **defer** | Major stateful upgrade. Requires a separate Dolt/database/CLI compatibility rehearsal; the current Sylveste board also exhibited a server/data-directory mismatch. Do not couple that repair to the Pi harness sync. |
| [EveryInc/compound-engineering-plugin](https://github.com/EveryInc/compound-engineering-plugin/releases/tag/compound-engineering-v3.22.4) | v3.22.4 | **defer package; adopt portability lesson** | Shared/no-checkout host portability is relevant, but installing the plugin would add overlapping workflow control. Apply the portability principle to our own worktree adapters only. |
| [obra/superpowers-lab](https://github.com/obra/superpowers-lab) | head `51111f7` | **ignore for this sync** | Slack skill removal in favor of Slackline does not affect the independently owned Sylveste Slack integration or the Pi harness. |
| [obra/superpowers-developing-for-claude-code](https://github.com/obra/superpowers-developing-for-claude-code) | head `74afe93` | **ignore; no drift** | Baseline already matches the observed head. |
| [jvattimo1/canongraph](https://github.com/jvattimo1/canongraph/releases/tag/v0.2.0) | v0.2.0 | **defer** | Entity-graph memory remains optional enrichment. It is not needed for runtime provenance, situation ownership, or safe background workflows. |

## Oracle 0.17.3 audit and installation receipt

- Package: `@steipete/oracle@0.17.3`, MIT, Node >=24.
- npm registry integrity: `sha512-xoziw8brto9rEtOROHcMr4vHu70DDGQJ41bwMHpkJgA77MIZ11B+IQtGqKpZ48WkihmHkEUVEvWsf+eDwxtwgg==`.
- Registry SHA-1: `bda7cc2d576007f68c3fc535cdc5a58ed8b977b8`.
- Downloaded tarball SHA-256: `9933f177884d6ca662f1131dbb9c17b95c0b01ccd877a2d93e5ee5f0778b357f`.
- Artifact: 285 entries, 7,279,898 packed bytes, 9,872,119 unpacked bytes.
- Executables: `oracle`, `oracle-mcp`, bundled macOS notifier. No `preinstall`, `install`, or `postinstall` script; registry installation used `--ignore-scripts` to avoid lifecycle execution, including `prepare`.
- Isolated install: 304 dependency packages audited; **0 vulnerabilities**.
- Supply-chain checks: 304 verified registry signatures; 27 verified attestations.
- Licenses: MIT, ISC, Apache-2.0, BSD-2-Clause, BSD-3-Clause, and `(MIT OR CC0-1.0)`; no unknown licenses.
- Installed transition: 0.9.0 → **0.17.3**.
- Installed CLI/MCP bytes match the audited tarball copies.
- Rollback: `npm install -g --ignore-scripts @steipete/oracle@0.9.0`.

## Deferred operational work

- Clavain upstream-sync issue [#17](https://github.com/mistakeknot/Clavain/issues/17) remains open. Its baseline is intentionally not advanced here because the formal PR-number decision gate was not exercised and Beads remains deferred.
- The Sylveste Beads board failed with a Dolt server/data-directory mismatch. Repair it separately; do not run `bd init` over the existing board.
- Intercore `ic init` sequential-migration defect remains tracked as `intercore-2e8`; schema 30→37 was handled with the versioned migrator, and this execution only used the rehearsed 37→39 path.
