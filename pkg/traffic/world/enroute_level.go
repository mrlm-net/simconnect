package world

import (
	"fmt"
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// En route traffic is planned clear of the others before it appears: its
// way across, at its cruise level, is flown ahead (wayLookAhead) against
// everyone's planned ways, and where it would meet one it is given another
// level of its direction (the user, live: DLH1050 and THY1880 created in
// the same second at FL400 on converging tracks, "they'll cross both at
// FL400"); with none clear it waits (ErrSpawnBlocked) and is tried again.
// Where it appears was all that was looked at.

// enrouteLookAhead: how far ahead an en route flight's way is flown against
// the others before it appears, at least: its whole way and enrouteLookPast
// on when that is longer, up to enrouteLookAheadMax. An overflight takes
// 30 min and more across the area and out (offline, LKPR at 60 an hour:
// BAW1799, planned 20 min ahead, met DLH298 22 min after it appeared,
// leaving the area). Past its way's end it flies on a few minutes more
// before it is removed, and the conflict watch looks 5 min past that
// (offline, LZKZ: UAE1710 planned clear of RYR226 to 8.8 NM at the end of
// its way, predicted 4.4 NM 5 min on from its last minute).
const (
	enrouteLookAhead    = 20 * time.Minute
	enrouteLookPast     = 10 * time.Minute
	enrouteLookAheadMax = 50 * time.Minute
)

// enrouteBufferNM: a way is planned clear with this to spare on the lateral
// minima, a prediction off by a little not a conflict once flown (offline,
// LKPR: BAW904 planned 5.05 NM from QTR1311, the conflict watch's 5.0 NM
// four minutes before; as climbOpts).
const enrouteBufferNM = 1.0

// wayLookAhead is how far ahead route is flown against the others: the
// time it takes along its legs (each at the speed of the point it heads
// for) and enrouteLookPast, within enrouteLookAhead and enrouteLookAheadMax.
func wayLookAhead(route []traffic.RoutePoint) time.Duration {
	var hours float64
	for i := 1; i < len(route); i++ {
		a, b := route[i-1], route[i]
		kts := b.Kts
		if kts <= 0 {
			kts = a.Kts
		}
		if kts <= 0 {
			continue
		}
		hours += calc.HaversineNM(a.Position.Lat, a.Position.Lon, b.Position.Lat, b.Position.Lon) / kts
	}
	d := time.Duration(hours*float64(time.Hour)) + enrouteLookPast
	return min(max(d, enrouteLookAhead), enrouteLookAheadMax)
}

// enrouteLevelSteps are the levels tried after the planned one, in feet off
// it: the same direction's levels (2000 ft apart), lower first.
var enrouteLevelSteps = []float64{-2000, 2000, -4000, 4000}

// enrouteTopFt: no level above this is given.
const enrouteTopFt = 41000.0

// clearLevel is route with its cruise level moved where it meets nobody
// along its way (wayLookAhead), and the one it would meet at the planned level
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

// legKts is the speed route's first leg is flown at, as its way is
// predicted: the speed of the point it heads for, as MSFS AI flies a leg
// (enrouteChain), else its first point's. An arrival appearing below FL100
// at 250 kt flies on at its cruise speed: predicted at 250 kt all the way,
// OKILI, climbing out of 7300 ft at 460 kt, was planned 15 NM and more
// from SWR1395 head-on at FL350, which it met in 4 min (offline, LKPR).
func legKts(route []traffic.RoutePoint) float64 {
	if len(route) > 1 && route[1].Kts > 0 {
		return route[1].Kts
	}
	if len(route) > 0 {
		return route[0].Kts
	}
	return 0
}

// meetsAlong names who of others a flight appearing at route's first point
// and flying it would meet (ConflictOptions with each one's plan, over its
// way's look-ahead, with enrouteBufferNM to spare), the soonest; "" none.
func meetsAlong(callsign string, route []traffic.RoutePoint, others []traffic.TrackedAircraft, plans map[string][]traffic.RoutePoint) string {
	if len(others) == 0 {
		return ""
	}
	p0, p1 := route[0], route[1]
	me := traffic.TrackedAircraft{Observation: traffic.Observation{Tail: callsign, Position: p0.Position, AltFt: p0.AltFt, GroundKts: legKts(route),
		Heading: calc.BearingDegrees(p0.Position.Lat, p0.Position.Lon, p1.Position.Lat, p1.Position.Lon)}, Phase: traffic.PhaseEnroute, Ours: true}
	opts := conflictOpts
	opts.LookAhead = wayLookAhead(route)
	opts.MinNM, opts.TerminalNM = sepMinNM+enrouteBufferNM, sepTerminalNM+enrouteBufferNM
	opts.Profile = func(a traffic.TrackedAircraft) []traffic.RoutePoint {
		if a.Tail == callsign {
			return route
		}
		return plans[a.Tail]
	}
	// Pair by pair: the others among themselves are not looked at.
	who, soonest := "", time.Duration(math.MaxInt64)
	for _, o := range others {
		for _, c := range traffic.PredictConflicts([]traffic.TrackedAircraft{me, o}, opts) {
			if c.In >= soonest {
				continue
			}
			switch callsign {
			case c.A:
				who, soonest = c.B, c.In
			case c.B:
				who, soonest = c.A, c.In
			}
		}
	}
	return who
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
	// The virtual overflights (#1482): where each is now, on its way ahead.
	virtual, ways := s.virtualAircraft(s.cc.clock.Now())
	for _, a := range virtual {
		if seen[a.Tail] || a.Tail == callsign {
			continue
		}
		seen[a.Tail] = true
		out = append(out, a)
		if w := ways[a.Tail]; len(w) > 0 {
			plans[a.Tail] = w
		}
	}
	for _, e := range append(pending, mapValues(enroute)...) {
		if seen[e.f.Callsign] || e.f.Callsign == callsign || len(e.route) < 2 {
			continue
		}
		seen[e.f.Callsign] = true
		p0, p1 := e.route[0], e.route[1]
		out = append(out, traffic.TrackedAircraft{Observation: traffic.Observation{Tail: e.f.Callsign, Position: p0.Position, AltFt: p0.AltFt, GroundKts: legKts(e.route),
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
