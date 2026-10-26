// Package events is an in-process pub/sub that feeds the SSE endpoint, so the frontend can
// refresh what changed instead of polling.
package events

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type Event struct {
	Type string `json:"type"` // e.g. "job.finished", "prices.updated", "todo.updated"
	Data any    `json:"data,omitempty"`
}

type Broker struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
}

func NewBroker() *Broker { return &Broker{subs: map[chan Event]struct{}{}} }

// Publish never blocks: a subscriber whose buffer is full misses the event (it refetches on the
// next one anyway).
func (b *Broker) Publish(typ string, data any) {
	if b == nil {
		return
	}
	ev := Event{Type: typ, Data: data}
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (b *Broker) subscribe() chan Event {
	ch := make(chan Event, 32)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Broker) unsubscribe(ch chan Event) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
}

// ServeSSE streams events as Server-Sent Events until the client disconnects.
func (b *Broker) ServeSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // nginx: do not buffer the stream

	ch := b.subscribe()
	defer b.unsubscribe(ch)

	_, _ = fmt.Fprint(w, "retry: 5000\n: connected\n\n")
	flusher.Flush()

	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case ev := <-ch:
			data, err := json.Marshal(ev)
			if err != nil {
				slog.Error("sse marshal", "err", err)
				continue
			}
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, data)
			flusher.Flush()
		}
	}
}
