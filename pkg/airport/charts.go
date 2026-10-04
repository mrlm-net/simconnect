package airport

import (
	"fmt"
	"math"
	"slices"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Charts are the procedures as charts show them (#314): SIDs, STARs and
// approaches as paths with their fixes, constraints and track labels, and
// the fixes they use with their roles — what the airport map draws, for
// any map to draw the same (JSON as the map's /api/procedures).

// ChartSet is an airport's procedures for a chart.
type ChartSet struct {
	SIDs       []ChartProc `json:"sids"`
	STARs      []ChartProc `json:"stars"`
	Approaches []ChartProc `json:"approaches"`
	Fixes      []ChartFix  `json:"fixes"`
}

// ChartProc is one procedure: its paths (one per runway or enroute
// transition), the missed approach, and its first and last fixes (a SID's
// end, a STAR's start; an approach's entry, "vectors" when flown direct).
type ChartProc struct {
	Name    string      `json:"name"`
	Runways []string    `json:"runways"`
	Paths   []ChartPath `json:"paths"`
	Missed  []ChartPath `json:"missed,omitempty"`
	From    string      `json:"from,omitempty"`
	To      string      `json:"to,omitempty"`
}

// ChartPath is a drawn path.
type ChartPath struct {
	Label string `json:"label"`
	// Runway is the runway transition's runway ("" for a path common to
	// all): a procedure for several runways draws only the chosen one.
	Runway string      `json:"runway,omitempty"`
	Points []LatLon    `json:"points"`
	Marks  []ChartMark `json:"marks"`
	// Tracks label the fix-to-fix legs as charts do: magnetic track and
	// distance at the leg's middle.
	Tracks []ChartTrack `json:"tracks,omitempty"`
	// Vectors is where a STAR ends in radar vectors ("continue on track
	// 240°, vectoring will be provided"): from its last fix along the
	// track, drawn as arrows, not as a flown path.
	Vectors      []LatLon `json:"vectors,omitempty"`
	VectorsLabel string   `json:"vectorsLabel,omitempty"`
}

// ChartTrack is a leg's track label.
type ChartTrack struct {
	Position LatLon  `json:"position"`
	Bearing  float64 `json:"bearing"` // true, for the label's rotation
	Text     string  `json:"text"`
}

// ChartMark is a fix on a path as a chart shows it: symbol, ident and the
// constraint there.
type ChartMark struct {
	Ident      string `json:"ident"`
	Kind       string `json:"kind"`
	Position   LatLon `json:"position"`
	Constraint string `json:"constraint,omitempty"`
	FlyOver    bool   `json:"flyOver,omitempty"`
}

// ChartFix is a fix the procedures use, with its roles.
type ChartFix struct {
	Ident    string   `json:"ident"`
	Kind     string   `json:"kind"` // W waypoint, V VOR, N NDB, R runway, A airport
	Position LatLon   `json:"position"`
	Roles    []string `json:"roles,omitempty"` // IAF, FAF, MAP
}

// chartVectorsMeters is how far a chart's vector arrows reach.
const chartVectorsMeters = 9000.0

// Charts builds the chart of l's procedures p.
func Charts(l *Layout, p Procedures) ChartSet {
	out := ChartSet{SIDs: []ChartProc{}, STARs: []ChartProc{}, Approaches: []ChartProc{}, Fixes: []ChartFix{}}
	fixes := map[string]*ChartFix{}
	addFixes := func(legs []Leg) {
		for _, lg := range legs {
			if !lg.HasFix() {
				continue
			}
			f := fixes[lg.Fix+lg.FixKind]
			if f == nil {
				f = &ChartFix{Ident: lg.Fix, Kind: lg.FixKind, Position: lg.Position}
				fixes[lg.Fix+lg.FixKind] = f
			}
			for role, on := range map[string]bool{"IAF": lg.IAF, "FAF": lg.FAF, "MAP": lg.MAP} {
				if on && !slices.Contains(f.Roles, role) {
					f.Roles = append(f.Roles, role)
				}
			}
		}
	}
	for _, d := range p.Departures {
		v := ChartProc{Name: d.Name, Runways: nonNil(d.Runways()), Paths: []ChartPath{}}
		var common []LatLon
		for _, tr := range d.RunwayTransitions {
			start, alt := l.DepartureStart(tr.Runway)
			mk := chartPath(d.Name+" "+tr.Runway, append(slices.Clone(tr.Legs), d.Legs...), start, alt, p.MagVar, TurnRadiusEnroute, false)
			mk.Runway = tr.Runway
			v.Paths = append(v.Paths, mk)
			common = mk.Points
			addFixes(tr.Legs)
		}
		if len(d.RunwayTransitions) == 0 {
			mk := chartPath(d.Name, d.Legs, LatLon{}, l.Altitude, p.MagVar, TurnRadiusEnroute, false)
			common = mk.Points
			v.Paths = append(v.Paths, mk)
		}
		for _, e := range d.EnrouteTransitions {
			v.Paths = append(v.Paths, chartPath(d.Name+"."+e.Name, e.Legs, lastPoint(common), l.Altitude+1000, p.MagVar, TurnRadiusEnroute, false))
			addFixes(e.Legs)
		}
		addFixes(d.Legs)
		var all []Leg // runway transition → common → enroute transition
		if len(d.RunwayTransitions) > 0 {
			all = append(all, d.RunwayTransitions[0].Legs...)
		}
		all = append(all, d.Legs...)
		if len(d.EnrouteTransitions) > 0 {
			all = append(all, d.EnrouteTransitions[0].Legs...)
		}
		v.From, v.To = endFixes(all)
		out.SIDs = append(out.SIDs, v)
	}
	for _, a := range p.Arrivals {
		v := ChartProc{Name: a.Name, Runways: nonNil(a.Runways()), Paths: []ChartPath{}}
		common := ProcedurePath(a.Legs, LatLon{}, 3000, p.MagVar, 0)
		for _, e := range a.EnrouteTransitions {
			v.Paths = append(v.Paths, chartPath(e.Name+"."+a.Name, append(slices.Clone(e.Legs), a.Legs...), LatLon{}, 3000, p.MagVar, TurnRadiusEnroute, len(a.RunwayTransitions) == 0))
			addFixes(e.Legs)
		}
		if len(a.EnrouteTransitions) == 0 && len(common) > 1 {
			v.Paths = append(v.Paths, chartPath(a.Name, a.Legs, LatLon{}, 3000, p.MagVar, TurnRadiusEnroute, len(a.RunwayTransitions) == 0))
		}
		for _, tr := range a.RunwayTransitions {
			start := lastPoint(common)
			if len(common) == 0 && len(a.EnrouteTransitions) > 0 {
				start = lastPoint(ProcedurePath(a.EnrouteTransitions[0].Legs, LatLon{}, 3000, p.MagVar, 0))
			}
			mk := chartPath(a.Name+" "+tr.Runway, tr.Legs, start, 1500, p.MagVar, TurnRadiusEnroute, true)
			mk.Runway = tr.Runway
			v.Paths = append(v.Paths, mk)
			addFixes(tr.Legs)
		}
		addFixes(a.Legs)
		var all []Leg // enroute transition → common → runway transition
		if len(a.EnrouteTransitions) > 0 {
			all = append(all, a.EnrouteTransitions[0].Legs...)
		}
		all = append(all, a.Legs...)
		if len(a.RunwayTransitions) > 0 {
			all = append(all, a.RunwayTransitions[0].Legs...)
		}
		v.From, v.To = endFixes(all)
		out.STARs = append(out.STARs, v)
	}
	// An approach is flown from one entry: each transition ("ILS 24 via
	// ERASU") or direct, radar vectors to the final. Every entry is its own
	// procedure on the chart; the final and missed approach are the same.
	for _, ap := range p.Approaches {
		final := chartPath(ap.Name, ap.Final, LatLon{}, 1000, p.MagVar, TurnRadiusApproach, false)
		missed := []ChartPath{chartPath(ap.Name+" missed", ap.Missed, lastPoint(final.Points), l.Altitude+100, p.MagVar, TurnRadiusApproach, false)}
		first, _ := endFixes(ap.Final)
		out.Approaches = append(out.Approaches, ChartProc{Name: ap.Name, Runways: []string{ap.Runway}, Paths: []ChartPath{final}, Missed: missed, From: "vectors", To: first})
		for _, tr := range ap.Transitions {
			path := chartPath(ap.Name+" via "+tr.Name, append(slices.Clone(tr.Legs), ap.Final...), LatLon{}, 1500, p.MagVar, TurnRadiusApproach, false)
			out.Approaches = append(out.Approaches, ChartProc{Name: ap.Name + " via " + tr.Name, Runways: []string{ap.Runway},
				Paths: []ChartPath{path}, Missed: missed, From: tr.Name, To: first})
			addFixes(tr.Legs)
		}
		addFixes(ap.Final)
		addFixes(ap.Missed)
	}
	for _, f := range fixes {
		out.Fixes = append(out.Fixes, *f)
	}
	return out
}

// chartPath is a chart path: the legs from start and the fixes with their
// constraints. As on charts, fix-to-fix legs are straight lines through the
// fixes; turns are drawn only where they are flown by heading (a charted
// turn direction, a course intercept, a course reversal). With vectors, the
// open legs after the last fix (a STAR's VM: heading, then vectors) become
// the Vectors arrow instead of a flown path.
func chartPath(label string, legs []Leg, start LatLon, alt, magVar, radius float64, vectors bool) ChartPath {
	marks := []ChartMark{} // [] not null in the JSON, as the other lists
	for _, l := range legs {
		if !l.HasFix() || l.FixKind == "R" {
			continue
		}
		marks = append(marks, ChartMark{Ident: l.Fix, Kind: l.FixKind, Position: l.Position, Constraint: l.Constraint(), FlyOver: l.FlyOver})
	}
	var tail []Leg
	if vectors {
		n := len(legs)
		for n > 0 && !legs[n-1].HasFix() {
			n--
		}
		if n > 0 && n < len(legs) {
			legs, tail = legs[:n], legs[n:]
		}
	}
	p := ChartPath{Label: label, Points: ProcedurePath(legs, start, alt, magVar, radius), Marks: marks, Tracks: chartTracks(legs, magVar)}
	if p.Points == nil {
		p.Points = []LatLon{}
	}
	if len(tail) > 0 && len(p.Points) > 0 {
		from, c := lastPoint(p.Points), tail[0].Course
		lat, lon := calc.DisplaceByHeading(from.Lat, from.Lon, trueFromMagnetic(c, magVar), chartVectorsMeters)
		p.Vectors = []LatLon{from, {Lat: lat, Lon: lon}}
		p.VectorsLabel = fmt.Sprintf("%03.0f°", c)
		if tail[0].Type == types.SIMCONNECT_FACILITY_LEG_TYPE_VM || tail[0].Type == types.SIMCONNECT_FACILITY_LEG_TYPE_FM {
			p.VectorsLabel += " vectors"
		}
	}
	return p
}

// trueFromMagnetic turns a magnetic course true with the facility's MAGVAR
// (356 is 4° east).
func trueFromMagnetic(mag, magVar float64) float64 {
	if magVar > 180 {
		magVar -= 360
	}
	return math.Mod(mag-magVar+720, 360)
}

// chartTracks are the magnetic track and NM of each fix-to-fix leg of at
// least 2 NM, as charts print them.
func chartTracks(legs []Leg, magVar float64) []ChartTrack {
	if magVar > 180 {
		magVar -= 360
	}
	var out []ChartTrack
	var prev *Leg
	for i := range legs {
		l := &legs[i]
		if !l.HasFix() {
			prev = nil
			continue
		}
		if prev != nil && prev.Position != l.Position {
			a, b := prev.Position, l.Position
			d := calc.HaversineMeters(a.Lat, a.Lon, b.Lat, b.Lon)
			brg := calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon)
			if d >= 2*1852 {
				out = append(out, ChartTrack{Position: LatLon{Lat: (a.Lat + b.Lat) / 2, Lon: (a.Lon + b.Lon) / 2}, Bearing: brg,
					Text: fmt.Sprintf("%03.0f° %.1f", math.Mod(brg+magVar+720, 360), d/1852)})
			}
		}
		prev = l
	}
	return out
}

// DepartureStart is where a SID from runway starts: the far end of the
// runway, at the airport's altitude (meters).
func (l *Layout) DepartureStart(runway string) (LatLon, float64) {
	rwy, end, ok := l.RunwayEnd(runway)
	if !ok {
		return LatLon{}, l.Altitude
	}
	if end.Name == rwy.Primary.Name {
		return rwy.Secondary.Threshold, l.Altitude
	}
	return rwy.Primary.Threshold, l.Altitude
}

func lastPoint(pts []LatLon) LatLon {
	if len(pts) == 0 {
		return LatLon{}
	}
	return pts[len(pts)-1]
}

// endFixes are the first and last fix idents of legs (runways left out).
func endFixes(legs []Leg) (string, string) {
	first, lastFix := "", ""
	for _, l := range legs {
		if l.HasFix() && l.FixKind != "R" {
			if first == "" {
				first = l.Fix
			}
			lastFix = l.Fix
		}
	}
	return first, lastFix
}

// nonNil is s, or an empty list for nil: [] in the JSON, not null.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
