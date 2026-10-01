//go:build windows
// +build windows

package main

import (
	"fmt"
	"math"
	"net/http"
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Procedures on the map (#314): SIDs, STARs and approaches as paths, and
// the fixes they use (waypoints, VORs, NDBs) with their roles.

type procPath struct {
	Label string `json:"label"`
	// Runway is the runway transition's runway ("" for a path common to
	// all): a procedure for several runways draws only the chosen one.
	Runway string           `json:"runway,omitempty"`
	Points []airport.LatLon `json:"points"`
	Marks  []procMark       `json:"marks"`
	// Tracks label the fix-to-fix legs as charts do: magnetic track and
	// distance at the leg's middle.
	Tracks []trackLabel `json:"tracks,omitempty"`
	// Vectors is where a STAR ends in radar vectors ("continue on track
	// 240°, vectoring will be provided"): from its last fix along the
	// track, drawn as arrows, not as a flown path.
	Vectors      []airport.LatLon `json:"vectors,omitempty"`
	VectorsLabel string           `json:"vectorsLabel,omitempty"`
}

type trackLabel struct {
	Position airport.LatLon `json:"position"`
	Bearing  float64        `json:"bearing"` // true, for the label's rotation
	Text     string         `json:"text"`
}

// procMark is a fix on a path as a chart shows it: symbol, ident and the
// constraint there.
type procMark struct {
	Ident      string         `json:"ident"`
	Kind       string         `json:"kind"`
	Position   airport.LatLon `json:"position"`
	Constraint string         `json:"constraint,omitempty"`
	FlyOver    bool           `json:"flyOver,omitempty"`
}

// vectorsMeters is how far a chart's vector arrows reach.
const vectorsMeters = 9000.0

// mkPath is a chart path: the legs from start and the fixes with their
// constraints. As on charts, fix-to-fix legs are straight lines through
// the fixes; turns are drawn only where they are flown by heading (a
// charted turn direction, a course intercept, a course reversal). With
// vectors, the open legs after the last fix (a STAR's VM: heading, then
// vectors) become the Vectors arrow instead of a flown path.
func mkPath(label string, legs []airport.Leg, start airport.LatLon, alt, magVar, radius float64, vectors bool) procPath {
	var marks []procMark
	for _, l := range legs {
		if !l.HasFix() || l.FixKind == "R" {
			continue
		}
		marks = append(marks, procMark{Ident: l.Fix, Kind: l.FixKind, Position: l.Position, Constraint: l.Constraint(), FlyOver: l.FlyOver})
	}
	var tail []airport.Leg
	if vectors {
		n := len(legs)
		for n > 0 && !legs[n-1].HasFix() {
			n--
		}
		if n > 0 && n < len(legs) {
			legs, tail = legs[:n], legs[n:]
		}
	}
	p := procPath{Label: label, Points: airport.ProcedurePath(legs, start, alt, magVar, radius), Marks: marks, Tracks: trackLabels(legs, magVar)}
	if len(tail) > 0 && len(p.Points) > 0 {
		from, c := last(p.Points), tail[0].Course
		lat, lon := calc.DisplaceByHeading(from.Lat, from.Lon, trueCourse(c, magVar), vectorsMeters)
		p.Vectors = []airport.LatLon{from, {Lat: lat, Lon: lon}}
		p.VectorsLabel = fmt.Sprintf("%03.0f°", c)
		if tail[0].Type == types.SIMCONNECT_FACILITY_LEG_TYPE_VM || tail[0].Type == types.SIMCONNECT_FACILITY_LEG_TYPE_FM {
			p.VectorsLabel += " vectors"
		}
	}
	return p
}

// trueCourse turns a magnetic course true with the facility's MAGVAR (356
// is 4° east).
func trueCourse(mag, magVar float64) float64 {
	if magVar > 180 {
		magVar -= 360
	}
	return math.Mod(mag-magVar+720, 360)
}

// trackLabels are the magnetic track and NM of each fix-to-fix leg of at
// least 2 NM, as charts print them.
func trackLabels(legs []airport.Leg, magVar float64) []trackLabel {
	if magVar > 180 {
		magVar -= 360
	}
	var out []trackLabel
	var prev *airport.Leg
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
				out = append(out, trackLabel{Position: airport.LatLon{Lat: (a.Lat + b.Lat) / 2, Lon: (a.Lon + b.Lon) / 2}, Bearing: brg,
					Text: fmt.Sprintf("%03.0f° %.1f", math.Mod(brg+magVar+720, 360), d/1852)})
			}
		}
		prev = l
	}
	return out
}

type procView struct {
	Name    string     `json:"name"`
	Runways []string   `json:"runways"`
	Paths   []procPath `json:"paths"`
	Missed  []procPath `json:"missed,omitempty"`
	// From and To are the first and last fixes (a SID's end, a STAR's start).
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

type fixView struct {
	Ident    string         `json:"ident"`
	Kind     string         `json:"kind"` // W waypoint, V VOR, N NDB, R runway, A airport
	Position airport.LatLon `json:"position"`
	Roles    []string       `json:"roles,omitempty"` // IAF, FAF, MAP
}

type proceduresView struct {
	SIDs       []procView `json:"sids"`
	STARs      []procView `json:"stars"`
	Approaches []procView `json:"approaches"`
	Fixes      []fixView  `json:"fixes"`
}

// departureEnd is where a SID starts: the far end of the runway.
func departureEnd(l *airport.Layout, runway string) (airport.LatLon, float64) {
	rwy, end, ok := l.RunwayEnd(runway)
	if !ok {
		return airport.LatLon{}, l.Altitude
	}
	if end.Name == rwy.Primary.Name {
		return rwy.Secondary.Threshold, l.Altitude
	}
	return rwy.Primary.Threshold, l.Altitude
}

func last(pts []airport.LatLon) airport.LatLon {
	if len(pts) == 0 {
		return airport.LatLon{}
	}
	return pts[len(pts)-1]
}

func buildProcedures(l *airport.Layout, p airport.Procedures) proceduresView {
	out := proceduresView{SIDs: []procView{}, STARs: []procView{}, Approaches: []procView{}, Fixes: []fixView{}}
	fixes := map[string]*fixView{}
	addFixes := func(legs []airport.Leg) {
		for _, lg := range legs {
			if !lg.HasFix() {
				continue
			}
			f := fixes[lg.Fix+lg.FixKind]
			if f == nil {
				f = &fixView{Ident: lg.Fix, Kind: lg.FixKind, Position: lg.Position}
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
		v := procView{Name: d.Name, Runways: d.Runways()}
		var common []airport.LatLon
		for _, tr := range d.RunwayTransitions {
			start, alt := departureEnd(l, tr.Runway)
			mk := mkPath(d.Name+" "+tr.Runway, append(slices.Clone(tr.Legs), d.Legs...), start, alt, p.MagVar, airport.TurnRadiusEnroute, false)
			mk.Runway = tr.Runway
			v.Paths = append(v.Paths, mk)
			common = mk.Points
			addFixes(tr.Legs)
		}
		if len(d.RunwayTransitions) == 0 {
			mk := mkPath(d.Name, d.Legs, airport.LatLon{}, l.Altitude, p.MagVar, airport.TurnRadiusEnroute, false)
			common = mk.Points
			v.Paths = append(v.Paths, mk)
		}
		for _, e := range d.EnrouteTransitions {
			v.Paths = append(v.Paths, mkPath(d.Name+"."+e.Name, e.Legs, last(common), l.Altitude+1000, p.MagVar, airport.TurnRadiusEnroute, false))
			addFixes(e.Legs)
		}
		addFixes(d.Legs)
		var all []airport.Leg // runway transition → common → enroute transition
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
		v := procView{Name: a.Name, Runways: a.Runways()}
		common := airport.ProcedurePath(a.Legs, airport.LatLon{}, 3000, p.MagVar, 0)
		for _, e := range a.EnrouteTransitions {
			v.Paths = append(v.Paths, mkPath(e.Name+"."+a.Name, append(slices.Clone(e.Legs), a.Legs...), airport.LatLon{}, 3000, p.MagVar, airport.TurnRadiusEnroute, len(a.RunwayTransitions) == 0))
			addFixes(e.Legs)
		}
		if len(a.EnrouteTransitions) == 0 && len(common) > 1 {
			v.Paths = append(v.Paths, mkPath(a.Name, a.Legs, airport.LatLon{}, 3000, p.MagVar, airport.TurnRadiusEnroute, len(a.RunwayTransitions) == 0))
		}
		for _, tr := range a.RunwayTransitions {
			start := last(common)
			if len(common) == 0 && len(a.EnrouteTransitions) > 0 {
				start = last(airport.ProcedurePath(a.EnrouteTransitions[0].Legs, airport.LatLon{}, 3000, p.MagVar, 0))
			}
			mk := mkPath(a.Name+" "+tr.Runway, tr.Legs, start, 1500, p.MagVar, airport.TurnRadiusEnroute, true)
			mk.Runway = tr.Runway
			v.Paths = append(v.Paths, mk)
			addFixes(tr.Legs)
		}
		addFixes(a.Legs)
		var all []airport.Leg // enroute transition → common → runway transition
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
	// procedure on the map; the final and missed approach are the same.
	for _, ap := range p.Approaches {
		final := mkPath(ap.Name, ap.Final, airport.LatLon{}, 1000, p.MagVar, airport.TurnRadiusApproach, false)
		missed := []procPath{mkPath(ap.Name+" missed", ap.Missed, last(final.Points), l.Altitude+100, p.MagVar, airport.TurnRadiusApproach, false)}
		first, _ := endFixes(ap.Final)
		direct := procView{Name: ap.Name, Runways: []string{ap.Runway}, Paths: []procPath{final}, Missed: missed, From: "vectors", To: first}
		out.Approaches = append(out.Approaches, direct)
		for _, tr := range ap.Transitions {
			path := mkPath(ap.Name+" via "+tr.Name, append(slices.Clone(tr.Legs), ap.Final...), airport.LatLon{}, 1500, p.MagVar, airport.TurnRadiusApproach, false)
			out.Approaches = append(out.Approaches, procView{Name: ap.Name + " via " + tr.Name, Runways: []string{ap.Runway},
				Paths: []procPath{path}, Missed: missed, From: tr.Name, To: first})
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

// registerProcedures serves GET /api/procedures?icao=X.
func registerProcedures(mux *http.ServeMux, st *state) {
	mux.HandleFunc("GET /api/procedures", func(w http.ResponseWriter, r *http.Request) {
		icao := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("icao")))
		g, err := st.cache.Graph(icao)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		st.mu.Lock()
		p, ok := st.procedures[g.Layout.ICAO]
		st.mu.Unlock()
		if !ok {
			http.Error(w, "procedures of "+g.Layout.ICAO+" not loaded (yet)", http.StatusNotFound)
			return
		}
		writeJSON(w, buildProcedures(g.Layout, p))
	})
}

// endFixes are the first and last fix idents of legs.
func endFixes(legs []airport.Leg) (string, string) {
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
