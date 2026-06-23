package config

import "time"

const (
	DefaultTotalAgents = 100
	DefaultTotalBits   = 256
	DefaultSourcePort  = 10000
	DefaultBasePort    = 10001
	DefaultParallelism = 32
	DefaultSeed        = 42
	DefaultRatios      = "1/3"

	// DefaultRho = 0 means derive from rho = max{1, k*sqrt(gamma*beta/(8n))}.
	DefaultRho = 0.0

	// DefaultRhoDivisor/PLnCoeff/PRhoCoeff are the paper's constants; lower them to relax guarantees.
	DefaultRhoDivisor = 8.0
	DefaultPLnCoeff   = 6.0
	DefaultPRhoCoeff  = 4.0

	DefaultRequestTimeout = 5 * time.Second
	DefaultSourceDelay    = 0 * time.Millisecond
	DefaultRoundTimeout   = 10 * time.Second
	DefaultAdversary      = "flood"
)
