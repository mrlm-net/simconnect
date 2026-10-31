package world

import (
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

const (
	// landAheadNM: an arrival this close to the threshold, ahead in the
	// sequence, is the traffic landing ahead.
	landAheadNM = 3.0
	// finalTrafficNM: the furthest an arrival is named as traffic on final.
	finalTrafficNM = 10.0
)

// qualified is clearance t with what the player asked of it (#1331): a take-off
// "?immediate=1" or "?nodelay=1", a limit "?taxiway=B".
func (it *controlled) qualified(action string, t traffic.Transmission, r *http.Request) traffic.Transmission {
	if s := r.URL.Query().Get("taxiway"); action == "upto" && s != "" {
		return traffic.HoldShortOf(t, strings.ToUpper(s))
	}
	if action != "takeoff" {
		return t
	}
	switch q := r.URL.Query(); {
	case q.Get("immediate") == "1":
		return traffic.Rushed(t)
	case q.Get("nodelay") == "1":
		return traffic.NoDelayed(t, it.finalNM())
	}
	return t
}

// finalNM is how far the nearest arrival on final of its runway is from the
// threshold, 0 none within finalTrafficNM.
func (it *controlled) finalNM() float64 {
	if it.graph == nil {
		return 0
	}
	it.mu.Lock()
	rwy := it.view.Runway
	it.mu.Unlock()
	_, end, ok := it.graph.Layout.RunwayEnd(rwy)
	if !ok {
		return 0
	}
	nm := 0.0
	for _, o := range it.cc.snapshotItems() {
		if o.arr == nil || o.ICAO != it.ICAO {
			continue
		}
		o.mu.Lock()
		v := o.view
		o.mu.Unlock()
		if v.Runway != rwy || v.State != traffic.ArrivalApproaching.String() && v.State != traffic.ArrivalLanding.String() {
			continue
		}
		d := calc.HaversineMeters(v.Position.Lat, v.Position.Lon, end.Threshold.Lat, end.Threshold.Lon) / 1852
		if d <= finalTrafficNM && (nm == 0 || d < nm) {
			nm = d
		}
	}
	return nm
}

// landAhead is the traffic ahead of a landing on its runway as said: a
// departure on its take-off roll "departing", an arrival on its landing roll or
// close ahead in the sequence "landing", "" none.
func (it *controlled) landAhead() string {
	if it.cc == nil {
		return ""
	}
	it.mu.Lock()
	rwy := it.view.Runway
	it.mu.Unlock()
	ahead := ""
	for _, o := range it.cc.snapshotItems() {
		if o == it || o.ICAO != it.ICAO {
			continue
		}
		o.mu.Lock()
		v := o.view
		o.mu.Unlock()
		switch {
		case v.Runway != rwy:
		case o.dep != nil && v.State == traffic.TaxiDeparting.String() && v.OnGround:
			return "departing"
		case o.arr != nil && v.State == traffic.ArrivalRollout.String():
			ahead = "landing"
		}
	}
	if lead, togo, _ := it.leaderToGo(); ahead == "" && lead != "" && togo <= landAheadNM {
		ahead = "landing"
	}
	return ahead
}

// crossingAt is the taxiway meeting node other than the one it is on, "" none:
// the name of a limit there.
func (it *controlled) crossingAt(node airport.NodeID, on string) string {
	if it.graph == nil {
		return on
	}
	for _, e := range it.graph.Adj[node] {
		if e.Name != "" && e.Name != on {
			return e.Name
		}
	}
	return ""
}

// holdShortNode is the node where its taxi route next meets taxiway, for a
// limit given by name: "taxi via A, hold short of B" (#1331).
func (it *controlled) holdShortNode(taxiway string) (airport.NodeID, error) {
	var r *airport.Route
	if it.dep != nil {
		r = it.dep.Route()
	} else if p := it.arr.Plan(); p != nil {
		r = p.TaxiRoute()
	}
	if r == nil || it.graph == nil {
		return -1, fmt.Errorf("%s: no taxi route", it.Tail)
	}
	it.mu.Lock()
	pos := it.view.Position
	it.mu.Unlock()
	from, best := 0, math.Inf(1)
	for i, n := range r.Nodes {
		p := it.graph.Nodes[n].Position
		if d := calc.HaversineMeters(pos.Lat, pos.Lon, p.Lat, p.Lon); d < best {
			from, best = i, d
		}
	}
	for i := from + 1; i < len(r.Nodes); i++ {
		if strings.EqualFold(r.Edges[i-1].Name, taxiway) {
			continue // on it already
		}
		for _, e := range it.graph.Adj[r.Nodes[i]] {
			if strings.EqualFold(e.Name, taxiway) {
				return r.Nodes[i], nil
			}
		}
	}
	return -1, fmt.Errorf("%s: its route does not meet taxiway %s", it.Tail, taxiway)
}

// throughSaid is a level to expedite through as said, "" none.
func throughSaid(ft, transitionFt float64) string {
	if ft <= 0 {
		return ""
	}
	return traffic.LevelSaidAbove(ft, transitionFt)
}
