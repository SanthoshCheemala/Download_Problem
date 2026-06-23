#!/usr/bin/env python3
"""
Run all tasks in grid_feasible.csv in parallel with proper port isolation.
No run_grid_task.sh — assigns unique port slots via Python queue (no NFS issues).

Usage:
    python3 scripts/run_grid_multiseed.py [grid_csv] [out_dir] [n_concurrent]

Defaults:
    grid_csv     = benchmarks/results/10_drip_grid_multiseed/grid_feasible.csv
    out_dir      = benchmarks/results/10_drip_grid_multiseed/tasks
    n_concurrent = 40
"""

import csv, os, queue, subprocess, sys, threading

GRID_CSV     = sys.argv[1] if len(sys.argv) > 1 else "benchmarks/results/10_drip_grid_multiseed/grid_feasible.csv"
OUT_DIR      = sys.argv[2] if len(sys.argv) > 2 else "benchmarks/results/10_drip_grid_multiseed/tasks"
N_CONCURRENT = int(sys.argv[3]) if len(sys.argv) > 3 else 40
BINARY       = os.environ.get("BINARY", "./bin/download-sim")
PORT_BASE    = 20000
PORT_STRIDE  = 200   # each slot: src_port + 100 agents = 101 ports; 200 gives headroom

os.makedirs(OUT_DIR, exist_ok=True)

if not os.path.isfile(GRID_CSV):
    print(f"ERROR: grid CSV not found: {GRID_CSV}", flush=True)
    sys.exit(1)

if not os.access(BINARY, os.X_OK):
    print(f"ERROR: binary not found or not executable: {BINARY}", flush=True)
    sys.exit(1)

# Semaphore-style port slot pool
slot_pool = queue.Queue()
for s in range(N_CONCURRENT):
    slot_pool.put(s)

lock = threading.Lock()
done_count = [0]

def run_task(i, row):
    slot = slot_pool.get()
    try:
        src_port   = PORT_BASE + slot * PORT_STRIDE
        agent_base = src_port + 1

        k        = row["k_agents"]
        n        = row["n_bits"]
        beta     = row["beta"]
        rhodiv   = row["rho_divisor"]
        lncoeff  = row["p_ln_coeff"]
        rhocoeff = row["p_rho_coeff"]
        adv      = row["adversary"]
        seed     = row["seed"]

        csv_out = f"{OUT_DIR}/task_{i}.csv"
        log_out = f"{OUT_DIR}/task_{i}.log"

        cmd = [
            BINARY,
            f"-agents={k}",
            f"-bits={n}",
            f"-ratios={beta}",
            f"-rho-divisor={rhodiv}",
            f"-p-ln-coeff={lncoeff}",
            f"-p-rho-coeff={rhocoeff}",
            f"-adversary={adv}",
            f"-seed={seed}",
            f"-source-port={src_port}",
            f"-base-port={agent_base}",
            f"-csv={csv_out}",
        ]

        with lock:
            print(f"[start] task {i:3d} k={k} n={n} beta={beta} seed={seed} slot={slot}", flush=True)

        with open(log_out, "w") as lf:
            result = subprocess.run(cmd, stdout=lf, stderr=lf)

        with lock:
            done_count[0] += 1
            status = "ok" if result.returncode == 0 else f"EXIT {result.returncode}"
            print(f"[done ] task {i:3d} {status}  ({done_count[0]}/{total})", flush=True)

    finally:
        slot_pool.put(slot)

# Load grid
rows = []
with open(GRID_CSV) as f:
    for r in csv.DictReader(f):
        rows.append(r)

total = len(rows)
print(f"Launching {total} tasks, {N_CONCURRENT} concurrent, ports {PORT_BASE}+", flush=True)

threads = [threading.Thread(target=run_task, args=(i, row), daemon=True)
           for i, row in enumerate(rows)]

for t in threads:
    t.start()

for t in threads:
    t.join()

print(f"\nALL DONE — results in {OUT_DIR}", flush=True)
