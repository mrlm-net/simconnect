package world

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Radar instructions to an arrival still en route (#1419). An en route
// arrival is in the landing sequence long before the handover at its STAR
// entry (enroute.go), and until then it has no arrival controller. On a
// staffed approach the player may give it a speed, a heading and a level
// all the same: approach's actions "speed", "heading" and "descend" are
// flown by MSFS AI on its waypoint chain, the one paceEnroute and the
// conflict watch already change in flight, from where the aircraft is. Its
// route as flown (e.route) is what the conflict watch predicts it on and
// what it still flies to its STAR entry, where it is handed over as
// before. Said by approach and read back as on its STAR.
//
// The other choice, an early handover at the first instruction, would have
// the arrival controller fly the STAR's procedure from far out: planned at
// most at ProcedureTopFt and at STAR speeds, it would descend and slow a
// hundred miles early, off the plan the others were planned clear of.
//
// Not staffed, or not told anything, it flies as before: the AI paces it
// (paceEnroute), approach takes it at the entry. Told something, it is
// the player's: paceEnroute leaves its speed alone.

// enrouteToldActions are the approach actions an en route arrival takes.
var enrouteToldActions = []string{"speed", "heading", "descend"}

// enrouteHeadingLegNM is how far a heading is flown with nothing further
// said, as on a STAR (traffic: headingLegNM): then own navigation, direct
// to the named point of its route nearest there.
const enrouteHeadingLegNM = 30.0

// enrouteLegEndNM: "resume own navigation" is said this near the end of
// the heading's leg (or once it is past it).
const enrouteLegEndNM = 3.0

// enrouteTold is what the player told an en route arrival, in force.
type enrouteTold struct {
	icao string
	// kts: the speed assigned, 0 its own; levelFt the level it was cleared
	// down to, 0 its plan's.
	kts, levelFt float64
	// resumeAt: the end of a heading's leg, where it resumes own navigation
	// direct resumeFix; "" none due.
	resumeAt  airport.LatLon
	resumeFix string
}

// approachStaffed reports whether approach at icao is staffed (its
// station as said without a sector, as an arrival's first call names it).
func (cc *controlCenter) approachStaffed(icao string) (StaffedStation, bool) {
	station, _ := cc.stationOf(icao, traffic.PosApproach)
	return cc.core.staffedAt(icao, traffic.PosApproach, station)
}

// toldEnroute is callsign's en route arrival at icao, when the player may
// instruct it: one of the schedule's, created, approach staffed there, no
// arrival controller yet; nil otherwise.
func (q *sequences) toldEnroute(icao, callsign string) *enrouteAC {
	if q.s == nil || q.cc == nil || q.cc.byTail(callsign) != nil {
		return nil
	}
	q.s.mu.Lock()
	e := q.s.enroute[callsign]
	q.s.mu.Unlock()
	if e == nil || e.arrive == nil || e.corridor != "" || e.objectID == 0 || !strings.EqualFold(e.f.Airport, icao) {
		return nil
	}
	if _, staffed := q.cc.approachStaffed(icao); !staffed {
		return nil
	}
	return e
}

// enrouteForbidden reports whether a client working position as may not
// instruct callsign, an en route arrival at icao (network play, #511): only
// the approach player, or the one staffing it.
func (q *sequences) enrouteForbidden(as, icao, callsign string) bool {
	if as == "" || as == "all" || q.toldEnroute(icao, callsign) == nil {
		return false
	}
	s, _ := q.cc.approachStaffed(icao)
	if s.By != "" {
		return s.By != as
	}
	return as != string(traffic.PosApproach)
}

// enrouteAction does approach action for callsign when it is an en route
// arrival the player may instruct (toldEnroute); false when not, for
// approachDo to go on as before.
func (q *sequences) enrouteAction(icao, callsign, action string, kts, ft float64, sp spacing) (bool, error) {
	if !slices.Contains(enrouteToldActions, action) {
		return false, nil
	}
	e := q.toldEnroute(icao, callsign)
	if e == nil {
		return false, nil
	}
	var a traffic.TrackedAircraft
	seen := false
	for _, x := range q.cc.world.Aircraft() {
		if x.ObjectID == e.objectID {
			a, seen = x, true
		}
	}
	s := q.s
	s.mu.Lock()
	handing := e.handing
	told := enrouteTold{icao: icao}
	if e.told != nil {
		told = *e.told
	}
	route := slices.Clone(e.route)
	s.mu.Unlock()
	if handing {
		return true, errors.New("being handed to approach: again in a moment")
	}
	if !seen || len(route) < 2 {
		return true, errors.New("not seen yet")
	}
	here := traffic.RoutePoint{Position: a.Position, AltFt: a.AltFt, Kts: a.GroundKts}
	ahead := traffic.ProfileAhead(a.Position, route)
	if len(ahead) == 0 {
		return true, errors.New("no route ahead")
	}
	ta, mv := q.cc.taOf(icao), q.cc.magVar(icao)
	var said traffic.Transmission
	var what string
	next := append([]traffic.RoutePoint{here}, ahead...)
	switch action {
	case "speed":
		if kts < 0 {
			return true, errors.New("speed needs knots, 0 its own speed")
		}
		told.kts = 0
		if kts > 0 {
			told.kts = math.Max(math.Min(traffic.MinProcedureSpeedKts, e.cruiseKts), math.Min(kts, e.cruiseKts))
		}
		said = traffic.SpeedAssigned(callsign, told.kts, a.GroundKts)
		if sp.expedite && told.kts > 0 {
			said = traffic.Expedited(said, "")
		}
		what = "normal speed"
		if told.kts > 0 {
			what = fmt.Sprintf("speed %.0f kt", told.kts)
		}
	case "heading":
		if sp.heading < 1 || sp.heading > 360 {
			return true, errors.New("heading needs a magnetic heading, 1 to 360")
		}
		hdg := math.Mod(sp.heading-mv+360, 360)
		lat, lon := calc.DisplaceByHeading(a.Position.Lat, a.Position.Lon, hdg, enrouteHeadingLegNM*1852)
		end := airport.LatLon{Lat: lat, Lon: lon}
		// Back on its route at the named point nearest the leg's end.
		k, fix, best := -1, "", math.Inf(1)
		for i, p := range ahead {
			name := enroutePointName(e, p.Position, i == len(ahead)-1)
			if name == "" {
				continue
			}
			if d := calc.HaversineNM(end.Lat, end.Lon, p.Position.Lat, p.Position.Lon); d < best {
				k, fix, best = i, name, d
			}
		}
		if k < 0 {
			return true, errors.New("no named point on its route to resume to")
		}
		alt := a.AltFt
		if told.levelFt > 0 {
			alt = told.levelFt
		}
		next = append([]traffic.RoutePoint{here, {Position: end, AltFt: alt}}, ahead[k:]...)
		told.resumeAt, told.resumeFix = end, fix
		v := traffic.Vector{HeadingDeg: hdg, Turn: traffic.TurnTo(a.Heading, hdg)}
		said = traffic.Vectored(callsign, v, mv)
		what = "heading " + traffic.HeadingSaid(hdg, mv)
	case "descend":
		if ft <= 0 {
			return true, errors.New("descend needs a level in feet")
		}
		if ft >= a.AltFt-100 {
			return true, fmt.Errorf("at %.0f ft: %.0f ft is no descent", a.AltFt, ft)
		}
		if t := sp.throughFt; t > 0 && (t <= ft || t >= a.AltFt) {
			return true, fmt.Errorf("at %.0f ft, descending to %.0f ft: %.0f ft is not on the way", a.AltFt, ft, t)
		}
		told.levelFt = ft
		perNM := traffic.ProcedureDescentFtPerNm
		if sp.expedite {
			perNM *= 2
		}
		next = descentAlong(next, ft, (a.AltFt-ft)/perNM)
		said = traffic.Descend(traffic.PosApproach, callsign, ft, ta)
		what = "descend to " + traffic.LevelSaidAbove(ft, ta)
		if sp.expedite {
			said = traffic.Expedited(said, throughSaid(sp.throughFt, ta))
			what = "expedite " + what
		}
	}
	// Its speeds: as planned at each level, the STAR entry at an arrival's,
	// no faster than the speed assigned.
	for i := 1; i < len(next); i++ {
		p := &next[i]
		p.Kts = traffic.EnrouteSpeedKts(p.AltFt, e.cruiseKts)
		if i == len(next)-1 {
			p.Kts = entryKts(p.AltFt, e.cruiseKts)
		}
		if told.kts > 0 {
			p.Kts = math.Min(p.Kts, told.kts)
		}
	}
	_, wps, err := traffic.EnrouteStart(next)
	if err != nil {
		return true, err
	}
	if err := q.cc.do(func() error { return q.cc.sim.SetRoute(e.objectID, wps) }); err != nil {
		return true, err
	}
	s.mu.Lock()
	e.route, e.told, e.pacedKts = next, &told, 0
	s.mu.Unlock()
	_, said.Frequency = q.cc.stationOf(icao, traffic.PosApproach)
	q.cc.radio.Transmit(icao, said)
	q.cc.log.printf("%-6s approach: %s, en route (the player)", callsign, what)
	return true, nil
}

// enroutePointName is the name of point p of e's route: one of its named fixes,
// or its STAR entry (entry: the route's last point); "" unnamed.
func enroutePointName(e *enrouteAC, p airport.LatLon, entry bool) string {
	if entry && len(e.arrive.route) > 0 && e.arrive.route[0].Ident != "" {
		return e.arrive.route[0].Ident
	}
	for _, f := range e.fixes {
		if calc.HaversineNM(f.Lat, f.Lon, p.Lat, p.Lon) < 0.1 && !profilePoint(e, f.Ident) {
			return f.Ident
		}
	}
	return ""
}

// profilePoint reports whether ident is a point of e's plan's profile
// (top of climb, top of descent): no fix to fly to.
func profilePoint(e *enrouteAC, ident string) bool {
	if e.arrive == nil || e.arrive.plan == nil {
		return false
	}
	for _, w := range e.arrive.plan.Waypoints {
		if w.Ident == ident && w.Kind == nav.PointProfile {
			return true
		}
	}
	return false
}

// descentAlong is route (its first point where the aircraft is) descending
// from there to ft over nm along it and level at ft after: a point added
// where it reaches ft, the points before on the way down. A route shorter
// than nm reaches ft at its end.
func descentAlong(route []traffic.RoutePoint, ft, nm float64) []traffic.RoutePoint {
	legNM := func(i int) float64 {
		a, b := route[i-1].Position, route[i].Position
		return calc.HaversineNM(a.Lat, a.Lon, b.Lat, b.Lon)
	}
	total := 0.0
	for i := 1; i < len(route); i++ {
		total += legNM(i)
	}
	nm = math.Max(0.1, math.Min(nm, total))
	from := route[0].AltFt
	out := []traffic.RoutePoint{route[0]}
	gone, reached := 0.0, false
	for i := 1; i < len(route); i++ {
		p, prev := route[i], route[i-1]
		leg := legNM(i)
		if !reached && gone+leg >= nm {
			if f := (nm - gone) / math.Max(leg, 1e-9); f < 0.98 {
				brg := calc.BearingDegrees(prev.Position.Lat, prev.Position.Lon, p.Position.Lat, p.Position.Lon)
				lat, lon := calc.DisplaceByHeading(prev.Position.Lat, prev.Position.Lon, brg, (nm-gone)*1852)
				out = append(out, traffic.RoutePoint{Position: airport.LatLon{Lat: lat, Lon: lon}, AltFt: ft})
			}
			reached = true
		}
		gone += leg
		if reached || i == len(route)-1 {
			p.AltFt = ft
		} else {
			p.AltFt = from + (ft-from)*gone/nm
		}
		out = append(out, p)
	}
	return out
}

// enrouteResumes says "resume own navigation" to the en route arrivals at
// the end of a heading's leg (scheduler goroutine); a route changed since
// (the conflict watch's) leaves nothing to say.
func (s *scheduler) enrouteResumes() {
	type due struct {
		e    *enrouteAC
		told enrouteTold
	}
	var list []due
	s.mu.Lock()
	for _, e := range s.enroute {
		if e.told != nil && e.told.resumeFix != "" && !e.handing {
			list = append(list, due{e, *e.told})
		}
	}
	s.mu.Unlock()
	if len(list) == 0 {
		return
	}
	pos := map[uint32]airport.LatLon{}
	for _, a := range s.cc.world.Aircraft() {
		pos[a.ObjectID] = a.Position
	}
	for _, d := range list {
		p, ok := pos[d.e.objectID]
		if !ok {
			continue
		}
		s.mu.Lock()
		var pts []airport.LatLon
		k := -1
		for i, r := range d.e.route {
			pts = append(pts, r.Position)
			if r.Position == d.told.resumeAt {
				k = i
			}
		}
		s.mu.Unlock()
		passed := k >= 0 && len(pts)-len(traffic.RouteAhead(p, pts)) > k
		near := k >= 0 && calc.HaversineNM(p.Lat, p.Lon, d.told.resumeAt.Lat, d.told.resumeAt.Lon) < enrouteLegEndNM
		if k >= 0 && !passed && !near {
			continue
		}
		s.mu.Lock()
		if d.e.told != nil {
			d.e.told.resumeFix, d.e.told.resumeAt = "", airport.LatLon{}
		}
		s.mu.Unlock()
		if k < 0 {
			continue // its route changed: off the leg already
		}
		tx := traffic.Vectored(d.e.f.Callsign, traffic.Vector{Fix: d.told.resumeFix}, s.cc.magVar(d.told.icao))
		_, tx.Frequency = s.cc.stationOf(d.told.icao, traffic.PosApproach)
		s.cc.radio.Transmit(d.told.icao, tx)
		s.cc.log.printf("%-6s approach: resume own navigation direct %s, en route", d.e.f.Callsign, d.told.resumeFix)
	}
}

// handingAt reports whether an en route arrival is being handed over at
// the STAR entry at entry (s.mu held): a second one there waits for the
// next look, the first one's spawn not yet in nearAirborne (offline: two
// at GOLOP in one look, the player's vector having brought one back beside
// the other, the second's handover refused and its flight cancelled).
func (s *scheduler) handingAt(entry airport.LatLon) bool {
	for _, e := range s.enroute {
		if e.handing && e.arrive != nil && len(e.arrive.route) > 0 && e.arrive.route[0].Position == entry {
			return true
		}
	}
	return false
}
