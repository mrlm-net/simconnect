package world

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/nav"
)

// Runway closures (traffic ideas: works, a NOTAM). A closed runway is never
// chosen for departures or arrivals (nav.RunwayLimits.Closed): the traffic,
// its ATC, the ATIS and the host's player runway follow, and flights on it
// change runway as with a wind change. With none open at an airport, no new
// flights are started there until one opens.

type closures struct {
	mu sync.Mutex
	by map[string][]string // ICAO → runway names closed
}

func (k *core) closures() *closures {
	k.closedOnce.Do(func() { k.runwaysClosed = &closures{by: map[string][]string{}} })
	return k.runwaysClosed
}

// closed is icao's closed runways.
func (k *core) closed(icao string) []string {
	c := k.closures()
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.by[strings.ToUpper(icao)])
}

// CloseRunway closes runway (its name "06/24", or an end "24") at icao, or
// opens it again (closed false).
func (w *World) CloseRunway(icao, runway string, closed bool) {
	icao, runway = strings.ToUpper(strings.TrimSpace(icao)), strings.ToUpper(strings.TrimSpace(runway))
	c := w.st.core.closures()
	c.mu.Lock()
	list := slices.DeleteFunc(c.by[icao], func(s string) bool { return s == runway })
	if closed {
		list = append(list, runway)
	}
	if len(list) == 0 {
		delete(c.by, icao)
	} else {
		c.by[icao] = list
	}
	c.mu.Unlock()
	state := "open again"
	if closed {
		state = "closed"
	}
	w.st.core.log.printf("%s: runway %s %s", icao, runway, state)
}

// ClosedRunways are the runways closed at icao.
func (w *World) ClosedRunways(icao string) []string { return w.st.core.closed(icao) }

// allClosed reports whether every runway of l is closed.
func (k *core) allClosed(l *airport.Layout) bool {
	if l == nil || len(l.Runways) == 0 {
		return false
	}
	lim := nav.RunwayLimits{Closed: k.closed(l.ICAO)}
	for _, r := range l.Runways {
		if !lim.ClosedEnd(r.Primary.Name) && !lim.ClosedEnd(r.Secondary.Name) {
			return false
		}
	}
	return true
}

// registerClosures serves GET /api/closures?icao= (the closed runways) and
// POST /api/closures {icao, runway, closed}.
func registerClosures(mux *http.ServeMux, w *World) {
	mux.HandleFunc("GET /api/closures", func(rw http.ResponseWriter, r *http.Request) {
		writeJSON(rw, map[string]any{"closed": w.ClosedRunways(strings.ToUpper(r.URL.Query().Get("icao")))})
	})
	mux.HandleFunc("POST /api/closures", func(rw http.ResponseWriter, r *http.Request) {
		var req struct {
			ICAO   string `json:"icao"`
			Runway string `json:"runway"`
			Closed bool   `json:"closed"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ICAO == "" || req.Runway == "" {
			http.Error(rw, "icao and runway needed", http.StatusBadRequest)
			return
		}
		w.CloseRunway(req.ICAO, req.Runway, req.Closed)
		writeJSON(rw, map[string]any{"closed": w.ClosedRunways(req.ICAO)})
	})
}
