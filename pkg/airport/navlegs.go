package airport

import (
	"errors"
	"math"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Errors from resolving procedures into navigable points (#313).
var (
	ErrNoProcedure  = errors.New("airport: no such procedure")
	ErrNoTransition = errors.New("airport: no such transition")
)

// NavPoint is one point to fly to on a resolved procedure: a charted fix,
// or a computed point where a leg without a fix ends (a climb to an
// altitude, a heading to a manual termination, an intercept). Altitudes are
// meters above sea level, 0 meaning no constraint; speeds knots.
type NavPoint struct {
	Ident string `json:"ident,omitempty"` // "" for a computed point
	// Kind is the fix kind: 'W' waypoint, 'V' VOR, 'N' NDB, 'R' runway,
	// 'A' airport; "" for a computed point.
	Kind     string  `json:"kind,omitempty"`
	Position LatLon  `json:"position"`
	AltMin   float64 `json:"altMin,omitempty"`
	AltMax   float64 `json:"altMax,omitempty"`
	SpeedMax float64 `json:"speedMax,omitempty"`
	FlyOver  bool    `json:"flyOver,omitempty"`
	// LegType is the leg that ends here.
	LegType types.SIMCONNECT_FACILITY_LEG_TYPE `json:"legType"`
	IAF     bool                               `json:"iaf,omitempty"`
	FAF     bool                               `json:"faf,omitempty"`
	MAP     bool                               `json:"map,omitempty"`
	// Course is the true course flown toward this point: the charted one
	// (turned true) for course and heading legs, else the bearing from the
	// previous point; 0 for the first point without a start.
	Course float64 `json:"course"`
	// Vectors marks a manual termination (VM, FM): fly the heading, then
	// expect radar vectors. The point is openLegMeters along it.
	Vectors bool `json:"vectors,omitempty"`
}

// Computed returns whether the point is not a charted fix.
func (n NavPoint) Computed() bool { return n.Ident == "" }

// ResolveSID resolves a SID for a runway into points to fly: the runway
// transition, the common route and the enroute transition ("" for none,
// ending at the last fix of the common route). start is where the
// departure begins (the departure end of the runway) at startAlt meters;
// open legs are computed from it (a climb at 5%, as ProcedurePath draws
// it). A SID with a single runway transition is used for any runway when
// runway is "".
func (p Procedures) ResolveSID(name, runway, enroute string, start LatLon, startAlt float64) ([]NavPoint, error) {
	sid, ok := findProcedure(p.Departures, name)
	if !ok {
		return nil, ErrNoProcedure
	}
	rt, ok := runwayTransition(sid.RunwayTransitions, runway)
	if !ok {
		return nil, ErrNoTransition
	}
	legs := append(append([]Leg(nil), rt.Legs...), sid.Legs...)
	if enroute != "" {
		et, ok := namedTransition(sid.EnrouteTransitions, enroute)
		if !ok {
			return nil, ErrNoTransition
		}
		legs = append(legs, et.Legs...)
	}
	return dedupe(resolveLegs(legs, start, startAlt, p.MagVar)), nil
}

// ResolveSTAR resolves a STAR into points to fly: the enroute transition
// ("" for none), the common route and the transition to runway. A STAR at
// LKPR ends in a heading to a manual termination: its last point has
// Vectors set.
func (p Procedures) ResolveSTAR(name, enroute, runway string) ([]NavPoint, error) {
	star, ok := findProcedure(p.Arrivals, name)
	if !ok {
		return nil, ErrNoProcedure
	}
	var legs []Leg
	if enroute != "" {
		et, ok := namedTransition(star.EnrouteTransitions, enroute)
		if !ok {
			return nil, ErrNoTransition
		}
		legs = append(legs, et.Legs...)
	}
	rt, ok := runwayTransition(star.RunwayTransitions, runway)
	if !ok {
		return nil, ErrNoTransition
	}
	legs = append(append(legs, star.Legs...), rt.Legs...)
	return dedupe(resolveLegs(legs, LatLon{}, 0, p.MagVar)), nil
}

// ResolveApproach resolves an approach ("ILS 06", "RNAV 24") into points
// to fly: the approach transition ("" for none: vectors to final, the
// final's first fix) and the final approach down to the missed approach
// point, usually the runway threshold.
func (p Procedures) ResolveApproach(name, transition string) ([]NavPoint, error) {
	a, ok := p.findApproach(name)
	if !ok {
		return nil, ErrNoProcedure
	}
	var legs []Leg
	if transition != "" {
		t, ok := namedTransition(a.Transitions, transition)
		if !ok {
			return nil, ErrNoTransition
		}
		legs = append(legs, t.Legs...)
	}
	legs = append(legs, a.Final...)
	return dedupe(resolveLegs(legs, LatLon{}, 0, p.MagVar)), nil
}

// MissedApproach resolves an approach's missed approach from its missed
// approach point (the final's last fix, not repeated) at its altitude.
func (p Procedures) MissedApproach(name string) ([]NavPoint, error) {
	a, ok := p.findApproach(name)
	if !ok {
		return nil, ErrNoProcedure
	}
	var start LatLon
	alt := 0.0
	for i := len(a.Final) - 1; i >= 0; i-- {
		if a.Final[i].HasFix() {
			start, alt = a.Final[i].Position, a.Final[i].Alt1
			break
		}
	}
	return dedupe(resolveLegs(a.Missed, start, alt, p.MagVar)), nil
}

// SIDsFor lists the SIDs from a runway (and those without runway
// transitions, which serve all).
func (p Procedures) SIDsFor(runway string) []Procedure { return proceduresFor(p.Departures, runway) }

// STARsFor lists the STARs to a runway (and those without runway
// transitions, which serve all).
func (p Procedures) STARsFor(runway string) []Procedure { return proceduresFor(p.Arrivals, runway) }

// ApproachesFor lists the approaches to a runway.
func (p Procedures) ApproachesFor(runway string) []Approach {
	var out []Approach
	for _, a := range p.Approaches {
		if runwayMatches(a.Runway, runway) {
			out = append(out, a)
		}
	}
	return out
}

// SIDToward finds a SID from a runway that ends at fix, the exit point of
// a flight plan: its common route (then the enroute transition is "") or
// one of its enroute transitions ends there. The first such SID in the
// facility data wins.
func (p Procedures) SIDToward(runway, fix string) (Procedure, string, bool) {
	fix = strings.ToUpper(strings.TrimSpace(fix))
	for _, sid := range p.SIDsFor(runway) {
		rt, _ := runwayTransition(sid.RunwayTransitions, runway)
		if lastFix(append(append([]Leg(nil), rt.Legs...), sid.Legs...)) == fix {
			return sid, "", true
		}
		for _, et := range sid.EnrouteTransitions {
			if lastFix(et.Legs) == fix || strings.EqualFold(et.Name, fix) {
				return sid, et.Name, true
			}
		}
	}
	return Procedure{}, "", false
}

// STARFrom finds a STAR to a runway that starts at fix, the entry point of
// a flight plan: its common route (then the enroute transition is "") or
// one of its enroute transitions starts there.
func (p Procedures) STARFrom(runway, fix string) (Procedure, string, bool) {
	fix = strings.ToUpper(strings.TrimSpace(fix))
	for _, star := range p.STARsFor(runway) {
		rt, _ := runwayTransition(star.RunwayTransitions, runway)
		if firstFix(append(append([]Leg(nil), star.Legs...), rt.Legs...)) == fix {
			return star, "", true
		}
		for _, et := range star.EnrouteTransitions {
			if firstFix(et.Legs) == fix || strings.EqualFold(et.Name, fix) {
				return star, et.Name, true
			}
		}
	}
	return Procedure{}, "", false
}

// BestApproach picks the approach to a runway as ATC would assign it:
// ILS, then RNAV (GPS), localizer, VOR, NDB, any other; the first in the
// facility data among equals.
func (p Procedures) BestApproach(runway string) (Approach, bool) {
	best, rank := Approach{}, -1
	for _, a := range p.ApproachesFor(runway) {
		if r := approachRank(a.Type); rank < 0 || r < rank {
			best, rank = a, r
		}
	}
	return best, rank >= 0
}

func approachRank(t types.SIMCONNECT_FACILITY_APPROACH_TYPE) int {
	switch t {
	case types.SIMCONNECT_FACILITY_APPROACH_TYPE_ILS:
		return 0
	case types.SIMCONNECT_FACILITY_APPROACH_TYPE_RNAV, types.SIMCONNECT_FACILITY_APPROACH_TYPE_GPS:
		return 1
	case types.SIMCONNECT_FACILITY_APPROACH_TYPE_LOCALIZER, types.SIMCONNECT_FACILITY_APPROACH_TYPE_LDA,
		types.SIMCONNECT_FACILITY_APPROACH_TYPE_SDF, types.SIMCONNECT_FACILITY_APPROACH_TYPE_LOCALIZER_BACK_COURSE:
		return 2
	case types.SIMCONNECT_FACILITY_APPROACH_TYPE_VOR, types.SIMCONNECT_FACILITY_APPROACH_TYPE_VORDME:
		return 3
	case types.SIMCONNECT_FACILITY_APPROACH_TYPE_NDB, types.SIMCONNECT_FACILITY_APPROACH_TYPE_NDBDME:
		return 4
	}
	return 5
}

// Arrival is the whole arrival to a runway from a flight plan's entry
// fix: the STAR starting there (if there is one), then the best approach
// (BestApproach) through the transition that starts where the STAR ends,
// down to the threshold. When no transition starts there, the STAR keeps
// its closing radar vectors and the approach joins at its final. Without a
// STAR, a transition starting at entryFix is used if the approach has one.
func (p Procedures) Arrival(runway, entryFix string) ([]NavPoint, error) {
	a, ok := p.BestApproach(runway)
	if !ok {
		return nil, ErrNoProcedure
	}
	var pts []NavPoint
	join := entryFix
	if star, enroute, ok := p.STARFrom(runway, entryFix); entryFix != "" && ok {
		s, err := p.ResolveSTAR(star.Name, enroute, runway)
		if err != nil {
			return nil, err
		}
		pts, join = s, ""
		for i := len(s) - 1; i >= 0; i-- {
			if !s[i].Computed() {
				join = s[i].Ident
				if _, ok := namedTransition(a.Transitions, join); ok {
					pts = s[:i+1] // the approach takes over from the STAR's last fix
				}
				break
			}
		}
	}
	transition := ""
	if t, ok := namedTransition(a.Transitions, join); join != "" && ok {
		transition = t.Name
	}
	app, err := p.ResolveApproach(a.Name, transition)
	if err != nil {
		return nil, err
	}
	return dedupe(append(pts, app...)), nil
}

// resolveLegs turns legs into NavPoints. start (zero when unknown) is
// where the aircraft is before the first leg, at alt meters; it is not
// itself a point.
func resolveLegs(legs []Leg, start LatLon, alt, magVar float64) []NavPoint {
	if magVar > 180 {
		magVar -= 360
	}
	trueCourse := func(l Leg) float64 { return math.Mod(l.Course-magVar+360, 360) }
	cur, have := start, start.Lat != 0 || start.Lon != 0
	var out []NavPoint
	emit := func(n NavPoint) {
		out = append(out, n)
		cur, have = n.Position, true
	}
	// fixPoint is the point at a leg's fix, flown toward on course (or
	// the bearing from the current position when 0).
	fixPoint := func(l Leg, course float64) NavPoint {
		n := NavPoint{Ident: l.Fix, Kind: l.FixKind, Position: l.Position, FlyOver: l.FlyOver, LegType: l.Type,
			IAF: l.IAF, FAF: l.FAF, MAP: l.MAP, SpeedMax: l.Speed, Course: course}
		if course == 0 && have {
			n.Course = calc.BearingDegrees(cur.Lat, cur.Lon, l.Position.Lat, l.Position.Lon)
		}
		n.AltMin, n.AltMax = altLimits(l)
		return n
	}
	for i, l := range legs {
		switch l.Type {
		case types.SIMCONNECT_FACILITY_LEG_TYPE_RF, types.SIMCONNECT_FACILITY_LEG_TYPE_AF:
			// Arcs: points every 15° on the way, then the fix.
			if have && l.HasFix() && (l.ArcCenter.Lat != 0 || l.ArcCenter.Lon != 0) {
				arc := arcPoints(l.ArcCenter, cur, l.Position, l.TurnRight)
				for k := 2; k < len(arc)-1; k += 3 {
					emit(NavPoint{Position: arc[k], LegType: l.Type,
						Course: calc.BearingDegrees(cur.Lat, cur.Lon, arc[k].Lat, arc[k].Lon)})
				}
			}
		case types.SIMCONNECT_FACILITY_LEG_TYPE_FA, types.SIMCONNECT_FACILITY_LEG_TYPE_FC,
			types.SIMCONNECT_FACILITY_LEG_TYPE_FD, types.SIMCONNECT_FACILITY_LEG_TYPE_FM:
			// From the fix, along the course.
			if l.HasFix() && (!have || calc.HaversineMeters(cur.Lat, cur.Lon, l.Position.Lat, l.Position.Lon) > 50) {
				f := fixPoint(l, 0)
				if l.Type == types.SIMCONNECT_FACILITY_LEG_TYPE_FA {
					f.AltMin, f.AltMax = 0, 0 // the altitude is where the leg ends
				}
				emit(f)
			}
			if have {
				emit(openPoint(l, cur, &alt, trueCourse(l), nil))
			}
			continue
		}
		if l.HasFix() {
			c := 0.0
			if l.Type == types.SIMCONNECT_FACILITY_LEG_TYPE_CF {
				c = trueCourse(l)
			}
			f := fixPoint(l, c)
			emit(f)
			if l.Alt1 > 0 {
				alt = l.Alt1
			}
			continue
		}
		if !have {
			continue
		}
		var next *Leg
		if i+1 < len(legs) {
			next = &legs[i+1]
		}
		emit(openPoint(l, cur, &alt, trueCourse(l), func() (LatLon, float64, bool) {
			if next == nil || !next.HasFix() || next.Type != types.SIMCONNECT_FACILITY_LEG_TYPE_CF {
				return LatLon{}, 0, false
			}
			return next.Position, trueCourse(*next), true
		}))
	}
	return out
}

// openPoint is where a leg without a fix ends, flown from cur on course c
// (true): a climb (CA, VA, FA) at climbGradient to its altitude, a DME
// distance (CD, VD, FC, FD) its distance, an intercept (CI, VI) where it
// meets the next leg's course, anything else openLegMeters. alt is the
// altitude so far, raised by climbs.
func openPoint(l Leg, cur LatLon, alt *float64, c float64, intercept func() (LatLon, float64, bool)) NavPoint {
	n := NavPoint{LegType: l.Type, Course: c, SpeedMax: l.Speed}
	n.AltMin, n.AltMax = altLimits(l)
	d := openLegMeters
	switch l.Type {
	case types.SIMCONNECT_FACILITY_LEG_TYPE_CA, types.SIMCONNECT_FACILITY_LEG_TYPE_VA, types.SIMCONNECT_FACILITY_LEG_TYPE_FA:
		d = math.Max(1000, math.Min(20000, (l.Alt1-*alt)/climbGradient))
		*alt = math.Max(*alt, l.Alt1)
		if n.AltMin == 0 && n.AltMax == 0 {
			n.AltMin = l.Alt1 // "climb to"
		}
	case types.SIMCONNECT_FACILITY_LEG_TYPE_CD, types.SIMCONNECT_FACILITY_LEG_TYPE_VD,
		types.SIMCONNECT_FACILITY_LEG_TYPE_FC, types.SIMCONNECT_FACILITY_LEG_TYPE_FD:
		if l.Distance > 0 {
			d = math.Min(l.Distance, 20000)
		}
	case types.SIMCONNECT_FACILITY_LEG_TYPE_CI, types.SIMCONNECT_FACILITY_LEG_TYPE_VI:
		if intercept != nil {
			if fix, fc, ok := intercept(); ok {
				if x, ok := interceptMeters(cur, c, fix, fc); ok {
					d = x
				}
			}
		}
	case types.SIMCONNECT_FACILITY_LEG_TYPE_VM, types.SIMCONNECT_FACILITY_LEG_TYPE_FM:
		n.Vectors = true
	}
	n.Position = displace(cur, c, d)
	return n
}

// interceptMeters is how far along course c from p the course fc into fix
// is met (flat earth, fine for a few tens of kilometers); false when the
// courses are near parallel, the intercept is behind, beyond the fix or
// further than 50 km.
func interceptMeters(p LatLon, c float64, fix LatLon, fc float64) (float64, bool) {
	// Local east/north meters around p.
	kLat := 111320.0
	kLon := kLat * math.Cos(p.Lat*math.Pi/180)
	fx, fy := (fix.Lon-p.Lon)*kLon, (fix.Lat-p.Lat)*kLat
	ax, ay := math.Sin(c*math.Pi/180), math.Cos(c*math.Pi/180)
	bx, by := math.Sin(fc*math.Pi/180), math.Cos(fc*math.Pi/180)
	// p + t·a = fix + s·b, s <= 0 (the intercept is before the fix).
	den := ax*by - ay*bx
	if math.Abs(den) < 0.05 {
		return 0, false
	}
	t := (fx*by - fy*bx) / den
	s := (fx*ay - fy*ax) / den
	if t < 100 || t > 50000 || s > 0 {
		return 0, false
	}
	return t, true
}

// altLimits maps a leg's altitude descriptor to a window, as Constraint
// prints it: AT both, AT_OR_ABOVE the minimum, AT_OR_BELOW the maximum,
// BETWEEN Alt2 (lower) to Alt1 (upper).
func altLimits(l Leg) (lo, hi float64) {
	switch l.AltDesc {
	case types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR_AT:
		return l.Alt1, l.Alt1
	case types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR_AT_OR_ABOVE:
		return l.Alt1, 0
	case types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR_AT_OR_BELOW:
		return 0, l.Alt1
	case types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR_BETWEEN:
		return l.Alt2, l.Alt1
	}
	return 0, 0
}

// dedupe merges consecutive points at the same fix (a STAR ending at the
// approach's IAF, a transition ending at the final's first fix): the
// first keeps its leg and course, flags are combined and the tighter
// constraints kept.
func dedupe(pts []NavPoint) []NavPoint {
	var out []NavPoint
	for _, n := range pts {
		if k := len(out) - 1; k >= 0 && n.Ident != "" && out[k].Ident == n.Ident &&
			calc.HaversineMeters(out[k].Position.Lat, out[k].Position.Lon, n.Position.Lat, n.Position.Lon) < 100 {
			m := &out[k]
			m.IAF, m.FAF, m.MAP, m.FlyOver = m.IAF || n.IAF, m.FAF || n.FAF, m.MAP || n.MAP, m.FlyOver || n.FlyOver
			m.AltMin = math.Max(m.AltMin, n.AltMin)
			m.AltMax, m.SpeedMax = minSet(m.AltMax, n.AltMax), minSet(m.SpeedMax, n.SpeedMax)
			continue
		}
		out = append(out, n)
	}
	return out
}

// minSet is the smaller of two limits, 0 meaning none.
func minSet(a, b float64) float64 {
	if a == 0 || (b != 0 && b < a) {
		return b
	}
	return a
}

func findProcedure(ps []Procedure, name string) (Procedure, bool) {
	for _, p := range ps {
		if strings.EqualFold(p.Name, strings.TrimSpace(name)) {
			return p, true
		}
	}
	return Procedure{}, false
}

func (p Procedures) findApproach(name string) (Approach, bool) {
	want := strings.Join(strings.Fields(strings.ToUpper(name)), " ")
	for _, a := range p.Approaches {
		if strings.ToUpper(a.Name) == want {
			return a, true
		}
	}
	return Approach{}, false
}

// namedTransition finds an enroute or approach transition by name.
func namedTransition(ts []Transition, name string) (Transition, bool) {
	for _, t := range ts {
		if strings.EqualFold(t.Name, strings.TrimSpace(name)) {
			return t, true
		}
	}
	return Transition{}, false
}

// runwayTransition picks a procedure's transition for a runway: the exact
// one, else one that matches loosely (runwayMatches); none needed when
// the procedure has no runway transitions, and the only one when runway
// is "".
func runwayTransition(ts []Transition, runway string) (Transition, bool) {
	if len(ts) == 0 {
		return Transition{}, true
	}
	if strings.TrimSpace(runway) == "" {
		return ts[0], len(ts) == 1
	}
	for _, t := range ts {
		if normRunway(t.Runway) == normRunway(runway) {
			return t, true
		}
	}
	for _, t := range ts {
		if runwayMatches(t.Runway, runway) {
			return t, true
		}
	}
	return Transition{}, false
}

// normRunway normalizes a runway name: "RW6" → "06", "24l" → "24L".
func normRunway(s string) string {
	s = strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(s)), "RW")
	if len(s) == 1 || (len(s) == 2 && s[1] >= 'A') {
		s = "0" + s
	}
	return s
}

// runwayMatches reports whether a procedure's runway (have) serves the
// runway end want: the same end, "ALL", "24B" (both parallels) for 24L or
// 24R, or "24" for any of 24L, 24C, 24R.
func runwayMatches(have, want string) bool {
	h, w := normRunway(have), normRunway(want)
	if h == w || h == "ALL" {
		return true
	}
	if len(h) < 2 || len(w) < 2 || h[:2] != w[:2] {
		return false
	}
	return len(h) == 2 || (len(h) == 3 && h[2] == 'B')
}

func proceduresFor(ps []Procedure, runway string) []Procedure {
	var out []Procedure
	for _, p := range ps {
		if _, ok := runwayTransition(p.RunwayTransitions, runway); ok {
			out = append(out, p)
		}
	}
	return out
}

func firstFix(legs []Leg) string {
	for _, l := range legs {
		if l.HasFix() {
			return strings.ToUpper(l.Fix)
		}
	}
	return ""
}

func lastFix(legs []Leg) string {
	for i := len(legs) - 1; i >= 0; i-- {
		if legs[i].HasFix() {
			return strings.ToUpper(legs[i].Fix)
		}
	}
	return ""
}

// FitSTAR is the STAR for an arrival to runway entering at entryFix, as
// ATC would replace a filed STAR that does not serve the runway in use
// (#755): the filed one when it serves runway (with the enroute transition
// from entryFix, if any); else one from entryFix, the filed one's family
// first (LKPR: "VLM5S" filed for 24, "VLM6T" for 06); else false.
func (p Procedures) FitSTAR(filed, runway, entryFix string) (Procedure, string, bool) {
	filed = strings.ToUpper(strings.TrimSpace(filed))
	enrouteOf := func(s Procedure) string {
		for _, et := range s.EnrouteTransitions {
			if firstFix(et.Legs) == strings.ToUpper(entryFix) || strings.EqualFold(et.Name, entryFix) {
				return et.Name
			}
		}
		return ""
	}
	for _, s := range p.STARsFor(runway) {
		if strings.EqualFold(s.Name, filed) {
			return s, enrouteOf(s), true
		}
	}
	// The family: the letters before the number, the fix it is named
	// after ("VLM5S" for 24, "VLM6T" for 06).
	family := strings.TrimRightFunc(filed, func(r rune) bool { return r < 'A' || r > 'Z' })
	if i := strings.IndexFunc(filed, func(r rune) bool { return r >= '0' && r <= '9' }); i > 0 {
		family = filed[:i]
	}
	var from []Procedure
	for _, s := range p.STARsFor(runway) {
		rt, _ := runwayTransition(s.RunwayTransitions, runway)
		starts := firstFix(append(append([]Leg(nil), s.Legs...), rt.Legs...)) == strings.ToUpper(entryFix) || enrouteOf(s) != ""
		if starts {
			from = append(from, s)
		}
	}
	for _, s := range from {
		if family != "" && strings.HasPrefix(strings.ToUpper(s.Name), family) {
			return s, enrouteOf(s), true
		}
	}
	if len(from) > 0 {
		return from[0], enrouteOf(from[0]), true
	}
	return Procedure{}, "", false
}

// Missed is an approach's published missed approach (#756): its points
// from the missed approach point, the first charted fix and the altitude
// it climbs to (feet; 0 unknown).
type Missed struct {
	Points     []NavPoint `json:"points"`
	Fix        string     `json:"fix,omitempty"`
	AltitudeFt float64    `json:"altitudeFt,omitempty"`
}

// MissedOf is approach name's missed approach, summed up for a go-around
// instruction (#756).
func (p Procedures) MissedOf(name string) (Missed, error) {
	pts, err := p.MissedApproach(name)
	if err != nil {
		return Missed{}, err
	}
	m := Missed{Points: pts}
	for _, n := range pts {
		if m.Fix == "" && !n.Computed() {
			m.Fix = n.Ident
		}
		m.AltitudeFt = math.Max(m.AltitudeFt, math.Round(math.Max(n.AltMin, n.AltMax)/0.3048))
	}
	return m, nil
}
