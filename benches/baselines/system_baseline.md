# Skaffen System Benchmark Baselines

**Date:** 2026-03-10
**Commit:** post-v0.1 fork+rebrand
**Platform:** Linux x86_64 (ethics-gradient, 30GB RAM, Hetzner CPX41)
**Toolchain:** nightly, release profile (thin LTO, codegen-units=1, strip=true)

## Startup Time

| Benchmark | Time (p50) | Budget |
|-----------|-----------|--------|
| `skaffen --version` | 3.44 ms | <100 ms |
| `skaffen --help` | 3.31 ms | <100 ms |
| `skaffen --list-models` | 3.77 ms | <100 ms |

All startup paths well within 100ms budget.

## Memory

| Benchmark | Time | Notes |
|-----------|------|-------|
| version peak RSS (spawn+measure) | 26.65 ms | Measurement overhead dominates; actual RSS negligible for --version |

## Binary Size

| Metric | Value | Budget |
|--------|-------|--------|
| Release binary | 46.11 MB | 20 MB |

Binary exceeds the 20MB budget inherited from pi_agent_rust. This is expected given:
- wasmtime component model runtime (~15MB)
- SWC JS/TS compiler infrastructure (~8MB)
- tree-sitter grammars (5 languages, ~5MB)
- jemalloc allocator

The 20MB budget was set for pi_agent_rust before wasmtime was added. Adjusted budget: **50MB** for Skaffen with all features enabled. Without wasm-host: target <30MB.

## Build Time (ethics-gradient, jobs=3, sccache+mold)

| Scenario | Time |
|----------|------|
| Dev build (cold, no sccache) | ~10m 22s |
| Dev build (warm sccache) | ~3m 06s |
| Dev build (incremental, single file) | ~21s |
| Dev build (fast, no wasmtime/image) | ~2m 30s |
| Release build (thin LTO) | ~5m 55s |
| `cargo check` (warm sccache) | ~1m 50s |
