//go:build windows
// +build windows

package nav

import (
	"math"
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// ParallelMode is how parallel runways in use work together.
//
// The spacings are the ICAO draft manual on simultaneous operations on
// parallel instrument runways (AN-Conf/11-IP/3): parallels less than 760 m
// apart are one runway for wake turbulence (2.3.3.2); dependent approaches
// from 915 m (2.3.1.1); departures side by side from 760 m (3.3.2).
// Independent approaches need the distance in Annex 14, Volume I:
// ParallelIndependentM.
type ParallelMode int

const (
	// ParallelAuto (RunwayLimits) chooses the mode from the spacing.
	ParallelAuto ParallelMode = iota
	// ParallelNone: one runway for everything.
	ParallelNone
	// ParallelSegregated: arrivals on one runway, departures on the other.
	ParallelSegregated
	// ParallelDependent: both runways take arrivals and departures; the
	// approaches are dependent — ParallelDiagonalNM between aircraft on
	// adjacent finals, the full spacing on the same one.
	ParallelDependent
	// ParallelIndependent: both runways take arrivals and departures, and
	// each final is sequenced on its own.
	ParallelIndependent
)

// Runway spacings (between centre lines) for the modes, and the diagonal
// spacing of dependent approaches (AN-Conf/11-IP/3 2.3.2.2 b: 2.0 NM).
const (
	ParallelSegregatedM  = 760.0
	ParallelDependentM   = 915.0
	ParallelIndependentM = 1035.0
	ParallelDiagonalNM   = 2.0
	// parallelMaxDeg: runway ends this close in heading are parallel.
	parallelMaxDeg = 15.0
)

func (m ParallelMode) String() string {
	switch m {
	case ParallelNone:
		return "single runway"
	case ParallelSegregated:
		return "segregated"
	case ParallelDependent:
		return "dependent parallel"
	case ParallelIndependent:
		return "independent parallel"
	}
	return "auto"
}

// ParallelModeFor is the mode spacing (meters between centre lines) allows:
// ParallelNone under ParallelSegregatedM.
func ParallelModeFor(spacingM float64) ParallelMode {
	switch {
	case spacingM >= ParallelIndependentM:
		return ParallelIndependent
	case spacingM >= ParallelDependentM:
		return ParallelDependent
	case spacingM >= ParallelSegregatedM:
		return ParallelSegregated
	}
	return ParallelNone
}

// RunwaySpacingM is the distance between two runways' centre lines: the
// second's centre off the first's line. Runways crossing or not parallel
// (more than 15° apart) give 0.
func RunwaySpacingM(a, b airport.Runway) float64 {
	if math.Abs(headingDiff(a.Heading, b.Heading)) > parallelMaxDeg && math.Abs(headingDiff(a.Heading, b.Heading+180)) > parallelMaxDeg {
		return 0
	}
	t := a.Primary.Threshold
	far := a.Secondary.Threshold
	return math.Abs(calc.CrossTrackMeters(t.Lat, t.Lon, far.Lat, far.Lon, b.Center.Lat, b.Center.Lon))
}

func headingDiff(a, b float64) float64 { return math.Mod(b-a+540, 360) - 180 }

// withParallels adds to u the runways parallel to its departure end, in the
// same direction and within the wind limits, that can be used together
// with it, and sets the mode: the spacing's (ParallelModeFor), or lim's
// when set. Segregated: arrivals keep u's end, departures go to the
// parallel. A third parallel joins only spaced from both.
func withParallels(l *airport.Layout, u RunwayUse, cands []runwayCandidate, lim RunwayLimits) RunwayUse {
	u.Departures, u.Arrivals = []airport.RunwayEnd{u.Departure}, []airport.RunwayEnd{u.Arrival}
	if l == nil || lim.Parallel == ParallelNone || u.Departure.Name == "" || !u.Single() {
		return u
	}
	own, ok := runwayOfEnd(l, u.Departure.Name)
	if !ok {
		return u
	}
	in := []airport.Runway{own}
	ends := []airport.RunwayEnd{u.Departure}
	minSpacing := math.Inf(1)
	for _, c := range cands {
		if !c.withinLimits || math.Abs(headingDiff(c.end.Heading, u.Departure.Heading)) > parallelMaxDeg {
			continue
		}
		r, ok := runwayOfEnd(l, c.end.Name)
		if !ok || slices.ContainsFunc(in, func(x airport.Runway) bool { return x.Index == r.Index }) {
			continue
		}
		s := math.Inf(1)
		for _, x := range in {
			s = math.Min(s, RunwaySpacingM(x, r))
		}
		if s < ParallelSegregatedM {
			continue // one runway for wake turbulence, or crossing
		}
		in, ends, minSpacing = append(in, r), append(ends, c.end), math.Min(minSpacing, s)
	}
	if len(ends) < 2 {
		return u
	}
	mode := ParallelModeFor(minSpacing)
	if lim.Parallel != ParallelAuto && lim.Parallel < mode {
		mode = lim.Parallel // the airport's rule, never more than the spacing allows
	}
	u.Parallel, u.SpacingM = mode, minSpacing
	switch mode {
	case ParallelSegregated:
		u.Departures = []airport.RunwayEnd{ends[1]}
		u.Departure = ends[1]
	case ParallelDependent, ParallelIndependent:
		u.Departures, u.Arrivals = ends, slices.Clone(ends)
	}
	return u
}

// runwayOfEnd is the runway with end name.
func runwayOfEnd(l *airport.Layout, name string) (airport.Runway, bool) {
	for _, r := range l.Runways {
		if r.Primary.Name == name || r.Secondary.Name == name {
			return r, true
		}
	}
	return airport.Runway{}, false
}

// Names are the runway ends' names.
func Names(ends []airport.RunwayEnd) []string {
	out := make([]string, len(ends))
	for i, e := range ends {
		out[i] = e.Name
	}
	return out
}

// key identifies the runways in use: the same key, the same configuration.
func (u RunwayUse) key() string {
	return strings.Join(Names(u.Departures), ",") + "/" + strings.Join(Names(u.Arrivals), ",") + "/" + u.Departure.Name + "/" + u.Arrival.Name
}

// Nearest is the end of ends whose runway is nearest to p (a stand): the
// parallel a flight from or to it uses. The first end when there is one.
func Nearest(l *airport.Layout, ends []airport.RunwayEnd, p airport.LatLon) airport.RunwayEnd {
	if len(ends) < 2 || l == nil {
		if len(ends) == 0 {
			return airport.RunwayEnd{}
		}
		return ends[0]
	}
	best, bestD := ends[0], math.Inf(1)
	for _, e := range ends {
		r, ok := runwayOfEnd(l, e.Name)
		if !ok {
			continue
		}
		t, far := r.Primary.Threshold, r.Secondary.Threshold
		if d := math.Abs(calc.CrossTrackMeters(t.Lat, t.Lon, far.Lat, far.Lon, p.Lat, p.Lon)); d < bestD {
			best, bestD = e, d
		}
	}
	return best
}
