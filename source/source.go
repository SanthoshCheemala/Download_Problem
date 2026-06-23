package source

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"sync/atomic"
)

// Source stores the n-bit input array X = [b_1, ..., b_n] and answers
// Query(i) with the true bit b_i. One byte per bit, each 0 or 1.
type Source struct {
	bits       []byte
	totalBits  int
	queryCount atomic.Int64
}

type BitResponse struct {
	Index int  `json:"index"`
	Bit   byte `json:"bit"`
}

func New(totalBits int, seed int64) *Source {
	if totalBits < 1 {
		totalBits = 1
	}

	bits := make([]byte, totalBits)
	rng := rand.New(rand.NewSource(seed))
	for i := range bits {
		bits[i] = byte(rng.Intn(2))
	}

	return &Source{
		bits:      bits,
		totalBits: totalBits,
	}
}

func (s *Source) BitCount() int {
	return s.totalBits
}

func (s *Source) QueryCount() int64 {
	return s.queryCount.Load()
}

func (s *Source) ResetQueryCount() {
	s.queryCount.Store(0)
}

// Object returns a copy of the full input bit array; simulator-only ground truth.
func (s *Source) Object() []byte {
	copied := make([]byte, len(s.bits))
	copy(copied, s.bits)
	return copied
}

func (s *Source) GetBit(index int) (byte, bool) {
	if index < 0 || index >= s.totalBits {
		return 0, false
	}
	return s.bits[index], true
}

func (s *Source) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/bit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		index, err := strconv.Atoi(r.URL.Query().Get("index"))
		if err != nil {
			http.Error(w, "missing or invalid index", http.StatusBadRequest)
			return
		}

		bit, ok := s.GetBit(index)
		if !ok {
			http.Error(w, "bit not found", http.StatusNotFound)
			return
		}

		s.queryCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(BitResponse{Index: index, Bit: bit}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
	return mux
}

func (s *Source) Start(port int) (*http.Server, error) {
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	server := &http.Server{
		Handler:  s.Handler(),
		ErrorLog: log.New(io.Discard, "", 0),
	}
	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Printf("[source] server error: %v\n", err)
		}
	}()
	return server, nil
}
