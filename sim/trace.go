package sim

import (
	"fmt"
	"sort"
	"strings"

	"Download_Problem/agent"
)

// printScenarioHeader announces a scenario with all derived parameters at the
// top, so a viewer reading the per-round trace can refer back to them.
func printScenarioHeader(env *scenarioEnv, rho, p float64) {
	opts := env.opts
	gamma := env.gamma()
	beta := env.actualRatio()
	mu := gamma * float64(opts.TotalAgents) * p

	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("SCENARIO: beta = %.4f   adversary = %s   seed = %d\n",
		beta, opts.Adversary, opts.Seed)
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("Setup:")
	fmt.Printf("  k (agents)        : %d   (honest = %d, Byzantine = %d)\n",
		opts.TotalAgents, env.honestCount, env.byzantineCount)
	fmt.Printf("  n (bits)          : %d\n", opts.BitCount)
	fmt.Printf("  gamma (honest)    : %.4f\n", gamma)
	fmt.Printf("  beta  (Byzantine) : %.4f\n", beta)
	fmt.Println("Derived (paper):")
	fmt.Printf("  rho  = k * sqrt(gamma*beta / (%v*n))           = %.3f\n",
		opts.RhoDivisor, rho)
	fmt.Printf("  p    = (%v*ln n + %v*rho) / (gamma*k)          = %.4f\n",
		opts.PLnCoeff, opts.PRhoCoeff, p)
	fmt.Printf("  mu   = gamma * k * p   (expected committee)   = %.2f\n", mu)
	fmt.Printf("  Delta (round timeout)                         = %s\n",
		opts.RoundTimeout)
	fmt.Println(strings.Repeat("-", 70))
	fmt.Println("PROTOCOL EXECUTION")
	fmt.Println(strings.Repeat("-", 70))
}

// printRoundTrace prints one round's committee, votes, conflict decision, and
// any blacklisting that happened. Uses traceAgent's message store and blacklist
// as a representative honest view; all honest agents see the same votes.
func printRoundTrace(bit int, trueBit byte, honestCommittee []int, traceAgent *agent.Agent, rho float64, preBlacklist []int) {
	sort.Ints(honestCommittee)

	votes := traceAgent.Messages(bit, 0)
	var votersFor0, votersFor1 []int
	for _, v := range votes {
		if len(v.Value) == 0 {
			continue
		}
		if v.Value[0] == 0 {
			votersFor0 = append(votersFor0, v.Sender)
		} else {
			votersFor1 = append(votersFor1, v.Sender)
		}
	}
	sort.Ints(votersFor0)
	sort.Ints(votersFor1)

	blacklistSet := make(map[int]bool, len(preBlacklist))
	for _, id := range preBlacklist {
		blacklistSet[id] = true
	}
	s0Counted := countNotBlacklisted(votersFor0, blacklistSet)
	s1Counted := countNotBlacklisted(votersFor1, blacklistSet)

	postBlacklist := traceAgent.BlacklistedIDs()
	newlyBlacklisted := diff(postBlacklist, preBlacklist)

	fmt.Printf("\n[Round %d / Bit %d]  ground truth = %d\n", bit, bit, trueBit)
	fmt.Printf("  Committee election (private coin per agent):\n")
	fmt.Printf("    Honest committee: %s  (size = %d)\n",
		formatIDs(honestCommittee), len(honestCommittee))

	fmt.Printf("  Votes received (representative honest view, agent %d):\n", traceAgent.ID)
	fmt.Printf("    For 0: %s  (count = %d", formatIDs(votersFor0), len(votersFor0))
	if len(votersFor0) != s0Counted {
		fmt.Printf(", %d ignored as blacklisted", len(votersFor0)-s0Counted)
	}
	fmt.Println(")")
	fmt.Printf("    For 1: %s  (count = %d", formatIDs(votersFor1), len(votersFor1))
	if len(votersFor1) != s1Counted {
		fmt.Printf(", %d ignored as blacklisted", len(votersFor1)-s1Counted)
	}
	fmt.Println(")")

	fmt.Printf("  Tally after filter: s0 = %d, s1 = %d, rho = %.3f\n",
		s0Counted, s1Counted, rho)

	smaller := s0Counted
	if s1Counted < s0Counted {
		smaller = s1Counted
	}
	switch {
	case s0Counted+s1Counted == 0:
		fmt.Printf("  Decision: NO COMMITTEE VOTE -> query source, store bit %d\n", trueBit)
	case float64(smaller) > rho:
		fmt.Printf("  Decision: CONFLICT (min(s0,s1) = %d > rho = %.3f)\n", smaller, rho)
		fmt.Printf("    -> query source (truth = %d)\n", trueBit)
		if len(newlyBlacklisted) > 0 {
			fmt.Printf("    -> blacklist new liars: %s\n", formatIDs(newlyBlacklisted))
		}
	default:
		majority := byte(0)
		if s1Counted > s0Counted {
			majority = 1
		}
		fmt.Printf("  Decision: ACCEPT majority (bit = %d, no source query)\n", majority)
	}

	fmt.Printf("  Cumulative blacklist size: %d  %s\n",
		len(postBlacklist), formatIDs(postBlacklist))
}

func countNotBlacklisted(voters []int, blacklist map[int]bool) int {
	n := 0
	for _, id := range voters {
		if !blacklist[id] {
			n++
		}
	}
	return n
}

func diff(after, before []int) []int {
	beforeSet := make(map[int]bool, len(before))
	for _, id := range before {
		beforeSet[id] = true
	}
	out := make([]int, 0, len(after)-len(before))
	for _, id := range after {
		if !beforeSet[id] {
			out = append(out, id)
		}
	}
	return out
}

func formatIDs(ids []int) string {
	if len(ids) == 0 {
		return "[]"
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%d", id)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
