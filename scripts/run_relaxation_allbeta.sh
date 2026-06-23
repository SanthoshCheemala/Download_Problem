#!/usr/bin/env bash
# Exp 3 gap: relaxation across all beta values
# k=120, n=64, beta=0.1/0.3/0.5/0.7/0.9, 5 configs x 50 seeds each = 1250 runs
# Ports 13000-13001 (separate from other scripts)
# Output: benchmarks/results/10_drip/relaxation_allbeta.csv
set -euo pipefail

BINARY="${BINARY:-./bin/download-sim}"
OUT="benchmarks/results/10_drip/relaxation_allbeta.csv"
mkdir -p benchmarks/results/10_drip

K=120; N=64; SEEDS=50

BETAS=(0.1 0.3 0.5 0.7 0.9)

CONFIGS=(
  "8 6.000 4.000"
  "8 3.000 2.000"
  "8 1.500 1.000"
  "8 0.750 0.500"
  "8 0.375 0.250"
)

CONFIG_LABELS=("Paper(6,4)" "Halve1(3,2)" "Halve2(1.5,1)" "Halve3(0.75,0.5)" "Halve4(0.375,0.25)")

if [[ ! -x "$BINARY" ]]; then echo "binary not found: $BINARY" >&2; exit 1; fi

total=$(( ${#BETAS[@]} * ${#CONFIGS[@]} * SEEDS ))
i=0

for BETA in "${BETAS[@]}"; do
  for idx in "${!CONFIGS[@]}"; do
    cfg="${CONFIGS[$idx]}"
    label="${CONFIG_LABELS[$idx]}"
    read -r DIV LN RHO <<< "$cfg"
    for SEED in $(seq 0 $(( SEEDS - 1 ))); do
      i=$((i+1))
      printf "[%4d/%d] beta=%-4s %s seed=%-3d ... " "$i" "$total" "$BETA" "$label" "$SEED"
      "$BINARY" \
        -agents=$K -bits=$N -ratios="$BETA" \
        -rho-divisor="$DIV" -p-ln-coeff="$LN" -p-rho-coeff="$RHO" \
        -adversary=drip -seed=$SEED \
        -source-port=13000 -base-port=13001 \
        -csv="$OUT" >/dev/null 2>&1
      echo "done"
    done
    echo "=== beta=$BETA $label complete ==="
  done
done

echo ""
echo "DONE → $OUT  ($(( $(wc -l < "$OUT") - 1 )) rows)"
