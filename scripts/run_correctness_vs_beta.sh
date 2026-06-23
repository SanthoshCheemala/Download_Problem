#!/usr/bin/env bash
# Correctness vs beta sweep — paper constants, 100 seeds each
# k=120, n=64, beta=0.05..0.90, adversary=flood
# Output: benchmarks/results/09_correctness_vs_beta/results.csv
set -euo pipefail

BINARY="${BINARY:-./bin/download-sim}"
OUT_DIR="benchmarks/results/09_correctness_vs_beta"
OUT="$OUT_DIR/results.csv"

mkdir -p "$OUT_DIR"

BETAS=(0.05 0.10 0.20 0.30 0.40 0.50 0.60 0.70 0.80 0.90)
K=120
N=64

total=$(( ${#BETAS[@]} * 100 ))
i=0

for BETA in "${BETAS[@]}"; do
  for SEED in $(seq 0 99); do
    i=$((i+1))
    printf "[%4d/%d] beta=%-5s seed=%-3d ... " "$i" "$total" "$BETA" "$SEED"
    "$BINARY" \
      -agents=$K -bits=$N -ratios="$BETA" \
      -rho-divisor=8 -p-ln-coeff=6 -p-rho-coeff=4 \
      -adversary=flood -seed=$SEED \
      -csv="$OUT" >/dev/null 2>&1
    echo "done"
  done
  echo "=== beta=$BETA complete ==="
done

echo ""
echo "SWEEP DONE → $OUT  ($(( $(wc -l < "$OUT") - 1 )) rows)"
