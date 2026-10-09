package nav

import (
	"math"
	"slices"
	"strings"
	"sync"
	"time"

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
	// Parallel is how parallel runways are used together: ParallelAuto
	// (0) the most their spacing allows (ParallelModeFor), ParallelNone one
	// runway, or a mode the airport uses (never more than the spacing
	// allows).
	Parallel ParallelMode
	// ThresholdEntry, when set, tells whether a runway end can be entered
	// at its take-off threshold, without backtracking (airport.Graph's
	// ThresholdEntry). Parallels in use together where one can and the
	// other cannot are segregated: departures on the one with the entry,
	// arrivals on the other.
	ThresholdEntry func(end string) bool `json:"-"`
}

// RunwayLimitsFrom returns runway limits with the airport's preferential
// runways (airport.LimitsFor) for departures and arrivals and the default
// wind limits.
func RunwayLimitsFrom(l airport.Limits) RunwayLimits {
	return RunwayLimits{Preferred: slices.Clone(l.PreferredRunways)}
}

// RunwayUse is the runway configuration of an airport.
type RunwayUse struct {
	// Departure and Arrival are the runway ends in use, the first of
	// Departures and Arrivals: all of them, with parallel runways used
	// together (Parallel).
	Departure  airport.RunwayEnd
	Arrival    airport.RunwayEnd
	Departures []airport.RunwayEnd
	Arrivals   []airport.RunwayEnd
	// Parallel is how the parallels are used (ParallelNone: one runway),
	// SpacingM the distance between their centre lines.
	Parallel ParallelMode
	SpacingM float64
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
	return withParallels(l, RunwayUse{
		Departure:    dep.end,
		Arrival:      arr.end,
		HeadwindKts:  arr.headwind,
		CrosswindKts: arr.cross,
		WithinLimits: depOK && arrOK && len(cands) > 0,
		Approach:     ApproachFor(w),
		Parallel:     ParallelNone,
	}, cands, lim)
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

// RunwayChangeAfter is how long another runway must be the better choice
// before a RunwaySelector changes the runway in use.
const RunwayChangeAfter = 10 * time.Minute

// RunwaySelector keeps the runway in use as an airport does: it changes
// only when the runway in use is out of its wind limits (gusts included),
// or when another has been the better choice for ChangeAfter — not with
// every wind shift near a limit.
type RunwaySelector struct {
	// ChangeAfter: 0 means RunwayChangeAfter.
	ChangeAfter time.Duration
	// Ready, when set, picks the moment of a change that is due (the better
	// choice held ChangeAfter): false waits, for a gap in the traffic, at
	// most MaxChangeWait more (0: RunwayChangeMaxWait). A runway out of its
	// limits changes at once all the same. It is called with the selector
	// locked: it must not call the selector.
	Ready         func(from, to RunwayUse) bool
	MaxChangeWait time.Duration

	mu         sync.Mutex
	use        RunwayUse
	have       bool
	since      time.Time // when the choice first differed
	pending    string    // the better choice (RunwayUse.key)
	pendingUse RunwayUse
	gone       time.Time // when the better choice was last gone
}

// RunwayChangeMaxWait is how long a change that is due waits for its
// RunwaySelector.Ready moment.
const RunwayChangeMaxWait = 15 * time.Minute

// Seed starts the selector from the runway in use already, e.g. saved by an
// earlier run: it is kept as one in use is, not chosen afresh with the
// tighter margin (two selectors started apart otherwise hold different
// runways near a limit).
func (s *RunwaySelector) Seed(use RunwayUse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if use.Departure.Name == "" {
		return
	}
	s.use, s.have, s.since, s.pending = use, true, time.Time{}, ""
}

// Pending is the change coming, if any: the better runway in use and since
// when it has been better (it changes ChangeAfter after that, at the Ready
// moment).
func (s *RunwaySelector) Pending() (RunwayUse, time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.since.IsZero() {
		return RunwayUse{}, time.Time{}, false
	}
	return s.pendingUse, s.since, true
}

// RunwayChoiceMarginKts is how far within its wind limits a runway must be
// to be chosen; one in use is kept up to the limits themselves. Without it
// a runway right at its tailwind limit was chosen and dropped at the next
// gust (LKPR, live: 24 with 4.2 kt of tailwind at the start, 06 a minute
// later, flights spawned for both).
const RunwayChoiceMarginKts = 2.0

// RunwayCalmKts: a wind below this keeps the runway in use while it is
// within its limits; no change is chosen for it.
const RunwayCalmKts = 3.0

// RunwayPendingClearAfter is how long the better choice must be gone
// before its pending change is dropped.
const RunwayPendingClearAfter = time.Minute

// Choose is the runway in use at now: ActiveRunways with the wind limits
// RunwayChoiceMarginKts tighter, held as above.
func (s *RunwaySelector) Choose(now time.Time, l *airport.Layout, w Weather, lim RunwayLimits) RunwayUse {
	fresh := ActiveRunways(l, w, withMargin(lim, RunwayChoiceMarginKts))
	s.mu.Lock()
	defer s.mu.Unlock()
	after := s.ChangeAfter
	if after == 0 {
		after = RunwayChangeAfter
	}
	if !s.have || fresh.Departure.Name == "" {
		s.use, s.have = fresh, fresh.Departure.Name != ""
		return fresh
	}
	if fresh.key() == s.use.key() {
		// The same runways: current wind figures. A pending change goes only
		// once the better choice has been gone RunwayPendingClearAfter (a
		// puff to it and back flickered Pending every few seconds).
		if s.gone.IsZero() {
			s.gone = now
		}
		if now.Sub(s.gone) >= RunwayPendingClearAfter {
			s.since = time.Time{}
		}
		s.use = fresh
		return fresh
	}
	// Near calm: the runway in use stays while within its limits, no
	// better choice (live LKPR: 083/2–3 kt made 06 the choice at each puff).
	if w.WindKts < RunwayCalmKts && !slices.ContainsFunc(append(slices.Clone(s.use.Departures), s.use.Arrivals...), func(e airport.RunwayEnd) bool { return !endWithin(e, w, lim) }) {
		s.since = time.Time{}
		kept := s.use
		kept.HeadwindKts, kept.CrosswindKts = w.Components(kept.Arrival.Heading)
		kept.Approach = ApproachFor(w)
		return kept
	}
	s.gone = time.Time{}
	// Out of limits: change now, to one within them (none within them, the
	// best of the out-of-limits ones would flip with every gust).
	if fresh.WithinLimits && slices.ContainsFunc(append(slices.Clone(s.use.Departures), s.use.Arrivals...), func(e airport.RunwayEnd) bool { return !endWithin(e, w, lim) }) {
		s.use, s.since = fresh, time.Time{}
		return fresh
	}
	// A better choice: only once it has held.
	if s.since.IsZero() || fresh.key() != s.pending {
		s.since, s.pending = now, fresh.key()
	}
	s.pendingUse = fresh
	// Due: at the Ready moment, or once that has been waited for too long.
	wait := s.MaxChangeWait
	if wait == 0 {
		wait = RunwayChangeMaxWait
	}
	if held := now.Sub(s.since); held >= after && (s.Ready == nil || held >= after+wait || s.Ready(s.use, fresh)) {
		s.use, s.since = fresh, time.Time{}
		return fresh
	}
	kept := s.use
	kept.HeadwindKts, kept.CrosswindKts = w.Components(kept.Arrival.Heading)
	kept.Approach = ApproachFor(w)
	return kept
}

// withMargin is lim with its wind limits margin knots tighter.
func withMargin(lim RunwayLimits, margin float64) RunwayLimits {
	tail := lim.MaxTailwindKts
	switch {
	case tail == 0:
		tail = DefaultMaxTailwindKts
	case tail < 0:
		tail = 0
	}
	if tail -= margin; tail <= 0 {
		lim.MaxTailwindKts = -1 // none
	} else {
		lim.MaxTailwindKts = tail
	}
	cross := lim.MaxCrosswindKts
	if cross <= 0 {
		cross = DefaultMaxCrosswindKts
	}
	lim.MaxCrosswindKts = max(cross-margin, 1)
	return lim
}

// endWithin reports whether a runway end is within the wind limits in w,
// gusts included.
func endWithin(e airport.RunwayEnd, w Weather, lim RunwayLimits) bool {
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
	g := w
	g.WindKts = max(w.WindKts, w.GustKts)
	h, x := g.Components(e.Heading)
	return -h <= maxTail+1e-9 && x <= maxCross+1e-9
}
