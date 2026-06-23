#!/usr/bin/env bash
# Parameter relaxation — drip adversary, 50 seeds each
# k=120, n=64, beta=0.5, paper constants axis only
# Ports 12500-12620 (separate from beta sweep at 11000)
# Output: benchmarks/results/10_drip/relaxation.csv
set -euo pipefail

BINARY="${BINARY:-./bin/download-sim}"
OUT="benchmarks/results/10_drip/relaxation.csv"
mkdir -p benchmarks/results/10_drip

K=120; N=64; BETA=0.5; SEEDS=50

# rho_divisor  p_ln_coeff  p_rho_coeff
CONFIGS=(
  "8   6.000  4.000"   # paper
  "8   3.000  2.000"   # halve 1x
  "8   1.500  1.000"   # halve 2x
  "8   0.750  0.500"   # halve 3x
  "8   0.375  0.250"   # halve 4x
)

if [[ ! -x "$BINARY" ]]; then echo "binary not found" >&2; exit 1; fi

total=$(( ${#CONFIGS[@]} * SEEDS ))
i=0

for cfg in "${CONFIGS[@]}"; do
  read -r DIV LN RHO <<< "$cfg"
  for SEED in $(seq 0 $(( SEEDS - 1 ))); do
    i=$((i+1))
    printf "[%4d/%d] ln=%-6s rho=%-5s seed=%-3d ... " "$i" "$total" "$LN" "$RHO" "$SEED"
    "$BINARY" \
      -agents=$K -bits=$N -ratios="$BETA" \
      -rho-divisor="$DIV" -p-ln-coeff="$LN" -p-rho-coeff="$RHO" \
      -adversary=drip -seed=$SEED \
      -source-port=12500 -base-port=12501 \
      -csv="$OUT" >/dev/null 2>&1
    echo "done"
  done
  echo "=== ln=$LN rho=$RHO complete ==="
done

echo ""
echo "DONE → $OUT  ($(( $(wc -l < "$OUT") - 1 )) rows)"
