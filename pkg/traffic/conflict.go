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
	// Profile is the points still ahead with their altitudes (AltFt 0:
	// none), in order: predicted along them laterally as Route, and
	// vertically toward each point's altitude, at its vertical speed or
	// faster where it must to make it by the point; level past the last.
	// Before Route; nil or empty: Route and the vertical speed now (live,
	// TVS524 and BAW1413, a departure and an arrival on their SID and STAR,
	// predicted in conflict climbing on and level on, #657).
	Profile func(TrackedAircraft) []RoutePoint
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
	// LossNM and LossFt are the lateral and vertical distances when the
	// minima are first lost (In): the vertical one under the minimum (1000
	// ft), where VerticalFt at the closest lateral point may not be.
	LossNM float64 `json:"lossNM"`
	LossFt float64 `json:"lossFt"`
	// AAltFt and BAltFt are A's and B's altitudes now.
	AAltFt float64 `json:"aAltFt"`
	BAltFt float64 `json:"bAltFt"`
}

// track is an aircraft flying on as it is now.
type track struct {
	lat, lon, altFt float64
	hdg, kts, fpm   float64
	// level: the vertical speed stops there (a cleared level); 0: none.
	level float64
	// path: the route's points ahead, flown in turn (nil: straight on).
	path []airport.LatLon
	// alts: the altitude at each point of path (0: none); nil: the
	// vertical speed on.
	alts []float64
}

// trackFor is a's track, along its route when o knows it.
func trackFor(a TrackedAircraft, o ConflictOptions) track {
	t := trackOf(a)
	if o.Profile != nil {
		if pts := o.Profile(a); len(pts) > 0 {
			for _, p := range pts {
				t.path = append(t.path, p.Position)
				t.alts = append(t.alts, p.AltFt)
			}
			return t.pastEnd()
		}
	}
	if o.Route != nil {
		t.path = o.Route(a)
	}
	return t.pastEnd()
}

// pastEnd is t straight on when every point of its path is behind it: past
// the end of its route, where MSFS AI flies on as it heads (live, TVS524
// predicted turning back to its last waypoint, 0.2 NM from THY1463 ahead
// of it on the same route, and THY1463 vectored for it, #657).
func (t track) pastEnd() track {
	for _, p := range t.path {
		b := calc.BearingDegrees(t.lat, t.lon, p.Lat, p.Lon)
		if math.Abs(math.Mod(b-t.hdg+540, 360)-180) <= 90 {
			return t
		}
	}
	t.path, t.alts = nil, nil
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
	if len(t.alts) > 0 && t.kts > 0 {
		return t.alongProfile(d)
	}
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
	// Both level: a standard level apart is separated, whatever the
	// altimetry's few feet say (live: CSA111 at FL370 and CSA1811 at FL360
	// predicted 999 ft apart and CSA111 sent up to FL390).
	minFt := o.MinFt
	if math.Abs(a.fpm) < LevelFlightFpm && math.Abs(b.fpm) < LevelFlightFpm {
		minFt -= LevelToleranceFt
	}
	for d := time.Duration(0); d <= o.LookAhead; d += o.Step {
		alat, alon, aft := a.at(d)
		blat, blon, bft := b.at(d)
		l := calc.HaversineNM(alat, alon, blat, blon)
		v := math.Abs(aft - bft)
		if l < c.ClosestNM {
			c.ClosestNM, c.ClosestIn, c.VerticalFt = l, d, v
		}
		if l < minNM && v < minFt && c.In < 0 {
			c.In, c.LossNM, c.LossFt = d, l, v
		}
	}
	c.MinNM = minNM
	return c, c.In >= 0
}

// LevelFlightFpm: slower than this an aircraft is level; LevelToleranceFt
// is how far two level ones may read under the vertical minimum and still
// be separated (one standard level apart).
const (
	LevelFlightFpm   = 300.0
	LevelToleranceFt = 100.0
)

func (o ConflictOptions) minFor(a, b TrackedAircraft) float64 {
	// Up to the terminal area's top level, as flown (live: TVS1750 at 10016 ft
	// and TVS554 at 10000 ft, in trail on their STARs 5 NM apart, held to
	// the en-route 5 NM and slowed again and again).
	top := o.TerminalBelowFt + LevelToleranceFt
	if a.Airport != "" && b.Airport != "" && a.AltFt < top && b.AltFt < top {
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
				c.AAltFt, c.BAltFt = a.AltFt, b.AltFt
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
	// ResolveCross: a level to be at (or above, climbing; below,
	// descending) by Fix (at Direct), the climb or descent going on (#662).
	ResolveCross ResolutionKind = "cross"
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
	// Maintain (level): a level aircraft whose route would have it climb or
	// descend is held at AltFt ("maintain flight level 100", Doc 4444
	// 12.3.2.3 a), to go on once clear of the traffic (#697).
	Maintain bool `json:"maintain,omitempty"`
	// Fix and Direct (direct): the fix of the route flown to.
	Fix    string          `json:"fix,omitempty"`
	Direct *airport.LatLon `json:"direct,omitempty"`
	// Why: the conflict it resolves, as ATC would say it.
	Why string `json:"why"`
	// KeepsFt is the smallest vertical distance the change keeps from the
	// traffic of the conflict while within the lateral minimum of it, over
	// the look-ahead: at least the vertical minimum (1000 ft) by
	// construction; 0 when they are never that close laterally.
	KeepsFt float64 `json:"keepsFt"`
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
func candidates(a TrackedAircraft, base track, sameRoute bool, fixes []DirectFix, other TrackedAircraft, minFt float64) []resolutionCandidate {
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
	// Climbing or descending toward the traffic's level: stopped as close to
	// it as the vertical minimum allows (THY1463 climbing to FL240 through
	// BAW1413 level at 10000 ft: 9000 ft, not 4000), the cheapest change
	// that keeps the climb going longest (#657). Every level from there
	// back to the next one on its way, the highest first: the traffic may
	// itself be descending (climbing) toward it, and the first that keeps
	// clear of it all the look-ahead is taken.
	if base.fpm != 0 {
		up := base.fpm > 0
		next := math.Ceil((a.AltFt+500)/1000) * 1000
		top := math.Floor((other.AltFt-minFt)/1000) * 1000
		step := -1000.0
		if !up {
			next = math.Floor((a.AltFt-500)/1000) * 1000
			top = math.Ceil((other.AltFt+minFt)/1000) * 1000
			step = 1000
		}
		if up == (other.AltFt > a.AltFt) {
			for k, alt := 0, top; up && alt >= next || !up && alt <= next; k, alt = k+1, alt+step {
				if alt < a.AltFt-a.AGLFt+1500 {
					break // too low over the ground
				}
				t := base
				t.level = alt
				add(Resolution{Kind: ResolveLevel, AltFt: alt, Stop: true}, t, levelCost+0.7+float64(k)*0.002)
			}
		}
	}
	// Climbing (descending) toward the traffic's level: over (under) it by
	// a fix ahead, the climb going on, "cross VOZ at or above 7000 feet"
	// (Doc 4444 12.3.2.4 a). Where its route would level it off below the
	// traffic (a SID's level) and it makes the level at no more than its
	// rate now, at the nearest fix it can; cheaper than a stop (#662).
	if base.fpm != 0 && len(base.path) > 0 {
		up := base.fpm > 0
		target := math.Ceil((other.AltFt+minFt)/1000) * 1000
		if !up {
			target = math.Floor((other.AltFt-minFt)/1000) * 1000
		}
		if up == (other.AltFt > a.AltFt) && goesTo(base, target, up) && target >= a.AltFt-a.AGLFt+1500 {
			for _, f := range fixes {
				k := nearestOn(base.path, f.Position)
				if k < 0 || calc.HaversineNM(base.path[k].Lat, base.path[k].Lon, f.Position.Lat, f.Position.Lon) > 0.5 {
					continue // not a point of its route
				}
				nm := base.pathNM(k)
				if nm < 1 || base.kts <= 0 || math.Abs(target-a.AltFt)/(nm/base.kts*60) > math.Abs(base.fpm) {
					continue // too near to make the level at its rate
				}
				t := base
				t.alts = crossAlts(base, k, target, up)
				p := f.Position
				add(Resolution{Kind: ResolveCross, Fix: f.Ident, Direct: &p, AltFt: target}, t, levelCost+0.6)
				break // the nearest it can make
			}
		}
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
	// Level now, its route climbing or descending ahead (an arrival about
	// to descend on its STAR): held where it is, "maintain" (#697).
	way := profileWay(base)
	if base.fpm == 0 && way != 0 {
		hold := math.Round(a.AltFt/100) * 100
		t := base
		t.level = hold
		add(Resolution{Kind: ResolveLevel, AltFt: hold, Maintain: true}, t, levelCost+0.7)
	}
	level := math.Round(a.AltFt/1000) * 1000
	for _, dft := range []float64{1000, -1000, 2000, -2000} {
		alt := level + dft
		if base.fpm != 0 && (dft > 0) != (base.fpm > 0) {
			continue // not turned back
		}
		if base.fpm == 0 && way != 0 && (dft > 0) != (way > 0) {
			continue // not against its route's climb or descent
		}
		if alt < a.AltFt-a.AGLFt+1500 {
			continue // too low over the ground
		}
		t := base
		cost := levelCost + 2 + math.Abs(dft)/2000
		// Climb or descend there at 1500 fpm, then level.
		t.fpm, t.level, t.alts = math.Copysign(1500, alt-a.AltFt), alt, nil
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
		t.alts = fromFixAlts(base.path, base.alts, f.Position)
		p := f.Position
		add(Resolution{Kind: ResolveDirect, Fix: f.Ident, Direct: &p}, t, directCost+1+turn/60)
	}
	for _, turn := range []float64{20, -20, 30, -30, 45, -45} {
		t := base
		t.hdg, t.path, t.alts = math.Mod(base.hdg+turn+360, 360), nil, nil // off the route: straight out
		t.fpm, t.level = 0, 0 // level, as ResolvedRoute flies the heading
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
		for _, cand := range candidates(*me, trackFor(*me, o), sameRoute, fixes, *other, o.MinFt) {
			if best != nil && cand.cost >= best.cost {
				break // sorted: nothing cheaper follows
			}
			if !canSteer(*me, cand.r.Kind) {
				continue
			}
			if clearOfAll(cand.t, *me, aircraft, o) {
				cand := cand
				cand.r.Why = fmt.Sprintf("traffic %s, %.1f NM in %s", callsignOf(*other), c.ClosestNM, c.ClosestIn.Round(time.Second))
				cand.r.KeepsFt = verticalWithin(cand.t, trackFor(*other, o), o.minFor(*me, *other), o)
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

// alongProfile is at for a track with the altitudes of its points: the
// altitude moves toward the next point's that has one, at the vertical
// speed now or the rate that makes it by the point if faster; a stopped
// level (t.level) is not passed.
func (t track) alongProfile(d time.Duration) (lat, lon, altFt float64) {
	nm := t.kts * d.Seconds() / 3600
	lat, lon, altFt = t.lat, t.lon, t.altFt
	hdg := t.hdg
	rate := math.Abs(t.fpm) / 60 // ft a second
	last := 0.0                  // the altitude the last leg flew toward
	for i, p := range t.path {
		leg := calc.HaversineNM(lat, lon, p.Lat, p.Lon)
		if leg < 0.01 {
			continue
		}
		hdg = calc.BearingDegrees(lat, lon, p.Lat, p.Lon)
		// The next altitude ahead and how far it is: one the way it climbs
		// or descends now (an altitude behind it, overflown or an "at or
		// above", does not turn it back).
		target, togo := 0.0, leg
		for j := i; j < len(t.alts); j++ {
			if x := t.alts[j]; x > 0 && !(t.fpm > 0 && x < altFt-100) && !(t.fpm < 0 && x > altFt+100) {
				target = t.alts[j]
				break
			}
			if j+1 < len(t.path) {
				togo += calc.HaversineNM(t.path[j].Lat, t.path[j].Lon, t.path[j+1].Lat, t.path[j+1].Lon)
			}
		}
		flown := math.Min(leg, nm)
		last = target
		if target <= 0 { // none ahead: the vertical speed on
			altFt += t.fpm * flown / t.kts * 60
		} else {
			need := math.Abs(target-altFt) / (togo / t.kts * 3600)
			step := math.Max(rate, need) * flown / t.kts * 3600
			altFt += math.Copysign(math.Min(step, math.Abs(target-altFt)), target-altFt)
		}
		if leg >= nm {
			lat, lon = calc.DisplaceByHeading(lat, lon, hdg, nm*1852)
			return lat, lon, t.clampLevel(altFt)
		}
		lat, lon, nm = p.Lat, p.Lon, nm-leg
	}
	lat, lon = calc.DisplaceByHeading(lat, lon, hdg, nm*1852)
	if last <= 0 { // no altitude to level at: the vertical speed on
		altFt += t.fpm * nm / t.kts * 60
	}
	return lat, lon, t.clampLevel(altFt)
}

// clampLevel keeps altFt from passing t.level from where the track starts.
func (t track) clampLevel(altFt float64) float64 {
	if t.level == 0 {
		return altFt
	}
	if t.altFt <= t.level && altFt > t.level || t.altFt >= t.level && altFt < t.level {
		return t.level
	}
	return altFt
}

// PathClear reports whether a, flown along path (the points it would fly
// in turn, e.g. direct to a fix and on along its route) at its speed and
// vertical speed now, keeps separation from every other airborne aircraft
// through the look-ahead: before a direct is cleared (live, PHGVV cleared
// direct DONAD, stopped at 4000 ft for TVS440 eleven seconds later).
func PathClear(a TrackedAircraft, path []airport.LatLon, aircraft []TrackedAircraft, o ConflictOptions) bool {
	o = o.withDefaults()
	base := trackFor(a, o)
	t := base
	t.path, t.alts = path, nil
	if len(path) > 0 {
		// Along its profile from the fix on, as the direct candidates of
		// ResolveConflict (a direct asked for refused for a climb predicted
		// through the traffic, #657).
		if alts := fromFixAlts(base.path, base.alts, path[0]); len(alts) == len(path) {
			t.alts = alts
		}
	}
	return clearOfAll(t, a, aircraft, o)
}

// verticalWithin is the smallest vertical distance between a and b at the
// moments they are within minNM laterally over the look-ahead; 0 never.
func verticalWithin(a, b track, minNM float64, o ConflictOptions) float64 {
	least := math.Inf(1)
	for d := time.Duration(0); d <= o.LookAhead; d += o.Step {
		alat, alon, aft := a.at(d)
		blat, blon, bft := b.at(d)
		if calc.HaversineNM(alat, alon, blat, blon) < minNM {
			least = math.Min(least, math.Abs(aft-bft))
		}
	}
	if math.IsInf(least, 1) {
		return 0
	}
	return least
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
	if r.Kind == ResolveCross && r.Direct != nil && len(ahead) > 0 {
		k, best := -1, math.Inf(1)
		for i, p := range ahead {
			if d := calc.HaversineNM(r.Direct.Lat, r.Direct.Lon, p.Position.Lat, p.Position.Lon); d < best {
				k, best = i, d
			}
		}
		up := r.AltFt > a.AltFt
		cum := make([]float64, len(ahead))
		prev := here.Position
		for i, p := range ahead {
			d := calc.HaversineNM(prev.Lat, prev.Lon, p.Position.Lat, p.Position.Lon)
			if i > 0 {
				d += cum[i-1]
			}
			cum[i], prev = d, p.Position
		}
		out := []RoutePoint{here}
		for i, p := range ahead {
			if i <= k && cum[k] > 0 {
				want := a.AltFt + (r.AltFt-a.AltFt)*cum[i]/cum[k]
				if up && p.AltFt < want || !up && (p.AltFt == 0 || p.AltFt > want) {
					p.AltFt = want
				}
			}
			out = append(out, p)
		}
		return out
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
	at := nearestOn(path, fix)
	out := []airport.LatLon{fix}
	if at >= 0 {
		out = append(out, path[at+1:]...)
	}
	return out
}

// fromFixAlts is the altitudes of fromFix(path, fix): the nearest point's
// at the fix, then on; nil without alts.
func fromFixAlts(path []airport.LatLon, alts []float64, fix airport.LatLon) []float64 {
	if len(alts) != len(path) || len(alts) == 0 {
		return nil
	}
	at := nearestOn(path, fix)
	return append([]float64{alts[at]}, alts[at+1:]...)
}

// nearestOn is the index of path's point nearest p; -1 for an empty path.
func nearestOn(path []airport.LatLon, p airport.LatLon) int {
	at, best := -1, math.Inf(1)
	for i, q := range path {
		if d := calc.HaversineNM(p.Lat, p.Lon, q.Lat, q.Lon); d < best {
			at, best = i, d
		}
	}
	return at
}

// goesTo reports whether t climbs (up) or descends to target anyway: a
// point of its profile at or beyond it, or no profile and no stop.
func goesTo(t track, target float64, up bool) bool {
	if len(t.alts) == 0 {
		return t.level == 0
	}
	for _, x := range t.alts {
		if x > 0 && (up && x >= target || !up && x <= target) {
			return true
		}
	}
	return false
}

// pathNM is how far t flies along its path to point k: to its first
// point, then leg by leg.
func (t track) pathNM(k int) float64 {
	nm, lat, lon := 0.0, t.lat, t.lon
	for i := 0; i <= k && i < len(t.path); i++ {
		nm += calc.HaversineNM(lat, lon, t.path[i].Lat, t.path[i].Lon)
		lat, lon = t.path[i].Lat, t.path[i].Lon
	}
	return nm
}

// crossAlts is t's profile with target at point k, at or above it (up)
// or at or below it.
func crossAlts(t track, k int, target float64, up bool) []float64 {
	alts := make([]float64, len(t.path))
	if len(t.alts) == len(t.path) {
		copy(alts, t.alts)
	}
	if x := alts[k]; x == 0 || up && x < target || !up && x > target {
		alts[k] = target
	}
	return alts
}

// profileWay is where t's profile takes it next: +1 climbing, -1
// descending (its next altitude more than 300 ft off), 0 level or none.
func profileWay(t track) int {
	for _, x := range t.alts {
		if x <= 0 {
			continue
		}
		switch {
		case x > t.altFt+300:
			return 1
		case x < t.altFt-300:
			return -1
		}
		return 0
	}
	return 0
}
