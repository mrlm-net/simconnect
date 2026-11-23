package world

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// An arrival backtracking to the one way between runway and apron and a
// departure there are planned as a pair, before either moves (#1641: TFFJ,
// the departure pushed or stood in the lane the arrival came in along, and
// each waited for the other):
//
//   - a departure's stand is chosen out of every such arrival's way in, its
//     own way out clear of that arrival's stand (clearDepartureStand);
//   - the arrival's way in and stand are planned, until it touches down,
//     clear of where departures stand and of their ways out (wayoff.go);
//   - the departure neither pushes nor taxis onto that way from the time it
//     could no longer be gone before the touchdown until the arrival is
//     parked (pairedArrival). One standing where the arrival cannot come by
//     is ordered with it by departureFirst, which every hold asks.
//
// A player's flight keeps its route, stand and clearances (#1096).

// pairLeadS is how long before an arrival's touchdown a departure still on
// its stand no longer fits in ahead of it, its taxi out not counted: the
// push (a turboprop's took 270 s at TFFJ), the start, the taxi call and the
// line-up.
const pairLeadS = 2*pushStartS + pushLegMarginS

// clearStandTries is how many stands clearDepartureStand searches a way out
// from at most.
const clearStandTries = 8

// apronPlaces is what an arrival's way in and stand are planned clear of.
type apronPlaces struct {
	bodies []airport.Occupied // where they stand or will after the push
	out    []apronWay         // the departures' ways out
	key    string             // who and in what state: a push shrinks every frame
}

// apronWay is a way taxied by an aircraft of half span half.
type apronWay struct {
	points []airport.LatLon
	half   float64
}

// halfSpanOf is half the span of model.
func halfSpanOf(model string) float64 {
	if h := traffic.MotionProfileFor(model).SpanMeters / 2; h > 0 {
		return h
	}
	return traffic.DefaultHalfSpanMeters
}

// placeNow is where it stands now, false when not placed yet.
func (it *controlled) placeNow() (airport.Occupied, bool) {
	it.mu.Lock()
	pos, hdg, model := it.view.Position, it.view.Heading, it.view.Model
	it.mu.Unlock()
	if pos == (airport.LatLon{}) {
		return airport.Occupied{}, false
	}
	return traffic.StandingPlace(pos, hdg, traffic.MotionProfileFor(model)), true
}

// placeOnStand is where an aircraft of model will stand on stand i of g, as
// spawn places it (standPlace).
func (cc *controlCenter) placeOnStand(g *airport.Graph, i int, model string) airport.Occupied {
	nose, side := cc.standPlaceRuled(g, i, model)
	p := g.Layout.Parking[i]
	return traffic.StandingPlace(traffic.StandPointSide(p, nose, side), p.Heading, traffic.MotionProfileFor(model))
}

// meetsPlace is whether way comes too near place for both to be there.
func meetsPlace(w apronWay, place airport.Occupied) bool {
	return wayMeets(w.points, place, w.half+place.HalfSpan+airport.DefaultWingtipMargin, math.Inf(1))
}

// sampledWay is way with a point at least every pushPassStepM.
func sampledWay(way []airport.LatLon) []airport.LatLon {
	if len(way) == 0 {
		return nil
	}
	out := []airport.LatLon{way[0]}
	for i := 1; i < len(way); i++ {
		a, b := way[i-1], way[i]
		n := max(1, int(math.Ceil(calc.HaversineMeters(a.Lat, a.Lon, b.Lat, b.Lon)/pushPassStepM)))
		for k := 1; k <= n; k++ {
			f := float64(k) / float64(n)
			out = append(out, airport.LatLon{Lat: a.Lat + f*(b.Lat-a.Lat), Lon: a.Lon + f*(b.Lon-a.Lon)})
		}
	}
	return out
}

// apronPlacesFor are the places standing aircraft of ours take at it's
// airport, and the departures' ways out from them.
func apronPlacesFor(it *controlled, items []*controlled) apronPlaces {
	var ap apronPlaces
	var key strings.Builder
	for _, o := range items {
		if o == it || o.ICAO != it.ICAO {
			continue
		}
		switch {
		case o.arr != nil:
			if o.arr.State() != traffic.ArrivalParked {
				continue
			}
			o.mu.Lock()
			done := o.view.Done
			o.mu.Unlock()
			if done {
				continue
			}
			if p, ok := o.placeNow(); ok {
				ap.bodies = append(ap.bodies, p)
				fmt.Fprintf(&key, "%s;", o.Tail)
			}
		case o.dep != nil:
			st := o.dep.State()
			if st != traffic.TaxiAwaitingPushback && st != traffic.TaxiPushback && st != traffic.TaxiAwaitingTaxi {
				continue
			}
			p, ok := o.placeNow()
			if !ok {
				continue
			}
			ap.bodies = append(ap.bodies, p)
			// Pushing, or cleared to: where the push takes it too.
			if st != traffic.TaxiAwaitingTaxi {
				if push, ok := o.dep.Occupies(); ok {
					push.HalfSpan = p.HalfSpan
					ap.bodies = append(ap.bodies, push)
				}
			}
			n := 0
			if r := o.dep.Route(); r != nil && len(r.Points) > 1 {
				o.mu.Lock()
				model := o.view.Model
				o.mu.Unlock()
				n = len(r.Points)
				ap.out = append(ap.out, apronWay{r.Points, halfSpanOf(model)})
			}
			fmt.Fprintf(&key, "%s/%d/%d;", o.Tail, st, n)
		}
	}
	ap.key = key.String()
	return ap
}

// standClear is whether an aircraft of model parked on stand i of g is out
// of the departures' ways out.
func (ap apronPlaces) standClear(cc *controlCenter, g *airport.Graph, i int, model string) bool {
	if len(ap.out) == 0 || g == nil || i < 0 || i >= len(g.Layout.Parking) {
		return true
	}
	place := cc.placeOnStand(g, i, model)
	for _, w := range ap.out {
		if meetsPlace(w, place) {
			return false
		}
	}
	return true
}

// wayIn is a backtracking arrival's way from the runway to its stand, until
// it is parked; false for any other. Off the runway, the way it drives,
// its turn onto the stand included, once its ground path goes in to the
// stand (TaxiInWay, #1694): the route alone left out the swing past the
// stand.
func (it *controlled) wayIn() (apronWay, bool) {
	if it.arr == nil {
		return apronWay{}, false
	}
	p := it.arr.Plan()
	if p == nil || !p.Backtrack || p.Route == nil || len(p.Route.Points) < 2 {
		return apronWay{}, false
	}
	var pts []airport.LatLon
	switch it.arr.State() {
	case traffic.ArrivalApproaching:
		pts = p.Route.Points[min(max(p.ClearAt(), 0), len(p.Route.Points)-2):]
	case traffic.ArrivalParking:
		pts = p.Route.Points[len(p.Route.Points)-2:]
	default:
		if pts = it.backtrackWay(true); len(pts) < 2 {
			return apronWay{}, false
		}
	}
	switch it.arr.State() {
	case traffic.ArrivalVacating, traffic.ArrivalAwaitingTaxi, traffic.ArrivalTaxiing, traffic.ArrivalParking:
		end := p.Route.Points[len(p.Route.Points)-1]
		if way := it.arr.TaxiInWay(); len(way) >= 2 && calc.HaversineMeters(way[len(way)-1].Lat, way[len(way)-1].Lon, end.Lat, end.Lon) < taxiInEndM {
			pts = way
		}
	}
	it.mu.Lock()
	model := it.view.Model
	it.mu.Unlock()
	return apronWay{pts, halfSpanOf(model)}, true
}

// taxiInEndM: an arrival's driven way ending this near its route's end
// goes in to its stand (wayIn); one ending further off is still only its
// way off the runway.
const taxiInEndM = 80.0

// pairedArrival is the backtracking arrival departure it waits on its stand
// for (or where it was pushed to): one whose way in meets its way out or
// its push, in whichever direction, that it can't be gone before. True with
// it while that arrival is still on approach. One standing in the
// arrival's way in goes first or waits as departureFirst says.
func (it *controlled) pairedArrival() (*controlled, bool) {
	if it.dep == nil {
		return nil, false
	}
	switch it.dep.State() {
	case traffic.TaxiAwaitingPushback, traffic.TaxiAwaitingTaxi:
	default:
		return nil, false
	}
	r := it.dep.Route()
	if r == nil || len(r.Points) < 2 {
		return nil, false
	}
	it.mu.Lock()
	half := halfSpanOf(it.view.Model)
	it.mu.Unlock()
	var out, push airport.Occupied
	var looked, hasPush bool
	var coming *controlled
	for _, o := range it.cc.snapshotItems() {
		if o == it || o.ICAO != it.ICAO {
			continue
		}
		in, ok := o.wayIn()
		if !ok {
			continue
		}
		approaching := o.arr.State() == traffic.ArrivalApproaching
		if approaching {
			_, before, ok := o.inboundWay()
			if !ok || before >= pairLeadS+r.Length/(traffic.TaxiSpeedKts*1852/3600) {
				continue // gone before it is down
			}
		}
		if place, ok := it.placeNow(); ok && meetsPlace(in, place) {
			if it.departureFirst(o) {
				continue // in its way where it stands, and first
			}
			if !approaching {
				return o, false
			}
			if coming == nil {
				coming = o
			}
			continue
		}
		if !looked {
			looked = true
			out = airport.Occupied{Points: sampledWay(r.Points), HalfSpan: half}
			push, hasPush = it.dep.PushArea()
			push.HalfSpan = math.Max(push.HalfSpan, half)
		}
		if !waysOpposed(r.Points, in.points, backtrackOpposedM) && !meetsPlace(in, out) && !(hasPush && meetsPlace(in, push)) {
			continue
		}
		if !approaching {
			return o, false
		}
		if coming == nil {
			coming = o
		}
	}
	return coming, coming != nil
}

// departureFirst is the one order between departure it, standing in the
// way in of backtracking arrival arr, and that arrival (#1641); the holds
// on its push and taxi (pairedArrival, passingFor) ask it. Before arr
// lands it goes first where it can be at its holding point before arr is
// down (pairLeadS and its taxi), or where arr has no way in clear of it
// (one whose way in is not planned, a player's (#1096), or none found:
// apronBlocked, keepWayOffClear); else it waits, and arr's way in is
// planned round it. From touchdown arr goes first: a departure put there
// since is not (standForDeparture). Once first, first for arr: it may be
// on its way.
func (it *controlled) departureFirst(arr *controlled) bool {
	it.mu.Lock()
	first := it.pairFirst[arr]
	it.mu.Unlock()
	if first {
		return true
	}
	if arr.arr == nil || it.dep == nil || arr.arr.State() != traffic.ArrivalApproaching {
		return false
	}
	if arr.apronLooked.Load() && !arr.apronBlocked.Load() && !arr.gates.Load() {
		r := it.dep.Route()
		_, before, ok := arr.inboundWay()
		if r == nil || !ok || before < pairLeadS+r.Length/(traffic.TaxiSpeedKts*1852/3600) {
			return false
		}
	}
	it.mu.Lock()
	if it.pairFirst == nil {
		it.pairFirst = map[*controlled]bool{}
	}
	it.pairFirst[arr] = true
	it.mu.Unlock()
	return true
}

// wayInBlockedBy is whether departure dep stands in arrival it's way in.
func (it *controlled) wayInBlockedBy(dep *controlled) bool {
	in, ok := it.wayIn()
	if !ok {
		return false
	}
	place, ok := dep.placeNow()
	return ok && meetsPlace(in, place)
}

// clearDepartureStand picks and reserves, for req.Owner, a stand for a
// departure from runway that is out of every backtracking arrival's way in
// and whose way out passes none of their stands. ok false: no such arrival,
// the stand is assigned as ever. A route search for each stand tried, so
// never on the connection's goroutine.
func (cc *controlCenter) clearDepartureStand(g *airport.Graph, alloc *traffic.StandAllocator, req traffic.StandRequirements, use, model, runway string) (stand int, ok bool, err error) {
	if g == nil || alloc == nil {
		return -1, false, nil
	}
	var ins []apronWay
	var parked []airport.Occupied
	for _, o := range cc.snapshotItems() {
		if o.ICAO != g.Layout.ICAO {
			continue
		}
		in, is := o.wayIn()
		if !is {
			continue
		}
		o.mu.Lock()
		to, theirs := o.stand, o.view.Model
		o.mu.Unlock()
		if to >= 0 && to < len(g.Layout.Parking) {
			place := cc.placeOnStand(g, to, theirs)
			in.points = append(append([]airport.LatLon(nil), in.points...), traffic.StandPoint(g.Layout.Parking[to], 0))
			parked = append(parked, place)
		}
		ins = append(ins, in)
	}
	if len(ins) == 0 {
		return -1, false, nil
	}
	half := req.HalfSpan
	if half <= 0 {
		half = traffic.DefaultHalfSpanMeters
	}
	for _, i := range g.Layout.SuitableStands(half) {
		place := cc.placeOnStand(g, i, model)
		for _, in := range ins {
			if meetsPlace(in, place) {
				req.Avoid = append(req.Avoid, i)
				break
			}
		}
	}
	if runway == "" || strings.EqualFold(runway, "active") {
		if runway, err = cc.pickRunwayFor(g, false, -1, model); err != nil {
			return -1, true, err
		}
	}
	opts := airport.RouteOptions{HalfSpan: half}
	for range clearStandTries {
		i, err := assignStand(alloc, req, use)
		if err != nil {
			return -1, true, err
		}
		r, err := g.RouteToRunway(i, runway, opts)
		clear := err == nil
		if clear {
			out := apronWay{r.Points, half}
			for _, p := range parked {
				if meetsPlace(out, p) {
					clear = false
					break
				}
			}
		}
		if clear {
			return i, true, nil
		}
		alloc.Release(i)
		req.Avoid = append(req.Avoid, i)
	}
	return -1, true, fmt.Errorf("%w: none out of a backtracking arrival's way", traffic.ErrNoStand)
}

// standForDeparture is the stand for a departure spawned without one, out
// of the arrivals' ways in (clearDepartureStand): -1 for spawn to assign it
// as ever. A player's is left as it is. Off the connection's goroutine.
func (cc *controlCenter) standForDeparture(g *airport.Graph, r SpawnRequest) int {
	stand, _ := cc.departureStand(g, r)
	return stand
}

// departureStand is standForDeparture, and wait where there is none such
// while one of those arrivals has landed, or will before the departure,
// on the stand spawn would give it, could be at its holding point: it is
// not spawned yet, as the arrival would come by it there or meet it on
// its way out (#1641: TFFJ, every stand that fits on the one way in).
func (cc *controlCenter) departureStand(g *airport.Graph, r SpawnRequest) (stand int, wait bool) {
	if r.Kind != "departure" || r.Stand >= 0 || r.Gates || r.adopt != 0 {
		return -1, false
	}
	model, _, _ := strings.Cut(r.Model, liverySep)
	if model == "" {
		model = "FSLTL A320 Air France SL"
	}
	airline := airlineOf(r.Tail)
	req := traffic.StandRequirements{Owner: "(stand search) " + r.Tail, Airline: airline, HalfSpan: traffic.ProfileFor(model).Motion.SpanMeters / 2, OffBlock: r.pushAt,
		NoJetway: traffic.RemoteStands(model, airline), Overnight: r.overnight, Bridged: cc.jetwayStands(g.Layout.ICAO), Prefer: cc.jetwayGates(g.Layout.ICAO), Other: r.Other}
	use := r.StandUse
	if r.Circuit {
		use = standGA
	}
	alloc := cc.allocator(g)
	stand, ok, err := cc.clearDepartureStand(g, alloc, req, use, model, r.Runway)
	if !ok {
		return -1, false
	}
	if err != nil {
		if cc.landedWayIn(g.Layout.ICAO, cc.departureLeadS(g, alloc, req, use, model, r.Runway)) {
			return -1, true
		}
		cc.log.printf("%-6s departure: no stand out of the arrivals' way in: %v", r.Tail, err)
		return -1, false
	}
	// Only looked up: spawn reserves it under the call sign it gives it.
	alloc.Release(stand)
	return stand, false
}

// landedWayIn is whether a backtracking arrival at icao has a way in to
// come along still and has landed, or is down within leadS: from then
// until it is parked.
func (cc *controlCenter) landedWayIn(icao string, leadS float64) bool {
	for _, o := range cc.snapshotItems() {
		if o.ICAO != icao || o.arr == nil {
			continue
		}
		if _, ok := o.wayIn(); !ok {
			continue
		}
		if o.arr.State() != traffic.ArrivalApproaching {
			return true
		}
		if _, before, ok := o.inboundWay(); ok && before < leadS {
			return true
		}
	}
	return false
}

// departureLeadS is how long a departure spawned on the stand assigned as
// ever takes to be at its holding point: pairLeadS and its taxi (as
// pairedArrival counts it). pairLeadS alone where it has no route.
func (cc *controlCenter) departureLeadS(g *airport.Graph, alloc *traffic.StandAllocator, req traffic.StandRequirements, use, model, runway string) float64 {
	i, err := assignStand(alloc, req, use)
	if err != nil {
		return pairLeadS
	}
	defer alloc.Release(i)
	if runway == "" || strings.EqualFold(runway, "active") {
		if runway, err = cc.pickRunwayFor(g, false, -1, model); err != nil {
			return pairLeadS
		}
	}
	half := req.HalfSpan
	if half <= 0 {
		half = traffic.DefaultHalfSpanMeters
	}
	r, err := g.RouteToRunway(i, runway, airport.RouteOptions{HalfSpan: half})
	if err != nil {
		return pairLeadS
	}
	return pairLeadS + r.Length/(traffic.TaxiSpeedKts*1852/3600)
}

// spawnWhenClear spawns departure r, put off by departureStand, on a
// stand out of the landed arrivals' way in once one is free; asked again
// each wayOffRetry till then. Off the connection's goroutine.
func (cc *controlCenter) spawnWhenClear(g *airport.Graph, r SpawnRequest) {
	again := func() { cc.pending.later(cc.clock.Now().Add(wayOffRetry), func() { go cc.spawnWhenClear(g, r) }) }
	s, wait := cc.departureStand(g, r)
	if wait {
		again()
		return
	}
	r.Stand = s
	err := cc.do(func() error {
		_, e := cc.spawn(g, r)
		return e
	})
	switch {
	case s >= 0 && errors.Is(err, traffic.ErrStandTaken), errors.Is(err, traffic.ErrSpawnBlocked):
		again() // taken since it was looked up, or not now
	case err != nil:
		cc.log.printf("%-6s departure: spawn at stand %d, runway %s failed: %v", r.Tail, s, r.Runway, err)
	}
}
