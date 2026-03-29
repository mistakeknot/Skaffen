---
bead: sylveste-8ve
type: plan
date: 2026-03-28
complexity: C3
---

# Plan: Confidence Cascade Integration

## Summary
Wire interfere's cascade metadata into Skaffen's FallbackProvider so that
confidence scores, models tried, and fallback decisions are captured as
structured events for Interspect evidence.

## Tasks

### Task 1: CascadeError type with structured metadata
**File:** `internal/provider/local/local.go`

Replace the flat `ErrCloudFallback` wrapping in `parseCascadeFallback` with a
`CascadeError` struct that implements `error` and carries:
- Decision (cloud/escalate)
- Confidence (float64)
- ModelsTried ([]string)
- ProbeTimeS (float64)

Make `errors.Is(err, ErrCloudFallback)` still work via `Unwrap()`.

### Task 2: CascadeObserver callback in FallbackConfig
**File:** `internal/provider/local/fallback.go`

Add to `FallbackConfig`:
```go
OnCascade       func(CascadeEvent) // called on every fallback decision
SelectCloudModel func(int) string  // complexity tier → cloud model
```

`CascadeEvent` struct holds the cascade metadata plus fallback target info.

In `Stream()`, when falling back:
1. Extract `CascadeError` metadata (if available)
2. Call `OnCascade` with the event
3. If `SelectCloudModel` is set, override `config.Model` for the cloud call

### Task 3: Wire callbacks in main.go
**File:** `cmd/skaffen/main.go`

In `resolveProvider`, pass:
- `OnCascade`: log to stderr + write JSONL to `~/.skaffen/cascade-events.jsonl`
- `SelectCloudModel`: call router's complexity→model mapping

### Task 4: Tests
**Files:** `internal/provider/local/local_test.go`, `fallback_test.go`

- Test CascadeError wraps ErrCloudFallback
- Test FallbackProvider calls OnCascade on fallback
- Test SelectCloudModel overrides model for cloud call
- Test OnCascade not called when local succeeds

## Ordering
Tasks 1-2 are independent. Task 3 depends on both. Task 4 alongside each.
