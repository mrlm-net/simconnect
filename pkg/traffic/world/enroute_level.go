package world

import (
	"fmt"
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// En route traffic is planned clear of the others before it appears: its
// way across, at its cruise level, is flown ahead enrouteLookAhead against
// everyone's planned ways, and where it would meet one it is given another
// level of its direction (the user, live: DLH1050 and THY1880 created in
// the same second at FL400 on converging tracks, "they'll cross both at
// FL400"); with none clear it waits (ErrSpawnBlocked) and is tried again.
// Where it appears was all that was looked at.

// enrouteLookAhead: how far ahead an en route flight's way is flown against
// the others before it appears.
const enrouteLookAhead = 20 * time.Minute

// enrouteLevelSteps are the levels tried after the planned one, in feet off
// it: the same direction's levels (2000 ft apart), lower first.
var enrouteLevelSteps = []float64{-2000, 2000, -4000, 4000}

// enrouteTopFt: no level above this is given.
const enrouteTopFt = 41000.0

// clearLevel is route with its cruise level moved where it meets nobody
// within enrouteLookAhead, and the one it would meet at the planned level
// ("" none); ok false when no level is clear.
func (s *scheduler) clearLevel(callsign string, route []traffic.RoutePoint) ([]traffic.RoutePoint, string, bool) {
	if len(route) < 2 {
		return route, "", true
	}
	others, plans := s.plannedTraffic(callsign)
	cruise := route[0].AltFt
	first := ""
	for _, step := range append([]float64{0}, enrouteLevelSteps...) {
		ft := cruise + step
		if step != 0 && (cruise < 10000 || ft < 10000 || ft > enrouteTopFt) {
			continue
		}
		r := atLevel(route, cruise, ft)
		who := meetsAlong(callsign, r, others, plans)
		if who == "" {
			return r, first, true
		}
		if step == 0 {
			first = who
		}
	}
	return route, first, false
}

// atLevel is route with its points at cruise (its cruise part) at ft.
func atLevel(route []traffic.RoutePoint, cruise, ft float64) []traffic.RoutePoint {
	out := append([]traffic.RoutePoint(nil), route...)
	if ft == cruise {
		return out
	}
	for i := range out {
		if out[i].AltFt == cruise {
			out[i].AltFt = ft
		}
	}
	return out
}

// meetsAlong names who of others a flight appearing at route's first point
// and flying it would meet (ConflictOptions with each one's plan); "" none.
func meetsAlong(callsign string, route []traffic.RoutePoint, others []traffic.TrackedAircraft, plans map[string][]traffic.RoutePoint) string {
	if len(others) == 0 {
		return ""
	}
	p0, p1 := route[0], route[1]
	me := traffic.TrackedAircraft{Observation: traffic.Observation{Tail: callsign, Position: p0.Position, AltFt: p0.AltFt, GroundKts: p0.Kts,
		Heading: calc.BearingDegrees(p0.Position.Lat, p0.Position.Lon, p1.Position.Lat, p1.Position.Lon)}, Phase: traffic.PhaseEnroute, Ours: true}
	all := append([]traffic.TrackedAircraft{me}, others...)
	opts := conflictOpts
	opts.LookAhead = enrouteLookAhead
	opts.Profile = func(a traffic.TrackedAircraft) []traffic.RoutePoint {
		if a.Tail == callsign {
			return route
		}
		return plans[a.Tail]
	}
	for _, c := range traffic.PredictConflicts(all, opts) {
		switch callsign {
		case c.A:
			return c.B
		case c.B:
			return c.A
		}
	}
	return ""
}

// plannedTraffic is everyone airborne but callsign, and the planned way
// ahead of ours (en route on their plan, departures on their climb,
// arrivals on their STAR); the others are flown on as they are. The en
// route flights just asked for, not reported yet, are there at their first
// point.
func (s *scheduler) plannedTraffic(callsign string) ([]traffic.TrackedAircraft, map[string][]traffic.RoutePoint) {
	plans := map[string][]traffic.RoutePoint{}
	var out []traffic.TrackedAircraft
	seen := map[string]bool{}
	s.mu.Lock()
	enroute := map[string]*enrouteAC{}
	for _, e := range s.enroute {
		enroute[e.f.Callsign] = e
	}
	var pending []*enrouteAC
	for _, e := range s.pending {
		pending = append(pending, e)
	}
	s.mu.Unlock()
	for _, a := range s.cc.world.Aircraft() {
		if a.Grounded() || a.Tail == callsign {
			continue
		}
		seen[a.Tail] = true
		out = append(out, a)
		if !a.Ours {
			continue
		}
		if e := enroute[a.Tail]; e != nil && e.objectID == a.ObjectID && len(e.route) > 1 {
			plans[a.Tail] = traffic.ProfileAhead(a.Position, e.route)
			continue
		}
		it := s.cc.byTail(a.Tail)
		if it == nil || it.object() != a.ObjectID {
			continue
		}
		// On a resolution's heading or level: flown on as it is, not along
		// its route (live, LKPR: DLH620 on "heading 080, due traffic"
		// predicted on its SID, FTMKL cleared to FL240 into it).
		if s.engaged != nil && s.engaged(a.Tail, s.cc.clock.Now()) {
			continue
		}
		switch {
		case it.dep != nil:
			plans[a.Tail] = reachable(a, it.dep.ClimbPlan(a.Position))
		case it.arr != nil && it.arr.State() == traffic.ArrivalApproaching:
			plans[a.Tail] = it.arr.ProcedurePlan()
		}
	}
	for _, e := range append(pending, mapValues(enroute)...) {
		if seen[e.f.Callsign] || e.f.Callsign == callsign || len(e.route) < 2 {
			continue
		}
		seen[e.f.Callsign] = true
		p0, p1 := e.route[0], e.route[1]
		out = append(out, traffic.TrackedAircraft{Observation: traffic.Observation{Tail: e.f.Callsign, Position: p0.Position, AltFt: p0.AltFt, GroundKts: p0.Kts,
			Heading: calc.BearingDegrees(p0.Position.Lat, p0.Position.Lon, p1.Position.Lat, p1.Position.Lon)}, Phase: traffic.PhaseEnroute, Ours: true})
		plans[e.f.Callsign] = e.route
	}
	return out, plans
}

func mapValues(m map[string]*enrouteAC) []*enrouteAC {
	out := make([]*enrouteAC, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

// flLog is ft as a flight level for the log: "FL380".
func flLog(ft float64) string {
	return fmt.Sprintf("FL%03d", int(math.Round(ft/100)))
}
