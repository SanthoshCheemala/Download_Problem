#!/usr/bin/env bash
# Exp 2 gap: correctness vs beta at larger n values
# k=120, n=128/256/512, beta=0.05..0.90, 30 seeds each = 900 runs
# Ports 14000-14001 (separate from other scripts)
# Output: benchmarks/results/10_drip/beta_sweep_largern.csv
set -euo pipefail

BINARY="${BINARY:-./bin/download-sim}"
OUT="benchmarks/results/10_drip/beta_sweep_largern.csv"
mkdir -p benchmarks/results/10_drip

K=120; SEEDS=30

NS=(128 256 512)
BETAS=(0.05 0.10 0.20 0.30 0.40 0.50 0.60 0.70 0.80 0.90)

if [[ ! -x "$BINARY" ]]; then echo "binary not found: $BINARY" >&2; exit 1; fi

total=$(( ${#NS[@]} * ${#BETAS[@]} * SEEDS ))
i=0

for N in "${NS[@]}"; do
  for BETA in "${BETAS[@]}"; do
    for SEED in $(seq 0 $(( SEEDS - 1 ))); do
      i=$((i+1))
      printf "[%4d/%d] n=%-4d beta=%-5s seed=%-3d ... " "$i" "$total" "$N" "$BETA" "$SEED"
      "$BINARY" \
        -agents=$K -bits=$N -ratios="$BETA" \
        -rho-divisor=8 -p-ln-coeff=6 -p-rho-coeff=4 \
        -adversary=drip -seed=$SEED \
        -source-port=14000 -base-port=14001 \
        -csv="$OUT" >/dev/null 2>&1
      echo "done"
    done
    echo "=== n=$N beta=$BETA complete ==="
  done
  echo "=== n=$N fully done ==="
done

echo ""
echo "DONE → $OUT  ($(( $(wc -l < "$OUT") - 1 )) rows)"
