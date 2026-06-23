#!/usr/bin/env bash
# Correctness vs beta — drip adversary, 30 seeds each
# k=120, n=64, beta=0.05..0.90
# Ports 11000-11120 (separate from relaxation script at 12500+)
# Output: benchmarks/results/10_drip/beta_sweep.csv
set -euo pipefail

BINARY="${BINARY:-./bin/download-sim}"
OUT="benchmarks/results/10_drip/beta_sweep.csv"
mkdir -p benchmarks/results/10_drip

BETAS=(0.05 0.10 0.20 0.30 0.40 0.50 0.60 0.70 0.80 0.90)
K=120; N=64; SEEDS=30

if [[ ! -x "$BINARY" ]]; then echo "binary not found" >&2; exit 1; fi

total=$(( ${#BETAS[@]} * SEEDS ))
i=0

for BETA in "${BETAS[@]}"; do
  for SEED in $(seq 0 $(( SEEDS - 1 ))); do
    i=$((i+1))
    printf "[%4d/%d] beta=%-5s seed=%-3d ... " "$i" "$total" "$BETA" "$SEED"
    "$BINARY" \
      -agents=$K -bits=$N -ratios="$BETA" \
      -rho-divisor=8 -p-ln-coeff=6 -p-rho-coeff=4 \
      -adversary=drip -seed=$SEED \
      -source-port=11000 -base-port=11001 \
      -csv="$OUT" >/dev/null 2>&1
    echo "done"
  done
  echo "=== beta=$BETA complete ==="
done

echo ""
echo "DONE → $OUT  ($(( $(wc -l < "$OUT") - 1 )) rows)"
