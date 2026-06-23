package sim

import (
	"bytes"
	"context"
	"math"
	"math/rand"
	"time"

	"Download_Problem/agent"
)

// runBlacklistScenario runs Algorithm 2 (Blacklist_Download): n bit-rounds,
// each electing a private committee that votes, resolves conflicts at the source, and
// blacklists caught liars.
func runBlacklistScenario(ctx context.Context, env *scenarioEnv) (ScenarioResult, error) {
	opts := env.opts
	n := opts.BitCount
	k := opts.TotalAgents
	gamma := env.gamma()
	beta := env.actualRatio()

	rho := opts.Rho
	if rho == 0 {
		rho = PaperRho(k, n, beta, opts.RhoDivisor)
	}
	p := agent.CommitteeProbability(n, k, gamma, rho, opts.PLnCoeff, opts.PRhoCoeff)

	trace := opts.Verbose || opts.Demo
	if opts.Demo {
		printDemoHeader(env, rho, p)
	} else if opts.Verbose {
		printScenarioHeader(env, rho, p)
	}

	// The drip adversary is coordinated and economical: each round it exposes
	// just over rho of its still-live agents -- enough to trip the conflict test
	// min(s0,s1) > rho and force every honest agent to spend a source query --
	// while holding the rest in reserve. Flood, by contrast, exposes everyone at
	// once and is wiped out in round 0. repHonest stands in for the adversary's
	// view of which of its agents have already been caught (all honest agents
	// blacklist the same liars, so any one of them is representative).
	var repHonest *agent.Agent
	exposePerRound := int(math.Floor(rho)) + 1
	if opts.Adversary == "drip" {
		repHonest = honestAgents(env.agents)[0]
	}

	protocolStart := time.Now()
	for i := 0; i < n; i++ {
		bit := i
		trueBit := env.groundTruth[bit]

		// Capture the honest committee membership before the round runs so we
		// can show "who was in the committee" in the trace.
		var honestCommittee []int
		if trace {
			for _, a := range env.agents {
				if !a.IsByzantine && a.InCommittee(bit, p) {
					honestCommittee = append(honestCommittee, a.ID)
				}
			}
		}

		// Pick which Byzantine agents reveal themselves this round: the first
		// exposePerRound that have not yet been blacklisted. The rest stay silent.
		exposed := map[int]bool{}
		if opts.Adversary == "drip" {
			live := 0
			for _, a := range env.agents {
				if a.IsByzantine && !repHonest.IsBlacklisted(a.ID) {
					exposed[a.ID] = true
					live++
					if live >= exposePerRound {
						break
					}
				}
			}
		}

		if _, err := runRoundJobs(ctx, env.agents, opts.Parallelism, opts.RoundTimeout, func(roundCtx context.Context, a *agent.Agent) error {
			if a.IsByzantine {
				if opts.Adversary == "drip" && !exposed[a.ID] {
					return nil // conserve this agent; stay silent this round
				}
				return byzantineVote(roundCtx, a, env.ports, opts.Adversary, bit, trueBit)
			}
			if !a.InCommittee(bit, p) {
				return nil
			}
			value, err := a.QueryBit(roundCtx, env.sourcePort, bit)
			if err != nil {
				return err
			}
			a.RecordPrimaryQuery()
			return a.Broadcast(roundCtx, env.ports, agent.Message{
				Sender: a.ID, Round: bit, Topic: 0, Value: []byte{value},
			})
		}); err != nil {
			return ScenarioResult{}, err
		}

		for _, a := range env.agents {
			a.SealRound(bit)
		}

		// Snapshot a representative honest agent's blacklist before decide so
		// we can show what was newly blacklisted this round.
		var traceAgent *agent.Agent
		var preBlacklist []int
		if trace {
			for _, a := range env.agents {
				if !a.IsByzantine {
					traceAgent = a
					preBlacklist = a.BlacklistedIDs()
					break
				}
			}
		}

		if err := runAgentJobs(ctx, honestAgents(env.agents), opts.Parallelism, func(a *agent.Agent) error {
			return decideBit(ctx, a, env.sourcePort, bit, rho)
		}); err != nil {
			return ScenarioResult{}, err
		}

		if traceAgent != nil {
			if opts.Demo {
				printRoundDemo(bit, trueBit, env.agents, honestCommittee, traceAgent, rho, preBlacklist, opts.Step)
			} else if opts.Verbose {
				printRoundTrace(bit, trueBit, honestCommittee, traceAgent, rho, preBlacklist)
			}
		}

		// Bit i is decided and its votes are never read again; free them so
		// memory stays bounded to one round instead of growing with n.
		for _, a := range env.agents {
			a.ForgetRound(bit)
		}
	}
	protocolTime := time.Since(protocolStart)

	result := env.baseResult()
	result.RoundComplexity = n
	result.ProtocolTime = protocolTime
	result.SourceQueries = env.src.QueryCount()

	for _, a := range env.agents {
		result.TotalSourceQueries += a.SourceQueries()
		if a.IsByzantine {
			continue
		}
		result.Messages += a.MessagesSent()

		sq := a.SourceQueries()
		result.AvgHonestSourceQueries += float64(sq)
		if sq > result.MaxHonestSourceQueries {
			result.MaxHonestSourceQueries = sq
			result.MaxAgentPrimaryQueries = a.PrimaryQueries()
			result.MaxAgentVerifyQueries = a.VerifyQueries()
		}
		if bl := a.BlacklistSize(); bl > result.MaxBlacklistSize {
			result.MaxBlacklistSize = bl
		}

		if reconstructedCorrectly(a, n, env.groundTruth) {
			result.CompletedHonest++
		} else {
			result.FailedHonest++
			result.MissingBits += int64(a.MissingBitCount(n))
		}
	}
	if env.honestCount > 0 {
		result.AvgHonestSourceQueries /= float64(env.honestCount)
	}

	return result, nil
}

// decideBit tallies votes for bit i and resolves conflicts by querying the source.
func decideBit(ctx context.Context, a *agent.Agent, sourcePort, bit int, rho float64) error {
	votes := a.Messages(bit, 0)

	var s0, s1 int
	var voters0, voters1 []int
	for _, v := range votes {
		if len(v.Value) == 0 || a.IsBlacklisted(v.Sender) {
			continue
		}
		if v.Value[0] == 0 {
			s0++
			voters0 = append(voters0, v.Sender)
		} else {
			s1++
			voters1 = append(voters1, v.Sender)
		}
	}

	conflict := float64(min(s0, s1)) > rho
	if conflict || s0+s1 == 0 {
		value, err := a.QueryBit(ctx, sourcePort, bit)
		if err != nil {
			return err
		}
		a.RecordVerifyQuery()
		a.StoreBit(bit, value)
		if conflict {
			losers := voters1
			if value == 1 {
				losers = voters0
			}
			for _, id := range losers {
				a.Blacklist(id)
			}
		}
		return nil
	}

	value := byte(0)
	if s1 > s0 {
		value = 1
	}
	a.StoreBit(bit, value)
	return nil
}

// byzantineVote broadcasts a fabricated vote. Wrong value = 1-trueBit.
// random: independent coin; collude/flood/drip: always votes the wrong value
// (drip differs only in WHICH agents are told to vote, decided by the caller).
func byzantineVote(ctx context.Context, a *agent.Agent, ports []int, adversary string, bit int, trueBit byte) error {
	var value byte
	switch adversary {
	case "random":
		rng := rand.New(rand.NewSource(int64(a.ID)*1_000_003 + int64(bit)))
		value = byte(rng.Intn(2))
	default: // collude, flood, drip
		value = trueBit ^ 1
	}
	return a.Broadcast(ctx, ports, agent.Message{
		Sender: a.ID, Round: bit, Topic: 0, Value: []byte{value},
	})
}

func reconstructedCorrectly(a *agent.Agent, totalBits int, groundTruth []byte) bool {
	data := a.Reconstruct(totalBits)
	return data != nil && bytes.Equal(data, groundTruth)
}

// PaperRho computes rho = max{1, k*sqrt(gamma*beta/(divisor*n))}.
// Paper uses divisor=8; increase it to shrink rho below the proven bound.
func PaperRho(k, n int, beta, divisor float64) float64 {
	gamma := 1 - beta
	r := float64(k) * math.Sqrt(gamma*beta/(divisor*float64(n)))
	if r < 1 {
		return 1
	}
	return r
}
