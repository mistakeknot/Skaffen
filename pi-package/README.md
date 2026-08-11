# @mistakeknot/pi-skaffen

Pi-hosted Skaffen policy adapter. The first slice observes Intercore without modifying it and exposes harness health, situation, and OODARC status inside Pi.

## Commands and tools

- `/harness` — refresh health and show the current project run.
- `/situation` — refresh the unified Intercore snapshot.
- `observe_situation` — equivalent read-only agent tool.

The footer shows the current lifecycle phase and predominant OODARC role when the active run belongs to the working directory. Unrelated active runs contribute only to the global count.

## Development

```bash
npm install
npm run check
pi --no-extensions -e . --version
```

## Safety boundary

This package does not initialize, migrate, or write to Intercore. A missing `ic` binary, timeout, malformed output, or schema mismatch produces visible degraded state while Pi continues to work.
