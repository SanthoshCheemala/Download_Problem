package sim

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"Download_Problem/agent"
)

// ANSI styling. Kept here so the visual demo is the only thing that depends on
// terminal colors; the plain trace in trace.go stays color-free for log files.
const (
	cReset  = "\033[0m"
	cBold   = "\033[1m"
	cDim    = "\033[2m"
	cRed    = "\033[31m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cBlue   = "\033[34m"
	cCyan   = "\033[36m"
	cGreenB = "\033[1;32m"
	cRedB   = "\033[1;31m"
	cCyanB  = "\033[1;36m"
)

// stepDelay paces the animation between the four steps of a round.
const stepDelay = 650 * time.Millisecond

var stdinReader = bufio.NewReader(os.Stdin)

func printDemoHeader(env *scenarioEnv, rho, p float64) {
	opts := env.opts
	gamma := env.gamma()
	mu := gamma * float64(opts.TotalAgents) * p

	fmt.Print("\n")
	fmt.Println(cBold + "  ┌────────────────────────────────────────────────────────────────┐" + cReset)
	fmt.Println(cBold + "  │           ALGORITHM 2  ·  BLACKLIST DOWNLOAD  (live demo)        │" + cReset)
	fmt.Println(cBold + "  └────────────────────────────────────────────────────────────────┘" + cReset)
	fmt.Println()
	fmt.Printf("  %d agents download a %d-bit file from one trusted source.\n",
		opts.TotalAgents, opts.BitCount)
	fmt.Printf("  %s%d are honest%s, %s%d are Byzantine%s (they lie). Adversary strategy: %s.\n",
		cGreen, env.honestCount, cReset, cRed, env.byzantineCount, cReset, opts.Adversary)
	fmt.Println()
	fmt.Println("  " + cBold + "Parameters derived from the paper:" + cReset)
	fmt.Printf("    ρ (committee strength)   = k·√(γβ/%v·n)        = %s%.2f%s\n",
		opts.RhoDivisor, cBold, rho, cReset)
	fmt.Printf("    p (join probability)     = (%v·ln n + %v·ρ)/(γk)  = %s%.3f%s\n",
		opts.PLnCoeff, opts.PRhoCoeff, cBold, p, cReset)
	fmt.Printf("    μ (expected committee)   = γ·k·p                = %s%.1f%s\n",
		cBold, mu, cReset)
	fmt.Println()
	fmt.Println("  " + cBold + "Legend:" + cReset)
	fmt.Printf("    %s●%s honest    %s◆%s Byzantine    %s[ ]%s in committee    %s✗%s blacklisted\n",
		cGreen, cReset, cRed, cReset, cYellow, cReset, cDim, cReset)
	fmt.Printf("    vote color: %scorrect%s   %slie%s\n", cGreen, cReset, cRed, cReset)
	fmt.Println()

	// Who is who, fixed for the whole run (the honest agents never learn this).
	fmt.Println("  " + cBold + "Agent roster (assigned at start, hidden from honest agents):" + cReset)
	printAgentRow(env.agents, func(a *agent.Agent) string {
		if a.IsByzantine {
			return cRed + "◆" + cReset
		}
		return cGreen + "●" + cReset
	})
	var honestIDs, byzIDs []int
	for _, a := range env.agents {
		if a.IsByzantine {
			byzIDs = append(byzIDs, a.ID)
		} else {
			honestIDs = append(honestIDs, a.ID)
		}
	}
	fmt.Printf("    %shonest   %s%s\n", cGreen, formatIDs(honestIDs), cReset)
	fmt.Printf("    %sByzantine %s%s\n", cRed, formatIDs(byzIDs), cReset)

	fmt.Println(strings.Repeat("─", 70))
	waitEnter("  Press Enter to start the protocol...")
}

// printRoundDemo animates one round: committee election, voting, tally, decision.
func printRoundDemo(bit int, trueBit byte, agents []*agent.Agent, honestCommittee []int, traceAgent *agent.Agent, rho float64, preBlacklist []int, step bool) {
	committeeSet := toSet(honestCommittee)
	preBL := toSet(preBlacklist)

	// Collect each agent's broadcast vote from the representative honest view.
	votes := map[int]byte{}
	for _, v := range traceAgent.Messages(bit, 0) {
		if len(v.Value) > 0 {
			votes[v.Sender] = v.Value[0]
		}
	}

	postBlacklist := traceAgent.BlacklistedIDs()
	newlyBlacklisted := diff(postBlacklist, preBlacklist)

	fmt.Printf("\n%s━━━ ROUND %d ━━━%s  downloading bit %d   (true value = %s%d%s)\n",
		cCyanB, bit, cReset, bit, cBold, trueBit, cReset)

	// STEP 1 — committee election
	fmt.Printf("\n  %sStep 1%s  Private committee election (each agent flips its own coin)\n", cBold, cReset)
	printAgentRow(agents, func(a *agent.Agent) string {
		switch {
		case a.IsByzantine:
			return cRed + "◆" + cReset
		case committeeSet[a.ID]:
			return cYellow + "[" + cGreenB + "●" + cYellow + "]" + cReset
		default:
			return cDim + "●" + cReset
		}
	})
	fmt.Printf("    honest committee = %s%s%s  (size %d)\n",
		cYellow, formatIDs(honestCommittee), cReset, len(honestCommittee))
	pause(step)

	// STEP 2 — committee queries source and broadcasts; Byzantine agents lie
	fmt.Printf("\n  %sStep 2%s  Committee queries the source, then broadcasts its vote\n", cBold, cReset)
	printAgentRow(agents, func(a *agent.Agent) string {
		val, voted := votes[a.ID]
		switch {
		case preBL[a.ID]:
			return cDim + "✗" + cReset // caught in an earlier round; out for good
		case !voted:
			return cDim + "·" + cReset // silent this round
		case val == trueBit:
			return cGreen + fmt.Sprintf("%d", val) + cReset
		default:
			return cRedB + fmt.Sprintf("%d", val) + cReset
		}
	})
	fmt.Printf("    %s✗%s already blacklisted   %s·%s silent this round   %s0/1%s vote (%scorrect%s / %slie%s)\n",
		cDim, cReset, cDim, cReset, cBold, cReset, cGreen, cReset, cRed, cReset)
	pause(step)

	// STEP 3 — tally
	s0, s1 := tally(votes, trueBit, preBL)
	fmt.Printf("\n  %sStep 3%s  Honest agent tallies the votes it trusts\n", cBold, cReset)
	printBar("votes for 0", s0, 0, trueBit)
	printBar("votes for 1", s1, 1, trueBit)
	smaller := s0
	if s1 < s0 {
		smaller = s1
	}
	fmt.Printf("    ρ = %.2f   →   min(s0, s1) = min(%d, %d) = %d\n", rho, s0, s1, smaller)
	pause(step)

	// STEP 4 — decision
	fmt.Printf("\n  %sStep 4%s  Decision\n", cBold, cReset)
	switch {
	case s0+s1 == 0:
		fmt.Printf("    no committee vote received → %squery source%s, store bit %d\n",
			cCyan, cReset, trueBit)
	case float64(smaller) > rho:
		fmt.Printf("    %s%d > %.2f → CONFLICT detected%s\n", cRedB, smaller, rho, cReset)
		fmt.Printf("    → %squery trusted source%s (truth = %d)\n", cCyan, cReset, trueBit)
		if len(newlyBlacklisted) > 0 {
			fmt.Printf("    → %sblacklist the liars: %s%s\n",
				cRedB, formatIDs(newlyBlacklisted), cReset)
		}
	default:
		majority := byte(0)
		if s1 > s0 {
			majority = 1
		}
		fmt.Printf("    %smin(s0,s1) = %d ≤ ρ = %.2f → no conflict%s\n", cGreen, smaller, rho, cReset)
		fmt.Printf("    → accept majority value %s%d%s with %sno source query%s\n",
			cBold, majority, cReset, cGreenB, cReset)
	}

	if len(postBlacklist) > 0 {
		fmt.Printf("\n    %sblacklist so far:%s %s%s%s\n",
			cBold, cReset, cRed, formatIDs(postBlacklist), cReset)
	}
	fmt.Println("  " + strings.Repeat("─", 66))

	if step {
		waitEnter("  Press Enter for the next round...")
	}
}

// agentsPerRow caps how many agents print on one line. Each agent is 4 columns
// wide, so 20 fits in ~84 cols -- narrow enough for any terminal. Wider rosters
// wrap into stacked blocks instead of overflowing and desyncing the index line
// from the colored glyph line (their byte lengths differ, so a terminal-level
// wrap would break their alignment).
const agentsPerRow = 20

// printAgentRow prints the agent index header and a glyph per agent from render,
// wrapping into aligned blocks of agentsPerRow so wide rosters stay readable.
func printAgentRow(agents []*agent.Agent, render func(*agent.Agent) string) {
	for start := 0; start < len(agents); start += agentsPerRow {
		end := start + agentsPerRow
		if end > len(agents) {
			end = len(agents)
		}

		var idx, glyph strings.Builder
		idx.WriteString("    ")
		glyph.WriteString("    ")
		for _, a := range agents[start:end] {
			idx.WriteString(fmt.Sprintf("%-4d", a.ID))
			// glyphs carry color codes, so pad by visible width (1 char) manually
			glyph.WriteString(render(a))
			glyph.WriteString(strings.Repeat(" ", 4-1))
		}
		fmt.Println(cDim + idx.String() + cReset)
		fmt.Println(glyph.String())
		if end < len(agents) {
			fmt.Println() // blank line separates blocks
		}
	}
}

func printBar(label string, count int, want, trueBit byte) {
	color := cGreen
	if want != trueBit {
		color = cRed
	}
	fmt.Printf("    %-12s %s%2d%s  %s%s%s\n",
		label, cBold, count, cReset, color, strings.Repeat("█", count), cReset)
}

func tally(votes map[int]byte, trueBit byte, preBL map[int]bool) (int, int) {
	s0, s1 := 0, 0
	for sender, v := range votes {
		if preBL[sender] {
			continue
		}
		if v == 0 {
			s0++
		} else {
			s1++
		}
	}
	return s0, s1
}

func toSet(ids []int) map[int]bool {
	m := make(map[int]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func pause(step bool) {
	if !step {
		time.Sleep(stepDelay)
	}
}

func waitEnter(prompt string) {
	fmt.Print(cDim + prompt + cReset)
	_, _ = stdinReader.ReadString('\n')
}
