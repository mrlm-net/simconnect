//go:build windows
// +build windows

package world

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Push (server-sent events): GET /api/events tells each open map what has
// changed ("control": our aircraft, a clearance, a state; "radio": a
// transmission), so it fetches that at once instead of waiting for its next
// poll. The polls stay as a fallback, slower while the stream is up.
// Changes are gathered for pushEvery: a burst is one event per topic.

const pushEvery = 150 * time.Millisecond

type pushHub struct {
	mu      sync.Mutex
	subs    map[chan string]bool
	pending map[string]bool
	timer   *time.Timer
}

var hub = &pushHub{subs: map[chan string]bool{}, pending: map[string]bool{}}

// publish marks topic changed; subscribers hear it within pushEvery.
func (h *pushHub) publish(topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.subs) == 0 {
		return
	}
	h.pending[topic] = true
	if h.timer == nil {
		h.timer = time.AfterFunc(pushEvery, h.flush)
	}
}

func (h *pushHub) flush() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.timer = nil
	for topic := range h.pending {
		for c := range h.subs {
			select {
			case c <- topic:
			default: // a slow client misses one: its poll still comes
			}
		}
	}
	clear(h.pending)
}

func (h *pushHub) subscribe() chan string {
	c := make(chan string, 16)
	h.mu.Lock()
	h.subs[c] = true
	h.mu.Unlock()
	return c
}

func (h *pushHub) unsubscribe(c chan string) {
	h.mu.Lock()
	delete(h.subs, c)
	h.mu.Unlock()
}

func registerPush(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		c := hub.subscribe()
		defer hub.unsubscribe(c)
		fmt.Fprint(w, "retry: 3000\n\n")
		fl.Flush()
		beat := time.NewTicker(15 * time.Second) // keeps idle connections open
		defer beat.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case topic := <-c:
				fmt.Fprintf(w, "event: %s\ndata: {}\n\n", topic)
			case <-beat.C:
				fmt.Fprint(w, ": beat\n\n")
			}
			fl.Flush()
		}
	})
}
