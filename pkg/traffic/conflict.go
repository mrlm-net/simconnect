//go:build windows
// +build windows

package traffic

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// Airborne conflicts (#395): pairs of airborne aircraft predicted to come
// closer than the minima within a look-ahead, flying on as they are now
// (track, ground speed, vertical speed), and the least disturbing change to
// one of ours that keeps them apart: a speed change, a level change or a
// heading offset. Other traffic is an intruder we avoid, never steer.

// ConflictOptions tune PredictConflicts and ResolveConflict.
type ConflictOptions struct {
	// LookAhead: how far ahead conflicts are predicted (default 5 min),
	// in Step steps (default 10 s).
	LookAhead, Step time.Duration
	// MinNM: the lateral minimum (default EnrouteSeparationNM); TerminalNM
	// applies where both are in a terminal area — at an airport (departing
	// or arriving) below TerminalBelowFt (defaults TerminalSeparationNM and
	// 10000 ft). MinFt: the vertical minimum (default VerticalSeparationFt).
	MinNM, TerminalNM, TerminalBelowFt, MinFt float64
	// DirectFixes are the named fixes ahead on an aircraft's route, in
	// route order, that a resolution may send it direct to (a shortcut);
	// nil: none.
	DirectFixes func(TrackedAircraft) []DirectFix
	// Route is the points still ahead on an aircraft's route, in order:
	// it is predicted along them, turning where they turn, not straight
	// on (live, CSA786 told to stop descent for KLM130 predicted straight
	// on where its STAR turned away); nil or empty: straight on.
	Route func(TrackedAircraft) []airport.LatLon
}

// DirectFix is a named fix of a route.
type DirectFix struct {
	Ident    string
	Position airport.LatLon
}

// SameRouteDeg: tracks within this of each other are on the same route
// (in trail); more apart they cross.
const SameRouteDeg = 45.0

func (o ConflictOptions) withDefaults() ConflictOptions {
	if o.LookAhead <= 0 {
		o.LookAhead = 5 * time.Minute
	}
	if o.Step <= 0 {
		o.Step = 10 * time.Second
	}
	if o.MinNM <= 0 {
		o.MinNM = EnrouteSeparationNM
	}
	if o.TerminalNM <= 0 {
		o.TerminalNM = TerminalSeparationNM
	}
	if o.TerminalBelowFt <= 0 {
		o.TerminalBelowFt = 10000
	}
	if o.MinFt <= 0 {
		o.MinFt = VerticalSeparationFt
	}
	return o
}

// Conflict is a pair predicted to lose separation.
type Conflict struct {
	A, B     string // call signs (tail, else title)
	AID, BID uint32
	// In: until the minima are first lost (0: lost now).
	In time.Duration `json:"in"`
	// The closest point of approach within the look-ahead.
	ClosestIn  time.Duration `json:"closestIn"`
	ClosestNM  float64       `json:"closestNM"`
	VerticalFt float64       `json:"verticalFt"` // at the closest point
	MinNM      float64       `json:"minNM"`      // the lateral minimum that applies
}

// track is an aircraft flying on as it is now.
type track struct {
	lat, lon, altFt float64
	hdg, kts, fpm   float64
	// level: the vertical speed stops there (a cleared level); 0: none.
	level float64
	// path: the route's points ahead, flown in turn (nil: straight on).
	path []airport.LatLon
}

// trackFor is a's track, along its route when o knows it.
func trackFor(a TrackedAircraft, o ConflictOptions) track {
	t := trackOf(a)
	if o.Route != nil {
		t.path = o.Route(a)
	}
	return t
}

func trackOf(a TrackedAircraft) track {
	fpm := a.VSFpm
	if math.Abs(fpm) < 300 {
		fpm = 0 // level (the noise of a level aircraft)
	}
	return track{lat: a.Position.Lat, lon: a.Position.Lon, altFt: a.AltFt, hdg: a.Heading, kts: a.GroundKts, fpm: fpm}
}

func (t track) at(d time.Duration) (lat, lon, altFt float64) {
	s := d.Seconds()
	nm := t.kts * s / 3600
	lat, lon = t.lat, t.lon
	hdg := t.hdg
	// Along its route first, turning where it turns; straight on past
	// the route's end.
	for _, p := range t.path {
		leg := calc.HaversineNM(lat, lon, p.Lat, p.Lon)
		if leg < 0.01 {
			continue
		}
		hdg = calc.BearingDegrees(lat, lon, p.Lat, p.Lon)
		if leg >= nm {
			break
		}
		lat, lon, nm = p.Lat, p.Lon, nm-leg
	}
	lat, lon = calc.DisplaceByHeading(lat, lon, hdg, nm*1852)
	altFt = t.altFt + t.fpm*s/60
	if t.level != 0 && (t.fpm > 0 && altFt > t.level || t.fpm < 0 && altFt < t.level) {
		altFt = t.level
	}
	return lat, lon, altFt
}

// conflictBetween flies a and b on over the look-ahead: the first loss of
// the minima and the closest point; ok when they lose separation.
func conflictBetween(a, b track, minNM float64, o ConflictOptions) (c Conflict, ok bool) {
	c.ClosestNM, c.In = math.Inf(1), -1
	for d := time.Duration(0); d <= o.LookAhead; d += o.Step {
		alat, alon, aft := a.at(d)
		blat, blon, bft := b.at(d)
		l := calc.HaversineNM(alat, alon, blat, blon)
		v := math.Abs(aft - bft)
		if l < c.ClosestNM {
			c.ClosestNM, c.ClosestIn, c.VerticalFt = l, d, v
		}
		if l < minNM && v < o.MinFt && c.In < 0 {
			c.In = d
		}
	}
	c.MinNM = minNM
	return c, c.In >= 0
}

func (o ConflictOptions) minFor(a, b TrackedAircraft) float64 {
	if a.Airport != "" && b.Airport != "" && a.AltFt < o.TerminalBelowFt && b.AltFt < o.TerminalBelowFt {
		return o.TerminalNM
	}
	return o.MinNM
}

func callsignOf(a TrackedAircraft) string {
	if a.Tail != "" {
		return a.Tail
	}
	return a.Title
}

// PredictConflicts lists the pairs of airborne aircraft that lose
// separation within the look-ahead flying on as they are, soonest first.
func PredictConflicts(aircraft []TrackedAircraft, o ConflictOptions) []Conflict {
	o = o.withDefaults()
	var air []TrackedAircraft
	for _, a := range aircraft {
		if !a.OnGround {
			air = append(air, a)
		}
	}
	var out []Conflict
	for i := 0; i < len(air); i++ {
		for j := i + 1; j < len(air); j++ {
			a, b := air[i], air[j]
			if a.Tail != "" && a.Tail == b.Tail {
				continue // one flight twice: handed over to a new object
			}
			if TowerPair(a, b) {
				continue // the tower's: runway separation
			}
			min := o.minFor(a, b)
			// Too far apart to meet within the look-ahead.
			reach := (a.GroundKts+b.GroundKts)*o.LookAhead.Hours() + min
			if calc.HaversineNM(a.Position.Lat, a.Position.Lon, b.Position.Lat, b.Position.Lon) > reach {
				continue
			}
			if c, ok := conflictBetween(trackFor(a, o), trackFor(b, o), min, o); ok {
				c.A, c.B, c.AID, c.BID = callsignOf(a), callsignOf(b), a.ObjectID, b.ObjectID
				out = append(out, c)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].In < out[j].In })
	return out
}

// ResolutionKind is what a resolution changes.
type ResolutionKind string

const (
	ResolveSpeed   ResolutionKind = "speed"
	ResolveLevel   ResolutionKind = "level"
	ResolveHeading ResolutionKind = "heading"
	// ResolveDirect: a shortcut, direct to Fix (at Direct) and on along
	// the route from there.
	ResolveDirect ResolutionKind = "direct"
)

// Resolution is a change to one of ours that keeps a conflict apart.
type Resolution struct {
	Callsign string         `json:"callsign"`
	ObjectID uint32         `json:"objectId"`
	Kind     ResolutionKind `json:"kind"`
	// The new ground speed (speed), altitude (level: hold it, climbing or
	// descending there) or true heading (heading: fly it for the look-ahead,
	// then resume).
	Kts        float64 `json:"kts,omitempty"`
	AltFt      float64 `json:"altFt,omitempty"`
	HeadingDeg float64 `json:"headingDeg,omitempty"`
	// Stop (level): a climb or descent stopped at AltFt on its way, to
	// go on once clear of the traffic ("stop climb at 5000 feet").
	Stop bool `json:"stop,omitempty"`
	// Fix and Direct (direct): the fix of the route flown to.
	Fix    string          `json:"fix,omitempty"`
	Direct *airport.LatLon `json:"direct,omitempty"`
	// Why: the conflict it resolves, as ATC would say it.
	Why string `json:"why"`
}

// resolutionCandidate is one change with its cost: the least disturbing
// resolution is the cheapest that keeps the aircraft clear of everyone.
type resolutionCandidate struct {
	r    Resolution
	t    track
	cost float64
}

// candidates are the changes to aircraft a, least disturbing first in cost.
// Crossing traffic is parted by altitude first: a climb or descent stopped
// on its way, else a level 1000 or 2000 ft up or down (by the semicircular
// rule when level above the transition), then a heading 20 to 45° off
// (right first), speed last. On the same route as the traffic (sameRoute:
// in trail) speed comes first (a tenth to a fifth slower or faster, 250 kt
// below 10000 ft), then a shortcut direct to a fix ahead (fixes) or a leg
// extended by a heading off and back, altitude last.
func candidates(a TrackedAircraft, base track, sameRoute bool, fixes []DirectFix) []resolutionCandidate {
	var out []resolutionCandidate
	speedCost, directCost, levelCost := 3.5, 3.5, 0.0
	if sameRoute {
		speedCost, directCost, levelCost = 0, 1.5, 3
	}
	add := func(r Resolution, t track, cost float64) {
		r.Callsign, r.ObjectID = callsignOf(a), a.ObjectID
		out = append(out, resolutionCandidate{r, t, cost})
	}
	for _, f := range []float64{0.9, 1.1, 0.8, 1.2} {
		kts := base.kts * f
		if a.AltFt < 10000 && kts > 250 {
			continue
		}
		t := base
		t.kts = kts
		add(Resolution{Kind: ResolveSpeed, Kts: math.Round(kts)}, t, speedCost+1+math.Abs(1-f)*5)
	}
	// Climbing or descending: stopped at a level on its way (a radar
	// controller's "stop climb at 5000 feet"), then on once clear; never
	// turned back the other way.
	if base.fpm != 0 {
		up := base.fpm > 0
		for i, cost := range []float64{0.8, 1.3} {
			alt := math.Ceil((a.AltFt+500)/1000)*1000 + float64(i)*1000
			if !up {
				alt = math.Floor((a.AltFt-500)/1000)*1000 - float64(i)*1000
			}
			if alt < a.AltFt-a.AGLFt+1500 {
				continue // too low over the ground
			}
			t := base
			t.level = alt
			add(Resolution{Kind: ResolveLevel, AltFt: alt, Stop: true}, t, levelCost+cost)
		}
	}
	level := math.Round(a.AltFt/1000) * 1000
	for _, dft := range []float64{1000, -1000, 2000, -2000} {
		alt := level + dft
		if base.fpm != 0 && (dft > 0) != (base.fpm > 0) {
			continue // not turned back
		}
		if alt < a.AltFt-a.AGLFt+1500 {
			continue // too low over the ground
		}
		t := base
		cost := levelCost + 2 + math.Abs(dft)/2000
		// Climb or descend there at 1500 fpm, then level.
		t.fpm, t.level = math.Copysign(1500, alt-a.AltFt), alt
		if base.fpm == 0 && alt >= 10000 {
			// Level cruise: the semicircular rule (odd thousands eastbound).
			odd := int(alt/1000)%2 == 1
			if east := math.Mod(a.Heading+360, 360) < 180; odd != east {
				cost += 1
			}
		}
		add(Resolution{Kind: ResolveLevel, AltFt: alt}, t, cost)
	}
	// A shortcut: direct to a fix past the next, within 60° of the
	// heading and far enough out to be one.
	for i, f := range fixes {
		if i == 0 {
			continue // the next fix: no shortcut
		}
		d := calc.HaversineNM(a.Position.Lat, a.Position.Lon, f.Position.Lat, f.Position.Lon)
		brg := calc.BearingDegrees(a.Position.Lat, a.Position.Lon, f.Position.Lat, f.Position.Lon)
		turn := math.Abs(math.Mod(brg-base.hdg+540, 360) - 180)
		if d < DirectMinNM || turn > DirectMaxTurnDeg {
			continue
		}
		t := base
		t.hdg, t.path = brg, fromFix(base.path, f.Position)
		p := f.Position
		add(Resolution{Kind: ResolveDirect, Fix: f.Ident, Direct: &p}, t, directCost+1+turn/60)
	}
	for _, turn := range []float64{20, -20, 30, -30, 45, -45} {
		t := base
		t.hdg, t.path = math.Mod(base.hdg+turn+360, 360), nil // off the route: straight out
		cost := 3 + math.Abs(turn)/45
		if turn < 0 {
			cost += 0.1 // right turns first
		}
		add(Resolution{Kind: ResolveHeading, HeadingDeg: math.Round(t.hdg)}, t, cost)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].cost < out[j].cost })
	return out
}

// A shortcut resolution goes to a fix at least DirectMinNM away, at most
// DirectMaxTurnDeg off the heading.
const (
	DirectMinNM      = 5.0
	DirectMaxTurnDeg = 60.0
)

// ResolveConflict picks the least disturbing change to one of ours
// (canSteer: which aircraft, and which kinds of change it can fly) that keeps the pair of c apart through the look-ahead without
// a new conflict with anyone else; both ours, either may move. ok is false
// when neither can be steered or nothing tried keeps them apart. A level
// change is predicted at 1500 fpm, a speed or heading change at once.
func ResolveConflict(c Conflict, aircraft []TrackedAircraft, canSteer func(TrackedAircraft, ResolutionKind) bool, o ConflictOptions) (Resolution, bool) {
	o = o.withDefaults()
	var a, b *TrackedAircraft
	for i := range aircraft {
		switch aircraft[i].ObjectID {
		case c.AID:
			a = &aircraft[i]
		case c.BID:
			b = &aircraft[i]
		}
	}
	if a == nil || b == nil {
		return Resolution{}, false
	}
	var best *resolutionCandidate
	// In trail (same route) or crossing: what parts them first.
	sameRoute := math.Abs(math.Mod(a.Heading-b.Heading+540, 360)-180) <= SameRouteDeg
	try := func(me, other *TrackedAircraft) {
		var fixes []DirectFix
		if o.DirectFixes != nil {
			fixes = o.DirectFixes(*me)
		}
		for _, cand := range candidates(*me, trackFor(*me, o), sameRoute, fixes) {
			if best != nil && cand.cost >= best.cost {
				break // sorted: nothing cheaper follows
			}
			if !canSteer(*me, cand.r.Kind) {
				continue
			}
			if clearOfAll(cand.t, *me, aircraft, o) {
				cand := cand
				cand.r.Why = fmt.Sprintf("traffic %s, %.1f NM in %s", callsignOf(*other), c.ClosestNM, c.ClosestIn.Round(time.Second))
				best = &cand
				break
			}
		}
	}
	try(a, b)
	try(b, a)
	if best == nil {
		return Resolution{}, false
	}
	return best.r, true
}

// PathClear reports whether a, flown along path (the points it would fly
// in turn, e.g. direct to a fix and on along its route) at its speed and
// vertical speed now, keeps separation from every other airborne aircraft
// through the look-ahead: before a direct is cleared (live, PHGVV cleared
// direct DONAD, stopped at 4000 ft for TVS440 eleven seconds later).
func PathClear(a TrackedAircraft, path []airport.LatLon, aircraft []TrackedAircraft, o ConflictOptions) bool {
	o = o.withDefaults()
	t := trackOf(a)
	t.path = path
	return clearOfAll(t, a, aircraft, o)
}

// clearOfAll reports whether me flying t keeps separation from every
// other airborne aircraft through the look-ahead.
func clearOfAll(t track, me TrackedAircraft, aircraft []TrackedAircraft, o ConflictOptions) bool {
	for _, x := range aircraft {
		if x.ObjectID == me.ObjectID || x.OnGround {
			continue
		}
		if _, lost := conflictBetween(t, trackFor(x, o), o.minFor(me, x), o); lost {
			return false
		}
	}
	return true
}

// ResolvedRoute is the rest of route for aircraft a flying resolution r,
// from where it is now: the change for lookAhead (at its speed) — a speed
// or level on the route up to that far, a heading straight out on it for
// half of it — then the route as planned from there. The route's points
// behind the aircraft are left out.
func ResolvedRoute(route []RoutePoint, a TrackedAircraft, r Resolution, lookAhead time.Duration) []RoutePoint {
	here := RoutePoint{Position: a.Position, AltFt: a.AltFt, Kts: a.GroundKts}
	// The points ahead: from the first within 90° of the heading.
	var ahead []RoutePoint
	for i, p := range route {
		b := calc.BearingDegrees(a.Position.Lat, a.Position.Lon, p.Position.Lat, p.Position.Lon)
		if math.Abs(math.Mod(b-a.Heading+540, 360)-180) < 90 {
			ahead = route[i:]
			break
		}
	}
	reach := a.GroundKts * lookAhead.Hours() // NM
	apply := func(p RoutePoint) RoutePoint {
		switch r.Kind {
		case ResolveSpeed:
			p.Kts = r.Kts
		case ResolveLevel:
			p.AltFt = r.AltFt
		}
		return p
	}
	if r.Kind == ResolveDirect && r.Direct != nil {
		// Straight to the fix, then the route on from the point there.
		at, best := -1, math.Inf(1)
		for i, p := range ahead {
			if d := calc.HaversineNM(r.Direct.Lat, r.Direct.Lon, p.Position.Lat, p.Position.Lon); d < best {
				at, best = i, d
			}
		}
		if at < 0 {
			return []RoutePoint{here, {Position: *r.Direct, AltFt: a.AltFt, Kts: a.GroundKts}}
		}
		fix := ahead[at]
		fix.Position = *r.Direct
		return append([]RoutePoint{here, fix}, ahead[at+1:]...)
	}
	if r.Kind == ResolveHeading {
		lat, lon := calc.DisplaceByHeading(a.Position.Lat, a.Position.Lon, r.HeadingDeg, reach/2*1852)
		out := []RoutePoint{here, {Position: airport.LatLon{Lat: lat, Lon: lon}, AltFt: a.AltFt, Kts: a.GroundKts}}
		// Back onto the route at its first point beyond the look-ahead.
		for i, p := range ahead {
			if calc.HaversineNM(a.Position.Lat, a.Position.Lon, p.Position.Lat, p.Position.Lon) > reach {
				return append(out, ahead[i:]...)
			}
		}
		return out
	}
	out := []RoutePoint{apply(here)}
	prev, gone := here, 0.0
	for i, p := range ahead {
		leg := calc.HaversineNM(prev.Position.Lat, prev.Position.Lon, p.Position.Lat, p.Position.Lon)
		if gone+leg < reach {
			out = append(out, apply(p))
			prev, gone = p, gone+leg
			continue
		}
		// The end of the change on this leg, then the plan.
		f := (reach - gone) / leg
		brg := calc.BearingDegrees(prev.Position.Lat, prev.Position.Lon, p.Position.Lat, p.Position.Lon)
		lat, lon := calc.DisplaceByHeading(prev.Position.Lat, prev.Position.Lon, brg, leg*f*1852)
		out = append(out, apply(RoutePoint{Position: airport.LatLon{Lat: lat, Lon: lon}, AltFt: prev.AltFt, Kts: prev.Kts}))
		return append(out, ahead[i:]...)
	}
	return out
}

// fromFix is a route from fix on: fix, then the points of path after the
// one nearest it.
func fromFix(path []airport.LatLon, fix airport.LatLon) []airport.LatLon {
	at, best := -1, math.Inf(1)
	for i, p := range path {
		if d := calc.HaversineNM(fix.Lat, fix.Lon, p.Lat, p.Lon); d < best {
			at, best = i, d
		}
	}
	out := []airport.LatLon{fix}
	if at >= 0 {
		out = append(out, path[at+1:]...)
	}
	return out
}
