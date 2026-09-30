//go:build windows
// +build windows

package main

import (
	"math"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The towers on the map (#393): a runway controller per airport and
// runway clears our departures to line up and take off, and our traffic
// to cross, by what the runway is doing — our arrivals, departures and
// crossings, and respected other traffic — and sends an arrival around
// when the runway is not free on short final (#394), to be sequenced again. Aircraft spawned with "hold at
// every clearance" are the user's: counted, never cleared.
//
//	GET /api/runways?icao=LKPR — each runway's users and who waits for what

type towers struct {
	cc *controlCenter
	s  *scheduler

	mu      sync.Mutex
	ctl     map[string]*traffic.RunwayController // by "ICAO runway"
	given   map[string]bool                      // clearances given: "tail action"
	waiting map[string]string                    // why each waits, as last logged
	last    map[string][]runwayUserView          // the users, for the API
}

type runwayUserView struct {
	Callsign string `json:"callsign"`
	Phase    string `json:"phase"`
	Waiting  string `json:"waiting,omitempty"`
}

func newTowers(cc *controlCenter, s *scheduler) *towers {
	return &towers{cc: cc, s: s, ctl: map[string]*traffic.RunwayController{}, given: map[string]bool{}, waiting: map[string]string{}, last: map[string][]runwayUserView{}}
}

var phaseNames = map[traffic.RunwayPhase]string{traffic.RunwayHoldingShort: "holding short", traffic.RunwayLinedUp: "lined up",
	traffic.RunwayRolling: "on the runway", traffic.RunwayAirborne: "airborne", traffic.RunwayFinal: "final"}

// runwayOf is the runway of a runway end or runway name ("06", "06/24").
func runwayOf(l *airport.Layout, name string) (airport.Runway, bool) {
	for _, r := range l.Runways {
		if r.Name() == name || r.Primary.Name == name || r.Secondary.Name == name {
			return r, true
		}
	}
	return airport.Runway{}, false
}

// onRunway reports whether p is on the runway's surface (and a margin).
func onRunway(r airport.Runway, p airport.LatLon) bool {
	a, b := r.Primary.Threshold, r.Secondary.Threshold
	along := calc.AlongTrackMeters(a.Lat, a.Lon, b.Lat, b.Lon, p.Lat, p.Lon)
	cross := math.Abs(calc.CrossTrackMeters(a.Lat, a.Lon, b.Lat, b.Lon, p.Lat, p.Lon))
	return along > -100 && along < r.Length+100 && cross < r.Width/2+15
}

func (t *towers) tick(now time.Time) {
	t.cc.mu.Lock()
	items := make([]*controlled, 0, len(t.cc.items))
	for _, it := range t.cc.items {
		items = append(items, it)
	}
	t.cc.mu.Unlock()
	type key struct{ icao, rwy string }
	users := map[key][]traffic.RunwayUser{}
	ours := map[string]*controlled{}
	graphs := map[string]*airport.Layout{}
	layout := func(icao string) *airport.Layout {
		if l, ok := graphs[icao]; ok {
			return l
		}
		g, err := t.cc.graph(icao)
		if err != nil {
			graphs[icao] = nil
			return nil
		}
		graphs[icao] = g.Layout
		return g.Layout
	}
	for _, it := range items {
		l := layout(it.ICAO)
		if l == nil {
			continue
		}
		it.mu.Lock()
		v := it.view
		it.mu.Unlock()
		if v.Done {
			continue
		}
		if v.State != "holding short" {
			t.forget(v.Tail) // the next crossing is cleared afresh
		}
		model := v.Model
		u := traffic.RunwayUser{Callsign: v.Tail, Wake: traffic.WakeFor(model), Route: v.Procedure, Other: it.gates}
		own, ok := runwayOf(l, v.Runway)
		if !ok {
			continue
		}
		// Taxiing across a runway (cleared, or not waiting to be): on it.
		if v.State == "taxiing" && v.OnGround {
			for _, r := range l.Runways {
				if onRunway(r, v.Position) {
					u.Phase, u.Crossing = traffic.RunwayRolling, true
					users[key{it.ICAO, r.Name()}] = append(users[key{it.ICAO, r.Name()}], u)
				}
			}
			continue
		}
		rk := key{it.ICAO, own.Name()}
		switch {
		case v.State == "holding short" && v.HoldingShortOf != "" && v.HoldingShortOf != own.Name():
			// Crossing another runway.
			x, ok := runwayOf(l, v.HoldingShortOf)
			if !ok {
				continue
			}
			u.Phase, u.Crossing = traffic.RunwayHoldingShort, true
			rk = key{it.ICAO, x.Name()}
		case it.dep != nil && v.State == "holding short":
			u.Phase = traffic.RunwayHoldingShort
		case it.dep != nil && (v.State == "lining up" || v.State == "lined up"):
			u.Phase = traffic.RunwayLinedUp
		case it.dep != nil && v.State == "departing" && v.OnGround:
			u.Phase = traffic.RunwayRolling
		case it.dep != nil && v.State == "departing":
			u.Phase = traffic.RunwayAirborne
		case it.arr != nil && (v.State == "approaching" || v.State == "landing") && !v.OnGround:
			_, end, _ := l.RunwayEnd(v.Runway)
			d := calc.HaversineNM(v.Position.Lat, v.Position.Lon, end.Threshold.Lat, end.Threshold.Lon)
			if d > 3 {
				t.forgetGoAround(v.Tail) // out again: another go-around may follow
			}
			if d > 20 {
				continue
			}
			u.Phase, u.Arrival, u.DistanceNM, u.GroundKts = traffic.RunwayFinal, true, d, v.GroundSpeed
			// Established: its STAR and approach flown, on the final (#486).
			u.Established = it.objectID != 0 && len(it.arr.ProcedureRoute()) == 0
		case it.arr != nil && (v.State == "landing" || v.State == "rollout" || v.State == "vacating"):
			u.Phase, u.Arrival = traffic.RunwayRolling, true
		default:
			continue
		}
		ours[v.Tail] = it
		users[rk] = append(users[rk], u)
	}
	// Respected other traffic: on a runway, or arriving on the one in use.
	if t.s.mgr.Options().Others == traffic.OtherRespect {
		for _, icao := range t.s.mgr.Airports() {
			l := layout(icao)
			if l == nil {
				continue
			}
			g, _ := t.cc.graph(icao)
			arrRwy := t.cc.activeRunway(g, true)
			_, end, _ := l.RunwayEnd(arrRwy)
			for _, a := range t.s.mgr.Others(icao) {
				u := traffic.RunwayUser{Callsign: nameOf(a), Wake: traffic.WakeFor(a.Title), Other: true}
				switch a.Phase {
				case traffic.PhaseRunway:
					for _, r := range l.Runways {
						if onRunway(r, a.Position) {
							u.Phase = traffic.RunwayRolling
							users[key{icao, r.Name()}] = append(users[key{icao, r.Name()}], u)
						}
					}
				case traffic.PhaseArriving:
					if r, ok := runwayOf(l, arrRwy); ok {
						u.Phase, u.Arrival = traffic.RunwayFinal, true
						u.DistanceNM, u.GroundKts = calc.HaversineNM(a.Position.Lat, a.Position.Lon, end.Threshold.Lat, end.Threshold.Lon), a.GroundKts
						users[key{icao, r.Name()}] = append(users[key{icao, r.Name()}], u)
					}
				}
			}
		}
	}
	for k, list := range users {
		t.mu.Lock()
		rc := t.ctl[k.icao+" "+k.rwy]
		if rc == nil {
			rc = traffic.NewRunwayController(traffic.RunwayControllerOptions{})
			t.ctl[k.icao+" "+k.rwy] = rc
		}
		t.mu.Unlock()
		c := rc.Decide(now, list)
		t.apply(k.icao, k.rwy, c, ours)
		var view []runwayUserView
		for _, u := range list {
			view = append(view, runwayUserView{Callsign: u.Callsign, Phase: phaseNames[u.Phase], Waiting: c.Waiting[u.Callsign]})
		}
		t.mu.Lock()
		t.last[k.icao+" "+k.rwy] = view
		t.mu.Unlock()
	}
}

// apply gives the clearances (once each) and logs who waits for what.
func (t *towers) apply(icao, rwy string, c traffic.RunwayClearances, ours map[string]*controlled) {
	give := func(tail, action string, said traffic.Transmission, f func(it *controlled) error) {
		it := ours[tail]
		if it == nil || it.gates {
			return
		}
		t.mu.Lock()
		done := t.given[tail+" "+action]
		t.mu.Unlock()
		if done {
			return
		}
		// Said here: the state change it causes is not logged again.
		spoken := []string{strings.Fields(action)[0]}
		if action == "takeoff" {
			spoken = append(spoken, "lineup") // line up and take off in one
		}
		it.mu.Lock()
		for _, k := range spoken {
			it.spoken[k] = true
		}
		it.mu.Unlock()
		t.mu.Lock()
		t.given[tail+" "+action] = true
		if action == "takeoff" {
			t.given[tail+" lineup"] = true // no "line up and wait" after it
		}
		t.mu.Unlock()
		// Said first; the crew acts once it has read it back (#462).
		it.say(said)
		it.actAfterReadback(traffic.PosTower, "tower: "+action, func() error { return f(it) })
	}
	takeoff := map[string]bool{}
	for _, cs := range c.Takeoff {
		takeoff[cs] = true
	}
	end := func(tail string) string {
		if it := ours[tail]; it != nil {
			return it.view.Runway
		}
		return rwy
	}
	for _, cs := range c.LineUp {
		if takeoff[cs] {
			give(cs, "takeoff", traffic.ClearedTakeoff(cs, end(cs), t.cc.windSaid(icao)), func(it *controlled) error {
				it.dep.ClearToLineUp()
				return it.dep.ClearForTakeoff()
			})
			continue
		}
		give(cs, "lineup", traffic.ClearedLineUp(cs, end(cs)), func(it *controlled) error { it.dep.ClearToLineUp(); return nil })
	}
	for _, cs := range c.Takeoff {
		give(cs, "takeoff", traffic.ClearedTakeoff(cs, end(cs), t.cc.windSaid(icao)), func(it *controlled) error { return it.dep.ClearForTakeoff() })
	}
	// The next arrival, the runway free: cleared to land (#462); on the
	// landing roll it is told to call ground when vacated.
	for _, cs := range c.Land {
		give(cs, "land", traffic.ClearedToLand(cs, end(cs), t.cc.windSaid(icao)), func(it *controlled) error { return nil })
	}
	for _, cs := range c.Cross {
		// A crossing is cleared once per holding point: forget it once done.
		give(cs, "cross "+rwy, traffic.ClearedCross(cs, oneDesignator(rwy)), func(it *controlled) error {
			if it.dep != nil {
				it.dep.ClearToCross()
			} else {
				it.arr.ClearToCross()
			}
			return nil
		})
	}
	for _, cs := range c.GoAround {
		why := c.Waiting[cs]
		give(cs, "goaround", traffic.GoAround(cs, why), func(it *controlled) error {
			if it.arr == nil {
				return nil
			}
			if err := it.arr.GoAround(); err != nil {
				return err
			}
			t.forgetLanding(cs) // the next approach is cleared again (#486)
			if t.cc.rejoin != nil {
				t.cc.rejoin(icao, cs)
			}
			return nil
		})
	}
	// Who waits, and why: logged when it changes.
	names := make([]string, 0, len(c.Waiting))
	for cs := range c.Waiting {
		names = append(names, cs)
	}
	sort.Strings(names)
	for _, cs := range names {
		why := c.Waiting[cs]
		kind := waitKind(why) // "1m20s behind QTR1" counting down is the same wait
		t.mu.Lock()
		changed := t.waiting[cs] != kind
		t.waiting[cs] = kind
		t.mu.Unlock()
		if changed && ours[cs] != nil && !ours[cs].gates && !slices.Contains(c.GoAround, cs) {
			tlog.printf("%-6s tower %s: waits — %s", cs, rwy, why)
		}
	}
}

// waitKind is a wait reason without its numbers: the same wait while its
// time or distance counts down.
func waitKind(why string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' || r == '.' {
			return -1
		}
		return r
	}, why)
}

// forgetLanding lets an arrival that went around be cleared to land on its
// next approach (#486).
func (t *towers) forgetLanding(tail string) {
	t.mu.Lock()
	delete(t.given, tail+" land")
	t.mu.Unlock()
}

// forgetGoAround lets an arrival be sent around again on its next approach.
func (t *towers) forgetGoAround(tail string) {
	t.mu.Lock()
	delete(t.given, tail+" goaround")
	t.mu.Unlock()
}

// forget lets a crossing be cleared again at the next holding point once
// the aircraft moves on.
func (t *towers) forget(tail string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for k := range t.given {
		if strings.HasPrefix(k, tail+" cross ") {
			delete(t.given, k)
		}
	}
}

func registerRunways(mux *http.ServeMux, st *state) {
	mux.HandleFunc("GET /api/runways", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		t := st.towers
		st.mu.Unlock()
		out := map[string][]runwayUserView{}
		if t != nil {
			icao := strings.ToUpper(r.URL.Query().Get("icao"))
			t.mu.Lock()
			for k, v := range t.last {
				if i, rwy, _ := strings.Cut(k, " "); i == icao {
					out[rwy] = v
				}
			}
			t.mu.Unlock()
		}
		writeJSON(w, out)
	})
}
