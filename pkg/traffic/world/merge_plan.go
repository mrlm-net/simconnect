package world

import (
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Arrivals are planned apart where their ways in meet, before they get
// there: each arrival's way (its STAR and approach as it flies them, any
// dog-leg or longer downwind included) is flown ahead mergeLookAhead
// against those landing before it, and where they would meet the later
// one loses the time before the meeting point, by speed and one vector
// (the user: "planning up front"). A merge needs no named fix in common,
// and an arrival given a longer downwind into another's STAR is seen the
// same way (live, LKPR: TVS1113 on its downwind extended for the
// changeover and TVS122 on LOMK 8S, caught by the conflict watch at
// 4m40s: "stop descent at 7000 ft").

// mergeLookAhead: how far ahead the ways in are flown against each other,
// well before the conflict watch's conflictLookAhead.
const mergeLookAhead = 12 * time.Minute

// mergeMinNeed: the least time a merge asks to lose (less is noise of
// the prediction; the conflict watch is there).
const mergeMinNeed = 20 * time.Second

// mergeMinWithinNM: a meeting closer than this along its way is the
// conflict watch's: too late for speed, and a dog-leg there is out and
// back at once (live, CSA253 "fly heading 355", "resume own navigation"
// a second later, 9 NM before it met DLH620).
const mergeMinWithinNM = 12.0

// mergeAhead is what e, an arrival of seq, must lose to pass where its way
// meets that of one landing before it MergeSpacingNM behind: the time,
// within how many NM of its way (before it gets there) and that one; zero
// when its way meets none.
func (q *sequences) mergeAhead(e traffic.SequenceEntry, seq []traffic.SequenceEntry, items []*controlled) (time.Duration, float64, string) {
	byTail := map[string]*controlled{}
	for _, it := range items {
		if it.arr != nil {
			byTail[it.Tail] = it
		}
	}
	me := byTail[e.Callsign]
	if me == nil {
		return 0, 0, ""
	}
	all := q.cc.world.Aircraft()
	seen := func(it *controlled) (traffic.TrackedAircraft, []traffic.RoutePoint, bool) {
		id := it.object()
		for _, a := range all {
			if a.ObjectID == id && id != 0 && !a.Grounded() {
				plan := it.arr.ProcedurePlan()
				return a, plan, len(plan) > 0
			}
		}
		return traffic.TrackedAircraft{}, nil, false
	}
	mine, myPlan, ok := seen(me)
	if !ok {
		return 0, 0, ""
	}
	// In a conflict resolution, or just out of one: the conflict watch's,
	// not planned over it the same second.
	engaged := func(cs string) bool { return q.engaged != nil && q.engaged(cs, q.cc.clock.Now()) }
	if engaged(e.Callsign) {
		return 0, 0, ""
	}
	var (
		need   time.Duration
		within float64
		lead   string
	)
	for _, l := range seq {
		if l.Number >= e.Number || l.Runway != "" {
			continue // behind it, or on the adjacent final
		}
		o := byTail[l.Callsign]
		if o == nil {
			continue
		}
		other, plan, ok := seen(o)
		if !ok || engaged(l.Callsign) {
			continue // on the final or vectored, or the conflict watch's
		}
		n, w := mergeNeed(mine, myPlan, other, heldPlan(plan, other))
		if w < mergeMinWithinNM {
			continue
		}
		if n > need {
			need, within, lead = n, w, l.Callsign
		}
	}
	// The departures climbing out across its way too: it passes behind
	// them (a week of logs: 94 arrival–departure conflicts after a vector
	// or an arrival's delay).
	if q.cc.plannedTraffic != nil {
		others, ways := q.cc.plannedTraffic(mine.Tail)
		for _, a := range others {
			o := q.cc.byTail(a.Tail)
			if o == nil || o.dep == nil || len(ways[a.Tail]) == 0 || engaged(a.Tail) {
				continue
			}
			if n, w := mergeNeed(mine, myPlan, a, ways[a.Tail]); n > need && w >= mergeMinWithinNM {
				need, within, lead = n, w, a.Tail
			}
		}
	}
	if need < mergeMinNeed {
		return 0, 0, ""
	}
	return need, within, lead
}

// mergeNeed is what me, flying plan, must lose to pass MergeSpacingNM
// behind lead, flying its own, where their ways meet within
// mergeLookAhead, and within how many NM of its way it must; zero none.
func mergeNeed(me traffic.TrackedAircraft, plan []traffic.RoutePoint, lead traffic.TrackedAircraft, leadPlan []traffic.RoutePoint) (time.Duration, float64) {
	plans := map[string][]traffic.RoutePoint{me.Tail: plan, lead.Tail: leadPlan}
	opts := climbOpts
	opts.LookAhead = mergeLookAhead
	opts.Profile = func(a traffic.TrackedAircraft) []traffic.RoutePoint { return plans[a.Tail] }
	cs := traffic.PredictConflicts([]traffic.TrackedAircraft{me, lead}, opts)
	if len(cs) == 0 {
		return 0, 0
	}
	c := cs[0]
	kts := math.Max(me.GroundKts, 180)
	return traffic.SeparationTime(math.Max(traffic.DefaultMergeSpacingNM-c.ClosestNM, 1), kts), math.Max(kts*c.In.Hours(), 3)
}
