package world

import (
	"fmt"
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Transit traffic (#1485), with OverflightTraffic.Transits: the
// departures and arrivals of the airports in and near the area crossing it
// on their climb or descent (traffic.Transits). Each is an overflight to
// the manager (its Enter and Exit), spawned airborne as NonATC on our
// waypoints like the others, but flown on its flight plan's vertical
// profile (nav.Plan: climb, cruise at the level for its distance and type,
// descent from its top of descent), from where it comes into view (its
// floor out of view, or the area's edge) to where it leaves it (60 NM past
// the area, or its floor on the descent, out of view). It is taken out
// where its way ends. Its level is planned up front against everyone's
// planned ways (clearTransitLevel): a lower top of climb or a later top of
// descent, never an orbit or a hold.

// transitStepNM: a transit's way has a point every this many NM.
const transitStepNM = 10.0

// transitPastNM: a transit's way goes on this far past the area (as an
// overflight's great circle).
const transitPastNM = 60.0

// transitEndNM: a transit this near the end of its way in the simulator
// has flown it.
const transitEndNM = 3.0

// isTransit reports whether a flight is a transit of ours.
func (s *scheduler) isTransit(callsign string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.over.transitCS[callsign]
}

// transitViewNM is how far from every viewer a transit appears and goes:
// past the materialise range (the tiers'; else 40 NM), with 5 NM to spare.
func (s *scheduler) transitViewNM() float64 {
	t := s.tiersNow()
	r := 40.0
	if t.On() {
		r = t.MaterialiseNM
	}
	return r + 5
}

// transits are the transits entering the area in an hour, none of their
// call signs among used.
func (s *scheduler) transits(o traffic.OverflightOptions, used map[string]bool, from, to time.Time) []traffic.Flight {
	t := traffic.Transits(s.config(), traffic.TransitOptions{Centre: o.Centre, RadiusNM: o.RadiusNM, Density: o.Density, Seed: o.Seed ^ 0x1485,
		Exclude: o.Exclude, ViewNM: s.transitViewNM()}, from, to)
	out := t[:0]
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.over.transitCS == nil {
		s.over.transitCS = map[string]bool{}
	}
	for _, f := range t {
		if used[f.Callsign] {
			continue
		}
		s.over.transitCS[f.Callsign] = true
		out = append(out, f)
	}
	return out
}

// spawnTransit flies transit f on its plan's profile (fp), from where it
// comes into view to where it leaves it.
func (s *scheduler) spawnTransit(f traffic.ManagedFlight, e *enrouteAC, model string, fp *nav.FlightPlan, kts float64) error {
	c, ok := s.cc.world.Centre()
	if !ok {
		return fmt.Errorf("%w: no area", traffic.ErrSpawnImpossible)
	}
	floor := traffic.TransitFloorFt
	view := s.transitViewNM()
	views := append(s.viewers(s.cc.world.Aircraft()), c)
	inView := func(p airport.LatLon) bool { return nearestNM(views, p) < view }
	// The area along its plan, every 5 NM; above its floor.
	dIn, dOut, dUp, dDown := -1.0, -1.0, -1.0, -1.0
	for d := 0.0; d <= fp.DistanceNM; d += 5 {
		p, alt, _ := fp.PositionAt(d)
		if calc.HaversineNM(c.Lat, c.Lon, p.Lat, p.Lon) <= overflightRadiusNM {
			if dIn < 0 {
				dIn = d
			}
			dOut = d
		}
		if alt >= floor {
			if dUp < 0 {
				dUp = d
			}
			dDown = d
		}
	}
	if dIn < 0 || dUp < 0 {
		return fmt.Errorf("%w: its plan %s → %s never crosses the area above %s", traffic.ErrSpawnImpossible, f.Origin, f.Destination, flLog(floor))
	}
	start := math.Max(dIn, dUp)
	end := math.Min(dOut+transitPastNM, dDown)
	for end > start && inView(pointOn(fp, end)) {
		end -= 5 // its end out of view
	}
	if p := pointOn(fp, start); inView(p) {
		return fmt.Errorf("%w: it would appear in view (%.0f NM from a viewer)", traffic.ErrSpawnImpossible, nearestNM(views, p))
	}
	// On from where it comes into view by the time since it was due there.
	dist := start
	if !f.Enter.IsZero() {
		dist += math.Max(0, s.cc.clock.Now().Sub(f.Enter).Hours()) * kts
	}
	if end-dist < 2*transitStepNM {
		return fmt.Errorf("%w: its way across is flown already", traffic.ErrSpawnImpossible)
	}
	// Where it appears and where it goes, both out of view: late, it is on
	// from its start, maybe into view (live: in at 15 NM, gone 5 s later at
	// the end of its way trimmed to its start).
	if p := pointOn(fp, dist); inView(p) {
		return fmt.Errorf("%w: late, it would appear in view (%.0f NM from a viewer)", traffic.ErrSpawnImpossible, nearestNM(views, p))
	}
	if p := pointOn(fp, end); inView(p) {
		return fmt.Errorf("%w: its way would end in view (%.0f NM from a viewer)", traffic.ErrSpawnImpossible, nearestNM(views, p))
	}
	cruise := float64(fp.CruiseFL) * 100
	var route []traffic.RoutePoint
	for d := dist; ; d += transitStepNM {
		d = math.Min(d, end)
		p, alt, _ := fp.PositionAt(d)
		route = append(route, traffic.RoutePoint{Position: p, AltFt: math.Round(alt/100) * 100})
		if d >= end {
			break
		}
	}
	// The level parts at its cruise level of the direction, leg by leg; the
	// speeds by level.
	for i := range route {
		a, b := route[max(i-1, 0)].Position, route[i].Position
		if i == 0 {
			b = route[1].Position
		}
		if cruise > 0 && math.Abs(route[i].AltFt-cruise) < 100 {
			route[i].AltFt = cruiseLevelFt(cruise, calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon))
		}
		route[i].Kts = traffic.EnrouteSpeedKts(route[i].AltFt, kts)
	}
	if len(route) < 2 {
		return fmt.Errorf("%w: its way across is flown already", traffic.ErrSpawnImpossible)
	}
	along := fp.Route
	if along == "" {
		along = "direct"
	}
	return s.spawnEnrouteOn(f, e, model, route, "transit, "+along)
}

// pointOn is where fp is d NM along it.
func pointOn(fp *nav.FlightPlan, d float64) airport.LatLon {
	p, _, _ := fp.PositionAt(d)
	return p
}

// clearWay is route planned clear of everyone's planned ways: a transit's
// by its top of climb or descent (clearTransitLevel), the others' by their
// cruise level (clearLevel).
func (s *scheduler) clearWay(f traffic.ManagedFlight, route []traffic.RoutePoint) ([]traffic.RoutePoint, string, bool) {
	if f.Overflight() && s.isTransit(f.Callsign) {
		return s.clearTransitLevel(f.Callsign, route)
	}
	return s.clearLevel(f.Callsign, route)
}

// transitLevelSteps: the tops tried below a transit's planned one, each a
// level of its direction 2000 ft lower.
const transitLevelSteps = 4

// clearTransitLevel is a transit's route with its top (of climb, or where
// its descent begins) where its way meets nobody: as planned, else capped
// at a level of its direction below, down to its ends' altitudes (a
// climb levels off sooner, a descent begins at the lower level sooner);
// the one it would meet as planned; ok false when none is clear.
func (s *scheduler) clearTransitLevel(callsign string, route []traffic.RoutePoint) ([]traffic.RoutePoint, string, bool) {
	if len(route) < 2 {
		return route, "", true
	}
	others, plans := s.plannedTraffic(callsign)
	top, ends := 0.0, math.Max(route[0].AltFt, route[len(route)-1].AltFt)
	for _, p := range route {
		top = math.Max(top, p.AltFt)
	}
	a, b := route[0].Position, route[len(route)-1].Position
	trk := calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon)
	caps := []float64{top}
	for c := levelBelowFt(top-1000, trk); len(caps) <= transitLevelSteps && c >= ends && c >= 10000; c = levelBelowFt(c-2000+1, trk) {
		if c < top {
			caps = append(caps, c)
		}
	}
	first := ""
	way := route
	if _, ok := s.keepsClear(callsign, route); !ok {
		// Each top cuts into a terminal airspace: round it (#1486).
		clear := false
		for _, c := range caps {
			if _, ok := s.keepsClear(callsign, capped(route, c)); ok {
				clear = true
				break
			}
		}
		if !clear {
			r, ok := s.aroundTerminals(callsign, route)
			if !ok {
				return route, "terminal airspace", false
			}
			way = r
		}
	}
	for i, c := range caps {
		r := capped(way, c)
		if _, ok := s.keepsClear(callsign, r); !ok {
			continue
		}
		who := meetsAlong(callsign, r, others, plans)
		if who == "" {
			return r, first, true
		}
		if i == 0 {
			first = who
		}
	}
	return route, first, false
}

// capped is route with no point above c.
func capped(route []traffic.RoutePoint, c float64) []traffic.RoutePoint {
	r := append([]traffic.RoutePoint(nil), route...)
	for j := range r {
		if r[j].AltFt > c {
			r[j].AltFt = c
			r[j].Kts = math.Min(r[j].Kts, traffic.EnrouteSpeedKts(c, r[j].Kts))
		}
	}
	return r
}

// transitsTick takes out the transits that have flown their way
// (scheduler goroutine).
func (s *scheduler) transitsTick(now time.Time) {
	s.mu.Lock()
	var done []string
	var inSim []*enrouteAC
	for cs, e := range s.enroute {
		if !s.over.transitCS[cs] || len(e.route) == 0 {
			continue
		}
		switch {
		case e.vt != nil && !e.materialising:
			if e.vt.Done() {
				done = append(done, cs)
			}
		case e.objectID != 0:
			inSim = append(inSim, e)
		}
	}
	s.mu.Unlock()
	if len(inSim) > 0 {
		pos := map[uint32]airport.LatLon{}
		for _, a := range s.cc.world.Aircraft() {
			pos[a.ObjectID] = a.Position
		}
		for _, e := range inSim {
			last := e.route[len(e.route)-1].Position
			if p, ok := pos[e.objectID]; ok && calc.HaversineNM(p.Lat, p.Lon, last.Lat, last.Lon) < transitEndNM {
				done = append(done, e.f.Callsign)
			}
		}
	}
	for _, cs := range done {
		s.cc.log.printf("%-6s schedule: transit at the end of its way", cs)
		s.mgr.Remove(cs, now)
	}
}
