package airport

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

// The flown route: a whole flight as the line an aircraft flies, from the
// departure runway through the SID, the filed route, the STAR and the
// approach to the arrival threshold, with the missed approach apart. Legs
// without a fix (course and heading legs, arcs) become points as on the
// charts (ProcedurePath); fix-to-fix legs run straight through their fixes.

// FlownRequest is a flight to lay out (FlownRouteFor).
type FlownRequest struct {
	// Departure and Arrival are the two airports' layouts (runway ends,
	// elevations); DepartureRunway and ArrivalRunway their runway ends.
	Departure, Arrival             *Layout
	DepartureRunway, ArrivalRunway string
	// DepartureProcedures and ArrivalProcedures are the airports'
	// procedures; empty, the departure goes out on the runway heading and
	// the arrival flies a synthetic final.
	DepartureProcedures, ArrivalProcedures Procedures
	// Route is the filed enroute fixes, in order (no SID or STAR).
	Route []RouteFix
	// The procedures cleared (or filed); "" chosen here: the SID and STAR
	// that join the route at the runways, the approach by type (ILS, then
	// RNAV, then VOR/NDB and the rest), its transition from the STAR's
	// last fix.
	SID, SIDTransition, STAR, STARTransition, Approach, ApproachTransition string
}

// RouteFix is a filed route's fix; AltFt 0 none.
type RouteFix struct {
	Ident    string  `json:"ident"`
	Position LatLon  `json:"position"`
	AltFt    float64 `json:"altFt,omitempty"`
}

// FlownPoint is a point of the flown route: a fix (Ident set) with its
// constraint and fly-over flag, or a point of a turn or a course leg.
type FlownPoint struct {
	Ident      string        `json:"ident,omitempty"`
	Position   LatLon        `json:"position"`
	Constraint LegConstraint `json:"constraint,omitempty"`
	FlyOver    bool          `json:"flyOver,omitempty"`
	// Phase is "sid", "enroute", "star", "approach" (the transition and
	// the final), or "missed".
	Phase string `json:"phase"`
}

// FlownRoute is the line flown, the missed approach apart, and the
// procedures it took ("" none; Approach "synthetic" for a final made up on
// the extended centreline).
type FlownRoute struct {
	Points []FlownPoint `json:"points"`
	Missed []FlownPoint `json:"missed"`

	SID                string `json:"sid,omitempty"`
	SIDTransition      string `json:"sidTransition,omitempty"`
	STAR               string `json:"star,omitempty"`
	STARTransition     string `json:"starTransition,omitempty"`
	Approach           string `json:"approach,omitempty"`
	ApproachTransition string `json:"approachTransition,omitempty"`
}

// Flown-route shapes.
const (
	// FlownClimbOutNM: without a SID, the runway heading this far before
	// turning on course (about 1500 ft above the field).
	FlownClimbOutNM = 5.0
	// FlownFinalNM: a synthetic final joins the extended centreline this
	// far out, on a 3° path, with an intercept of 90° at most.
	FlownFinalNM = 10.0
	// flownSameMeters: two points this close are one.
	flownSameMeters = 5.0
)

// FlownRouteFor lays out req's flight. It needs the two layouts and the
// runway ends; procedures and route may be empty.
func FlownRouteFor(req FlownRequest) (FlownRoute, error) {
	if req.Departure == nil || req.Arrival == nil {
		return FlownRoute{}, fmt.Errorf("airport: flown route needs both airports")
	}
	dep, ok := req.Departure.runwayEnd(req.DepartureRunway)
	if !ok {
		return FlownRoute{}, fmt.Errorf("%w: %s at %s", ErrUnknownRunway, req.DepartureRunway, req.Departure.ICAO)
	}
	arr, ok := req.Arrival.runwayEnd(req.ArrivalRunway)
	if !ok {
		return FlownRoute{}, fmt.Errorf("%w: %s at %s", ErrUnknownRunway, req.ArrivalRunway, req.Arrival.ICAO)
	}
	var out FlownRoute
	b := flownBuilder{}
	route := slices.Clone(req.Route)

	// Departure: the SID, joining the route at its last fix; else the
	// runway heading, then on course.
	start, startAlt := req.Departure.DepartureStart(dep.Name)
	sid, sidLegs, sidTr := pickSID(req, route)
	if sid != "" {
		out.SID, out.SIDTransition = sid, sidTr
		b.legs(sidLegs, start, startAlt, req.DepartureProcedures.MagVar, "sid")
		route = afterFix(route, lastFixIdent(sidLegs))
	} else {
		b.add(FlownPoint{Ident: "RW" + dep.Name, Position: start, Phase: "sid"})
		lat, lon := calc.DisplaceByHeading(start.Lat, start.Lon, dep.Heading, FlownClimbOutNM*1852)
		b.add(FlownPoint{Position: LatLon{Lat: lat, Lon: lon}, Phase: "sid"})
		if len(route) > 0 {
			b.turnToward(dep.Heading, route[0].Position, TurnRadiusEnroute, "sid")
		}
	}

	// Arrival: the STAR (entered at a route fix), cutting the route there.
	star, starLegs, starTr, entry := pickSTAR(req, route)
	if star != "" {
		out.STAR, out.STARTransition = star, starTr
		route = beforeFix(route, entry)
	}
	for _, f := range route {
		c := LegConstraint{}
		if f.AltFt > 0 {
			c.AtOrAboveFt, c.AtOrBelowFt = f.AltFt, f.AltFt
		}
		b.add(FlownPoint{Ident: f.Ident, Position: f.Position, Constraint: c, Phase: "enroute"})
	}
	// The approach, from the STAR's last fix; else a synthetic final. The
	// STAR's closing vectors (heading legs after its last fix) are left out
	// when an approach takes over there: flown, they lead away and back.
	lastIdent, lastAt := b.lastIdent(), b.last()
	if star != "" {
		lastIdent = lastFixIdent(starLegs)
		for i := len(starLegs) - 1; i >= 0; i-- {
			if starLegs[i].HasFix() && starLegs[i].Fix == lastIdent {
				lastAt = starLegs[i].Position
				break
			}
		}
	}
	ap, apLegs, apTr := pickApproach(req, arr, lastIdent, lastAt)
	if star != "" {
		if ap != nil {
			starLegs = throughFix(starLegs, lastIdent)
		}
		b.legs(starLegs, b.last(), 10000*0.3048, req.ArrivalProcedures.MagVar, "star")
	}
	thr := arr.Threshold
	if ap != nil {
		out.Approach, out.ApproachTransition = ap.Name, apTr
		b.legs(apLegs, b.last(), req.Arrival.Altitude+600, req.ArrivalProcedures.MagVar, "approach")
		if p := b.last(); !strings.HasPrefix(b.pts[len(b.pts)-1].Ident, "RW") && calc.HaversineMeters(p.Lat, p.Lon, thr.Lat, thr.Lon) > flownSameMeters {
			b.add(FlownPoint{Ident: "RW" + arr.Name, Position: thr, Phase: "approach"})
		}
		if len(ap.Missed) > 0 {
			mb := flownBuilder{}
			mb.legs(ap.Missed, b.last(), req.Arrival.Altitude+15, req.ArrivalProcedures.MagVar, "missed")
			out.Missed = mb.pts
			if len(out.Missed) > 0 && samePoint(out.Missed[0].Position, b.last()) {
				out.Missed = out.Missed[1:]
			}
		}
	} else {
		out.Approach = "synthetic"
		b.syntheticFinal(arr, req.Arrival.Altitude)
	}
	out.Points = b.pts
	if out.Missed == nil {
		out.Missed = []FlownPoint{}
	}
	return out, nil
}

// runwayEnd is l's runway end named name ("24", "RW24").
func (l *Layout) runwayEnd(name string) (RunwayEnd, bool) {
	name = normRunway(name)
	for _, r := range l.Runways {
		for _, e := range []RunwayEnd{r.Primary, r.Secondary} {
			if e.Name == name {
				return e, true
			}
		}
	}
	return RunwayEnd{}, false
}

// pickSID is the SID flown from req's runway: req.SID, else the one whose
// last fix (with an enroute transition, its name) is a route fix, the
// latest along the route. A req.SID that does not serve the runway (filed
// before the runway changed) is taken as not given: one of its family
// (LANU1F for 16, LANU1E for 11) is chosen, else any. Its legs: the
// runway transition, the common route, the enroute transition.
func pickSID(req FlownRequest, route []RouteFix) (string, []Leg, string) {
	procs := req.DepartureProcedures.SIDsFor(req.DepartureRunway)
	want, wantTr, fam := given(procs, req.SID, req.SIDTransition)
	name, legsOut, trOut, bestAt, bestFam := "", []Leg(nil), "", -2, false
	for _, d := range procs {
		if want != "" && !strings.EqualFold(d.Name, want) {
			continue
		}
		inFam := fam != "" && family(d.Name) == fam
		rt, _ := runwayTransition(d.RunwayTransitions, req.DepartureRunway)
		cands := d.EnrouteTransitions
		if len(cands) == 0 || wantTr == "" && (want != "" || inFam) && !anyOnRoute(cands, route) {
			cands = append([]Transition{{}}, cands...)
		}
		for _, tr := range cands {
			if wantTr != "" && tr.Name != "" && !strings.EqualFold(tr.Name, wantTr) {
				continue
			}
			legs := append(append(slices.Clone(rt.Legs), d.Legs...), tr.Legs...)
			at := routeIndex(route, lastFixIdent(legs))
			if tr.Name != "" {
				at = max(at, routeIndex(route, tr.Name))
			}
			if want == "" && at < 0 && !inFam {
				continue
			}
			if inFam && !bestFam || inFam == bestFam && at > bestAt {
				name, legsOut, trOut, bestAt, bestFam = d.Name, legs, tr.Name, at, inFam
			}
		}
	}
	return name, legsOut, trOut
}

// given is a procedure and transition asked for among procs (those
// serving the runway): as asked when one of procs; else none, with the
// asked one's family to prefer ("" when nothing was asked).
func given(procs []Procedure, name, transition string) (string, string, string) {
	if name == "" {
		return "", "", ""
	}
	for _, p := range procs {
		if strings.EqualFold(p.Name, name) {
			return name, transition, ""
		}
	}
	return "", "", family(name)
}

// family is a procedure's name before its number: "LANU" of LANU1F.
func family(name string) string {
	name = strings.ToUpper(strings.TrimSpace(name))
	if i := strings.IndexAny(name, "0123456789"); i > 0 {
		return name[:i]
	}
	return name
}

// anyOnRoute reports whether a transition's name is a route fix.
func anyOnRoute(ts []Transition, route []RouteFix) bool {
	for _, t := range ts {
		if routeIndex(route, t.Name) >= 0 {
			return true
		}
	}
	return false
}

// pickSTAR is the STAR flown to req's runway: req.STAR, else the one
// entered at a route fix (its enroute transition's name, else its first
// fix), the latest along the route; a req.STAR not serving the runway is
// taken as not given, its family preferred (as pickSID). Its legs: the
// enroute transition, the common route, the runway transition; entry the
// route fix it starts at.
func pickSTAR(req FlownRequest, route []RouteFix) (string, []Leg, string, string) {
	procs := req.ArrivalProcedures.STARsFor(req.ArrivalRunway)
	want, wantTr, fam := given(procs, req.STAR, req.STARTransition)
	name, legsOut, trOut, entryOut, bestAt, bestFam := "", []Leg(nil), "", "", -2, false
	for _, a := range procs {
		if want != "" && !strings.EqualFold(a.Name, want) {
			continue
		}
		inFam := fam != "" && family(a.Name) == fam
		rt, _ := runwayTransition(a.RunwayTransitions, req.ArrivalRunway)
		cands := a.EnrouteTransitions
		if len(cands) == 0 || wantTr == "" && (want != "" || inFam) && !anyOnRoute(cands, route) {
			cands = append([]Transition{{}}, cands...)
		}
		for _, tr := range cands {
			if wantTr != "" && tr.Name != "" && !strings.EqualFold(tr.Name, wantTr) {
				continue
			}
			legs := append(append(slices.Clone(tr.Legs), a.Legs...), rt.Legs...)
			entry := firstFixIdent(legs)
			at := routeIndex(route, entry)
			if tr.Name != "" && routeIndex(route, tr.Name) > at {
				entry, at = tr.Name, routeIndex(route, tr.Name)
			}
			if want == "" && at < 0 && !inFam {
				continue
			}
			if inFam && !bestFam || inFam == bestFam && at > bestAt {
				name, legsOut, trOut, entryOut, bestAt, bestFam = a.Name, legs, tr.Name, entry, at, inFam
			}
		}
	}
	return name, legsOut, trOut, entryOut
}

// pickApproach is the approach to arr: req.Approach, else BestApproach
// (ILS, RNAV, localizer, VOR, NDB); its legs from where the arrival is
// (last, at fix lastIdent): the transition req names, else the one
// starting at lastIdent, else the final from lastIdent when the final
// passes it, else the transition whose first fix is nearest, else the
// final from its start.
func pickApproach(req FlownRequest, arr RunwayEnd, lastIdent string, last LatLon) (*Approach, []Leg, string) {
	p := req.ArrivalProcedures
	var ap Approach
	var ok bool
	apTr := req.ApproachTransition
	if req.Approach != "" {
		ap, ok = p.findApproach(req.Approach)
		ok = ok && runwayMatches(ap.Runway, arr.Name) // one for another runway: as not given
	}
	if !ok {
		ap, ok = p.BestApproach(arr.Name)
		apTr = ""
	}
	if !ok || len(ap.Final) == 0 {
		return nil, nil, ""
	}
	final := ap.Final
	with := func(t Transition) (*Approach, []Leg, string) {
		return &ap, append(slices.Clone(t.Legs), final...), t.Name
	}
	if t, ok := namedTransition(ap.Transitions, apTr); apTr != "" && ok {
		return with(t)
	}
	if lastIdent != "" {
		for _, t := range ap.Transitions {
			if strings.EqualFold(t.Name, lastIdent) || strings.EqualFold(firstFixIdent(t.Legs), lastIdent) {
				return with(t)
			}
		}
		for i, l := range final {
			if strings.EqualFold(l.Fix, lastIdent) {
				return &ap, final[i:], ""
			}
		}
	}
	best, bestM := -1, math.Inf(1)
	for i, t := range ap.Transitions {
		for _, l := range t.Legs {
			if l.HasFix() {
				if d := calc.HaversineMeters(last.Lat, last.Lon, l.Position.Lat, l.Position.Lon); d < bestM {
					best, bestM = i, d
				}
				break
			}
		}
	}
	if best >= 0 {
		return with(ap.Transitions[best])
	}
	return &ap, final, ""
}

func firstFixIdent(legs []Leg) string {
	for _, l := range legs {
		if l.HasFix() && l.FixKind != "R" {
			return l.Fix
		}
	}
	return ""
}

func lastFixIdent(legs []Leg) string {
	for i := len(legs) - 1; i >= 0; i-- {
		if legs[i].HasFix() && legs[i].FixKind != "R" {
			return legs[i].Fix
		}
	}
	return ""
}

// routeIndex is the index of the route fix ident; -1 none.
func routeIndex(route []RouteFix, ident string) int {
	if ident == "" {
		return -1
	}
	for i, f := range route {
		if strings.EqualFold(f.Ident, ident) {
			return i
		}
	}
	return -1
}

// afterFix is route after its fix ident (all of it when not on it).
func afterFix(route []RouteFix, ident string) []RouteFix {
	if i := routeIndex(route, ident); i >= 0 {
		return route[i+1:]
	}
	return route
}

// beforeFix is route before its fix ident (all of it when not on it).
func beforeFix(route []RouteFix, ident string) []RouteFix {
	if i := routeIndex(route, ident); i >= 0 {
		return route[:i]
	}
	return route
}

func samePoint(a, b LatLon) bool {
	return calc.HaversineMeters(a.Lat, a.Lon, b.Lat, b.Lon) <= flownSameMeters
}

// flownBuilder collects the points, a point on the last one merged.
type flownBuilder struct {
	pts []FlownPoint
}

func (b *flownBuilder) add(p FlownPoint) {
	if n := len(b.pts); n > 0 && samePoint(b.pts[n-1].Position, p.Position) {
		if p.Ident != "" && b.pts[n-1].Ident == "" {
			b.pts[n-1].Ident, b.pts[n-1].Constraint, b.pts[n-1].FlyOver = p.Ident, p.Constraint, p.FlyOver
		}
		return
	}
	b.pts = append(b.pts, p)
}

func (b *flownBuilder) last() LatLon {
	if len(b.pts) == 0 {
		return LatLon{}
	}
	return b.pts[len(b.pts)-1].Position
}

// lastIdent is the last fix's ident so far.
func (b *flownBuilder) lastIdent() string {
	for i := len(b.pts) - 1; i >= 0; i-- {
		if b.pts[i].Ident != "" && !strings.HasPrefix(b.pts[i].Ident, "RW") {
			return b.pts[i].Ident
		}
	}
	return ""
}

// heading is the track into the last point; ok false with fewer than two.
func (b *flownBuilder) heading() (float64, bool) {
	n := len(b.pts)
	if n < 2 {
		return 0, false
	}
	a, c := b.pts[n-2].Position, b.pts[n-1].Position
	return calc.BearingDegrees(a.Lat, a.Lon, c.Lat, c.Lon), true
}

// legs adds legs flown from start (at startAltM), as ProcedurePath draws
// them, with each fix named and constrained: the path's point nearest it
// (within flownSnapMeters, after the last one named) is put on the fix,
// else the fix is put in there.
func (b *flownBuilder) legs(legs []Leg, start LatLon, startAltM, magVar float64, phase string) {
	path := ProcedurePath(legs, start, startAltM, magVar, TurnRadiusEnroute)
	pts := make([]FlownPoint, len(path))
	for i, p := range path {
		pts[i] = FlownPoint{Position: p, Phase: phase}
	}
	from := 0
	for _, l := range legs {
		if !l.HasFix() {
			continue
		}
		fp := FlownPoint{Ident: l.Fix, Position: l.Position, FlyOver: l.FlyOver, Phase: phase}
		if l.FixKind == "R" && !strings.HasPrefix(fp.Ident, "RW") {
			fp.Ident = "RW" + fp.Ident
		}
		fp.Constraint, _ = l.Constraints()
		best, bestM := -1, flownSnapMeters
		for j := from; j < len(pts); j++ {
			if pts[j].Ident != "" {
				continue
			}
			if d := calc.HaversineMeters(pts[j].Position.Lat, pts[j].Position.Lon, l.Position.Lat, l.Position.Lon); d <= bestM {
				best, bestM = j, d
			}
		}
		if best >= 0 {
			pts[best], from = fp, best+1
			continue
		}
		pts = slices.Insert(pts, from, fp)
		from++
	}
	for _, p := range pts {
		b.add(p)
	}
}

// flownSnapMeters: a path point this close to a fix is the fix.
const flownSnapMeters = 300.0

// throughFix is legs up to the last one ending at fix ident (all of them
// when none does).
func throughFix(legs []Leg, ident string) []Leg {
	for i := len(legs) - 1; i >= 0; i-- {
		if legs[i].HasFix() && legs[i].Fix == ident {
			return legs[:i+1]
		}
	}
	return legs
}

// turnToward adds a turn from heading hdg at the last point toward target
// (the shorter way, radius meters, 5° steps) until heading for it: no
// hairpin straight back over the runway.
func (b *flownBuilder) turnToward(hdg float64, target LatLon, radius float64, phase string) {
	at := b.last()
	to := calc.BearingDegrees(at.Lat, at.Lon, target.Lat, target.Lon)
	turn := math.Mod(to-hdg+540, 360) - 180 // + right
	if math.Abs(turn) <= 30 {
		return
	}
	side := 90.0
	if turn < 0 {
		side = -90
	}
	center := displace(at, hdg+side, radius)
	from := calc.BearingDegrees(center.Lat, center.Lon, at.Lat, at.Lon)
	for step := 5.0; step < 360; step += 5 {
		a := from + math.Copysign(step, turn)
		p := displace(center, a, radius)
		h := hdg + math.Copysign(step, turn)
		b.add(FlownPoint{Position: p, Phase: phase})
		want := calc.BearingDegrees(p.Lat, p.Lon, target.Lat, target.Lon)
		if math.Abs(math.Mod(want-h+540, 360)-180) <= 5 {
			return
		}
	}
}

// syntheticFinal adds a final to arr without a published approach: joined
// FlownFinalNM out on the extended centreline at 3° (the intercept 90° at
// most: a base leg first when coming from further round), then the
// threshold.
func (b *flownBuilder) syntheticFinal(arr RunwayEnd, elevM float64) {
	back := math.Mod(arr.Heading+180, 360)
	thr := arr.Threshold
	join := displace(thr, back, FlownFinalNM*1852)
	joinFt := elevM/0.3048 + FlownFinalNM*318
	if len(b.pts) > 0 {
		at := b.last()
		in := calc.BearingDegrees(at.Lat, at.Lon, join.Lat, join.Lon)
		if math.Abs(math.Mod(arr.Heading-in+540, 360)-180) > 90 {
			// A base leg: abeam a point further out, on the side it comes from.
			side := 90.0
			if math.Mod(calc.BearingDegrees(thr.Lat, thr.Lon, at.Lat, at.Lon)-arr.Heading+540, 360)-180 < 0 {
				side = -90
			}
			out := displace(thr, back, (FlownFinalNM+3)*1852)
			b.add(FlownPoint{Position: displace(out, arr.Heading+side, 3*1852), Phase: "approach"})
		}
	}
	b.add(FlownPoint{Ident: "FINAL", Position: join, Constraint: LegConstraint{AtOrAboveFt: math.Round(joinFt/100) * 100}, Phase: "approach"})
	b.add(FlownPoint{Ident: "RW" + arr.Name, Position: thr, Phase: "approach"})
}

// flownHairpinDeg: a corner turning this much or more is a reversal, never
// rounded off (Smoothed).
const flownHairpinDeg = 150.0

// Smoothed is the route's line with its fly-by corners rounded into turns
// of radius meters (SmoothPath): fly-over fixes are kept, the turn after
// them; reversals (flownHairpinDeg or more) are kept too, never a loop.
func (r FlownRoute) Smoothed(radius float64) []LatLon {
	pts := make([]LatLon, len(r.Points))
	keep := map[LatLon]bool{}
	for i, p := range r.Points {
		pts[i] = p.Position
		if p.FlyOver {
			keep[p.Position] = true
		}
		if i > 0 && i+1 < len(r.Points) {
			a, c := r.Points[i-1].Position, r.Points[i+1].Position
			in := calc.BearingDegrees(a.Lat, a.Lon, p.Position.Lat, p.Position.Lon)
			out := calc.BearingDegrees(p.Position.Lat, p.Position.Lon, c.Lat, c.Lon)
			if math.Abs(math.Mod(out-in+540, 360)-180) >= flownHairpinDeg {
				keep[p.Position] = true
			}
		}
	}
	return SmoothPath(pts, radius, func(p LatLon) bool { return keep[p] })
}
