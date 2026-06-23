package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
)

func (a *Agent) Handler() http.Handler {
	mux := http.NewServeMux()

	// /getbit: serves a stored bit; Byzantine agents flip it.
	mux.HandleFunc("/getbit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		index, err := strconv.Atoi(r.URL.Query().Get("index"))
		if err != nil {
			http.Error(w, "missing or invalid index", http.StatusBadRequest)
			return
		}

		bit, ok := a.GetBit(index)
		if !ok {
			http.Error(w, "bit not found", http.StatusNotFound)
			return
		}

		if a.IsByzantine {
			bit ^= 1
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(BitMessage{Index: index, Bit: bit}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	// /message: receives a round-keyed message (votes, proposals, …).
	mux.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var msg Message
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		a.StoreMessage(msg)
		w.WriteHeader(http.StatusOK)
	})

	return mux
}

func (a *Agent) StartServer() (*http.Server, error) {
	addr := fmt.Sprintf(":%d", a.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	server := &http.Server{
		Handler:  a.Handler(),
		ErrorLog: log.New(io.Discard, "", 0),
	}
	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Printf("[agent %d] server error: %v\n", a.ID, err)
		}
	}()
	return server, nil
}
