---
bead: sylveste-8ve
type: brainstorm
date: 2026-03-28
---

# Confidence Cascade Integration

## Problem
When FallbackProvider falls back from local to cloud, we lose the cascade metadata
(confidence score, models tried, escalation count, probe time). This data is needed
for Track B5 shadow→enforce transition — Interspect needs evidence about when and
why local inference was insufficient.

## Design

### Approach: Structured error with metadata
Extend `ErrCloudFallback` into a `CascadeError` type that carries structured metadata.
FallbackProvider extracts this metadata and emits evidence before falling back.

```go
type CascadeError struct {
    Decision   string   // "cloud", "escalate"
    Confidence float64  // avg confidence from probe
    ModelsTried []string
    ProbeTimeS float64
}
```

### Evidence emission
FallbackProvider accepts an optional `CascadeObserver` callback:
```go
type CascadeObserver func(CascadeEvent)

type CascadeEvent struct {
    Decision    string
    Confidence  float64
    ModelsTried []string
    ProbeTimeS  float64
    FallbackTo  string // "anthropic", "claude-code"
    Complexity  int    // estimated tier
}
```

This decouples evidence emission from the provider — the observer can write to
Intercore, JSONL, or both.

### Router re-selection on fallback
When falling back to cloud, the FallbackProvider currently passes through the
original `config.Model`. For cloud, this should be re-selected by the router
based on complexity. Options:
- **A) Router callback**: FallbackProvider holds a `SelectCloudModel func() string`
- **B) Hardcode sonnet**: Simple, good enough for now
- **C) Pass through**: Let the cloud provider use its own default

**Decision: Option A** — router callback is clean and testable.

## Rejected
- Separate channel (context.Value): too implicit, hard to test
- Middleware pattern: overengineered for one integration point
