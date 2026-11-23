package world

import (
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Ground holds a pushback while one of ours taxies through what the push
// will sweep: pushed first, the departure stood on that taxiway facing
// the traffic coming along it, and neither could pass (live, LKPR
// 2026-11-11: CSA1889 pushed from C26 onto H1 nose to nose with TVS1832
// taxiing in to C27; TVS1832 had given way to the push 245 m out). The
// push waits on its stand, told once ("standby, traffic passing behind"),
// and is answered once the traffic is by. Generic: any airport, any two.
const (
	// pushPassAheadM: traffic with the push area within this much of the
	// way ahead of it holds the push (a minute or two of taxiing).
	pushPassAheadM = 400.0
	// pushPassStepM: the way ahead is looked at in steps of this.
	pushPassStepM = 5.0
	// pushPassMaxWait: held this long, the push is answered anyway (the
	// traffic stopped for good, or for something else).
	pushPassMaxWait = 3 * time.Minute
	// pushPassRetry: a held push is asked about again after this.
	pushPassRetry = 10 * time.Second
	// pushLegM: the first this much of its taxi route, from the stand, is
	// the leg it taxis once pushed. An arrival coming the other way along
	// it while it pushes and taxis there holds the push too: pushed, it
	// would stand on that taxilane facing the arrival, and neither could
	// pass (#1574: XPLY, a single-lane apron loop, KLM1817 pushed facing
	// AFR919 coming in along it).
	pushLegM = 250.0
	// pushStartS: seconds from the push approved to taxiing (push, start,
	// taxi call), for when it is on its leg.
	pushStartS = 150.0
	// pushLegMarginS: the arrival comes to the leg no sooner than this
	// after the departure would be off it, or the push is held.
	pushLegMarginS = 60.0
	// rolloutMps: an arrival's mean speed from touchdown to its exit.
	rolloutMps = 30.0
	// pushLegMaxWait: held for an arrival coming along its leg,
	// pushPassMaxWait does not release it, this does: released after
	// pushPassMaxWait, it met the next one coming (#1574: LKPR, DP500 held
	// 3 minutes for AR500, then pushed onto H1 facing AR501).
	pushLegMaxWait = 10 * time.Minute
)

// passingBehind is one of ours at its airport taxiing with what its push
// will sweep on the next pushPassAheadM of its way, nil none.
func (it *controlled) passingBehind() *controlled {
	o, _ := it.passingFor()
	return o
}

// passingFor is passingBehind, or an arrival to come the other way along
// the leg it taxis once pushed (pushLegM) while it would be on it, with
// true; such an arrival first. Planned up front: when each comes is
// estimated from where it is (on approach, rolling out or taxiing) along
// its planned exit and taxi-in at planned speeds, against the push, the
// start and the taxi off the shared stretch (#1574: LKPR, DP301 pushed
// from C23 onto H1 as AR301 was on final for 06, to C31 along H1).
func (it *controlled) passingFor() (*controlled, bool) {
	if it.dep == nil {
		return nil, false
	}
	area, ok := it.dep.PushArea()
	if !ok || len(area.Points) == 0 {
		return nil, false
	}
	var behind *controlled
	var leg []airport.LatLon
	if r := it.dep.Route(); r != nil {
		leg = firstMeters(r.Points, pushLegM)
	}
	taxiMps := traffic.TaxiSpeedKts * 1852 / 3600
	for _, o := range it.cc.snapshotItems() {
		if o == it || o.ICAO != it.ICAO {
			continue
		}
		ahead := pushPassAheadM
		_, way, movable, ok := o.groundWay()
		before := 0.0
		if bt := o.backtrackWay(false); bt != nil {
			// Backtracking on the runway to its way off (#1563): its whole
			// taxi-in, the push not onto the one way it comes.
			way, movable, ok, ahead = bt, true, true, math.Inf(1)
		} else if !ok || !movable {
			way, before, ok = o.inboundWay()
			movable = ok
		}
		if !ok || !movable {
			continue
		}
		// Standing where a backtracking arrival can't come by: the order
		// is departureFirst's (#1641).
		if o.arr != nil && o.wayInBlockedBy(it) && it.departureFirst(o) {
			continue
		}
		o.mu.Lock()
		half := traffic.MotionProfileFor(o.view.Model).SpanMeters / 2
		o.mu.Unlock()
		reach := half + area.HalfSpan + traffic.HeadOnMarginMeters
		if fa, fb, ln, _, ok := traffic.OpposedStretch(way, leg, reach); ok && before+fa/taxiMps < pushStartS+(fb+ln)/taxiMps+pushLegMarginS {
			return o, true
		}
		if before == 0 && wayMeets(way, area, reach, ahead) {
			if math.IsInf(ahead, 1) {
				return o, false
			}
			if behind == nil {
				behind = o
			}
		}
	}
	return behind, false
}

// inboundWay is an arrival's taxi-in route from its exit on, before it
// taxis (on approach, landing, rolling out or vacating), and the seconds
// until it is at the exit: to touchdown at ApproachSpeedKts, then the
// rollout. ok false for any other.
func (it *controlled) inboundWay() (way []airport.LatLon, before float64, ok bool) {
	if it.arr == nil {
		return nil, 0, false
	}
	st := it.arr.State()
	if st != traffic.ArrivalApproaching && st != traffic.ArrivalLanding && st != traffic.ArrivalRollout && st != traffic.ArrivalVacating {
		return nil, 0, false
	}
	p := it.arr.Plan()
	if p == nil || p.Route == nil || p.VacateIndex < 0 || p.VacateIndex >= len(p.Route.Points)-1 {
		return nil, 0, false
	}
	if st == traffic.ArrivalApproaching || st == traffic.ArrivalLanding {
		it.mu.Lock()
		pos := it.view.Position
		it.mu.Unlock()
		th := p.End.LandingThreshold()
		before = calc.HaversineMeters(pos.Lat, pos.Lon, th.Lat, th.Lon)/(traffic.ApproachSpeedKts*1852/3600) + p.Exit.Along/rolloutMps
	}
	return p.Route.Points[p.VacateIndex:], before, true
}

// firstMeters is way up to meters along it from its first point.
func firstMeters(way []airport.LatLon, meters float64) []airport.LatLon {
	gone := 0.0
	for i := 1; i < len(way); i++ {
		gone += calc.HaversineMeters(way[i-1].Lat, way[i-1].Lon, way[i].Lat, way[i].Lon)
		if gone > meters {
			return way[:i+1]
		}
	}
	return way
}

// wayMeets reports whether the first ahead metres of way come within reach
// of any point of area.
func wayMeets(way []airport.LatLon, area airport.Occupied, reach, ahead float64) bool {
	near := func(p airport.LatLon) bool {
		for _, q := range area.Points {
			if calc.HaversineMeters(p.Lat, p.Lon, q.Lat, q.Lon) < reach {
				return true
			}
		}
		return false
	}
	gone := 0.0
	for i := 0; i+1 < len(way) && gone <= ahead; i++ {
		a, b := way[i], way[i+1]
		seg := calc.HaversineMeters(a.Lat, a.Lon, b.Lat, b.Lon)
		steps := int(math.Max(1, math.Ceil(seg/pushPassStepM)))
		for k := 0; k < steps && gone+float64(k)*seg/float64(steps) <= ahead; k++ {
			f := float64(k) / float64(steps)
			if near(airport.LatLon{Lat: a.Lat + f*(b.Lat-a.Lat), Lon: a.Lon + f*(b.Lon-a.Lon)}) {
				return true
			}
		}
		gone += seg
	}
	if len(way) > 0 && gone <= ahead {
		return near(way[len(way)-1])
	}
	return false
}

// holdPushForPassing is whether ground holds its push for traffic passing
// behind: told once, asked again after pushPassRetry; never longer than
// pushPassMaxWait in all (pushLegMaxWait for traffic along its leg).
func (it *controlled) holdPushForPassing(req string) bool {
	o, leg := it.passingFor()
	now := it.cc.clock.Now()
	wait := pushPassMaxWait
	if leg {
		wait = pushLegMaxWait
	}
	// One backtracking on the runway is not waited out: it has to come by.
	waitOut := o == nil || o.backtrackWay(false) == nil
	it.mu.Lock()
	if o == nil || it.request != req || waitOut && !it.pushPassFrom.IsZero() && now.Sub(it.pushPassFrom) > wait {
		it.mu.Unlock()
		return false
	}
	if it.pushPassFrom.IsZero() {
		it.pushPassFrom = now
	}
	first := !it.spoken["standby passing"]
	it.spoken["standby passing"] = true
	it.mu.Unlock()
	if first {
		it.cc.log.printf("%-6s ground: pushback held — %s taxiing through the push", it.Tail, o.Tail)
		it.say(traffic.Transmission{Position: traffic.PosGround, Callsign: it.Tail, Intent: traffic.IntentStandby,
			Text: it.Tail + ", standby, traffic passing behind"})
	}
	it.cc.pending.later(now.Add(pushPassRetry), func() { it.askGround(req) })
	return true
}

// backtrackWay is the way an arrival backtracking along its runway to its
// way off (#1563, ArrivalPlan.Backtrack) will taxi off the runway to its
// stand, while it lands, rolls and backtracks; with taxiIn, until it
// parks too (from where it is). nil otherwise.
func (it *controlled) backtrackWay(taxiIn bool) []airport.LatLon {
	if it.arr == nil {
		return nil
	}
	state := it.arr.State()
	switch state {
	case traffic.ArrivalLanding, traffic.ArrivalRollout:
	case traffic.ArrivalVacating, traffic.ArrivalAwaitingTaxi, traffic.ArrivalTaxiing:
		if !taxiIn {
			return nil
		}
	default:
		return nil
	}
	p := it.arr.Plan()
	if p == nil || !p.Backtrack || p.Route == nil || p.ClearAt() >= len(p.Route.Points)-1 {
		return nil
	}
	from := p.ClearAt()
	if state != traffic.ArrivalLanding && state != traffic.ArrivalRollout {
		it.mu.Lock()
		pos := it.view.Position
		it.mu.Unlock()
		best := math.Inf(1)
		for i := p.ClearAt(); i < len(p.Route.Points); i++ {
			q := p.Route.Points[i]
			if d := calc.HaversineMeters(pos.Lat, pos.Lon, q.Lat, q.Lon); d < best {
				from, best = i, d
			}
		}
		from = max(from-1, p.ClearAt()) // the edge it is on
	}
	return p.Route.Points[from:]
}

// Backtracking arrivals (#1563) come back along the one way between the
// runway and the apron where no taxiway runs beside the runway (TNCM):
// a departure going out along that way the other way is held on its
// stand, or where it is, till the arrival is by — told once, asked again
// every pushPassRetry. Only those arrivals: elsewhere the head-on check
// as before. Paired with it from its approach on, and wherever their ways
// meet (#1641, apron_pair.go: pairedArrival).

// backtrackOpposedM: a departure's route this near an arrival's way off,
// the other way, is held for it.
const backtrackOpposedM = 30.0

// waysOpposed reports whether ways a and b come within reach of each other
// heading at least 120° apart somewhere (sampled every pushPassStepM*2).
func waysOpposed(a, b []airport.LatLon, reach float64) bool {
	type sample struct {
		p   airport.LatLon
		hdg float64
	}
	sampled := func(w []airport.LatLon) []sample {
		var out []sample
		for i := 0; i+1 < len(w); i++ {
			p, q := w[i], w[i+1]
			seg := calc.HaversineMeters(p.Lat, p.Lon, q.Lat, q.Lon)
			if seg < 0.5 {
				continue
			}
			h := calc.BearingDegrees(p.Lat, p.Lon, q.Lat, q.Lon)
			n := int(math.Max(1, math.Ceil(seg/(2*pushPassStepM))))
			for k := 0; k < n; k++ {
				f := float64(k) / float64(n)
				out = append(out, sample{airport.LatLon{Lat: p.Lat + f*(q.Lat-p.Lat), Lon: p.Lon + f*(q.Lon-p.Lon)}, h})
			}
		}
		return out
	}
	sa, sb := sampled(a), sampled(b)
	for _, x := range sa {
		for _, y := range sb {
			if math.Abs(math.Mod(y.hdg-x.hdg+540, 360)-180) >= 120 && calc.HaversineMeters(x.p.Lat, x.p.Lon, y.p.Lat, y.p.Lon) < reach {
				return true
			}
		}
	}
	return false
}

// holdForBacktrack is whether ground holds its push or taxi (req) for a
// backtracking arrival it is paired with (pairedArrival): until that is
// parked, but for one still on approach no longer than pushLegMaxWait, or
// a stream of arrivals would hold it for good.
func (it *controlled) holdForBacktrack(req string) bool {
	o, approaching := it.pairedArrival()
	now := it.cc.clock.Now()
	it.mu.Lock()
	if o == nil {
		it.pairHeldFrom = time.Time{}
		it.mu.Unlock()
		return false
	}
	if it.request != req || approaching && !it.pairHeldFrom.IsZero() && now.Sub(it.pairHeldFrom) > pushLegMaxWait {
		it.mu.Unlock()
		return false
	}
	if it.pairHeldFrom.IsZero() {
		it.pairHeldFrom = now
	}
	first := !it.spoken["standby backtrack"]
	it.spoken["standby backtrack"] = true
	it.mu.Unlock()
	if first {
		it.cc.log.printf("%-6s ground: %s held — %s backtracking to its way in, coming the other way", it.Tail, req, o.Tail)
		it.say(traffic.Transmission{Position: traffic.PosGround, Callsign: it.Tail, Intent: traffic.IntentStandby,
			Text: it.Tail + ", standby, landing traffic to pass"})
	}
	it.cc.pending.later(now.Add(pushPassRetry), func() { it.askGround(req) })
	return true
}
