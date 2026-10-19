package world

import (
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Release planning: an arrival is released onto its STAR only when, flown
// ahead along it at its levels and speed, it keeps separated from every
// arrival of ours already on its way — as an upstream sector hands a
// stream over spaced, not one that approach must pull apart (the user:
// "resolving conflicts instead of managing traffic"; live, GSEXC and OKFLS
// on merging STARs at FL100, stopped and vectored). Else it waits and the
// manager tries again: a later release.
const (
	// releaseLookAhead: how far ahead a release is flown against the others.
	releaseLookAhead = 20 * time.Minute
	// releaseEntryFt: the level an arrival enters its STAR at, when its
	// first points give none (the arrivals' "flight level 100").
	releaseEntryFt = 10000.0
)

// releaseProfile is an arrival's way along entry (its STAR and approach) as
// the predictor flies it: each point at the lowest level its constraints
// allow so far (descending as the STAR has it), at procedure speed.
func releaseProfile(entry []airport.NavPoint) []traffic.RoutePoint {
	var out []traffic.RoutePoint
	alt := releaseEntryFt
	for _, n := range entry {
		if n.Kind == "R" || n.Position == (airport.LatLon{}) {
			continue
		}
		if a := math.Max(n.AltMin, n.AltMax) / 0.3048; a > 0 {
			alt = math.Min(alt, a)
		}
		out = append(out, traffic.RoutePoint{Position: n.Position, AltFt: alt, Kts: traffic.ProcedureSpeedKts})
	}
	return out
}

// releaseConflict names the arrival of ours that callsign, released now at
// the start of entry, would lose separation with on its way in (within
// releaseLookAhead, flown along their routes and levels); "" none.
func (s *scheduler) releaseConflict(callsign string, entry []airport.NavPoint) string {
	prof := releaseProfile(entry)
	if len(prof) < 2 {
		return ""
	}
	cc := s.cc
	plans := map[string][]traffic.RoutePoint{callsign: prof}
	first := prof[0]
	list := []traffic.TrackedAircraft{{
		Observation: traffic.Observation{Tail: callsign, Position: first.Position, AltFt: first.AltFt, GroundKts: first.Kts,
			Heading: calc.BearingDegrees(first.Position.Lat, first.Position.Lon, prof[1].Position.Lat, prof[1].Position.Lon)},
		Phase: traffic.PhaseArriving, Ours: true,
	}}
	for _, a := range cc.world.Aircraft() {
		if !a.Ours || a.OnGround || a.Tail == callsign {
			continue
		}
		it := cc.byTail(a.Tail)
		if it == nil || it.arr == nil || it.object() != a.ObjectID || it.arr.State() != traffic.ArrivalApproaching {
			continue
		}
		plans[a.Tail] = it.arr.ProcedurePlan()
		list = append(list, a)
	}
	if len(list) < 2 {
		return ""
	}
	opts := conflictOpts
	opts.LookAhead = releaseLookAhead
	opts.Profile = func(a traffic.TrackedAircraft) []traffic.RoutePoint { return plans[a.Tail] }
	opts.Route = func(a traffic.TrackedAircraft) []airport.LatLon {
		var pts []airport.LatLon
		for _, p := range plans[a.Tail] {
			pts = append(pts, p.Position)
		}
		return traffic.RouteAhead(a.Position, pts)
	}
	for _, c := range traffic.PredictConflicts(list, opts) {
		switch callsign {
		case c.A:
			return c.B
		case c.B:
			return c.A
		}
	}
	return ""
}

// Departure radar clears a climb above the arrivals only clear of them:
// the departure flown on to the level along its climb against our
// arrivals on their way in (live, CSA408 "climb to flight level 240"
// through CSA766 entering its STAR at FL100, 4.4 NM and 444 ft). Not clear:
// identified only, the climb when it is, looked at every
// climbRecheckEvery.
const climbRecheckEvery = 30 * time.Second

// climbOpts: a climb is cleared with a mile to spare on the minima, a
// prediction off by a little not a loss (live, SWR1007 cleared to FL240
// past TVS1496 at FL100, 5.0 NM and 557 ft as it went through).
var climbOpts = func() traffic.ConflictOptions {
	o := conflictOpts
	o.MinNM, o.TerminalNM = sepMinNM+climbBufferNM, sepTerminalNM+climbBufferNM
	return o
}()

const climbBufferNM = 1.0

// climbMeets names an arrival of ours the departure it would meet climbing
// on to ft now (within conflictLookAhead); "" none.
func (it *controlled) climbMeets(ft float64) string {
	cc := it.cc
	id := it.object()
	var me *traffic.TrackedAircraft
	all := cc.world.Aircraft()
	for i := range all {
		if all[i].ObjectID == id && id != 0 {
			me = &all[i]
		}
	}
	if me == nil || it.dep == nil {
		return ""
	}
	climb := it.dep.ClimbPlan(me.Position)
	for i := range climb {
		climb[i].AltFt = math.Max(climb[i].AltFt, ft)
	}
	if len(climb) == 0 {
		climb = []traffic.RoutePoint{{Position: me.Position, AltFt: ft}}
	}
	plans := map[string][]traffic.RoutePoint{me.Tail: reachable(*me, climb)}
	list := []traffic.TrackedAircraft{*me}
	for _, a := range all {
		if !a.Ours || a.Grounded() || a.ObjectID == id {
			continue
		}
		o := cc.byTail(a.Tail)
		if o == nil || o.arr == nil || o.object() != a.ObjectID || o.arr.State() != traffic.ArrivalApproaching {
			continue
		}
		plans[a.Tail] = heldPlan(o.arr.ProcedurePlan(), a)
		list = append(list, a)
	}
	if len(list) < 2 {
		return ""
	}
	opts := climbOpts
	opts.Profile = func(a traffic.TrackedAircraft) []traffic.RoutePoint { return plans[a.Tail] }
	for _, c := range traffic.PredictConflicts(list, opts) {
		switch me.Tail {
		case c.A:
			return c.B
		case c.B:
			return c.A
		}
	}
	return ""
}

// reachable is a departure's climb route as it can fly it: no point
// higher than it climbs to by then from where it is, at its climb rate now
// (climbRateFpm at least), none lower than it is. A climb clearance puts
// every point of the route at the cleared level or higher; taken as it
// stands the prediction climbed it there by the next point and over all
// traffic (live, KLM216 cleared to FL240 into SWR811 at FL100, stopped at
// 7200 ft 20 s before the loss).
func reachable(me traffic.TrackedAircraft, route []traffic.RoutePoint) []traffic.RoutePoint {
	rate := math.Max(me.VSFpm, climbRateFpm)
	kts := me.GroundKts
	if kts < 150 {
		kts = 250
	}
	out := append([]traffic.RoutePoint(nil), route...)
	prev, minutes := me.Position, 0.0
	for i := range out {
		k := out[i].Kts
		if k <= 0 {
			k = kts
		}
		minutes += calc.HaversineNM(prev.Lat, prev.Lon, out[i].Position.Lat, out[i].Position.Lon) / k * 60
		prev = out[i].Position
		out[i].AltFt = math.Max(math.Min(out[i].AltFt, me.AltFt+rate*minutes), me.AltFt)
	}
	return out
}

// climbRateFpm: a departure's climb rate taken at least.
const climbRateFpm = 1500.0

// heldPlan is an arrival's plan as a climb through it is judged: level
// now, it is taken to stay at that level (stopped for traffic, its descent
// not given yet), not to descend as its STAR has it (live, AUA939 cleared
// to FL240 through TVS706 stopped at FL095, 4.8 NM and 946 ft).
func heldPlan(plan []traffic.RoutePoint, a traffic.TrackedAircraft) []traffic.RoutePoint {
	if math.Abs(a.VSFpm) >= heldLevelVSFpm {
		return plan
	}
	out := append([]traffic.RoutePoint(nil), plan...)
	for i := range out {
		out[i].AltFt = math.Max(out[i].AltFt, a.AltFt)
	}
	return out
}

// heldLevelVSFpm: under this vertical speed an arrival counts as level.
const heldLevelVSFpm = 300.0

// climbWhenClear clears the departure's climb to the level departure gave
// it once clear of the arrivals, looking again every climbRecheckEvery.
func (it *controlled) climbWhenClear() {
	it.mu.Lock()
	ft, done := it.radarFt, it.view.Done
	it.mu.Unlock()
	if done || ft <= 0 {
		return
	}
	if who := it.climbMeets(ft); who != "" {
		p := it.cc.pending
		p.later(it.cc.clock.Now().Add(climbRecheckEvery), it.climbWhenClear)
		return
	}
	it.climbOn()
	tx := traffic.ContinueLevelAbove(traffic.PosDeparture, it.Tail, ft, true, it.cc.taOf(it.ICAO))
	it.call(traffic.PosDeparture, prioSeparation, func() { it.say(tx) })
}

// climbRestriction is a level restriction that lets a departure climb on
// to ft clear of the arrivals instead of holding its climb (the user: a
// level until a point): "cross (fix) at or below (level)" (Doc 4444
// 12.3.2.4 a), the level a thousand feet under the arrival it would meet,
// the fix the first named one of its route past which it may climb on.
// The route to fly with it; false when no fix of its route will do.
func (it *controlled) climbRestriction(ft float64) (fix string, belowFt float64, route []traffic.RoutePoint, ok bool) {
	me, climb := it.climbSelf()
	if me == nil || len(climb) == 0 {
		return "", 0, nil, false
	}
	up := func(p []traffic.RoutePoint) []traffic.RoutePoint {
		out := append([]traffic.RoutePoint(nil), p...)
		for i := range out {
			out[i].AltFt = math.Max(out[i].AltFt, ft)
		}
		return out
	}
	who, whoFt := it.meetsAlong(me, up(climb))
	if who == "" {
		return "", 0, nil, false
	}
	belowFt = math.Floor((whoFt-1000)/1000) * 1000
	if belowFt <= me.AltFt+500 {
		return "", 0, nil, false // above that already: no restriction keeps it under
	}
	var pts []airport.LatLon
	for _, p := range climb {
		pts = append(pts, p.Position)
	}
	it.mu.Lock()
	own := it.fixes
	it.mu.Unlock()
	for _, f := range fixesAhead(own, pts) {
		k := -1 // the climb point at the fix
		for i, p := range climb {
			if calc.HaversineNM(p.Position.Lat, p.Position.Lon, f.Lat, f.Lon) < 1.5 {
				k = i
				break
			}
		}
		if k < 0 {
			continue
		}
		cand := up(climb)
		for i := 0; i <= k; i++ { // up to the fix: not above belowFt
			cand[i].AltFt = math.Min(cand[i].AltFt, belowFt)
		}
		if w, _ := it.meetsAlong(me, cand); w == "" {
			return f.Ident, belowFt, cand, true
		}
	}
	return "", 0, nil, false
}

// climbSelf is the departure as seen and its climb route from there.
func (it *controlled) climbSelf() (*traffic.TrackedAircraft, []traffic.RoutePoint) {
	id := it.object()
	if it.dep == nil || id == 0 {
		return nil, nil
	}
	for _, a := range it.cc.world.Aircraft() {
		if a.ObjectID == id {
			a := a
			return &a, it.dep.ClimbPlan(a.Position)
		}
	}
	return nil, nil
}

// meetsAlong names an arrival of ours me would meet flying profile, and
// its altitude now; "" none.
func (it *controlled) meetsAlong(me *traffic.TrackedAircraft, profile []traffic.RoutePoint) (string, float64) {
	cc := it.cc
	plans := map[string][]traffic.RoutePoint{me.Tail: reachable(*me, profile)}
	alts := map[string]float64{}
	list := []traffic.TrackedAircraft{*me}
	for _, a := range cc.world.Aircraft() {
		if !a.Ours || a.Grounded() || a.ObjectID == me.ObjectID {
			continue
		}
		o := cc.byTail(a.Tail)
		if o == nil || o.arr == nil || o.object() != a.ObjectID || o.arr.State() != traffic.ArrivalApproaching {
			continue
		}
		plans[a.Tail], alts[a.Tail] = heldPlan(o.arr.ProcedurePlan(), a), a.AltFt
		list = append(list, a)
	}
	if len(list) < 2 {
		return "", 0
	}
	opts := climbOpts
	opts.Profile = func(a traffic.TrackedAircraft) []traffic.RoutePoint { return plans[a.Tail] }
	for _, c := range traffic.PredictConflicts(list, opts) {
		switch me.Tail {
		case c.A:
			return c.B, alts[c.B]
		case c.B:
			return c.A, alts[c.A]
		}
	}
	return "", 0
}
