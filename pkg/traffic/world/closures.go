package world

import (
	"encoding/json"
	"math"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
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

// closedGoAroundNM: ours on a final this close to a runway that closes go
// around; farther out they are re-sequenced (another runway) or held.
const closedGoAroundNM = 4.0

// An airport with every runway closed (closures.go): our arrivals still in
// the air hold at their STAR's fix ("all runways at LKPR are closed, expect
// holding"); a runway open again, the sequence releases them as from any
// hold. Held divertAfter, each diverts to the nearest other airport
// (the worldwide list, divertMinNM or more away): sent direct toward it and
// gone once divertAwayNM from the airport (or after divertGoneAfter).
const (
	divertAfter     = 20 * time.Minute
	divertMinNM     = 30.0
	divertAwayNM    = 25.0
	divertGoneAfter = 8 * time.Minute
	divertAheadNM   = 40.0 // the point it is sent direct to, toward the alternate
)

// closedHold is an arrival held for a closed airport.
type closedHold struct {
	icao      string
	since     time.Time
	to        string    // the alternate, once diverting
	divertsAt time.Time // when the diversion was given
}

// alternate is the nearest airport to icao's position at least divertMinNM
// away with a four-letter ICAO code; "" none known.
func (q *sequences) alternate(icao string, at airport.LatLon) string {
	st := q.s.st
	st.mu.Lock()
	defer st.mu.Unlock()
	best, bestD := "", math.Inf(1)
	for code, r := range st.airportRefs {
		if code == icao || len(code) != 4 || strings.IndexFunc(code, func(c rune) bool { return c < 'A' || c > 'Z' }) >= 0 {
			continue
		}
		d := calc.HaversineNM(at.Lat, at.Lon, r.Position.Lat, r.Position.Lon)
		if d >= divertMinNM && d < bestD {
			best, bestD = code, d
		}
	}
	return best
}

// closedAirport handles arrival it at icao with every runway closed: true
// when it did (held, diverting or gone).
func (q *sequences) closedAirport(now time.Time, icao string, it *controlled, e traffic.SequenceEntry) bool {
	g, err := q.cc.graph(icao)
	if err != nil {
		return false
	}
	field := airport.LatLon{Lat: g.Layout.Latitude, Lon: g.Layout.Longitude}
	q.mu.Lock()
	if q.closedHolds == nil {
		q.closedHolds = map[string]*closedHold{}
	}
	ch := q.closedHolds[e.Callsign]
	q.mu.Unlock()
	if ch != nil && ch.to != "" {
		it.mu.Lock()
		pos := it.view.Position
		it.mu.Unlock()
		if calc.HaversineNM(pos.Lat, pos.Lon, field.Lat, field.Lon) >= divertAwayNM || now.Sub(ch.divertsAt) >= divertGoneAfter {
			q.cc.log.printf("%-6s diverted to %s: out of the area", e.Callsign, ch.to)
			_ = q.cc.remove(it)
			q.mu.Lock()
			delete(q.closedHolds, e.Callsign)
			q.mu.Unlock()
		}
		return true
	}
	if _, _, holding := it.arr.Holding(); holding {
		if ch == nil {
			ch = &closedHold{icao: icao, since: now}
			q.mu.Lock()
			q.closedHolds[e.Callsign] = ch
			q.mu.Unlock()
		}
		if now.Sub(ch.since) < divertAfter {
			return true // kept holding while closed
		}
		to := q.alternate(icao, field)
		if to == "" {
			return true // nowhere known to go: holds on
		}
		if err := q.cc.do(it.arr.LeaveHold); err != nil {
			return true
		}
		st := q.s.st
		st.mu.Lock()
		r := st.airportRefs[to]
		st.mu.Unlock()
		brg := calc.BearingDegrees(field.Lat, field.Lon, r.Position.Lat, r.Position.Lon)
		lat, lon := calc.DisplaceByHeading(field.Lat, field.Lon, brg, divertAheadNM*1852)
		_ = q.cc.do(func() error { _, _, err := it.arr.DirectTo(airport.LatLon{Lat: lat, Lon: lon}); return err })
		ch.to, ch.divertsAt = to, now
		tx := traffic.Divert(e.Callsign, icao, to)
		it.call(traffic.PosApproach, prioApproach, func() { it.say(tx) })
		q.cc.log.printf("%-6s %s closed %s: diverting to %s", e.Callsign, icao, now.Sub(ch.since).Round(time.Minute), to)
		return true
	}
	if ch != nil {
		return false // told already, no fix to hold at: on as it goes
	}
	// Not holding yet: told once, then held at its STAR's fix.
	tx := traffic.AirportClosed(e.Callsign, icao)
	it.call(traffic.PosApproach, prioApproach, func() { it.say(tx) })
	q.mu.Lock()
	q.closedHolds[e.Callsign] = &closedHold{icao: icao, since: now}
	q.mu.Unlock()
	return q.enterHold(now, icao, it, e, divertAfter) == nil
}

// openAgain forgets the closure holds at icao (a runway open again): the
// sequence releases them as from any hold.
func (q *sequences) openAgain(icao string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for cs, ch := range q.closedHolds {
		if ch.icao == icao && ch.to == "" {
			delete(q.closedHolds, cs)
		}
	}
}

// closedClearances is the tower's decision on a closed runway: no line-up,
// take-off or landing; ours on a final within closedGoAroundNM go around;
// crossing stays allowed.
func closedClearances(c *traffic.RunwayClearances, list []traffic.RunwayUser) {
	c.LineUp, c.Takeoff, c.Land, c.LineUpBehind, c.CrossBehind, c.LineUpBehindDeparting = nil, nil, nil, nil, nil, nil
	if c.Waiting == nil {
		c.Waiting = map[string]string{}
	}
	for _, u := range list {
		if u.Other {
			continue
		}
		if u.Arrival && u.Phase == traffic.RunwayFinal && u.DistanceNM <= closedGoAroundNM && !slices.Contains(c.GoAround, u.Callsign) {
			c.GoAround = append(c.GoAround, u.Callsign)
		}
		c.Waiting[u.Callsign] = "runway closed"
	}
}
