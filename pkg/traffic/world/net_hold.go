package world

import (
	"maps"
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// A change the safety net gave one of the player's doesn't end by itself
// in a level, speed or route the player didn't clear (#1662): the net
// resolves, what follows is the player's, as every routine clearance is.
// Where the player cleared a level (or a speed) the route before the
// change has it, and a level (or speed) change ends back on it as before.

// netHold is a safety net's change held on for the player.
type netHold struct {
	id      uint32
	icao    string
	pos     traffic.Position
	r       traffic.Resolution
	since   time.Time
	planned []traffic.RoutePoint // the route before it, flown again off the staffed station
	leg     airport.LatLon       // a heading's: the end of its leg, put on while it is held
}

// netChange is the safety net's change r to a, the player's en route
// aircraft e, as it is held for the player: the route to fly and the hold;
// a nil hold where it is not the net's, or ends by itself on what the
// player cleared. A shortcut and a crossing restriction have no end of
// their own.
func (w *conflictWatch) netChange(e *enrouteAC, a traffic.TrackedAircraft, r traffic.Resolution) ([]traffic.RoutePoint, *netHold) {
	w.mu.Lock()
	net := w.net[a.Tail]
	w.mu.Unlock()
	if !net {
		return nil, nil
	}
	who := w.s.enrouteStaffed(e, a.Position)
	if who.mode == "" {
		return nil, nil
	}
	w.s.mu.Lock()
	route, levelCleared, speedCleared := e.route, e.clearedFt > 0, e.speedCleared
	w.s.mu.Unlock()
	h := &netHold{id: a.ObjectID, icao: who.icao, pos: who.pos, r: r, since: w.s.cc.clock.Now(), planned: route}
	switch {
	case r.Kind == traffic.ResolveLevel && !levelCleared, r.Kind == traffic.ResolveSpeed && !speedCleared:
		// As the player's own level or speed: the rest of its way.
		return traffic.ResolvedRoute(route, a, r, instructRestOf), h
	case r.Kind == traffic.ResolveHeading:
		var held []traffic.RoutePoint
		held, h.leg = headingHeld(route, a, r.HeadingDeg)
		return held, h
	}
	return nil, nil
}

// headingHeld is route with heading hdg held from where a is: level for the
// look-ahead it was resolved for, on it to the leg's end (instructHeadingFor
// away, put on before it is reached: headingOn), then the route from its
// point nearest there, so that it keeps its end.
func headingHeld(route []traffic.RoutePoint, a traffic.TrackedAircraft, hdg float64) ([]traffic.RoutePoint, airport.LatLon) {
	at := func(d time.Duration) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(a.Position.Lat, a.Position.Lon, hdg, a.GroundKts*d.Hours()*1852)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	end := at(instructHeadingFor)
	out := []traffic.RoutePoint{
		{Position: a.Position, AltFt: a.AltFt, Kts: a.GroundKts},
		{Position: at(conflictLookAhead / 2), AltFt: a.AltFt, Kts: a.GroundKts},
		{Position: end, AltFt: a.AltFt, Kts: a.GroundKts},
	}
	if len(route) == 0 {
		return out, end
	}
	ahead := traffic.ProfileAhead(a.Position, route)
	k, best := 0, math.Inf(1)
	for i, p := range ahead {
		if d := calc.HaversineNM(end.Lat, end.Lon, p.Position.Lat, p.Position.Lon); d < best {
			k, best = i, d
		}
	}
	if ahead[0].AltFt > 0 {
		out[2].AltFt = ahead[0].AltFt // on to the level it was going to
	}
	return append(out, ahead[k:]...), end
}

// legSameM: a route point this near the end of a held heading's leg is it.
const legSameM = 100.0

// legIndex is where route has the end of a held heading's leg; -1 when the
// route has been given another since (a heading, a direct-to).
func legIndex(route []traffic.RoutePoint, leg airport.LatLon) int {
	for i, p := range route {
		if calc.HaversineMeters(p.Position.Lat, p.Position.Lon, leg.Lat, leg.Lon) < legSameM {
			return i
		}
	}
	return -1
}

// offNetLeg is cs's route from where a held heading's leg ends, for the
// player's heading or direct-to to replace the leg; route with none held.
func (s *scheduler) offNetLeg(cs string, route []traffic.RoutePoint) []traffic.RoutePoint {
	s.st.mu.Lock()
	w := s.st.conflicts
	s.st.mu.Unlock()
	if w == nil {
		return route
	}
	w.mu.Lock()
	h, ok := w.netHolds[cs]
	w.mu.Unlock()
	if !ok || h.r.Kind != traffic.ResolveHeading {
		return route
	}
	if k := legIndex(route, h.leg); k >= 0 && k+1 < len(route) {
		return route[k+1:]
	}
	return route
}

// headingOn is route with the held heading's leg, ending at point k, put on
// from where a is.
func headingOn(route []traffic.RoutePoint, k int, a traffic.TrackedAircraft, hdg float64) ([]traffic.RoutePoint, airport.LatLon) {
	lat, lon := calc.DisplaceByHeading(a.Position.Lat, a.Position.Lon, hdg, a.GroundKts*instructHeadingFor.Hours()*1852)
	end := route[k]
	end.Position = airport.LatLon{Lat: lat, Lon: lon}
	out := []traffic.RoutePoint{{Position: a.Position, AltFt: a.AltFt, Kts: a.GroundKts}, end}
	return append(out, route[k+1:]...), end.Position
}

// keepNetHold keeps h, held for cs from now on.
func (w *conflictWatch) keepNetHold(cs string, h netHold) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.netHolds == nil {
		w.netHolds = map[string]netHold{}
	}
	if old, ok := w.netHolds[cs]; ok && old.id == h.id {
		h.planned = old.planned // back to before the first of them
	}
	w.netHolds[cs] = h
}

// heldFlight is an aircraft with a hold as tendNetHolds works it.
type heldFlight struct {
	who   enrouteStaff
	route []traffic.RoutePoint
	set   func([]traffic.RoutePoint) error
	fixAt func(airport.LatLon) string // the name of a point of its route
}

// heldEnroute is e, an en route aircraft of ours, as a heldFlight.
func (w *conflictWatch) heldEnroute(e *enrouteAC, a traffic.TrackedAircraft) heldFlight {
	f := heldFlight{who: w.s.enrouteStaffed(e, a.Position), fixAt: func(p airport.LatLon) string { return enroutePointName(e, p, false) }}
	w.s.mu.Lock()
	f.route = e.route
	w.s.mu.Unlock()
	f.set = func(route []traffic.RoutePoint) error {
		_, wps, err := traffic.EnrouteStart(route)
		if err != nil {
			return err
		}
		if err := w.s.cc.do(func() error { return w.s.cc.sim.SetRoute(e.objectID, wps) }); err != nil {
			return err
		}
		w.s.mu.Lock()
		e.route = route
		w.s.mu.Unlock()
		return nil
	}
	return f
}

// tendNetHolds looks after the safety net's held changes: a heading put on
// while it is the player's; the player's instruction since ends a held
// level or speed, their own heading or direct-to a held heading; off the
// staffed station, and clear of the conflict, the AI gives it its route
// before the change again, unless the player has instructed it since.
func (w *conflictWatch) tendNetHolds(now time.Time, aircraft []traffic.TrackedAircraft, involved map[string]bool, flight func(traffic.TrackedAircraft) (heldFlight, bool)) {
	w.mu.Lock()
	holds := maps.Clone(w.netHolds)
	w.mu.Unlock()
	for _, a := range aircraft {
		h, ok := holds[a.Tail]
		if !ok || !a.Ours || a.ObjectID != h.id {
			continue
		}
		cs := a.Tail
		drop := func() {
			w.mu.Lock()
			delete(w.netHolds, cs)
			w.mu.Unlock()
		}
		f, ok := flight(a)
		if !ok || a.OnGround {
			drop() // handed on, or down
			continue
		}
		told := !f.who.playerAt.IsZero() && !f.who.playerAt.Before(h.since)
		heading, k := h.r.Kind == traffic.ResolveHeading, -1
		if heading {
			if k = legIndex(f.route, h.leg); k < 0 {
				drop()
				continue
			}
		} else if told {
			drop() // their clearance stands
			continue
		}
		if f.who.mode != "" {
			if !heading || calc.HaversineNM(a.Position.Lat, a.Position.Lon, h.leg.Lat, h.leg.Lon) > a.GroundKts*conflictLookAhead.Hours() {
				continue
			}
			route, leg := headingOn(f.route, k, a, h.r.HeadingDeg)
			if err := f.set(route); err != nil {
				w.s.cc.log.printf("%-6s safety net: heading not held on: %v", cs, err)
				continue
			}
			w.mu.Lock()
			if cur, ok := w.netHolds[cs]; ok && cur.since.Equal(h.since) {
				cur.leg = leg
				w.netHolds[cs] = cur
			}
			w.mu.Unlock()
			continue
		}
		if told {
			drop()
			continue
		}
		w.mu.Lock()
		busy := now.Before(w.busy[cs])
		w.mu.Unlock()
		if involved[cs] || busy || len(h.planned) == 0 {
			continue
		}
		ahead := traffic.ProfileAhead(a.Position, h.planned)
		back := append([]traffic.RoutePoint{{Position: a.Position, AltFt: a.AltFt, Kts: a.GroundKts}}, ahead...)
		if err := f.set(back); err != nil {
			w.s.cc.log.printf("%-6s safety net: route not given again: %v", cs, err)
			continue
		}
		drop()
		w.s.cc.log.printf("%-6s safety net's %s over: off the staffed station, its route before it again", cs, h.r.Kind)
		var tx traffic.Transmission
		switch h.r.Kind {
		case traffic.ResolveLevel:
			to := ahead[0].AltFt
			ta := 0.0
			if g, err := w.s.st.cache.Graph(h.icao); err == nil {
				ta = w.s.cc.limitsOf(g).TransitionAltitudeFt
			}
			tx = traffic.ContinueLevelAbove(h.pos, cs, to, to > a.AltFt, ta)
		case traffic.ResolveSpeed:
			tx = traffic.SpeedAssigned(cs, 0, a.GroundKts)
		case traffic.ResolveHeading:
			for _, p := range ahead {
				if fix := f.fixAt(p.Position); fix != "" {
					tx = traffic.Vectored(cs, traffic.Vector{Fix: fix}, w.s.cc.magVar(h.icao))
					break
				}
			}
		}
		if tx.Text == "" {
			continue
		}
		tx.Position = h.pos
		icao := h.icao
		w.s.cc.callAt(icao, h.pos, cs, prioApproach, func() { w.s.cc.radio.Transmit(icao, tx) })
	}
}
