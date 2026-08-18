# @mistakeknot/pi-skaffen

Pi-hosted Skaffen policy adapter. It observes Intercore without modifying it and exposes harness health, build provenance, situation ownership, and OODARC status inside Pi.

## Commands and tools

- `/harness` — refresh health and show the current project run.
- `/situation` — refresh the unified Intercore snapshot.
- `observe_situation` — equivalent read-only agent tool.

The footer shows the current lifecycle phase and predominant OODARC role when the active run belongs to the working directory. Unrelated active runs contribute only to the global count. Detail views validate and report the installed Intercore version, schema, commit/source/dirty provenance, optional run producers, and current agency lifecycle summaries. Legacy snapshots without these optional ownership fields remain valid.

## Development

```bash
npm install
npm run check
pi --no-extensions -e . --version
```

## Safety boundary

This package does not initialize, migrate, or write to Intercore. Its only subprocess calls are bounded, concurrent reads: `ic health --json`, `ic version --json`, and `ic situation snapshot --json`. A missing binary, timeout, malformed output, or schema mismatch produces visible degraded state while Pi continues to work. Build provenance is classified independently as verified, unverified, or unavailable, so a readable database never disguises an unknown or dirty runtime.
