package agent

import (
	"math"
	"math/rand"
)

// CommitteeProbability returns p = min{(lnCoeff*ln(n) + rhoCoeff*rho) / (gamma*k), 1}.
// Paper uses lnCoeff=6, rhoCoeff=4.
func CommitteeProbability(n, k int, gamma, rho, lnCoeff, rhoCoeff float64) float64 {
	if k <= 0 || gamma <= 0 {
		return 1
	}
	nn := n
	if nn < 2 {
		nn = 2
	}
	p := (lnCoeff*math.Log(float64(nn)) + rhoCoeff*rho) / (gamma * float64(k))
	if p > 1 {
		return 1
	}
	if p < 0 {
		return 0
	}
	return p
}

// InCommittee flips a biased coin for this agent and round. The seed is derived
// deterministically so the same global seed always produces the same committees.
func (a *Agent) InCommittee(round int, p float64) bool {
	if p >= 1 {
		return true
	}
	if p <= 0 {
		return false
	}
	rng := rand.New(rand.NewSource(a.committeeSeed + int64(round)*1_000_003 + int64(a.ID)))
	return rng.Float64() < p
}
