# @mistakeknot/pi-skaffen

Pi-hosted Skaffen policy adapter. The first slice observes Intercore without modifying it and exposes harness health, situation, and OODARC status inside Pi.

## Development

```bash
npm install
npm run check
pi --no-extensions -e . --version
```

## Safety boundary

This package does not initialize, migrate, or write to Intercore. A missing `ic` binary, timeout, malformed output, or schema mismatch produces visible degraded state while Pi continues to work.
