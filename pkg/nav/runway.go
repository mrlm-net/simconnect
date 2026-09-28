//go:build windows
// +build windows

package nav

import (
	"math"
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Defaults for RunwayLimits.
const (
	DefaultMaxTailwindKts  = 5
	DefaultMaxCrosswindKts = 25
)

// Approach hints of RunwayUse.Approach.
const (
	ApproachILS    = "ILS"
	ApproachVisual = "visual/RNAV"
)

// Weather below either limit calls for an ILS approach.
const (
	ilsVisibilityM = 5000
	ilsCeilingFt   = 1500
)

// RunwayLimits are an airport's rules for choosing the runway in use.
type RunwayLimits struct {
	// MaxTailwindKts is the most tailwind a runway end may have, gusts
	// included; 0 means DefaultMaxTailwindKts, a negative value no tailwind.
	MaxTailwindKts float64
	// MaxCrosswindKts is the most crosswind, gusts included; 0 means
	// DefaultMaxCrosswindKts.
	MaxCrosswindKts float64
	// Preferred are the preferential runway ends in order, e.g. "24", "06"
	// at LKPR. The first one within the wind limits is used even when
	// another end has more headwind, as preferential runway systems do.
	Preferred []string
	// PreferredArrival, when set, replaces Preferred for arrivals.
	PreferredArrival []string
	// MinLengthM leaves out shorter runways.
	MinLengthM float64
}

// RunwayUse is the runway configuration of an airport.
type RunwayUse struct {
	Departure airport.RunwayEnd
	Arrival   airport.RunwayEnd
	// HeadwindKts (negative: tailwind) and CrosswindKts are the mean wind
	// components on the arrival end.
	HeadwindKts  float64
	CrosswindKts float64
	// WithinLimits is false when no runway end meets the wind limits and the
	// ends with the most headwind were taken anyway.
	WithinLimits bool
	// Approach is ApproachILS in low visibility or a low ceiling, else
	// ApproachVisual; see airport.Procedures for the approaches themselves.
	Approach string
}

// Single reports whether departures and arrivals use the same runway end.
func (u RunwayUse) Single() bool { return u.Departure.Name == u.Arrival.Name }

// ApproachFor returns the approach hint for w: ApproachILS when visibility is
// below 5000 m or the ceiling below 1500 ft, else ApproachVisual.
func ApproachFor(w Weather) string {
	if w.VisibilityM > 0 && w.VisibilityM < ilsVisibilityM || w.CeilingFt > 0 && w.CeilingFt < ilsCeilingFt {
		return ApproachILS
	}
	return ApproachVisual
}

type runwayCandidate struct {
	end             airport.RunwayEnd
	length          float64
	headwind, cross float64 // mean wind
	limitH, limitX  float64 // gusts included
	withinLimits    bool
}

// ActiveRunways chooses the departure and arrival runway ends of l for the
// wind in w. The first Preferred end within the limits wins; otherwise the
// end with the most headwind, ties (within 1 kt) going to the longer runway.
// Runway headings are the layout's true headings and the wind is true, so no
// magnetic variation is involved. Names are empty when l has no runway.
func ActiveRunways(l *airport.Layout, w Weather, lim RunwayLimits) RunwayUse {
	maxTail, maxCross := lim.MaxTailwindKts, lim.MaxCrosswindKts
	switch {
	case maxTail == 0:
		maxTail = DefaultMaxTailwindKts
	case maxTail < 0:
		maxTail = 0
	}
	if maxCross <= 0 {
		maxCross = DefaultMaxCrosswindKts
	}
	gustW := w
	gustW.WindKts = max(w.WindKts, w.GustKts)
	var cands []runwayCandidate
	if l != nil {
		for _, r := range l.Runways {
			if r.Length < lim.MinLengthM {
				continue
			}
			for _, e := range []airport.RunwayEnd{r.Primary, r.Secondary} {
				c := runwayCandidate{end: e, length: r.Length}
				c.headwind, c.cross = w.Components(e.Heading)
				c.limitH, c.limitX = gustW.Components(e.Heading)
				c.withinLimits = -c.limitH <= maxTail+1e-9 && c.limitX <= maxCross+1e-9
				cands = append(cands, c)
			}
		}
	}
	dep, depOK := chooseRunway(cands, lim.Preferred)
	arrPref := lim.Preferred
	if len(lim.PreferredArrival) > 0 {
		arrPref = lim.PreferredArrival
	}
	arr, arrOK := chooseRunway(cands, arrPref)
	return RunwayUse{
		Departure:    dep.end,
		Arrival:      arr.end,
		HeadwindKts:  arr.headwind,
		CrosswindKts: arr.cross,
		WithinLimits: depOK && arrOK && len(cands) > 0,
		Approach:     ApproachFor(w),
	}
}

// chooseRunway picks from cands; ok is false when none is within limits.
func chooseRunway(cands []runwayCandidate, preferred []string) (runwayCandidate, bool) {
	for _, p := range preferred {
		p = normalizeEnd(p)
		for _, c := range cands {
			if c.withinLimits && c.end.Name == p {
				return c, true
			}
		}
	}
	within := slices.ContainsFunc(cands, func(c runwayCandidate) bool { return c.withinLimits })
	var best runwayCandidate
	found := false
	for _, c := range cands {
		if within && !c.withinLimits {
			continue
		}
		if !found || c.headwind > best.headwind+1 ||
			math.Abs(c.headwind-best.headwind) <= 1 && c.length > best.length {
			best, found = c, true
		}
	}
	return best, within
}

// normalizeEnd turns "6", "rwy06" or "rw 24l" into "06", "24L".
func normalizeEnd(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(s, "RWY"), "RW"))
	if s != "" && isDigit(s[0]) && (len(s) == 1 || !isDigit(s[1])) {
		s = "0" + s
	}
	return s
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }
