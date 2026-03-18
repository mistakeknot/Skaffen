#!/usr/bin/env bash
set -euo pipefail
# os/Skaffen/interlab.sh — wraps Skaffen Go benchmarks for interlab consumption.
# Primary metric: sandbox_check_ns (BenchmarkCheckPathAllowed)
# Secondary: skill_parse_ns, sandbox_merge_ns

MONOREPO="$(cd "$(dirname "$0")/../.." && pwd)"
HARNESS="${INTERLAB_HARNESS:-$MONOREPO/interverse/interlab/scripts/go-bench-harness.sh}"
DIR="$(cd "$(dirname "$0")" && pwd)"

echo "--- sandbox ---" >&2
bash "$HARNESS" --pkg ./internal/sandbox/ --bench 'BenchmarkCheckPathAllowed$' --metric sandbox_check_ns --dir "$DIR"

echo "--- skill parse ---" >&2
bash "$HARNESS" --pkg ./internal/skill/ --bench 'BenchmarkParseFrontmatter$' --metric skill_parse_ns --dir "$DIR"

echo "--- sandbox merge ---" >&2
bash "$HARNESS" --pkg ./internal/sandbox/ --bench 'BenchmarkMergePolicy$' --metric sandbox_merge_ns --dir "$DIR"
