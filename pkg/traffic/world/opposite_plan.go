package world

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Opposite directions on one runway (the user, 2026-11-27): a departure
// off one end keeps traffic.OppositeSeparationNM horizontally from an
// arrival to the other end, whatever their levels. Planned, not watched:
// before a take-off its way out is flown against the arrivals' ways in,
// and when straight out meets one it is given a turn after departure that
// keeps clear (held when none does); before an arrival appears its way
// in is flown against the departures off the other end cleared or
// airborne, and it appears only clear of them (a later release).
// TestDeclaredField had them 0.1 NM apart on the extended centreline,
// 5000 ft above each other.

const (
	// oppositeBufferNM: planned with this to spare on the minimum.
	oppositeBufferNM = 1.0
	// oppositeCornerPastEndNM: a turn after departure begins this far past
	// the far end of the runway; oppositeCornerLateNM later as well (a
	// slow climb handed over farther on) is planned clear too.
	oppositeCornerPastEndNM = 1.0
	oppositeCornerLateNM    = 2.0
	// oppositeReleaseKts: an arrival not here yet is planned at this speed
	// (and oppositeReleaseScales of it), oppositeCircuitKts in a circuit.
	oppositeReleaseKts = 130.0
	oppositeCircuitKts = 90.0
)

// oppositeTurnDeg are the turns off the take-off heading tried, and
// oppositeLegNM how far each is flown before back on the way out.
var (
	oppositeTurnDeg = []float64{30, 45, 60, 90}
	oppositeLegNM   = []float64{2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
)

// The predictions are flown at the speeds planned times these, a
// departure lifting off oppositeLiftoffIn from now: a little early or
// late, slow or fast, it still keeps clear.
var (
	oppositeDepScales     = []float64{0.8, 1, 1.25}
	oppositeArrScales     = []float64{0.85, 1, 1.15}
	oppositeReleaseScales = []float64{0.6, 1, 1.6}
	oppositeLiftoffIn     = []time.Duration{20 * time.Second, 60 * time.Second, 120 * time.Second}
)

// oppositeWay is a way in or out as the predictor flies it.
type oppositeWay struct {
	tail string
	path []traffic.RoutePoint
}

// oppositeClimbKts is a departure's speed nm from its lift-off as
// predicted: MSFS AI on its climb waypoints (TakeoffClimb, 200 to 280 kt).
func oppositeClimbKts(nm float64) float64 {
	switch {
	case nm < 2:
		return 180
	case nm < 6:
		return 240
	}
	return 280
}

// climbWay is a departure's path from start through pts at
// oppositeClimbKts, nm0 from its lift-off at start.
func climbWay(start airport.LatLon, nm0 float64, pts []airport.LatLon) []traffic.RoutePoint {
	out := []traffic.RoutePoint{{Position: start, Kts: oppositeClimbKts(nm0)}}
	nm, prev := nm0, start
	for _, p := range pts {
		nm += calc.HaversineNM(prev.Lat, prev.Lon, p.Lat, p.Lon)
		out = append(out, traffic.RoutePoint{Position: p, Kts: oppositeClimbKts(nm)})
		prev = p
	}
	return out
}

// farEnd is the other end of rwy from end.
func farEnd(rwy airport.Runway, end airport.RunwayEnd) airport.RunwayEnd {
	if rwy.Primary.Name == end.Name {
		return rwy.Secondary
	}
	return rwy.Primary
}

// liftoffAt is where a departure off end is predicted airborne
// (takeoffLiftoffNM down it).
func liftoffAt(end airport.RunwayEnd) airport.LatLon {
	lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, end.Heading, takeoffLiftoffNM*1852)
	return airport.LatLon{Lat: lat, Lon: lon}
}

// oppositeCorner is where a turn after departure off end of rwy begins,
// pastNM past the far end on the extended centreline.
func oppositeCorner(rwy airport.Runway, end airport.RunwayEnd, pastNM float64) airport.LatLon {
	far := farEnd(rwy, end).Threshold
	lat, lon := calc.DisplaceByHeading(far.Lat, far.Lon, end.Heading, pastNM*1852)
	return airport.LatLon{Lat: lat, Lon: lon}
}

// plannedWayOut is a departure off end of rwy from its lift-off along sid
// (straight out, none), turned as turn (nil not) with its corner lateNM
// farther on.
func plannedWayOut(rwy airport.Runway, end airport.RunwayEnd, sid []airport.NavPoint, turn *traffic.DepartureTurn, lateNM float64) []traffic.RoutePoint {
	off := liftoffAt(end)
	if turn != nil {
		t := *turn
		lat, lon := calc.DisplaceByHeading(t.Corner.Lat, t.Corner.Lon, end.Heading, lateNM*1852)
		t.Corner = airport.LatLon{Lat: lat, Lon: lon}
		return climbWay(off, 0, traffic.DepartureTurnPath(off, end.Heading, t, sid))
	}
	var pts []airport.LatLon
	if len(sid) > 0 {
		for _, w := range traffic.DepartureWaypoints(off, end.Heading, 0, sid) {
			pts = append(pts, airport.LatLon{Lat: w.Latitude, Lon: w.Longitude})
		}
	} else {
		for _, nm := range []float64{2, 6, 20, traffic.ProcedureContinueMeters / 1852} {
			lat, lon := calc.DisplaceByHeading(off.Lat, off.Lon, end.Heading, nm*1852)
			pts = append(pts, airport.LatLon{Lat: lat, Lon: lon})
		}
	}
	return climbWay(off, 0, pts)
}

// sameRunwayOther reports whether runway end b is the other end of a's
// runway at layout l.
func sameRunwayOther(l *airport.Layout, a, b string) bool {
	ra, ea, ok1 := l.RunwayEnd(a)
	rb, eb, ok2 := l.RunwayEnd(b)
	return ok1 && ok2 && ra.Name() == rb.Name() && ea.Name != eb.Name
}

// waysIn are the arrivals of ours flying in to the other end of depEnd's
// runway at g (IFR or a circuit), not tail: from where each is along what
// it still flies to the landing threshold.
func (cc *controlCenter) waysIn(g *airport.Graph, depEnd, tail string) []oppositeWay {
	var out []oppositeWay
	for _, a := range cc.world.Aircraft() {
		if !a.Ours || a.Grounded() || a.Tail == tail {
			continue
		}
		o := cc.byTail(a.Tail)
		if o == nil || o.arr == nil || o.ICAO != g.Layout.ICAO || o.object() != a.ObjectID || o.arr.State() != traffic.ArrivalApproaching {
			continue
		}
		o.mu.Lock()
		rwy := o.view.Runway
		o.mu.Unlock()
		if !sameRunwayOther(g.Layout, depEnd, rwy) {
			continue
		}
		_, end, _ := g.Layout.RunwayEnd(rwy)
		path := []traffic.RoutePoint{{Position: a.Position, Kts: a.GroundKts}}
		for _, p := range o.arr.ProcedurePlan() {
			path = append(path, traffic.RoutePoint{Position: p.Position, Kts: p.Kts})
		}
		path = append(path, traffic.RoutePoint{Position: end.LandingThreshold(), Kts: math.Min(a.GroundKts, 160)})
		out = append(out, oppositeWay{tail: a.Tail, path: path})
	}
	return out
}

// oppositeMeets is the first of ins that out(lateNM), lifting off within
// oppositeLiftoffIn, comes within the minimum and the buffer of, flown at
// the speeds planned give or take; "" none.
func oppositeMeets(out func(lateNM float64) []traffic.RoutePoint, ins []oppositeWay) string {
	need := traffic.OppositeSeparationNM + oppositeBufferNM
	for _, w := range ins {
		for _, late := range []float64{0, oppositeCornerLateNM} {
			p := out(late)
			for _, d := range oppositeLiftoffIn {
				for _, ds := range oppositeDepScales {
					for _, as := range oppositeArrScales {
						if traffic.ClosestApproachNM(p, d, ds, w.path, as) < need {
							return w.tail
						}
					}
				}
			}
		}
	}
	return ""
}

// oppositeTurn is departure it, cleared for take-off now, against the
// arrivals to the other end of its runway: nil and "" clear as it goes
// (or with the turn given it already); a turn after departure that keeps
// it clear, the least off its way first; or nil and who it would meet,
// no turn clearing it (VFR: none tried).
func (cc *controlCenter) oppositeTurn(g *airport.Graph, it *controlled) (*traffic.DepartureTurn, string) {
	if it.dep == nil {
		return nil, ""
	}
	it.mu.Lock()
	name, vfr, had := it.view.Runway, it.view.Rules == "VFR", it.depTurn
	it.mu.Unlock()
	rwy, end, ok := g.Layout.RunwayEnd(name)
	if !ok {
		return nil, ""
	}
	ins := cc.waysIn(g, name, it.Tail)
	if len(ins) == 0 {
		return nil, ""
	}
	sid := it.dep.DepartureRoute()
	way := func(t *traffic.DepartureTurn) func(float64) []traffic.RoutePoint {
		return func(late float64) []traffic.RoutePoint { return plannedWayOut(rwy, end, sid, t, late) }
	}
	if had != nil && oppositeMeets(way(had), ins) == "" {
		return nil, ""
	}
	who := oppositeMeets(way(nil), ins)
	if who == "" || vfr {
		return nil, who
	}
	for _, t := range oppositeTurns(rwy, end, sid, ins, cc.magVar(g.Layout.ICAO)) {
		if oppositeMeets(way(&t), ins) == "" {
			return &t, ""
		}
	}
	return nil, who
}

// oppositeTurns are the turns after departure off end of rwy tried, the
// least off the way out first: each turn of oppositeTurnDeg, toward its
// SID's first point first (with none away from the first arrival's way
// in), a heading as said (HeadingSaid, magVar), each leg of oppositeLegNM;
// by how far the leg takes it off the centreline.
func oppositeTurns(rwy airport.Runway, end airport.RunwayEnd, sid []airport.NavPoint, ins []oppositeWay, magVar float64) []traffic.DepartureTurn {
	corner := oppositeCorner(rwy, end, oppositeCornerPastEndNM)
	side := 1.0 // right
	switch {
	case len(sid) > 0:
		if headingDiff(end.Heading, calc.BearingDegrees(corner.Lat, corner.Lon, sid[0].Position.Lat, sid[0].Position.Lon)) < 0 {
			side = -1
		}
	case len(ins) > 0 && len(ins[0].path) > 0:
		p := ins[0].path[0].Position
		if headingDiff(end.Heading, calc.BearingDegrees(corner.Lat, corner.Lon, p.Lat, p.Lon)) > 0 {
			side = -1
		}
	}
	type cand struct {
		t   traffic.DepartureTurn
		off float64
		deg float64
		alt bool
	}
	var cs []cand
	for _, deg := range oppositeTurnDeg {
		for _, s := range []float64{side, -side} {
			h := end.Heading + s*deg
			h = math.Mod(math.Round((h+magVar)/traffic.HeadingStepDeg)*traffic.HeadingStepDeg-magVar+720, 360)
			off := math.Abs(math.Sin(headingDiff(end.Heading, h) * math.Pi / 180))
			for _, leg := range oppositeLegNM {
				cs = append(cs, cand{traffic.DepartureTurn{Corner: corner, Heading: h, LegNM: leg}, leg * off, deg, s != side})
			}
		}
	}
	slices.SortStableFunc(cs, func(a, b cand) int {
		switch {
		case a.off != b.off:
			return cmpFloat(a.off, b.off)
		case a.deg != b.deg:
			return cmpFloat(a.deg, b.deg)
		case a.alt != b.alt:
			if a.alt {
				return 1
			}
			return -1
		}
		return 0
	})
	out := make([]traffic.DepartureTurn, len(cs))
	for i, c := range cs {
		out[i] = c.t
	}
	return out
}

// cmpFloat orders a before b ascending.
func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// giveTurn gives departure it turn after departure, said once, off end;
// false when it is handed over already.
func (cc *controlCenter) giveTurn(it *controlled, end airport.RunwayEnd, turn traffic.DepartureTurn) bool {
	it.mu.Lock()
	had := it.depTurn
	it.mu.Unlock()
	if had != nil && *had == turn {
		return true
	}
	if !it.dep.TurnAfterDeparture(turn) {
		return false
	}
	it.mu.Lock()
	it.depTurn = &turn
	it.mu.Unlock()
	mv := cc.magVar(it.ICAO)
	dir := traffic.TurnTo(end.Heading, turn.Heading)
	cc.log.printf("%-6s take-off: after departure turn %s heading %s for %.0f NM, clear of the arrivals to the other end", it.Tail, dir, traffic.HeadingSaid(turn.Heading, mv), turn.LegNM)
	it.say(traffic.AfterDepartureTurn(it.Tail, dir, turn.Heading, mv))
	return true
}

// waysOut are the departures of ours off the other end of arrEnd's runway
// at layout l cleared for take-off, rolling or airborne (handed over, its
// controller done with it: still in the picture), not tail: how each
// flies on from now, an airborne one from where it is along its climb; a
// tail marked + still to lift off.
func (cc *controlCenter) waysOut(l *airport.Layout, arrEnd, tail string) []oppositeWay {
	where := map[uint32]traffic.TrackedAircraft{}
	for _, a := range cc.world.Aircraft() {
		where[a.ObjectID] = a
	}
	var out []oppositeWay
	for _, it := range cc.snapshotItems() {
		if it.dep == nil || it.ICAO != l.ICAO || it.Tail == tail {
			continue
		}
		it.mu.Lock()
		name, turn := it.view.Runway, it.depTurn
		it.mu.Unlock()
		if !sameRunwayOther(l, arrEnd, name) {
			continue
		}
		st := it.dep.State()
		a, seen := where[it.object()]
		switch {
		case st == traffic.TaxiComplete && seen && !a.Grounded():
			plan := it.dep.ClimbPlan(a.Position)
			path := []traffic.RoutePoint{{Position: a.Position, Kts: a.GroundKts}}
			for _, p := range plan {
				if math.Abs(headingDiff(a.Heading, calc.BearingDegrees(a.Position.Lat, a.Position.Lon, p.Position.Lat, p.Position.Lon))) > 90 {
					continue // behind it: flown past
				}
				path = append(path, traffic.RoutePoint{Position: p.Position, Kts: p.Kts})
			}
			out = append(out, oppositeWay{tail: it.Tail, path: onAlong(path, a.Heading)})
		case st == traffic.TaxiDeparting || it.dep.Cleared("takeoff") && st != traffic.TaxiComplete:
			rwy, end, _ := l.RunwayEnd(name)
			out = append(out, oppositeWay{tail: "+" + it.Tail, path: plannedWayOut(rwy, end, it.dep.DepartureRoute(), turn, 0)})
		}
	}
	return out
}

// onAlong is an airborne departure's path flown on along its last leg
// for ProcedureContinueMeters past its last climb point, as MSFS AI flies
// on, or on hdg with none left ahead (TestDeclaredField: its climb ended
// 10 NM out, the arrival met 31 NM out head-on).
func onAlong(path []traffic.RoutePoint, hdg float64) []traffic.RoutePoint {
	n := len(path)
	if n == 0 {
		return path
	}
	b := path[n-1].Position
	if n >= 2 && path[n-2].Position != b {
		a := path[n-2].Position
		hdg = calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon)
	}
	lat, lon := calc.DisplaceByHeading(b.Lat, b.Lon, hdg, traffic.ProcedureContinueMeters)
	return append(path, traffic.RoutePoint{Position: airport.LatLon{Lat: lat, Lon: lon}, Kts: math.Max(path[n-1].Kts, 1)})
}

// releaseMeets names a departure off the other end of arrEnd's runway at
// layout l that an arrival appearing now and flying in along way (at
// kts, give or take) would come within the minimum and the buffer of;
// "" none. A departure still to lift off is flown from its lift-off
// within oppositeLiftoffIn.
func (cc *controlCenter) releaseMeets(l *airport.Layout, arrEnd, tail string, way []airport.LatLon, kts float64) string {
	if len(way) < 2 {
		return ""
	}
	outs := cc.waysOut(l, arrEnd, tail)
	if len(outs) == 0 {
		return ""
	}
	in := make([]traffic.RoutePoint, len(way))
	for i, p := range way {
		in[i] = traffic.RoutePoint{Position: p, Kts: kts}
	}
	need := traffic.OppositeSeparationNM + oppositeBufferNM
	for _, o := range outs {
		delays := []time.Duration{0}
		name := o.tail
		if name[0] == '+' {
			name, delays = name[1:], append([]time.Duration{0}, oppositeLiftoffIn...)
		}
		for _, d := range delays {
			for _, ds := range oppositeDepScales {
				for _, as := range oppositeReleaseScales {
					if traffic.ClosestApproachNM(o.path, d, ds, in, as) < need {
						return name
					}
				}
			}
		}
	}
	return ""
}

// arrivalWayIn is the way an arrival to end appearing now flies in, as
// release planning has it: its circuit (from entry, if any), else its
// procedure, else straight in from spawnNm on the final; to the landing
// threshold. The speed is the one it is planned at.
func arrivalWayIn(end airport.RunwayEnd, proc []airport.NavPoint, circuit *traffic.Circuit, entry *traffic.ReportingPoint, spawnNm float64) ([]airport.LatLon, float64) {
	var way []airport.LatLon
	kts := oppositeReleaseKts
	switch {
	case circuit != nil:
		if entry != nil {
			way = append(way, entry.Position)
		}
		for _, p := range circuit.Points {
			way = append(way, p.Position)
		}
		kts = oppositeCircuitKts
	case len(proc) > 0:
		for _, n := range proc {
			if n.Position != (airport.LatLon{}) {
				way = append(way, n.Position)
			}
		}
	default:
		if spawnNm <= 0 {
			spawnNm = traffic.DefaultSpawnNm
		}
		thr := end.LandingThreshold()
		lat, lon := calc.DisplaceByHeading(thr.Lat, thr.Lon, end.Heading+180, spawnNm*1852)
		way = append(way, airport.LatLon{Lat: lat, Lon: lon})
	}
	return append(way, end.LandingThreshold()), kts
}

// releaseBlocked is the error an arrival to arrEnd at g, flying in along
// way at kts, appears with now when it would meet a departure off the
// other end (releaseMeets); nil none.
func (cc *controlCenter) releaseBlocked(g *airport.Graph, arrEnd, tail string, way []airport.LatLon, kts float64) error {
	if who := cc.releaseMeets(g.Layout, arrEnd, tail, way, kts); who != "" {
		return fmt.Errorf("%w: %s departing off the other end of runway %s would meet it; try again in a minute", traffic.ErrSpawnBlocked, who, arrEnd)
	}
	return nil
}
