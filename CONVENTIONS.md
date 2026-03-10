# Skaffen Conventions

## Naming

- Binary: `skaffen` (lowercase in CLI, proper case in prose)
- Crate: `skaffen`
- Config dir: `~/.skaffen/` (user) or `.skaffen/` (project)
- Session dir: `~/.skaffen/sessions/`

## Rust Conventions

- Edition: 2024 (nightly)
- `#![forbid(unsafe_code)]` — no exceptions
- Error handling: `thiserror` for library errors, `anyhow` at binary boundaries
- Async: `asupersync` (inherited from pi_agent_rust) — not tokio
- Formatting: `rustfmt` with project `.rustfmt.toml`
- Linting: `clippy` with `#![warn(clippy::all)]`
- Tests: inline `#[cfg(test)]` for unit tests, `tests/` for integration

## Git

- Trunk-based on `main`
- Commit messages: conventional commits (`feat:`, `fix:`, `chore:`, `docs:`)
- Beads tracked in Demarch monorepo (prefix `Demarch-`), not in this repo

## Artifact Naming

- Brainstorms: `docs/brainstorms/YYYY-MM-DD-<topic>.md`
- Plans: `docs/plans/YYYY-MM-DD-<topic>.md`
- Solutions: `docs/solutions/<category>/<slug>-YYYYMMDD.md`

## Dependencies

- Minimize external crates. Prefer pi_agent_rust's existing dependency choices.
- New dependencies require justification in commit message.
- No `unsafe` transitive dependencies without review.
