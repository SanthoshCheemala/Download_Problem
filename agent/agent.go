package agent

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Agent is a single peer node. Protocol logic lives in the sim package.
type Agent struct {
	ID          int
	Port        int
	IsByzantine bool

	PeerPorts []int

	bits        map[int]byte
	client      *http.Client
	sourceDelay time.Duration
	mu          sync.RWMutex

	// message store keyed by [round][topic]
	messages    map[int]map[int][]Message
	messageMu   sync.RWMutex
	sealedRound atomic.Int32

	blacklist   map[int]struct{}
	blacklistMu sync.RWMutex

	// fixed per-scenario so committee membership is reproducible
	committeeSeed int64

	sourceQueries  atomic.Int64 // Q metric
	messagesSent   atomic.Int64 // M metric
	primaryQueries atomic.Int64 // committee source queries
	verifyQueries  atomic.Int64 // conflict-resolution source queries
}

func New(id, port int, isByzantine bool, peers []int, client *http.Client, sourceDelay time.Duration) *Agent {
	a := &Agent{
		ID:          id,
		Port:        port,
		IsByzantine: isByzantine,
		PeerPorts:   append([]int(nil), peers...),
		bits:        make(map[int]byte),
		messages:    make(map[int]map[int][]Message),
		blacklist:   make(map[int]struct{}),
		client:      client,
		sourceDelay: sourceDelay,
	}
	a.sealedRound.Store(-1)
	return a
}

func (a *Agent) SetCommitteeSeed(seed int64) {
	a.committeeSeed = seed
}

func (a *Agent) httpClient() *http.Client {
	if a.client != nil {
		return a.client
	}
	return http.DefaultClient
}

func (a *Agent) StoreBit(index int, bit byte) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.bits[index] = bit
}

func (a *Agent) GetBit(index int) (byte, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	bit, ok := a.bits[index]
	return bit, ok
}

func (a *Agent) MissingBitCount(totalBits int) int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	missing := 0
	for i := 0; i < totalBits; i++ {
		if _, ok := a.bits[i]; !ok {
			missing++
		}
	}
	return missing
}

// Reconstruct returns the agent's output array, or nil if any bit is missing.
func (a *Agent) Reconstruct(totalBits int) []byte {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]byte, totalBits)
	for i := 0; i < totalBits; i++ {
		bit, ok := a.bits[i]
		if !ok {
			return nil
		}
		result[i] = bit
	}
	return result
}

func (a *Agent) SourceQueries() int64 { return a.sourceQueries.Load() }
func (a *Agent) MessagesSent() int64  { return a.messagesSent.Load() }

func (a *Agent) RecordPrimaryQuery()   { a.primaryQueries.Add(1) }
func (a *Agent) PrimaryQueries() int64 { return a.primaryQueries.Load() }

func (a *Agent) RecordVerifyQuery()   { a.verifyQueries.Add(1) }
func (a *Agent) VerifyQueries() int64 { return a.verifyQueries.Load() }
