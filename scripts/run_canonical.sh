#!/usr/bin/env bash
# Canonical benchmark for Algorithm 2 (Blacklist_Download), DISC 2024.
# ONE fixed script, no knobs.
#
#   k=100  n=256  beta=1/3,1/2  adversary=flood  seed=42
#   Each bit is decided by a private rho-representative committee.
#   Conflicts are resolved at the source and liars are blacklisted.
#   Expected: 100% success, Q well below naive n.
#
# Output: benchmarks/results/00_canonical/{results.csv,run.log}
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/benchmarks/results/00_canonical"
mkdir -p "$OUT"

ulimit -n 8192 || true
go build -o "$OUT/download-bench" "$ROOT"

echo "================ Algorithm 2 (Blacklist_Download) ================"
"$OUT/download-bench" -adversary=flood -agents=100 -bits=256 \
  -ratios=1/3,1/2 -parallelism=24 -seed=42 \
  -csv="$OUT/results.csv" | tee "$OUT/run.log"

echo
echo "csv: $OUT/results.csv"
