package world

import (
	"math"
	"sort"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// goAround sends it (an arrival) around, its circuit on the side of the
// runway the other arrivals coming in to it fly through least (#1458;
// offline LKPR 06: DLH299 on VLM6T sent round on the south side into
// TVS947 coming in on APRA2T from the south).
func (cc *controlCenter) goAround(it *controlled) error {
	streams := cc.arrivalStreams(it)
	it.arr.SetGoAroundStreams(streams)
	cc.log.printf("%-6s go-around: %d other arrivals coming in for its circuit side", it.Tail, len(streams))
	if err := it.arr.GoAround(); err != nil {
		return err
	}
	cc.planRejoin(it)
	return nil
}

// Rejoin planning after a go-around (#1573): its way back is planned there
// and then, its circuit flown ahead against every arrival of ours coming
// in, and its downwind extended (one vector) until it meets none of them
// on the way in — not left to the sequencer and the conflict watch as it
// closes in (offline LKPR 06, DLH299 round behind AFR971: "190 kt" at 180
// kt already, a dog-leg off its upwind, then the conflict watch).
const (
	// rejoinLookAhead: how far ahead the circuit is flown against the
	// others: round to the final and onto it.
	rejoinLookAhead = 15 * time.Minute
	// rejoinTries: extensions tried, each by what the last prediction
	// asked for.
	rejoinTries = 4
	// rejoinClimbFpm: the go-around's climb, flown ahead from its final
	// (its vertical speed there is the descent it is leaving).
	rejoinClimbFpm = 1500.0
)

// planRejoin plans me's way back from a go-around up front, against every
// arrival of ours coming in: its place in the sequence first (rejoinSlot,
// the time lost on a longer downwind: one vector), then its circuit flown
// ahead against their ways, its downwind extended further while that puts
// the meeting later, and where they still meet as one descends through
// the other's level, a level for that one (levels only for crossing).
// Speed, one vector and a level: no orbit, no hold.
//
// A player's go-around is the player's to space (#1096): no time to lose
// and no vector from the AI, only our own arrivals levelled clear of it
// (#1648: JBU1547 sent round in game mode, told by the AI to lose 58 s).
func (cc *controlCenter) planRejoin(me *controlled) {
	if me.gates.Load() {
		cc.log.printf("%-6s go-around: the player's, its way back the player's to space", me.Tail)
		cc.rejoinLevels(me)
		return
	}
	slot, behind, before := cc.rejoinSlot(me)
	if slot > 0 {
		a, err := me.arr.AbsorbDelay(slot)
		if err != nil {
			cc.log.printf("%-6s go-around: after %s, %s to lose: %v", me.Tail, behind, slot.Round(time.Second), err)
			return
		}
		cc.log.printf("%-6s go-around: back in after %s, %s to lose: %s", me.Tail, behind, slot.Round(time.Second), a)
	}
	var lastIn time.Duration
	for i := range rejoinTries {
		need, with, in := cc.rejoinNeed(me, before)
		if need <= 0 {
			return
		}
		if i > 0 && in <= lastIn {
			break // no later for a longer downwind: they cross
		}
		lastIn = in
		a, err := me.arr.AbsorbDelay(need)
		if err != nil {
			cc.log.printf("%-6s go-around: meets %s on its way back, %s to lose: %v", me.Tail, with, need.Round(time.Second), err)
			break
		}
		cc.log.printf("%-6s go-around: meets %s on its way back in %s, %s to lose: %s", me.Tail, with, in.Round(time.Second), need.Round(time.Second), a)
		if a.ExtraNM <= 0 {
			break
		}
	}
	cc.rejoinLevels(me)
}

// rejoinLevels stops the descent of each arrival still meeting me on its
// way back from a go-around (rejoinConflicts) a level above me's circuit,
// for its way up to where they would meet: it comes down through the
// go-around's level beside it (offline LKPR 24, AFR971 on its base at
// 5600 ft and DLH299 on its extended downwind at 5000, 3.8 NM apart). Not
// one already lower (never a climb).
func (cc *controlCenter) rejoinLevels(me *controlled) {
	meets, myFt := cc.rejoinConflicts(me)
	if myFt <= 0 {
		return
	}
	level := math.Ceil((myFt+traffic.VerticalSeparationFt)/500) * 500
	levels := map[string]float64{} // those levelled: at what
	stop := func(m rejoinMeet, ft float64, below string) bool {
		if m.it.gates.Load() {
			return false // the player's to level (#1096)
		}
		forNM := math.Max(m.at.GroundKts, traffic.ProcedureApproachSpeedKts) * (max(m.c.In, m.c.ClosestIn) + time.Minute).Hours()
		if err := m.it.arr.StopDescent(ft, forNM); err != nil {
			cc.log.printf("%-6s go-around: %s to stop descent at %.0f ft refused: %v", me.Tail, m.it.Tail, ft, err)
			return false
		}
		levels[m.it.Tail] = ft
		cc.log.printf("%-6s go-around: %s to stop descent at %.0f ft for %.0f NM, above %s", me.Tail, m.it.Tail, ft, forNM, below)
		it, tx := m.it, traffic.StopDescent(traffic.PosApproach, m.it.Tail, ft, below, cc.taOf(m.it.ICAO))
		it.call(traffic.PosApproach, prioSeparation, func() { it.say(tx) })
		return true
	}
	// The one landing first lowest, each after it a level above: a stack
	// they come down from in turn, never the one ahead through the one
	// behind (offline EGLL 27R: AFR1680, number 1, held above KLM1001).
	eta := func(m rejoinMeet) time.Duration { return planETA(m.at.Position, m.it.arr.ProcedurePlan()) }
	var first []rejoinMeet
	for _, m := range meets {
		if m.by == nil && m.c.LossFt < traffic.VerticalSeparationFt {
			first = append(first, m)
		}
	}
	sort.SliceStable(first, func(i, j int) bool { return eta(first[i]) < eta(first[j]) })
	for _, m := range first {
		ft := level + float64(len(levels))*traffic.VerticalSeparationFt
		if _, ok := levels[m.it.Tail]; ok || m.at.AltFt < ft-200 {
			continue
		}
		stop(m, ft, me.Tail)
	}
	// One levelled into another coming in above it (two held at one
	// level, offline LKPR 24: TVS947 and AFR971 both at 6000 ft): that one
	// a level higher, as a stack.
	for range rejoinTries {
		if len(levels) == 0 {
			return
		}
		more := false
		meets, _ = cc.rejoinConflicts(me)
		etas := map[string]time.Duration{}
		for _, m := range meets {
			etas[m.it.Tail] = eta(m)
		}
		for _, m := range meets {
			if m.by == nil {
				continue
			}
			ft, ok := levels[m.by.Tail]
			if !ok || m.c.LossFt >= traffic.VerticalSeparationFt {
				continue
			}
			if _, done := levels[m.it.Tail]; done {
				continue
			}
			if by, ok := etas[m.by.Tail]; !ok || eta(m) < by {
				continue // landing first: not held above the one after it
			}
			up := ft + traffic.VerticalSeparationFt
			if m.at.AltFt < up-200 {
				continue // lower already: never a climb
			}
			if stop(m, up, m.by.Tail) {
				more = true
			}
		}
		if !more {
			return
		}
	}
}

// rejoinSlot is the time me must lose on its way back from a go-around to
// reach the final DefaultMergeSpacingNM (and rejoinSlotBuffer) behind
// every arrival of ours for the same runway end it would reach the final
// with or after: its place in the sequence, timed along each one's way
// (theirs as planned, its own round the circuit); the last of them; and
// those it lands before.
func (cc *controlCenter) rejoinSlot(me *controlled) (time.Duration, string, map[string]bool) {
	p := me.arr.Plan()
	if p == nil {
		return 0, "", nil
	}
	all := cc.world.Aircraft()
	pos := func(it *controlled) (airport.LatLon, bool) {
		id := it.object()
		for _, a := range all {
			if a.ObjectID == id && id != 0 && !a.Grounded() {
				return a.Position, true
			}
		}
		return airport.LatLon{}, false
	}
	at, ok := pos(me)
	if !ok {
		return 0, "", nil
	}
	mine := planETA(at, me.arr.ProcedurePlan())
	if mine <= 0 {
		return 0, "", nil
	}
	type eta struct {
		tail string
		in   time.Duration
	}
	var others []eta
	for _, it := range cc.rejoinOthers(me) {
		if q := it.arr.Plan(); q == nil || !sameFinal(p.End, q.End) {
			continue
		}
		if x, ok := pos(it); ok {
			if in := planETA(x, it.arr.ProcedurePlan()); in > 0 {
				others = append(others, eta{it.Tail, in})
			}
		}
	}
	sort.Slice(others, func(i, j int) bool { return others[i].in < others[j].in })
	// Each of them the spacing behind the one before it (what the sequence
	// will have it lose: offline LKPR 24, AFR971 slowed behind TVS947 into
	// the go-around's place), the go-around in the first gap that holds it.
	spacing := traffic.SeparationTime(traffic.DefaultMergeSpacingNM, traffic.ProcedureApproachSpeedKts)
	gap := spacing + rejoinSlotBuffer
	t, behind, placed := mine, "", false
	before := map[string]bool{}
	prev := time.Duration(math.MinInt64 / 2)
	for _, o := range others {
		at := max(o.in, prev+spacing)
		if !placed && t+gap <= at {
			placed, prev = true, t
			at = max(o.in, prev+spacing)
		}
		if placed {
			before[o.tail] = true
		} else {
			t, behind = max(t, at+gap), o.tail
		}
		prev = at
	}
	return t - mine, behind, before
}

// rejoinSlotBuffer: what a rejoin's slot keeps beyond the spacing, for
// the speeds the others are given on their way in meanwhile.
const rejoinSlotBuffer = 30 * time.Second

// planETA is the time to fly plan from at, at each point's speed (the
// approach speed where it has none), its last point the final's join.
func planETA(at airport.LatLon, plan []traffic.RoutePoint) time.Duration {
	h := 0.0
	for _, r := range plan {
		kts := r.Kts
		if kts <= 0 {
			kts = traffic.ProcedureApproachSpeedKts
		}
		h += calc.HaversineNM(at.Lat, at.Lon, r.Position.Lat, r.Position.Lon) / kts
		at = r.Position
	}
	return time.Duration(h * float64(time.Hour))
}

// rejoinOthers are our other arrivals coming in to me's airport.
func (cc *controlCenter) rejoinOthers(me *controlled) []*controlled {
	var out []*controlled
	for _, it := range cc.snapshotItems() {
		if it == me || it.arr == nil || it.ICAO != me.ICAO || it.finished.Load() || it.arr.State() != traffic.ArrivalApproaching {
			continue
		}
		out = append(out, it)
	}
	return out
}

// rejoinMeet is an arrival me meets on its way back from a go-around:
// the conflict predicted, and where that one is now.
type rejoinMeet struct {
	it *controlled
	at traffic.TrackedAircraft
	c  traffic.Conflict
	// by: the other arrival it meets, when not me (nil).
	by *controlled
}

// rejoinConflicts flies me's way back from a go-around ahead against the
// planned ways of our other arrivals coming in (climbOpts, within
// rejoinLookAhead): those it meets, and the level of its circuit (0: not
// flying it).
func (cc *controlCenter) rejoinConflicts(me *controlled) ([]rejoinMeet, float64) {
	all := cc.world.Aircraft()
	tracked := func(it *controlled) (traffic.TrackedAircraft, bool) {
		id := it.object()
		for _, a := range all {
			if a.ObjectID == id && id != 0 && !a.Grounded() {
				return a, true
			}
		}
		return traffic.TrackedAircraft{}, false
	}
	mine, ok := tracked(me)
	myPlan := me.arr.ProcedurePlan()
	if !ok || len(myPlan) == 0 {
		return nil, 0
	}
	myFt := 0.0
	for _, r := range myPlan {
		myFt = math.Max(myFt, r.AltFt)
	}
	// Still low on its final as it goes around: the tower's pair (TowerPair)
	// a moment more, descending; flown ahead as the radar's, climbing at its
	// circuit speed.
	mine.AGLFt = math.Max(mine.AGLFt, traffic.TowerBelowFt)
	mine.GroundKts = math.Max(mine.GroundKts, traffic.ProcedureApproachSpeedKts)
	mine.VSFpm = math.Max(mine.VSFpm, rejoinClimbFpm)
	// The others down their ways as planned, not held at the level they
	// are level at now (heldPlan): the go-around is low, they come down to
	// it (offline LKPR 24: AFR971 level at 10000 ft, predicted clear of
	// DLH299 at 5000 all the way in).
	plans := map[string][]traffic.RoutePoint{mine.Tail: myPlan}
	air := []traffic.TrackedAircraft{mine}
	byID := map[uint32]*controlled{}
	for _, it := range cc.rejoinOthers(me) {
		if a, ok := tracked(it); ok {
			plans[a.Tail] = it.arr.ProcedurePlan()
			air = append(air, a)
			byID[a.ObjectID] = it
		}
	}
	if len(air) < 2 {
		return nil, myFt
	}
	opts := climbOpts
	opts.LookAhead = rejoinLookAhead
	opts.Profile = func(a traffic.TrackedAircraft) []traffic.RoutePoint { return plans[a.Tail] }
	var out []rejoinMeet
	for _, c := range traffic.PredictConflicts(air, opts) {
		id := c.AID
		if id == mine.ObjectID {
			id = c.BID
		} else if c.BID != mine.ObjectID {
			// Between the others: theirs, but for one levelled for me
			// (rejoinLevels): the one it is levelled into.
			if a, b := byID[c.AID], byID[c.BID]; a != nil && b != nil {
				for _, x := range air {
					if x.ObjectID == c.AID {
						out = append(out, rejoinMeet{it: a, at: x, c: c, by: b})
					}
					if x.ObjectID == c.BID {
						out = append(out, rejoinMeet{it: b, at: x, c: c, by: a})
					}
				}
			}
			continue
		}
		it := byID[id]
		if it == nil {
			continue
		}
		for _, a := range air {
			if a.ObjectID == id {
				out = append(out, rejoinMeet{it: it, at: a, c: c})
			}
		}
	}
	return out, myFt
}

// rejoinNeed is the time me must lose on its way back from a go-around to
// pass DefaultMergeSpacingNM behind the arrivals it would meet
// (rejoinConflicts): the most any of them asks, that one, and when they
// would meet; zero none. Not those it lands before (rejoinSlot): a longer
// downwind only takes it into them (offline EGLL 27R, DLH902 number 1
// extended 13.7 NM into AFR1680 and KLM1001 coming in to 27L).
func (cc *controlCenter) rejoinNeed(me *controlled, before map[string]bool) (time.Duration, string, time.Duration) {
	meets, _ := cc.rejoinConflicts(me)
	var (
		need time.Duration
		with string
		in   time.Duration
	)
	for _, m := range meets {
		if m.by != nil || before[m.it.Tail] {
			continue // between the others, or behind it
		}
		if n := traffic.SeparationTime(math.Max(traffic.DefaultMergeSpacingNM-m.c.ClosestNM, 1), traffic.ProcedureApproachSpeedKts); n > need {
			need, with, in = n, m.it.Tail, m.c.In
		}
	}
	return need, with, in
}

// arrivalStreams are the ways ahead of our other arrivals on their
// procedures to me's runway end.
func (cc *controlCenter) arrivalStreams(me *controlled) [][]airport.LatLon {
	p := me.arr.Plan()
	if p == nil {
		return nil
	}
	var out [][]airport.LatLon
	for _, it := range cc.snapshotItems() {
		if it == me || it.arr == nil || it.ICAO != me.ICAO || it.finished.Load() || it.arr.State() != traffic.ArrivalApproaching {
			continue
		}
		if q := it.arr.Plan(); q == nil || !sameFinal(p.End, q.End) {
			continue
		}
		plan := it.arr.ProcedurePlan()
		if len(plan) < 2 {
			continue
		}
		route := make([]airport.LatLon, len(plan))
		for i, r := range plan {
			route[i] = r.Position
		}
		out = append(out, route)
	}
	return out
}

// sameFinalNM: a parallel runway's final within this of a go-around's is
// one it joins with: its circuit crosses it or runs beside it (offline
// EGLL 27R: DLH902 round on the south side, KLM1001 coming down to 27L
// through its level, 1.0 NM apart).
const sameFinalNM = 3.0

// sameFinal reports whether arrivals to b fly the final of a, or one
// beside it: the same runway end, or a parallel one (within 15°) whose
// threshold is within sameFinalNM of a's centreline.
func sameFinal(a, b airport.RunwayEnd) bool {
	if a.Name == b.Name {
		return true
	}
	if math.Abs(headingDiff(a.Heading, b.Heading)) > 15 {
		return false
	}
	t, u := a.LandingThreshold(), b.LandingThreshold()
	lat, lon := calc.DisplaceByHeading(t.Lat, t.Lon, a.Heading, 10*1852)
	return math.Abs(calc.CrossTrackMeters(t.Lat, t.Lon, lat, lon, u.Lat, u.Lon)) <= sameFinalNM*1852
}
