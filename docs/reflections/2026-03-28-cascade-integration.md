---
bead: sylveste-8ve
type: reflection
date: 2026-03-28
---

# Reflection: Confidence Cascade Integration

## What went well
- Structured error type (`CascadeError` with `Unwrap()`) preserves `errors.Is` compatibility while carrying metadata — this pattern is reusable across providers.
- Callback-based observation (`OnCascade`) keeps the provider package decoupled from evidence/logging infrastructure.
- The `SelectCloudModel` callback cleanly separates routing policy from provider mechanics.

## What to improve
- The `estimateComplexity` function duplicates the router's `ComplexityClassifier.Classify` thresholds. If thresholds change, they'll drift. Consider extracting shared constants or having the FallbackProvider accept a `Classifier` interface.
- `OnCascade` is fire-and-forget (synchronous). If evidence emission becomes slow (e.g., network call to Intercore), it should be made async with a buffered channel.
- The main.go wiring currently logs to stderr only. JSONL file emission (for Interspect) is deferred to sylveste-fnx.

## Decisions worth remembering
- Chose callback over interface for observation — simpler for the 1-2 consumers we have, avoidable abstraction.
- Chose to estimate complexity from byte count in the provider rather than threading the router's classification through — keeps provider self-contained at the cost of slight threshold drift risk.
