package airport

import (
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

// The facility API gives a procedure's runway fix (FixKind "R": RW24, the
// MAP of most finals) at the runway's centre, not its threshold: 1 NM down
// a 3.7 km runway (#1573). AtRunwayEnds puts them on the runway ends.
const (
	// runwayFixSameMeters: a runway fix this close to its end is left as
	// it is.
	runwayFixSameMeters = 5.0
	// runwayFixReachMeters: a runway fix further from its end than the
	// runway's length and this is not that runway's (another airport's
	// layout, a renumbered runway): left as it is.
	runwayFixReachMeters = 1852.0
)

// AtRunwayEnds is p with its runway fixes (FixKind "R") placed on l's
// runway ends: the landing threshold (RunwayEnd.LandingThreshold) in
// arrivals and approaches (transitions, final and missed approach), the
// pavement end (RunwayEnd.Threshold), where the take-off starts, in
// departures. The facility data puts them at the runway's centre (#1573),
// so the MAP, the glide path and the flown final would end there. A fix
// already on its end (within a few metres), one whose runway l does not
// have, or one too far from it to be that runway's is kept; so are all of
// them when l is nil or another airport's. moved reports whether any fix
// moved. p's slices are shared where nothing moved: p itself comes back
// when no fix did, so placing the fixes again costs nothing.
func (p Procedures) AtRunwayEnds(l *Layout) (Procedures, bool) {
	if l == nil || len(l.Runways) == 0 || (p.ICAO != "" && l.ICAO != "" && !strings.EqualFold(p.ICAO, l.ICAO)) {
		return p, false
	}
	place := func(landing bool) func([]Leg) ([]Leg, bool) {
		return func(legs []Leg) ([]Leg, bool) {
			var out []Leg
			for i, lg := range legs {
				at, ok := l.runwayFixAt(lg, landing)
				if !ok {
					continue
				}
				if out == nil {
					out = slices.Clone(legs)
				}
				out[i].Position = at
			}
			if out == nil {
				return legs, false
			}
			return out, true
		}
	}
	dep, arr := place(false), place(true)
	out, moved := p, false
	if d, ok := placeProcedures(p.Departures, dep); ok {
		out.Departures, moved = d, true
	}
	if a, ok := placeProcedures(p.Arrivals, arr); ok {
		out.Arrivals, moved = a, true
	}
	var apps []Approach
	for i, ap := range p.Approaches {
		tr, trOK := placeTransitions(ap.Transitions, arr)
		final, finalOK := arr(ap.Final)
		missed, missedOK := arr(ap.Missed)
		if !trOK && !finalOK && !missedOK {
			continue
		}
		if apps == nil {
			apps = slices.Clone(p.Approaches)
		}
		apps[i].Transitions, apps[i].Final, apps[i].Missed = tr, final, missed
	}
	if apps != nil {
		out.Approaches, moved = apps, true
	}
	return out, moved
}

// placeProcedures is procs with place applied to every leg list; ok
// whether any moved (procs itself when none did).
func placeProcedures(procs []Procedure, place func([]Leg) ([]Leg, bool)) ([]Procedure, bool) {
	var out []Procedure
	for i, pr := range procs {
		legs, legsOK := place(pr.Legs)
		rwy, rwyOK := placeTransitions(pr.RunwayTransitions, place)
		enr, enrOK := placeTransitions(pr.EnrouteTransitions, place)
		if !legsOK && !rwyOK && !enrOK {
			continue
		}
		if out == nil {
			out = slices.Clone(procs)
		}
		out[i].Legs, out[i].RunwayTransitions, out[i].EnrouteTransitions = legs, rwy, enr
	}
	if out == nil {
		return procs, false
	}
	return out, true
}

// placeTransitions is ts with place applied to their legs; ok whether any
// moved (ts itself when none did).
func placeTransitions(ts []Transition, place func([]Leg) ([]Leg, bool)) ([]Transition, bool) {
	var out []Transition
	for i, t := range ts {
		legs, ok := place(t.Legs)
		if !ok {
			continue
		}
		if out == nil {
			out = slices.Clone(ts)
		}
		out[i].Legs = legs
	}
	if out == nil {
		return ts, false
	}
	return out, true
}

// runwayFixAt is where lg's runway fix belongs on l: its end's landing
// threshold (landing) or pavement end; ok false when lg is no runway fix,
// l has no such end, or the fix is on it already or too far from it
// (runwayFixSameMeters, runwayFixReachMeters).
func (l *Layout) runwayFixAt(lg Leg, landing bool) (LatLon, bool) {
	if lg.FixKind != "R" || !lg.HasFix() {
		return LatLon{}, false
	}
	name := normRunway(lg.Fix)
	for _, r := range l.Runways {
		for _, e := range []RunwayEnd{r.Primary, r.Secondary} {
			if e.Name != name {
				continue
			}
			at := e.Threshold
			if landing {
				at = e.LandingThreshold()
			}
			if at == (LatLon{}) {
				return LatLon{}, false
			}
			d := calc.HaversineMeters(at.Lat, at.Lon, lg.Position.Lat, lg.Position.Lon)
			if d <= runwayFixSameMeters || d > r.Length+runwayFixReachMeters {
				return LatLon{}, false
			}
			return at, true
		}
	}
	return LatLon{}, false
}
