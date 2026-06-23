// gridgen generates an experiment grid for Algorithm 2, predicts each config's
// wall-clock cost, and writes feasible/dropped CSVs that run_grid_task.sh runs.
//
// By default it crosses -agents x -bits x -ratios with the constant lists
// (-rho-divisor x -p-ln-coeff x -p-rho-coeff). For a targeted study you can
// pin the exact (k,n) configs with -configs and the exact constant tuples with
// -settings, and repeat every row across -seeds. Predictions are rough --
// calibrate -ms-per-message from a real probe run first.
package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"Download_Problem/agent"
	"Download_Problem/sim"
)

type gridConfig struct {
	k, n                            int
	beta                            float64
	rhoDivisor, pLnCoeff, pRhoCoeff float64
	seed                            int64
}

type prediction struct {
	cfg              gridConfig
	rho, p           float64
	predictedM       float64
	predictedSeconds float64
}

type knPair struct{ k, n int }

type constSetting struct{ div, ln, rho float64 }

func main() {
	var bitsPow2, bitsRaw, agentsSpec, ratiosSpec, rhoDivSpec, lnCoeffSpec, rhoCoeffSpec string
	var configsSpec, settingsSpec, seedsSpec string
	var maxSeconds, msPerMessage, safetyFactor float64
	var adversary string
	var outDir string

	flag.StringVar(&bitsPow2, "bits-pow2", "9,10,12,16,22", "comma list of exponents; each x becomes n=2^x")
	flag.StringVar(&bitsRaw, "bits", "", "comma list of raw n values, combined with -bits-pow2")
	flag.StringVar(&agentsSpec, "agents", "25,50,100,200,400,700,1000", "comma list of k values")
	flag.StringVar(&ratiosSpec, "ratios", "0,1/3,1/2,2/3,0.99", "comma list of beta values (fractions allowed)")
	flag.StringVar(&rhoDivSpec, "rho-divisor", "8", "comma list of rho-divisor values to sweep (paper=8)")
	flag.StringVar(&lnCoeffSpec, "p-ln-coeff", "6", "comma list of p's ln(n) coefficient to sweep (paper=6)")
	flag.StringVar(&rhoCoeffSpec, "p-rho-coeff", "4", "comma list of p's rho coefficient to sweep (paper=4)")
	flag.StringVar(&configsSpec, "configs", "", "explicit k:n pairs (e.g. 200:512,700:512); overrides -agents x -bits")
	flag.StringVar(&settingsSpec, "settings", "", "explicit div:ln:rho constant tuples; overrides the rho-divisor/ln/rho cross product")
	flag.StringVar(&seedsSpec, "seeds", "42", "comma list of seeds; every config is repeated once per seed")
	flag.Float64Var(&maxSeconds, "max-seconds", 86400, "drop any config predicted to exceed this wall-clock budget (default 24h)")
	flag.Float64Var(&msPerMessage, "ms-per-message", 0.012, "calibration: ms of wall-clock per broadcast message, see file header")
	flag.Float64Var(&safetyFactor, "safety-factor", 4.0, "multiplies the raw estimate to cover extrapolation uncertainty")
	flag.StringVar(&adversary, "adversary", "flood", "adversary passed through to every generated task")
	flag.StringVar(&outDir, "out-dir", "benchmarks/results/06_hpc_grid", "directory to write grid_feasible.csv and grid_dropped.csv")
	flag.Parse()

	betas, err := sim.ParseRatios(ratiosSpec)
	must(err)
	seeds, err := parseSeeds(seedsSpec)
	must(err)

	configs, err := resolveConfigs(configsSpec, agentsSpec, bitsPow2, bitsRaw)
	must(err)
	settings, err := resolveSettings(settingsSpec, rhoDivSpec, lnCoeffSpec, rhoCoeffSpec)
	must(err)

	var preds []prediction
	for _, c := range configs {
		k, n := c.k, c.n
		for _, beta := range betas {
			byzantineCount := int(math.Floor(beta * float64(k)))
			if byzantineCount >= k {
				byzantineCount = k - 1
			}
			honestCount := k - byzantineCount
			actualBeta := float64(byzantineCount) / float64(k)
			gamma := float64(honestCount) / float64(k)

			for _, s := range settings {
				rho := sim.PaperRho(k, n, actualBeta, s.div)
				p := agent.CommitteeProbability(n, k, gamma, rho, s.ln, s.rho)

				honestCommittee := float64(honestCount) * p
				broadcastersPerRound := honestCommittee + float64(byzantineCount)
				mPerRound := broadcastersPerRound * float64(k-1)
				mTotal := float64(n) * mPerRound
				seconds := mTotal * msPerMessage * safetyFactor / 1000.0

				for _, seed := range seeds {
					preds = append(preds, prediction{
						cfg: gridConfig{
							k: k, n: n, beta: actualBeta,
							rhoDivisor: s.div, pLnCoeff: s.ln, pRhoCoeff: s.rho,
							seed: seed,
						},
						rho: rho, p: p,
						predictedM:       mTotal,
						predictedSeconds: seconds,
					})
				}
			}
		}
	}

	sort.Slice(preds, func(i, j int) bool { return preds[i].predictedSeconds < preds[j].predictedSeconds })

	var feasible, dropped []prediction
	for _, pr := range preds {
		if pr.predictedSeconds <= maxSeconds {
			feasible = append(feasible, pr)
		} else {
			dropped = append(dropped, pr)
		}
	}

	must(os.MkdirAll(outDir, 0o755))
	must(writeGrid(filepath.Join(outDir, "grid_feasible.csv"), feasible, adversary))
	must(writeGrid(filepath.Join(outDir, "grid_dropped.csv"), dropped, adversary))

	var totalSeconds float64
	for _, pr := range feasible {
		totalSeconds += pr.predictedSeconds
	}
	fmt.Printf("generated %d tasks: %d feasible, %d dropped (budget=%.0fs)\n", len(preds), len(feasible), len(dropped), maxSeconds)
	fmt.Printf("wrote %s\n", filepath.Join(outDir, "grid_feasible.csv"))
	fmt.Printf("feasible set: %.1f core-hours if run fully serially\n", totalSeconds/3600)
	if len(feasible) > 0 {
		last := feasible[len(feasible)-1]
		fmt.Printf("longest feasible task: ~%.1f min (k=%d n=%d beta=%.4f)\n",
			last.predictedSeconds/60, last.cfg.k, last.cfg.n, last.cfg.beta)
	}
	if len(dropped) > 0 {
		first := dropped[0]
		fmt.Printf("shortest dropped task: ~%.1f hours (k=%d n=%d beta=%.4f) - see grid_dropped.csv for all %d\n",
			first.predictedSeconds/3600, first.cfg.k, first.cfg.n, first.cfg.beta, len(dropped))
	}
}

// resolveConfigs returns the (k,n) pairs to run: -configs if set, else the
// cross product of -agents and -bits.
func resolveConfigs(configsSpec, agentsSpec, bitsPow2, bitsRaw string) ([]knPair, error) {
	if strings.TrimSpace(configsSpec) != "" {
		return parseConfigs(configsSpec)
	}
	ks, err := parseInts(agentsSpec)
	if err != nil {
		return nil, err
	}
	ns, err := parseBits(bitsPow2, bitsRaw)
	if err != nil {
		return nil, err
	}
	var out []knPair
	for _, k := range ks {
		for _, n := range ns {
			out = append(out, knPair{k: k, n: n})
		}
	}
	return out, nil
}

// resolveSettings returns the constant tuples to run: -settings if set, else
// the cross product of the three constant lists.
func resolveSettings(settingsSpec, rhoDivSpec, lnCoeffSpec, rhoCoeffSpec string) ([]constSetting, error) {
	if strings.TrimSpace(settingsSpec) != "" {
		return parseSettings(settingsSpec)
	}
	divs, err := parseFloats(rhoDivSpec)
	if err != nil {
		return nil, err
	}
	lns, err := parseFloats(lnCoeffSpec)
	if err != nil {
		return nil, err
	}
	rhos, err := parseFloats(rhoCoeffSpec)
	if err != nil {
		return nil, err
	}
	var out []constSetting
	for _, div := range divs {
		for _, ln := range lns {
			for _, rho := range rhos {
				out = append(out, constSetting{div: div, ln: ln, rho: rho})
			}
		}
	}
	return out, nil
}

func parseConfigs(spec string) ([]knPair, error) {
	var out []knPair
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Split(part, ":")
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid config %q: want k:n", part)
		}
		k, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid k in config %q: %w", part, err)
		}
		n, err := strconv.Atoi(strings.TrimSpace(fields[1]))
		if err != nil {
			return nil, fmt.Errorf("invalid n in config %q: %w", part, err)
		}
		out = append(out, knPair{k: k, n: n})
	}
	if len(out) == 0 {
		return nil, errors.New("at least one config required")
	}
	return out, nil
}

func parseSettings(spec string) ([]constSetting, error) {
	var out []constSetting
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Split(part, ":")
		if len(fields) != 3 {
			return nil, fmt.Errorf("invalid setting %q: want div:ln:rho", part)
		}
		vals := make([]float64, 3)
		for i, f := range fields {
			v, err := strconv.ParseFloat(strings.TrimSpace(f), 64)
			if err != nil {
				return nil, fmt.Errorf("invalid number in setting %q: %w", part, err)
			}
			vals[i] = v
		}
		out = append(out, constSetting{div: vals[0], ln: vals[1], rho: vals[2]})
	}
	if len(out) == 0 {
		return nil, errors.New("at least one setting required")
	}
	return out, nil
}

func parseSeeds(spec string) ([]int64, error) {
	var out []int64
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid seed %q: %w", part, err)
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, errors.New("at least one seed required")
	}
	return out, nil
}

func writeGrid(path string, preds []prediction, adversary string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{
		"task_index", "k_agents", "n_bits", "beta", "rho_divisor", "p_ln_coeff", "p_rho_coeff",
		"adversary", "seed", "predicted_rho", "predicted_p", "predicted_M", "predicted_seconds",
	}
	if err := w.Write(header); err != nil {
		return err
	}
	for i, pr := range preds {
		row := []string{
			strconv.Itoa(i),
			strconv.Itoa(pr.cfg.k),
			strconv.Itoa(pr.cfg.n),
			strconv.FormatFloat(pr.cfg.beta, 'f', 6, 64),
			strconv.FormatFloat(pr.cfg.rhoDivisor, 'f', 4, 64),
			strconv.FormatFloat(pr.cfg.pLnCoeff, 'f', 4, 64),
			strconv.FormatFloat(pr.cfg.pRhoCoeff, 'f', 4, 64),
			adversary,
			strconv.FormatInt(pr.cfg.seed, 10),
			strconv.FormatFloat(pr.rho, 'f', 6, 64),
			strconv.FormatFloat(pr.p, 'f', 6, 64),
			strconv.FormatFloat(pr.predictedM, 'f', 0, 64),
			strconv.FormatFloat(pr.predictedSeconds, 'f', 2, 64),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return w.Error()
}

func parseBits(pow2Spec, rawSpec string) ([]int, error) {
	seen := map[int]struct{}{}
	var out []int
	add := func(n int) {
		if _, ok := seen[n]; !ok {
			seen[n] = struct{}{}
			out = append(out, n)
		}
	}
	if pow2Spec != "" {
		for _, part := range strings.Split(pow2Spec, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			exp, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid bits-pow2 exponent %q: %w", part, err)
			}
			add(1 << exp)
		}
	}
	if rawSpec != "" {
		ints, err := parseInts(rawSpec)
		if err != nil {
			return nil, err
		}
		for _, n := range ints {
			add(n)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("at least one n value required via -bits-pow2 or -bits")
	}
	sort.Ints(out)
	return out, nil
}

func parseInts(spec string) ([]int, error) {
	var out []int
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid integer %q: %w", part, err)
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, errors.New("at least one value required")
	}
	return out, nil
}

func parseFloats(spec string) ([]float64, error) {
	var out []float64
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float %q: %w", part, err)
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, errors.New("at least one value required")
	}
	return out, nil
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
