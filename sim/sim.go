package sim

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"Download_Problem/agent"
	"Download_Problem/config"
)

// Options configures a benchmark run.
type Options struct {
	TotalAgents      int
	BitCount         int
	SourcePort       int
	BasePort         int
	Parallelism      int
	Seed             int64
	RequestTimeout   time.Duration
	ByzantineRatios  []float64
	CSVPath          string
	Rho              float64
	RhoDivisor       float64
	PLnCoeff         float64
	PRhoCoeff        float64
	SourceQueryDelay time.Duration
	Adversary        string
	RoundTimeout     time.Duration
	Verbose          bool
	Demo             bool
	Step             bool
}

func DefaultOptions() Options {
	return Options{
		TotalAgents:      config.DefaultTotalAgents,
		BitCount:         config.DefaultTotalBits,
		SourcePort:       config.DefaultSourcePort,
		BasePort:         config.DefaultBasePort,
		Parallelism:      config.DefaultParallelism,
		Seed:             config.DefaultSeed,
		RequestTimeout:   config.DefaultRequestTimeout,
		ByzantineRatios:  []float64{1.0 / 3.0},
		Rho:              config.DefaultRho,
		RhoDivisor:       config.DefaultRhoDivisor,
		PLnCoeff:         config.DefaultPLnCoeff,
		PRhoCoeff:        config.DefaultPRhoCoeff,
		SourceQueryDelay: config.DefaultSourceDelay,
		Adversary:        config.DefaultAdversary,
		RoundTimeout:     config.DefaultRoundTimeout,
	}
}

func Run() error {
	opts := DefaultOptions()
	ratioSpec := config.DefaultRatios

	flag.IntVar(&opts.TotalAgents, "agents", opts.TotalAgents, "number of HTTP agents (k)")
	flag.IntVar(&opts.BitCount, "bits", opts.BitCount, "number of data bits in the source array (n)")
	flag.IntVar(&opts.SourcePort, "source-port", opts.SourcePort, "trusted source port for the first scenario")
	flag.IntVar(&opts.BasePort, "base-port", opts.BasePort, "first agent port for the first scenario")
	flag.IntVar(&opts.Parallelism, "parallelism", opts.Parallelism, "maximum concurrent HTTP jobs")
	flag.Int64Var(&opts.Seed, "seed", opts.Seed, "random seed")
	flag.DurationVar(&opts.RequestTimeout, "timeout", opts.RequestTimeout, "HTTP request timeout")
	flag.StringVar(&opts.CSVPath, "csv", "", "append machine-readable benchmark rows to this CSV file")
	flag.Float64Var(&opts.Rho, "rho", opts.Rho, "Algorithm 2 committee strength; 0 derives rho=max{1,k*sqrt(gamma*beta/(rho-divisor*n))}")
	flag.Float64Var(&opts.RhoDivisor, "rho-divisor", opts.RhoDivisor, "divisor inside rho's sqrt (paper=8); raise to relax (shrink) rho below the paper's bound")
	flag.Float64Var(&opts.PLnCoeff, "p-ln-coeff", opts.PLnCoeff, "coefficient on ln(n) in p=min{(c1*ln n + c2*rho)/(gamma*k),1} (paper=6); lower to relax")
	flag.Float64Var(&opts.PRhoCoeff, "p-rho-coeff", opts.PRhoCoeff, "coefficient on rho in p=min{(c1*ln n + c2*rho)/(gamma*k),1} (paper=4); lower to relax")
	flag.DurationVar(&opts.SourceQueryDelay, "source-delay", opts.SourceQueryDelay, "artificial delay added before every direct trusted-source query")
	flag.StringVar(&opts.Adversary, "adversary", opts.Adversary, "Byzantine strategy: random, collude, flood (all lie at once), or drip (expose ~rho per round to force conflicts)")
	flag.DurationVar(&opts.RoundTimeout, "round-timeout", opts.RoundTimeout, "synchronous round duration; agents that miss the deadline are cut off")
	flag.StringVar(&ratioSpec, "ratios", ratioSpec, "comma-separated Byzantine ratios; fractions like 1/3 are allowed")
	flag.BoolVar(&opts.Verbose, "verbose", opts.Verbose, "print per-round trace: committee election, vote tally, conflicts, blacklisting")
	flag.BoolVar(&opts.Demo, "demo", opts.Demo, "animated, color-coded terminal visualization of the protocol (best for small k,n)")
	flag.BoolVar(&opts.Step, "step", opts.Step, "with -demo: pause for Enter between rounds so you can narrate")
	flag.Parse()

	ratios, err := ParseRatios(ratioSpec)
	if err != nil {
		return err
	}
	opts.ByzantineRatios = ratios

	results, err := RunBenchmarks(context.Background(), opts)
	if err != nil {
		return err
	}
	PrintResults(opts, results)
	if opts.CSVPath != "" {
		if err := AppendCSV(opts, results); err != nil {
			return err
		}
	}
	return nil
}

func RunBenchmarks(ctx context.Context, opts Options) ([]ScenarioResult, error) {
	if err := validateOptions(opts); err != nil {
		return nil, err
	}

	results := make([]ScenarioResult, 0, len(opts.ByzantineRatios))
	for i, ratio := range opts.ByzantineRatios {
		result, err := runScenario(ctx, opts, ratio, i)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
		if i < len(opts.ByzantineRatios)-1 {
			time.Sleep(250 * time.Millisecond)
		}
	}
	return results, nil
}

func runScenario(ctx context.Context, opts Options, ratio float64, scenarioIndex int) (ScenarioResult, error) {
	env, cleanup, err := newScenarioEnv(ctx, opts, ratio, scenarioIndex)
	if err != nil {
		return ScenarioResult{}, err
	}
	defer cleanup()

	return runBlacklistScenario(ctx, env)
}

func validateOptions(opts Options) error {
	if opts.TotalAgents < 2 {
		return errors.New("agents must be at least 2")
	}
	if opts.BitCount < 1 {
		return errors.New("bits must be at least 1")
	}
	if opts.Parallelism < 1 {
		return errors.New("parallelism must be at least 1")
	}
	if opts.RequestTimeout <= 0 {
		return errors.New("timeout must be positive")
	}
	if opts.Rho < 0 {
		return errors.New("rho cannot be negative (use 0 to derive it)")
	}
	if opts.Rho > float64(opts.TotalAgents) {
		return fmt.Errorf("rho (%.2f) cannot exceed agents (%d): a committee can never guarantee more honest votes than there are peers", opts.Rho, opts.TotalAgents)
	}
	if opts.RhoDivisor <= 0 {
		return errors.New("rho-divisor must be positive (it divides inside a square root)")
	}
	if opts.PLnCoeff < 0 || opts.PRhoCoeff < 0 {
		return errors.New("p-ln-coeff and p-rho-coeff cannot be negative (p would be undefined as a probability)")
	}
	if len(opts.ByzantineRatios) == 0 {
		return errors.New("at least one Byzantine ratio is required")
	}
	for _, ratio := range opts.ByzantineRatios {
		if ratio < 0 || ratio >= 1 {
			return fmt.Errorf("Byzantine ratio %v must be in [0, 1)", ratio)
		}
	}
	if opts.SourceQueryDelay < 0 {
		return errors.New("source-delay cannot be negative")
	}
	if opts.Adversary != "random" && opts.Adversary != "collude" && opts.Adversary != "flood" && opts.Adversary != "drip" {
		return errors.New("adversary must be random, collude, flood, or drip")
	}
	if opts.RoundTimeout <= 0 {
		return errors.New("round-timeout must be positive")
	}
	return nil
}

func selectByzantine(totalAgents, byzantineCount int, seed int64) []bool {
	selected := make([]bool, totalAgents)
	rng := rand.New(rand.NewSource(seed))
	perm := rng.Perm(totalAgents)
	for i := 0; i < byzantineCount; i++ {
		selected[perm[i]] = true
	}
	return selected
}

func honestAgents(agents []*agent.Agent) []*agent.Agent {
	honest := make([]*agent.Agent, 0, len(agents))
	for _, a := range agents {
		if !a.IsByzantine {
			honest = append(honest, a)
		}
	}
	return honest
}

// runRoundJobs runs one protocol round with a hard deadline; peers that miss
// the cutoff are cancelled (counted as missed, not errors).
func runRoundJobs(ctx context.Context, agents []*agent.Agent, parallelism int, timeout time.Duration, fn func(context.Context, *agent.Agent) error) (int64, error) {
	roundCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	sem := make(chan struct{}, parallelism)
	errCh := make(chan error, len(agents))
	var missed atomic.Int64
	var wg sync.WaitGroup

	for _, a := range agents {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(a *agent.Agent) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := fn(roundCtx, a); err != nil {
				if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
					missed.Add(1)
					return
				}
				errCh <- err
			}
		}(a)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return missed.Load(), err
		}
	}
	return missed.Load(), ctx.Err()
}

func runAgentJobs(ctx context.Context, agents []*agent.Agent, parallelism int, fn func(*agent.Agent) error) error {
	sem := make(chan struct{}, parallelism)
	errCh := make(chan error, len(agents))
	var wg sync.WaitGroup

	for _, a := range agents {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(a *agent.Agent) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := fn(a); err != nil {
				errCh <- err
			}
		}(a)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return ctx.Err()
}

func shutdownServers(servers []*http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for _, server := range servers {
		_ = server.Shutdown(ctx)
	}
}

func ParseRatios(spec string) ([]float64, error) {
	parts := strings.Split(spec, ",")
	ratios := make([]float64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		ratio, err := parseRatio(part)
		if err != nil {
			return nil, err
		}
		if ratio < 0 || ratio >= 1 {
			return nil, fmt.Errorf("Byzantine ratio %q must be in [0, 1)", part)
		}
		ratios = append(ratios, ratio)
	}
	if len(ratios) == 0 {
		return nil, errors.New("at least one Byzantine ratio is required")
	}
	return ratios, nil
}

func parseRatio(part string) (float64, error) {
	if strings.Contains(part, "/") {
		pieces := strings.Split(part, "/")
		if len(pieces) != 2 {
			return 0, fmt.Errorf("invalid ratio %q", part)
		}
		num, err := strconv.ParseFloat(strings.TrimSpace(pieces[0]), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid ratio numerator %q: %w", part, err)
		}
		den, err := strconv.ParseFloat(strings.TrimSpace(pieces[1]), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid ratio denominator %q: %w", part, err)
		}
		if den == 0 {
			return 0, fmt.Errorf("invalid ratio %q: denominator is zero", part)
		}
		return num / den, nil
	}
	return strconv.ParseFloat(part, 64)
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}
