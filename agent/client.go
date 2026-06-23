package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const broadcastParallelism = 16

// BitMessage is the wire format for a single bit exchanged with the source.
type BitMessage struct {
	Index int  `json:"index"`
	Bit   byte `json:"bit"`
}

// QueryBit fetches bit i from the trusted source and increments the Q counter.
func (a *Agent) QueryBit(ctx context.Context, sourcePort, bitIndex int) (byte, error) {
	a.sourceQueries.Add(1)
	if a.sourceDelay > 0 {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(a.sourceDelay):
		}
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/bit?index=%d", sourcePort, bitIndex)
	resp, err := a.get(ctx, url)
	if err != nil {
		return 0, fmt.Errorf("agent %d query source bit %d: %w", a.ID, bitIndex, err)
	}
	defer drainAndClose(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("agent %d query source bit %d: status %s", a.ID, bitIndex, resp.Status)
	}

	var msg BitMessage
	if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
		return 0, err
	}
	if msg.Index != bitIndex {
		return 0, fmt.Errorf("agent %d query source bit %d: received bit %d", a.ID, bitIndex, msg.Index)
	}
	return msg.Bit, nil
}

// Broadcast sends msg to all peers and stores it locally so own vote counts in tallies.
// Only honest sends increment the M counter.
func (a *Agent) Broadcast(ctx context.Context, peerPorts []int, msg Message) error {
	a.StoreMessage(msg)

	errCh := make(chan error, len(peerPorts))
	sem := make(chan struct{}, broadcastParallelism)
	var wg sync.WaitGroup
	for _, port := range peerPorts {
		if port == a.Port {
			continue
		}

		sem <- struct{}{}
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := a.sendMessage(ctx, port, msg); err != nil {
				errCh <- err
				return
			}
			if !a.IsByzantine {
				a.messagesSent.Add(1)
			}
		}(port)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *Agent) sendMessage(ctx context.Context, peerPort int, msg Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/message", peerPort)
	resp, err := a.post(ctx, url, body)
	if err != nil {
		return err
	}
	defer drainAndClose(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("peer %d returned %s", peerPort, resp.Status)
	}
	return nil
}

func (a *Agent) get(ctx context.Context, url string) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}

		resp, err := a.httpClient().Do(req)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		delay := time.Duration(25*(attempt+1)) * time.Millisecond
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, lastErr
}

func (a *Agent) post(ctx context.Context, url string, body []byte) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := a.httpClient().Do(req)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		delay := time.Duration(25*(attempt+1)) * time.Millisecond
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, lastErr
}

func drainAndClose(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()
}
