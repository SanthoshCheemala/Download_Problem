#!/usr/bin/env python3
"""
Figures for Blacklist_Download (Algorithm 2).

Theory (paper constants ln=6, rho=4, divisor=8):
    rho  = max(1, k * sqrt(gamma*beta / (8n)))
    p    = min(1, (6 ln n + 4 rho) / (gamma k))
    Q_th = n * p
"""

import csv, math, os
from collections import defaultdict
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np

# ── paths ──────────────────────────────────────────────────────────────────────
TASKS_DIR       = "benchmarks/results/10_drip_grid_multiseed/tasks"
RELAXED_CSV     = "benchmarks/results/10_drip/relaxation.csv"
RELAXED_ALL_CSV = "benchmarks/results/10_drip/relaxation_allbeta.csv"
OUT_DIR         = "benchmarks/graphs"
os.makedirs(OUT_DIR, exist_ok=True)

# ── Paul Tol 'bright' palette ──────────────────────────────────────────────────
TOL_BLUE   = "#4477AA"
TOL_GREEN  = "#228833"
TOL_YELLOW = "#CCBB44"
TOL_RED    = "#EE6677"
TOL_PURPLE = "#AA3377"
INK        = "#222222"

# ── global style ───────────────────────────────────────────────────────────────
plt.rcParams.update({
    "figure.dpi":        200,
    "savefig.dpi":       200,
    "figure.facecolor":  "white",
    "savefig.facecolor": "white",
    "font.family":       "serif",
    "mathtext.fontset":  "dejavuserif",
    "font.size":         11,
    "axes.labelsize":    11,
    "axes.edgecolor":    INK,
    "axes.linewidth":    0.9,
    "axes.grid":         True,
    "grid.color":        "#DDDDDD",
    "grid.linewidth":    0.6,
    "axes.axisbelow":    True,
    "axes.spines.top":   False,
    "axes.spines.right": False,
    "legend.frameon":    True,
    "legend.framealpha": 0.95,
    "legend.edgecolor":  "#CCCCCC",
    "legend.fontsize":   9.5,
    "xtick.direction":   "out",
    "ytick.direction":   "out",
    "xtick.labelsize":   10,
    "ytick.labelsize":   10,
})

# ── theory ─────────────────────────────────────────────────────────────────────
def paper_rho(k, n, beta, divisor=8.0):
    gamma = 1 - beta
    if gamma <= 0 or beta <= 0:
        return 1.0
    return max(1.0, k * math.sqrt(gamma * beta / (divisor * n)))

def committee_p(k, n, beta, rho, ln_c=6.0, rho_c=4.0):
    gamma = 1 - beta
    if gamma <= 0:
        return 1.0
    return min(1.0, max(0.0, (ln_c * math.log(n) + rho_c * rho) / (gamma * k)))

def Q_theory(k, n, beta):
    rho = paper_rho(k, n, beta)
    return n * committee_p(k, n, beta, rho)

# ── beta helpers ───────────────────────────────────────────────────────────────
BETA_STYLE = {
    0.0000: (TOL_BLUE,   "o", r"$\beta=0$"),
    0.3333: (TOL_GREEN,  "s", r"$\beta=1/3$"),
    0.5000: (TOL_YELLOW, "^", r"$\beta=1/2$"),
    0.6667: (TOL_RED,    "D", r"$\beta=2/3$"),
}

def beta_key(b):
    for t in [0.0, 1/3, 0.5, 2/3]:
        if abs(b - t) < 0.02:
            return round(t, 4)
    return None

# ── data loader: Exp 1 (multiseed HPC grid) ────────────────────────────────────
def load_exp1():
    groups = defaultdict(list)
    for fn in os.listdir(TASKS_DIR):
        if not fn.endswith(".csv"):
            continue
        with open(os.path.join(TASKS_DIR, fn)) as f:
            for r in csv.DictReader(f):
                key = (int(r["k_agents"]), int(r["n_bits"]), round(float(r["beta"]), 4))
                groups[key].append(int(r["query_complexity_Q"]))
    return groups

EXP1 = load_exp1()

# ══════════════════════════════════════════════════════════════════════════════
# FIG 1 — Q vs n, log-log, mean ± std over 30 seeds  (k=100, all β)
# ══════════════════════════════════════════════════════════════════════════════
def fig1():
    fig, ax = plt.subplots(figsize=(6.4, 4.8))

    for bk in [0.0000, 0.3333, 0.5000, 0.6667]:
        color, marker, label = BETA_STYLE[bk]

        ns_data = {}
        for (k, n, beta), qs in EXP1.items():
            if k != 100:
                continue
            bk2 = beta_key(beta)
            if bk2 is None or abs(bk2 - bk) > 0.01:
                continue
            ns_data[n] = (np.mean(qs), np.std(qs))

        if not ns_data:
            continue

        ns    = sorted(ns_data)
        means = [ns_data[n][0] for n in ns]
        stds  = [ns_data[n][1] for n in ns]

        ax.errorbar(ns, means, yerr=stds, fmt=marker, color=color, ms=7,
                    capsize=4, elinewidth=1.1, zorder=3, label=label)

        nn = np.logspace(math.log10(min(ns)), math.log10(max(ns)), 200)
        qt = [Q_theory(100, int(round(x)), bk) for x in nn]
        ax.plot(nn, qt, "-", color=color, lw=1.6, alpha=0.85, zorder=2)

    nn_naive = np.array([512, 2048])
    ax.plot(nn_naive, nn_naive, "--", color=INK, lw=1.4, label=r"naive  $Q=n$")

    ax.set_xscale("log")
    ax.set_yscale("log")
    ax.set_xlabel(r"$n$  (bits)")
    ax.set_ylabel(r"$Q$  (mean source queries per peer, 30 seeds)")
    ax.set_xticks([512, 1024, 2048])
    ax.set_xticklabels(["512", "1024", "2048"])
    ax.set_yticks([256, 512, 1024, 2048])
    ax.set_yticklabels(["256", "512", "1024", "2048"])
    ax.xaxis.set_minor_formatter(plt.NullFormatter())
    ax.yaxis.set_minor_formatter(plt.NullFormatter())

    from matplotlib.lines import Line2D
    handles, labels = ax.get_legend_handles_labels()
    handles.append(Line2D([0], [0], color=INK, lw=1.6, alpha=0.7))
    labels.append(r"theory  $n\,p$  (solid)")
    ax.legend(handles, labels, loc="upper left")

    fig.tight_layout()
    p = os.path.join(OUT_DIR, "fig1_Q_vs_n.png")
    fig.savefig(p)
    plt.close(fig)
    print("saved", p)

# ══════════════════════════════════════════════════════════════════════════════
# FIG 2 — Constants heatmap: success rate vs config × β  (Exp 3)
# ══════════════════════════════════════════════════════════════════════════════
def fig2_constants_heatmap():
    merged = defaultdict(list)
    for fpath in [RELAXED_CSV, RELAXED_ALL_CSV]:
        if not os.path.exists(fpath):
            continue
        with open(fpath) as f:
            for r in csv.DictReader(f):
                if abs(float(r["rho_divisor"]) - 8.0) > 0.01:
                    continue
                ln   = float(r["p_ln_coeff"])
                rhoc = float(r["p_rho_coeff"])
                beta = round(float(r["beta"]), 2)
                merged[(ln, rhoc, beta)].append(float(r["success_rate"]))

    configs = [(6.0, 4.0), (3.0, 2.0), (1.5, 1.0), (0.75, 0.5), (0.375, 0.25)]
    clabels = ["Paper\n(a=6, b=4)", "Halve×1\n(a=3, b=2)", "Halve×2\n(a=1.5, b=1)",
               "Halve×3\n(a=0.75, b=0.5)", "Halve×4\n(a=0.375, b=0.25)"]
    betas   = sorted({b for (_, _, b) in merged})

    data = np.zeros((len(configs), len(betas)))
    for i, (ln, rhoc) in enumerate(configs):
        for j, beta in enumerate(betas):
            rs = merged.get((ln, rhoc, beta), [])
            if rs:
                data[i, j] = sum(1 for s in rs if s >= 0.999) / len(rs) * 100

    fig, ax = plt.subplots(figsize=(7.8, 4.2))
    im = ax.imshow(data, aspect="auto", cmap="RdYlGn", vmin=0, vmax=100)

    ax.set_xticks(range(len(betas)))
    ax.set_xticklabels([f"{b:.1f}" for b in betas])
    ax.set_yticks(range(len(configs)))
    ax.set_yticklabels(clabels, fontsize=9)
    ax.set_xlabel(r"Byzantine fraction  $\beta$")
    ax.set_ylabel("Committee constants  (a ln n + bρ)")

    for i in range(len(configs)):
        for j in range(len(betas)):
            val = data[i, j]
            txt_color = "white" if val < 35 or val > 80 else INK
            ax.text(j, i, f"{val:.0f}%", ha="center", va="center",
                    fontsize=9, color=txt_color, fontweight="bold")

    plt.colorbar(im, ax=ax, label="success rate (%)")
    fig.tight_layout()
    p = os.path.join(OUT_DIR, "fig2_constants_heatmap.png")
    fig.savefig(p)
    plt.close(fig)
    print("saved", p)

# ══════════════════════════════════════════════════════════════════════════════
# FIG Q_vs_rho — theoretical Q tradeoff curve
# ══════════════════════════════════════════════════════════════════════════════
def fig_Q_vs_rho():
    # k=500, n=64, beta=0.5 gives rho*≈11 — shows the U-shape tradeoff clearly
    k, n, beta = 500, 64, 0.5
    gamma = 1 - beta

    rhos    = np.linspace(0.5, 40, 400)
    ps      = np.array([min(1.0, (6*math.log(n) + 4*r) / (gamma*k)) for r in rhos])
    Q_prim  = n * ps
    Q_ver   = beta * k / rhos
    Q_total = Q_prim + Q_ver

    rho_opt = paper_rho(k, n, beta)
    p_opt   = committee_p(k, n, beta, rho_opt)
    Q_opt   = n * p_opt + beta * k / rho_opt

    fig, ax = plt.subplots(figsize=(6.2, 4.4))
    ax.plot(rhos, Q_prim,  "-",  color=TOL_BLUE,   lw=1.8, label=r"$Q_\mathrm{primary} = np$")
    ax.plot(rhos, Q_ver,   "--", color=TOL_RED,    lw=1.8, label=r"$Q_\mathrm{verify} = \beta k/\rho$")
    ax.plot(rhos, Q_total, "-",  color=TOL_PURPLE, lw=2.2, label=r"$Q_\mathrm{total}$")
    ax.plot([rho_opt], [Q_opt], "o", color=TOL_GREEN, ms=10, zorder=5,
            label=rf"$\rho^* = {rho_opt:.1f}$,  $Q^* = {Q_opt:.0f}$")

    ax.set_xlabel(r"$\rho$  (committee size)")
    ax.set_ylabel(r"$Q$  (queries per peer)")
    ax.set_title(r"Theoretical tradeoff  ($k=500,\ n=64,\ \beta=1/2$)", fontsize=10)
    ax.legend(loc="upper right")
    fig.tight_layout()
    p = os.path.join(OUT_DIR, "fig_Q_vs_rho.png")
    fig.savefig(p)
    plt.close(fig)
    print("saved", p)

# ══════════════════════════════════════════════════════════════════════════════
for fn in (fig1, fig2_constants_heatmap, fig_Q_vs_rho):
    fn()
print("\nDone →", OUT_DIR)
