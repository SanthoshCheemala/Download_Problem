package sim

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// ScenarioResult holds measured Q, M, T and success for one run.
type ScenarioResult struct {
	Algorithm      string
	Ratio          float64
	ActualRatio    float64
	ByzantineCount int
	HonestCount    int

	CompletedHonest int
	FailedHonest    int

	MaxHonestSourceQueries int64 // Q
	AvgHonestSourceQueries float64
	TotalSourceQueries     int64
	SourceQueries          int64 // queries seen by the source server
	Messages               int64 // M (honest peer-to-peer messages)
	RoundComplexity        int   // T
	ProtocolTime           time.Duration
	NaivePerPeer           int

	MaxAgentPrimaryQueries int64 // committee queries (Alg 2)
	MaxAgentVerifyQueries  int64 // conflict-resolution queries (Alg 2)

	MaxBlacklistSize int

	MissingBits int64
}

func (r ScenarioResult) successRate() float64 {
	if r.HonestCount == 0 {
		return 0
	}
	return float64(r.CompletedHonest) / float64(r.HonestCount)
}

func PrintResults(opts Options, results []ScenarioResult) {
	bar := strings.Repeat("=", 70)
	dash := strings.Repeat("-", 70)

	fmt.Println()
	fmt.Println(bar)
	fmt.Println("FINAL RESULTS - Algorithm 2 (Blacklist Download)")
	fmt.Println(bar)
	fmt.Printf("Configuration: k = %d agents | n = %d bits | adversary = %s | seed = %d\n",
		opts.TotalAgents, opts.BitCount, opts.Adversary, opts.Seed)
	fmt.Println(dash)

	fmt.Println("Complexity per scenario:")
	fmt.Println()
	fmt.Printf("  %-7s  %-13s  %7s  %12s  %8s  %9s  %8s\n",
		"beta", "byz/honest", "Q", "M", "T", "time_ms", "success")
	for _, r := range results {
		fmt.Printf("  %-7.4f  %4d / %-6d  %7d  %12d  %8d  %9d  %7.1f%%\n",
			r.ActualRatio,
			r.ByzantineCount,
			r.HonestCount,
			r.MaxHonestSourceQueries,
			r.Messages,
			r.RoundComplexity,
			r.ProtocolTime.Milliseconds(),
			100*r.successRate(),
		)
	}

	fmt.Println()
	fmt.Println("Q decomposition (max-Q honest agent):")
	for _, r := range results {
		ratio := float64(r.MaxHonestSourceQueries) / float64(opts.BitCount)
		fmt.Printf("  beta=%.4f   Q = %d  =  committee %d  +  conflict %d   (blacklist size = %d, Q/n = %.3f)\n",
			r.ActualRatio,
			r.MaxHonestSourceQueries,
			r.MaxAgentPrimaryQueries,
			r.MaxAgentVerifyQueries,
			r.MaxBlacklistSize,
			ratio,
		)
	}

	fmt.Println()
	fmt.Printf("Naive baseline (no committee, query source for every bit): %d queries / honest agent\n",
		opts.BitCount)
	fmt.Println("Q  = max source queries by one honest agent (worst case)")
	fmt.Println("M  = total honest peer-to-peer messages")
	fmt.Println("T  = round complexity (one round per bit)")
	fmt.Println(bar)
}

func AppendCSV(opts Options, results []ScenarioResult) error {
	file, err := os.OpenFile(opts.CSVPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open csv %s: %w", opts.CSVPath, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat csv %s: %w", opts.CSVPath, err)
	}

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if info.Size() == 0 {
		if err := writer.Write(csvHeader()); err != nil {
			return fmt.Errorf("write csv header: %w", err)
		}
	}

	for _, result := range results {
		if err := writer.Write(csvRow(opts, result)); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}
	return nil
}

func csvHeader() []string {
	return []string{
		"timestamp_utc",
		"algorithm",
		"k_agents",
		"n_bits",
		"beta",
		"rho",
		"rho_divisor",
		"p_ln_coeff",
		"p_rho_coeff",
		"query_complexity_Q",
		"message_complexity_M",
		"round_complexity_T",
		"time_complexity_ms",
		"success_rate",
	}
}

func csvRow(opts Options, result ScenarioResult) []string {
	rho := opts.Rho
	if rho == 0 {
		rho = PaperRho(opts.TotalAgents, opts.BitCount, result.ActualRatio, opts.RhoDivisor)
	}
	return []string{
		time.Now().UTC().Format(time.RFC3339),
		result.Algorithm,
		strconv.Itoa(opts.TotalAgents),
		strconv.Itoa(opts.BitCount),
		formatFloat(result.ActualRatio),
		formatFloat(rho),
		formatFloat(opts.RhoDivisor),
		formatFloat(opts.PLnCoeff),
		formatFloat(opts.PRhoCoeff),
		strconv.FormatInt(result.MaxHonestSourceQueries, 10),
		strconv.FormatInt(result.Messages, 10),
		strconv.Itoa(result.RoundComplexity),
		strconv.FormatInt(result.ProtocolTime.Milliseconds(), 10),
		formatFloat(result.successRate()),
	}
}
