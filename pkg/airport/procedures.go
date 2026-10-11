package airport

import (
	"encoding/binary"
	"math"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Procedures are an airport's instrument procedures from the facility data
// (#312): departures (SIDs), arrivals (STARs) and approaches, with their
// transitions and legs. Load them with ProcedureLoader.
type Procedures struct {
	ICAO string `json:"icao"`
	// MagVar is the airport's magnetic variation in degrees, as the
	// facility data gives it (MAGVAR); leg courses are magnetic.
	MagVar     float64     `json:"magVar"`
	Departures []Procedure `json:"departures"`
	Arrivals   []Procedure `json:"arrivals"`
	Approaches []Approach  `json:"approaches"`
}

// Procedure is a SID or a STAR. A SID is flown runway transition → Legs
// (the common route) → enroute transition; a STAR enroute transition →
// Legs → runway transition.
type Procedure struct {
	Name               string       `json:"name"`
	Legs               []Leg        `json:"legs"` // the common route
	RunwayTransitions  []Transition `json:"runwayTransitions"`
	EnrouteTransitions []Transition `json:"enrouteTransitions"`
}

// Runways lists the runway ends the procedure serves ("24", "06L"); empty
// when it has no runway transitions (it serves all).
func (p Procedure) Runways() []string {
	var out []string
	for _, t := range p.RunwayTransitions {
		out = append(out, t.Runway)
	}
	return out
}

// Transition is a runway transition (Runway set) or an enroute or approach
// transition (Name set: the fix it starts or ends at).
type Transition struct {
	Name   string `json:"name,omitempty"`
	Runway string `json:"runway,omitempty"`
	Legs   []Leg  `json:"legs"`
}

// Approach is an instrument approach to a runway.
type Approach struct {
	// Type is ILS, RNAV, VOR/DME…; Name the conventional label, e.g.
	// "ILS 24", "RNAV 24 Z".
	Type        types.SIMCONNECT_FACILITY_APPROACH_TYPE `json:"type"`
	Name        string                                  `json:"name"`
	Runway      string                                  `json:"runway"`
	Suffix      string                                  `json:"suffix,omitempty"`
	Transitions []Transition                            `json:"transitions"`
	Final       []Leg                                   `json:"final"`
	Missed      []Leg                                   `json:"missed"`
}

// Leg is one procedure leg (an ARINC 424 path terminator). Altitudes are
// meters above sea level, distances meters, courses degrees (true when
// TrueCourse), speeds knots.
type Leg struct {
	Type types.SIMCONNECT_FACILITY_LEG_TYPE `json:"type"`
	// Fix is the leg's fix (ident, region and kind: 'W' waypoint, 'V' VOR,
	// 'N' NDB, 'R' runway, 'A' airport); none for course and heading legs
	// that end at an altitude or a manual termination.
	Fix       string  `json:"fix,omitempty"`
	Region    string  `json:"region,omitempty"`
	FixKind   string  `json:"fixKind,omitempty"`
	Position  LatLon  `json:"position"` // zero without a fix
	FlyOver   bool    `json:"flyOver,omitempty"`
	TurnRight bool    `json:"turnRight,omitempty"`
	TurnLeft  bool    `json:"turnLeft,omitempty"`
	Course    float64 `json:"course,omitempty"`
	Distance  float64 `json:"distance,omitempty"` // leg length or DME distance
	// AltDesc qualifies Alt1 (and Alt2 for BETWEEN).
	AltDesc types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR `json:"altDesc,omitempty"`
	Alt1    float64                                       `json:"alt1,omitempty"`
	Alt2    float64                                       `json:"alt2,omitempty"`
	// Speed is the speed limit; 0 when none.
	Speed     float64 `json:"speed,omitempty"`
	ArcCenter LatLon  `json:"arcCenter"` // RF and AF legs
	Rho       float64 `json:"rho,omitempty"`
	IAF       bool    `json:"iaf,omitempty"`
	FAF       bool    `json:"faf,omitempty"`
	MAP       bool    `json:"map,omitempty"`
}

// HasFix reports whether the leg ends at a positioned fix.
func (l Leg) HasFix() bool { return l.Fix != "" && (l.Position.Lat != 0 || l.Position.Lon != 0) }

// legFields are the APPROACH_LEG fields requested, in the order
// decodeLeg reads them (112 bytes, packed).
var legFields = []string{"TYPE", "FIX_ICAO", "FIX_REGION", "FIX_TYPE", "FIX_LATITUDE", "FIX_LONGITUDE", "FIX_ALTITUDE",
	"FLY_OVER", "TURN_DIRECTION", "COURSE", "ROUTE_DISTANCE", "APPROACH_ALT_DESC", "ALTITUDE1", "ALTITUDE2", "SPEED_LIMIT",
	"ARC_CENTER_FIX_LATITUDE", "ARC_CENTER_FIX_LONGITUDE", "RHO", "IS_IAF", "IS_FAF", "IS_MAP"}

// recordReader reads packed facility record fields.
type recordReader struct {
	b   []byte
	off int
}

func (r *recordReader) take(n int) []byte {
	if r.off+n > len(r.b) {
		r.off = len(r.b)
		return make([]byte, n)
	}
	s := r.b[r.off : r.off+n]
	r.off += n
	return s
}
func (r *recordReader) i32() int32 { return int32(binary.LittleEndian.Uint32(r.take(4))) }
func (r *recordReader) f32() float64 {
	return float64(math.Float32frombits(binary.LittleEndian.Uint32(r.take(4))))
}
func (r *recordReader) f64() float64 {
	return math.Float64frombits(binary.LittleEndian.Uint64(r.take(8)))
}
func (r *recordReader) str(n int) string {
	s := string(r.take(n))
	if i := strings.IndexByte(s, 0); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func decodeLeg(b []byte) Leg {
	r := recordReader{b: b}
	l := Leg{Type: types.SIMCONNECT_FACILITY_LEG_TYPE(r.i32()), Fix: r.str(8), Region: r.str(8)}
	if k := r.i32(); k > 0 {
		l.FixKind = string(rune(k))
	}
	l.Position = LatLon{Lat: r.f64(), Lon: r.f64()}
	_ = r.f64() // FIX_ALTITUDE
	l.FlyOver = r.i32() != 0
	switch r.i32() { // TURN_DIRECTION: 1 left, 2 right
	case 1:
		l.TurnLeft = true
	case 2:
		l.TurnRight = true
	}
	l.Course, l.Distance = r.f32(), r.f32()
	l.AltDesc = types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR(r.i32())
	l.Alt1, l.Alt2 = r.f32(), r.f32()
	if s := r.f32(); s > 0 {
		l.Speed = s
	}
	l.ArcCenter = LatLon{Lat: r.f64(), Lon: r.f64()}
	l.Rho = r.f32()
	l.IAF, l.FAF, l.MAP = r.i32() != 0, r.i32() != 0, r.i32() != 0
	return l
}

// runwayName is a runway end from its number and designator: "24", "06L".
func runwayName(number, designator int32) string {
	n := ""
	if number < 10 {
		n = "0"
	}
	n += itoa(int(number))
	switch types.SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR(designator) {
	case types.SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR_LEFT:
		n += "L"
	case types.SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR_RIGHT:
		n += "R"
	case types.SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR_CENTER:
		n += "C"
	}
	return n
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// approachName is the conventional label: "ILS 24", "RNAV 24 Z".
func approachName(t types.SIMCONNECT_FACILITY_APPROACH_TYPE, runway, suffix string) string {
	kind := map[types.SIMCONNECT_FACILITY_APPROACH_TYPE]string{
		types.SIMCONNECT_FACILITY_APPROACH_TYPE_GPS: "GPS", types.SIMCONNECT_FACILITY_APPROACH_TYPE_VOR: "VOR",
		types.SIMCONNECT_FACILITY_APPROACH_TYPE_NDB: "NDB", types.SIMCONNECT_FACILITY_APPROACH_TYPE_ILS: "ILS",
		types.SIMCONNECT_FACILITY_APPROACH_TYPE_LOCALIZER: "LOC", types.SIMCONNECT_FACILITY_APPROACH_TYPE_SDF: "SDF",
		types.SIMCONNECT_FACILITY_APPROACH_TYPE_LDA: "LDA", types.SIMCONNECT_FACILITY_APPROACH_TYPE_VORDME: "VOR/DME",
		types.SIMCONNECT_FACILITY_APPROACH_TYPE_NDBDME: "NDB/DME", types.SIMCONNECT_FACILITY_APPROACH_TYPE_RNAV: "RNAV",
		types.SIMCONNECT_FACILITY_APPROACH_TYPE_LOCALIZER_BACK_COURSE: "LOC BC",
	}[t]
	if kind == "" {
		kind = "APP"
	}
	name := kind + " " + runway
	if suffix != "" {
		name += " " + suffix
	}
	return name
}

// Procedure path geometry: legs without a fix are drawn from the previous
// point along their course, a climb (CA, VA, FA) at climbGradient up to its
// altitude, others (FM, VM, CD, VD, CR, VR, CI, VI) for at most
// openLegMeters.
const (
	climbGradient = 0.05 // 5% (about 300 ft per NM)
	openLegMeters = 5000.0
)

// ProcedurePath is where the legs lead, as points for a map: from start
// (e.g. the departure end of the runway, at startAlt meters) through each
// leg's fix. It follows the aircraft's heading: a leg with a charted turn
// direction, a course to a fix after an open leg, or a turn of more than 90°
// (a course reversal) is joined by a turn of turnRadius meters (Dubins, in
// the charted direction); legs from a fix (FA, FC, FD, FM) continue along
// their course after it; legs without a fix (to an altitude, a radial, a
// DME distance, a manual termination) extend from the previous point; RF
// and DME arcs are sampled. Courses are magnetic, turned true with magVar
// (the facility's MAGVAR). An approximation for display, not guidance.
func ProcedurePath(legs []Leg, start LatLon, startAlt, magVar, turnRadius float64) []LatLon {
	if magVar > 180 {
		magVar -= 360
	}
	if turnRadius <= 0 {
		turnRadius = TurnRadiusEnroute
	}
	course := func(l Leg) float64 { return math.Mod(l.Course-magVar+360, 360) }
	var pts []LatLon
	cur, alt, have := start, startAlt, start.Lat != 0 || start.Lon != 0
	hdg, haveHdg := 0.0, false
	if have {
		pts = append(pts, cur)
	}
	open := func(l Leg) float64 {
		switch l.Type {
		case types.SIMCONNECT_FACILITY_LEG_TYPE_CA, types.SIMCONNECT_FACILITY_LEG_TYPE_VA, types.SIMCONNECT_FACILITY_LEG_TYPE_FA:
			d := math.Max(1000, math.Min(20000, (l.Alt1-alt)/climbGradient))
			alt = math.Max(alt, l.Alt1)
			return d
		case types.SIMCONNECT_FACILITY_LEG_TYPE_CD, types.SIMCONNECT_FACILITY_LEG_TYPE_VD,
			types.SIMCONNECT_FACILITY_LEG_TYPE_FC, types.SIMCONNECT_FACILITY_LEG_TYPE_FD:
			if l.Distance > 0 {
				return math.Min(l.Distance, 20000)
			}
		}
		return openLegMeters
	}
	// turnTo joins the path to p, arriving on heading to: a turn in the
	// charted direction (or the shorter way) when one is needed.
	turnTo := func(l Leg, p LatLon, to float64, force bool) {
		dir := 0
		if l.TurnRight {
			dir = 1
		} else if l.TurnLeft {
			dir = -1
		}
		change := math.Abs(math.Mod(to-hdg+540, 360) - 180)
		if have && haveHdg && (force || dir != 0 || change > 90) {
			if arc := calc.Dubins(cur.Lat, cur.Lon, hdg, p.Lat, p.Lon, to, turnRadius, 100, dir); arc != nil {
				for _, q := range arc[1:] {
					pts = append(pts, LatLon{Lat: q[0], Lon: q[1]})
				}
				return
			}
		}
		pts = append(pts, p)
	}
	// turnOnto flies an open leg: first the turn from the current heading
	// onto course c (the charted way, or the shorter), then along it.
	turnOnto := func(l Leg, c float64) {
		if haveHdg {
			turn := math.Mod(c-hdg+540, 360) - 180 // + right
			if l.TurnRight && turn < 0 {
				turn += 360
			} else if l.TurnLeft && turn > 0 {
				turn -= 360
			}
			if math.Abs(turn) > 2 {
				side := 90.0
				if turn < 0 {
					side = -90
				}
				center := displace(cur, hdg+side, turnRadius)
				from := calc.BearingDegrees(center.Lat, center.Lon, cur.Lat, cur.Lon)
				n := int(math.Abs(turn)/5) + 1
				for k := 1; k <= n; k++ {
					pts = append(pts, displace(center, from+turn*float64(k)/float64(n), turnRadius))
				}
				cur = pts[len(pts)-1]
			}
		}
		cur = displace(cur, c, open(l))
		pts = append(pts, cur)
		hdg, haveHdg = c, true
	}
	fixed := false // the previous leg ended at a fix
	for _, l := range legs {
		switch l.Type {
		case types.SIMCONNECT_FACILITY_LEG_TYPE_RF, types.SIMCONNECT_FACILITY_LEG_TYPE_AF:
			if have && l.HasFix() && (l.ArcCenter.Lat != 0 || l.ArcCenter.Lon != 0) {
				pts = append(pts, arcPoints(l.ArcCenter, cur, l.Position, l.TurnRight)...)
				if n := len(pts); n >= 2 {
					hdg, haveHdg = calc.BearingDegrees(pts[n-2].Lat, pts[n-2].Lon, pts[n-1].Lat, pts[n-1].Lon), true
				}
				cur, fixed = l.Position, true
				continue
			}
		case types.SIMCONNECT_FACILITY_LEG_TYPE_FA, types.SIMCONNECT_FACILITY_LEG_TYPE_FC,
			types.SIMCONNECT_FACILITY_LEG_TYPE_FD, types.SIMCONNECT_FACILITY_LEG_TYPE_FM:
			// From the fix, along the course.
			if l.HasFix() && (!have || cur != l.Position) {
				if have {
					hdg, haveHdg = calc.BearingDegrees(cur.Lat, cur.Lon, l.Position.Lat, l.Position.Lon), true
				}
				pts = append(pts, l.Position)
				cur, have = l.Position, true
			}
			if have {
				turnOnto(l, course(l))
				fixed = false
			}
			continue
		}
		if l.HasFix() {
			to := calc.BearingDegrees(cur.Lat, cur.Lon, l.Position.Lat, l.Position.Lon)
			force := false
			if l.Type == types.SIMCONNECT_FACILITY_LEG_TYPE_CF {
				to, force = course(l), !fixed // intercept the course after an open leg
			}
			if have && cur == l.Position {
				continue
			}
			if have {
				turnTo(l, l.Position, to, force)
			} else {
				pts = append(pts, l.Position)
			}
			hdg, haveHdg = to, have
			cur, have, fixed = l.Position, true, true
			if l.Alt1 > 0 {
				alt = l.Alt1
			}
			continue
		}
		if !have {
			continue
		}
		turnOnto(l, course(l))
		fixed = false
	}
	return pts
}

// displace moves p d meters along a true bearing.
func displace(p LatLon, bearing, d float64) LatLon {
	lat, lon := calc.DisplaceByHeading(p.Lat, p.Lon, bearing, d)
	return LatLon{Lat: lat, Lon: lon}
}

// arcPoints samples the arc around center from a to b (clockwise when
// right), every 5°.
func arcPoints(center, a, b LatLon, right bool) []LatLon {
	// The radius goes over from a's to b's along the arc, which ends on b
	// itself: the charted fixes are a little off one radius, and a sample
	// at a's radius then b made a short radial jog that the smoothing
	// turned into a loop (EDDM AKIN1N at DM044: ~330° round, live).
	ra := calc.HaversineMeters(center.Lat, center.Lon, a.Lat, a.Lon)
	rb := calc.HaversineMeters(center.Lat, center.Lon, b.Lat, b.Lon)
	from := calc.BearingDegrees(center.Lat, center.Lon, a.Lat, a.Lon)
	to := calc.BearingDegrees(center.Lat, center.Lon, b.Lat, b.Lon)
	sweep := math.Mod(to-from+360, 360) // clockwise
	if !right {
		sweep -= 360
	}
	var out []LatLon
	n := int(math.Abs(sweep)/5) + 1
	for i := 1; i < n; i++ {
		k := float64(i) / float64(n)
		out = append(out, displace(center, from+sweep*k, ra+(rb-ra)*k))
	}
	return append(out, b)
}

// Turn radii for SmoothPath: a standard 25° bank turn at 210 kt (SIDs,
// STARs) and at 180 kt (approaches).
const (
	TurnRadiusEnroute  = 2500.0
	TurnRadiusApproach = 1800.0
)

// SmoothPath rounds the corners of a procedure path into turns of radius
// meters, as charts draw them (fly-by); a corner's arc is limited by half
// of each adjoining segment. keep marks corners not to round (fly-over
// fixes: the turn starts after the fix). Arcs are sampled every 5°.
func SmoothPath(pts []LatLon, radius float64, keep func(LatLon) bool) []LatLon {
	if len(pts) < 3 {
		return pts
	}
	out := []LatLon{pts[0]}
	for i := 1; i+1 < len(pts); i++ {
		a, v, b := pts[i-1], pts[i], pts[i+1]
		if keep != nil && keep(v) {
			out = append(out, v)
			continue
		}
		in := calc.BearingDegrees(a.Lat, a.Lon, v.Lat, v.Lon)
		outB := calc.BearingDegrees(v.Lat, v.Lon, b.Lat, b.Lon)
		turn := math.Mod(outB-in+540, 360) - 180 // + right
		if math.Abs(turn) < 2 {
			out = append(out, v)
			continue
		}
		l1 := calc.HaversineMeters(a.Lat, a.Lon, v.Lat, v.Lon)
		l2 := calc.HaversineMeters(v.Lat, v.Lon, b.Lat, b.Lon)
		half := math.Abs(turn) / 2 * math.Pi / 180
		t := math.Min(radius*math.Tan(half), math.Min(l1, l2)/2)
		r := t / math.Tan(half)
		start := displace(v, in+180, t)
		side := 90.0
		if turn < 0 {
			side = -90
		}
		center := displace(start, in+side, r)
		from := calc.BearingDegrees(center.Lat, center.Lon, start.Lat, start.Lon)
		n := int(math.Abs(turn)/5) + 1
		for k := 0; k <= n; k++ {
			out = append(out, displace(center, from+turn*float64(k)/float64(n), r))
		}
	}
	return append(out, pts[len(pts)-1])
}

// Constraint is a leg's altitude and speed constraint as charts print it:
// "5000", "≥4000", "≤FL100", "FL070–FL100", "≤210KT". Altitudes above
// transition (18000 ft here, a display simplification) as flight levels.
func (l Leg) Constraint() string {
	ft := func(m float64) string {
		f := math.Round(m/0.3048/100) * 100
		if f >= 18000 {
			return "FL" + itoa(int(f/100))
		}
		return itoa(int(f))
	}
	s := ""
	switch l.AltDesc {
	case types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR_AT:
		s = ft(l.Alt1)
	case types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR_AT_OR_ABOVE:
		s = "≥" + ft(l.Alt1)
	case types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR_AT_OR_BELOW:
		s = "≤" + ft(l.Alt1)
	case types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR_BETWEEN:
		s = ft(l.Alt2) + "–" + ft(l.Alt1)
	}
	if l.Speed > 0 {
		if s != "" {
			s += " "
		}
		s += "≤" + itoa(int(l.Speed)) + "KT"
	}
	return s
}

// LegConstraint is a leg's altitude and speed constraint in feet and knots
// (#754): AtOrAboveFt and AtOrBelowFt (both for an "at" or a window, 0
// none), SpeedKts the speed limit (0 none).
type LegConstraint struct {
	AtOrAboveFt float64 `json:"atOrAboveFt,omitempty"`
	AtOrBelowFt float64 `json:"atOrBelowFt,omitempty"`
	SpeedKts    float64 `json:"speedKts,omitempty"`
}

// Constraints is the leg's constraint; false when it has none.
func (l Leg) Constraints() (LegConstraint, bool) {
	ft := func(m float64) float64 { return math.Round(m / 0.3048) }
	var c LegConstraint
	switch l.AltDesc {
	case types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR_AT:
		c.AtOrAboveFt, c.AtOrBelowFt = ft(l.Alt1), ft(l.Alt1)
	case types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR_AT_OR_ABOVE:
		c.AtOrAboveFt = ft(l.Alt1)
	case types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR_AT_OR_BELOW:
		c.AtOrBelowFt = ft(l.Alt1)
	case types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR_BETWEEN:
		c.AtOrAboveFt, c.AtOrBelowFt = ft(l.Alt2), ft(l.Alt1)
	}
	c.SpeedKts = l.Speed
	return c, c != LegConstraint{}
}

// FixConstraint is a fix of a procedure with its constraint.
type FixConstraint struct {
	Fix      string `json:"fix"`
	Position LatLon `json:"position"`
	LegConstraint
}

// LegConstraints are the constrained fixes of legs, in order (#754): what
// "descend via" keeps, and what to say "cross (fix) at or above" of.
func LegConstraints(legs []Leg) []FixConstraint {
	var out []FixConstraint
	for _, l := range legs {
		if c, ok := l.Constraints(); ok && l.HasFix() {
			out = append(out, FixConstraint{Fix: l.Fix, Position: l.Position, LegConstraint: c})
		}
	}
	return out
}
