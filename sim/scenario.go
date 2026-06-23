package sim

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"Download_Problem/agent"
	"Download_Problem/source"
)

// scenarioEnv holds shared setup for one run: source, agents, ports, Byzantine mask, ground truth.
type scenarioEnv struct {
	opts          Options
	ratio         float64
	scenarioIndex int

	sourcePort int
	basePort   int
	ports      []int

	byzantine      []bool
	byzantineCount int
	honestCount    int

	src         *source.Source
	groundTruth []byte
	agents      []*agent.Agent

}

// newScenarioEnv starts all servers and returns a cleanup function.
func newScenarioEnv(ctx context.Context, opts Options, ratio float64, scenarioIndex int) (*scenarioEnv, func(), error) {
	portOffset := scenarioIndex * (opts.TotalAgents + 32)
	sourcePort := opts.SourcePort + portOffset
	basePort := opts.BasePort + portOffset

	byzantineCount := int(math.Floor(ratio * float64(opts.TotalAgents)))
	if byzantineCount >= opts.TotalAgents {
		byzantineCount = opts.TotalAgents - 1
	}
	honestCount := opts.TotalAgents - byzantineCount

	src := source.New(opts.BitCount, opts.Seed+int64(7919*scenarioIndex))
	sourceServer, err := src.Start(sourcePort)
	if err != nil {
		return nil, nil, fmt.Errorf("start source on port %d: %w", sourcePort, err)
	}

	client := benchmarkHTTPClient(opts)
	byzantine := selectByzantine(opts.TotalAgents, byzantineCount, opts.Seed+int64(104729*scenarioIndex))

	ports := make([]int, opts.TotalAgents)
	for i := range ports {
		ports[i] = basePort + i
	}

	committeeSeed := opts.Seed + int64(1009*scenarioIndex)
	agents := make([]*agent.Agent, opts.TotalAgents)
	servers := make([]*http.Server, 0, opts.TotalAgents+1)
	servers = append(servers, sourceServer)
	for i := 0; i < opts.TotalAgents; i++ {
		peers := make([]int, 0, opts.TotalAgents-1)
		for j, port := range ports {
			if i != j {
				peers = append(peers, port)
			}
		}
		agents[i] = agent.New(i, ports[i], byzantine[i], peers, client, opts.SourceQueryDelay)
		agents[i].SetCommitteeSeed(committeeSeed)
		server, err := agents[i].StartServer()
		if err != nil {
			shutdownServers(servers)
			client.CloseIdleConnections()
			return nil, nil, fmt.Errorf("start agent %d on port %d: %w", i, ports[i], err)
		}
		servers = append(servers, server)
	}

	env := &scenarioEnv{
		opts:           opts,
		ratio:          ratio,
		scenarioIndex:  scenarioIndex,
		sourcePort:     sourcePort,
		basePort:       basePort,
		ports:          ports,
		byzantine:      byzantine,
		byzantineCount: byzantineCount,
		honestCount:    honestCount,
		src:            src,
		groundTruth:    src.Object(),
		agents:         agents,
	}

	cleanup := func() {
		shutdownServers(servers)
		client.CloseIdleConnections()
	}
	return env, cleanup, nil
}

func (e *scenarioEnv) gamma() float64 {
	return float64(e.honestCount) / float64(e.opts.TotalAgents)
}

func (e *scenarioEnv) actualRatio() float64 {
	return float64(e.byzantineCount) / float64(e.opts.TotalAgents)
}

func (e *scenarioEnv) baseResult() ScenarioResult {
	return ScenarioResult{
		Algorithm:      "blacklist",
		Ratio:          e.ratio,
		ActualRatio:    e.actualRatio(),
		ByzantineCount: e.byzantineCount,
		HonestCount:    e.honestCount,
		NaivePerPeer:   e.opts.BitCount,
		SourceQueries:  e.src.QueryCount(),
	}
}

func benchmarkHTTPClient(opts Options) *http.Client {
	maxConns := opts.Parallelism
	if maxConns < 1 {
		maxConns = 1
	}
	// keep idle pool under macOS's 10240 fd cap (2 fds per conn at large k)
	perHost := 8
	if (opts.TotalAgents+1)*perHost > 3200 {
		perHost = 3200 / (opts.TotalAgents + 1)
		if perHost < 2 {
			perHost = 2
		}
	}
	if perHost > maxConns {
		perHost = maxConns
	}
	return &http.Client{
		Timeout: opts.RequestTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        (opts.TotalAgents + 1) * perHost,
			MaxIdleConnsPerHost: perHost,
			MaxConnsPerHost:     perHost,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  true,
		},
	}
}
