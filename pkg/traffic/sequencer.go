//go:build windows
// +build windows

package traffic

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// The approach sequencer (#390) is the approach controller of one runway:
// it predicts when each arrival would land, puts them in order (first
// come, first served) and gives each a landing time that keeps the wake
// spacing on final behind the one before, and the runway free. What an
// arrival must lose to make its time is its delay, which speed control,
// path stretching (#391) and holding (#392) absorb.

// ApproachAircraft is an arrival the sequencer plans for.
type ApproachAircraft struct {
	Callsign string
	Wake     Wake
	// DistanceToGoNM is its track distance to the threshold (DistanceToGo).
	DistanceToGoNM float64
	// GroundKts is its speed now; FinalKts its speed on final (0: 140).
	GroundKts, FinalKts float64
	// Fixed: it cannot be delayed — other traffic, or already established.
	// The sequencer also fixes aircraft inside FreezeNM.
	Fixed bool
}

// SequenceEntry is an arrival's place in the landing sequence.
type SequenceEntry struct {
	Callsign string `json:"callsign"`
	Number   int    `json:"number"`           // 1 lands first
	Leader   string `json:"leader,omitempty"` // the one landing before
	Wake     Wake   `json:"wake"`
	// SpacingNM is the spacing it keeps behind its leader on final, and
	// SpacingWhy why it differs from the wake minimum (low visibility
	// procedures, contaminated runway, reduced separation, runway occupancy).
	SpacingNM  float64 `json:"spacingNM,omitempty"`
	SpacingWhy string  `json:"spacingWhy,omitempty"`
	// ETA is when it would land flying on as it is; Landing when it lands
	// in the sequence; Delay the difference it must absorb.
	ETA     time.Time     `json:"eta"`
	Landing time.Time     `json:"landing"`
	Delay   time.Duration `json:"delay"`
	Fixed   bool          `json:"fixed,omitempty"`
	// DistanceToGoNM as given.
	DistanceToGoNM float64 `json:"distanceToGoNM"`
}

// SequenceChange reports an arrival's new place or delay.
type SequenceChange struct {
	Runway   string
	Entry    SequenceEntry
	Previous int  // its number before; 0 when new
	Gone     bool // left the sequence (landed or removed)
}

// SequencerOptions tune an ApproachSequencer.
type SequencerOptions struct {
	Scheme SeparationScheme
	// FreezeNM: inside this distance to go an arrival is established and
	// keeps its place (default 8 NM, about the final approach fix).
	FreezeNM float64
	// FinalNM: the last part of the approach is flown at the final speed
	// (default 10 NM) — the rest at the ground speed now.
	FinalNM float64
	// DelayStep: a delay change smaller than this is not reported (30 s).
	DelayStep time.Duration
	// SwapMargin: arrivals in the sequence change places only when their
	// predicted landings part by more than this (default 90 s).
	SwapMargin time.Duration
	// AllowReduced uses the reduced radar separation (2.5 NM) where the
	// conditions allow it (and the airport is approved for it).
	AllowReduced bool
	// TimeBased keeps the spacing's time instead of its distance: in a
	// headwind the distance shrinks (time-based separation, TBS); the
	// default keeps the distance, which takes longer to fly into the wind.
	TimeBased bool
	// OnChange is called with every change of place or delay.
	OnChange func(SequenceChange)
}

// ApproachSequencer sequences the arrivals of one runway.
type ApproachSequencer struct {
	runway string
	opts   SequencerOptions

	mu   sync.Mutex
	last map[string]SequenceEntry
	seq  []SequenceEntry
	// first is each arrival's prediction when it joined the sequence.
	first map[string]time.Time
	cond  ApproachConditions
}

// NewApproachSequencer creates the sequencer of a runway end ("24").
func NewApproachSequencer(runway string, opts SequencerOptions) *ApproachSequencer {
	if opts.FreezeNM == 0 {
		opts.FreezeNM = 8
	}
	if opts.FinalNM == 0 {
		opts.FinalNM = 10
	}
	if opts.DelayStep == 0 {
		opts.DelayStep = 30 * time.Second
	}
	if opts.SwapMargin == 0 {
		opts.SwapMargin = 90 * time.Second
	}
	return &ApproachSequencer{runway: runway, opts: opts, last: map[string]SequenceEntry{}, first: map[string]time.Time{}}
}

// Runway is the sequencer's runway end.
func (s *ApproachSequencer) Runway() string { return s.runway }

// Sequence is the last sequence, first to land first.
func (s *ApproachSequencer) Sequence() []SequenceEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]SequenceEntry(nil), s.seq...)
}

// eta predicts when an arrival lands flying on as it is.
func (s *ApproachSequencer) eta(now time.Time, a ApproachAircraft, c ApproachConditions) time.Time {
	final := c.FinalGroundKts(a.FinalKts) // on final, into the wind
	gs := math.Max(a.GroundKts, final)
	outer := math.Max(0, a.DistanceToGoNM-s.opts.FinalNM)
	inner := math.Min(a.DistanceToGoNM, s.opts.FinalNM)
	h := outer/gs + inner/((gs+final)/2)
	if a.DistanceToGoNM <= s.opts.FinalNM {
		h = a.DistanceToGoNM / final
	}
	return now.Add(time.Duration(h * float64(time.Hour)))
}

// gap is the time between two landings in the conditions: the follower's
// spacing on final (ArrivalSpacing) flown at its ground speed on final —
// or, time-based, at its airspeed — and at least the leader's runway
// occupancy on the surface. It returns the time, the spacing and why the
// spacing differs from the wake minimum.
func (s *ApproachSequencer) gap(lead, follow ApproachAircraft, c ApproachConditions) (time.Duration, float64, string) {
	nm, why := ArrivalSpacing(lead.Wake, follow.Wake, s.opts.Scheme, c, s.opts.AllowReduced)
	kts := c.FinalGroundKts(follow.FinalKts)
	if s.opts.TimeBased {
		kts = ApproachConditions{}.FinalGroundKts(follow.FinalKts) // the time, not the distance, is kept
	}
	g := SeparationTime(nm, kts)
	if occ := RunwayOccupancyIn(lead.Wake, true, c.Surface); occ > g {
		g = occ
		if why == "" {
			why = "runway occupancy"
		}
	}
	return g, nm, why
}

// SetConditions sets the weather on final the spacing follows
// (ConditionsFrom); until set, calm and good visibility on a dry runway.
func (s *ApproachSequencer) SetConditions(c ApproachConditions) {
	s.mu.Lock()
	s.cond = c
	s.mu.Unlock()
}

// Conditions are the conditions in use.
func (s *ApproachSequencer) Conditions() ApproachConditions {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cond
}

// Update sequences the arrivals at now and returns the sequence. Fixed
// arrivals keep their predicted time; the others, in order of their
// predicted times, take the earliest landing time that keeps the gap to
// the one before and the one after.
func (s *ApproachSequencer) Update(now time.Time, arrivals []ApproachAircraft) []SequenceEntry {
	c := s.Conditions()
	type slot struct {
		a   ApproachAircraft
		eta time.Time
		at  time.Time
		key time.Time // the order: first come, first served
	}
	var fixed, free []slot
	for _, a := range arrivals {
		sl := slot{a: a, eta: s.eta(now, a, c)}
		if a.Fixed || a.DistanceToGoNM < s.opts.FreezeNM {
			sl.a.Fixed, sl.at = true, sl.eta
			fixed = append(fixed, sl)
		} else {
			free = append(free, sl)
		}
	}
	byETA := func(l []slot) {
		sort.SliceStable(l, func(i, j int) bool {
			if !l[i].eta.Equal(l[j].eta) {
				return l[i].eta.Before(l[j].eta)
			}
			return l[i].a.Callsign < l[j].a.Callsign
		})
	}
	byETA(fixed)
	byETA(free)
	// First come, first served by the unconstrained time: each arrival
	// keeps the prediction it had when it joined the sequence, so losing a
	// delay (slower, longer, holding) never costs it its place to a
	// newcomer. Its key is the earlier of that and its prediction now (a
	// shortcut still moves it up), a newcomer's its prediction; keys within
	// SwapMargin keep the order they had.
	s.mu.Lock()
	prev := map[string]int{}
	for cs, e := range s.last {
		prev[cs] = e.Number
	}
	for i, f := range free {
		first, ok := s.first[f.a.Callsign]
		if !ok || f.eta.Before(first) {
			first = f.eta
		}
		if !ok {
			s.first[f.a.Callsign] = f.eta
		}
		free[i].key = first
	}
	s.mu.Unlock()
	sort.SliceStable(free, func(i, j int) bool {
		pi, oki := prev[free[i].a.Callsign]
		pj, okj := prev[free[j].a.Callsign]
		if oki && okj && absDuration(free[i].key.Sub(free[j].key)) <= s.opts.SwapMargin {
			return pi < pj
		}
		return free[i].key.Before(free[j].key)
	})
	planned := fixed // sorted by landing time
	var lastFree *slot
	for k := range free {
		f := free[k]
		at := f.eta
		// Never before the one ahead of it in the order.
		if lastFree != nil {
			if g, _, _ := s.gap(lastFree.a, f.a, c); at.Before(lastFree.at.Add(g)) {
				at = lastFree.at.Add(g)
			}
		}
		for {
			moved := false
			for _, p := range planned {
				// Before p: f must land a gap before it; after: a gap after.
				gapAfter, _, _ := s.gap(p.a, f.a, c)
				gapBefore, _, _ := s.gap(f.a, p.a, c)
				if at.Before(p.at.Add(gapAfter)) && at.After(p.at.Add(-gapBefore)) {
					at = p.at.Add(gapAfter) // behind p
					moved = true
				}
			}
			if !moved {
				break
			}
		}
		f.at = at
		free[k].at = at
		lastFree = &free[k]
		planned = append(planned, f)
		sort.SliceStable(planned, func(i, j int) bool { return planned[i].at.Before(planned[j].at) })
	}
	out := make([]SequenceEntry, len(planned))
	for i, p := range planned {
		e := SequenceEntry{Callsign: p.a.Callsign, Number: i + 1, Wake: p.a.Wake, ETA: p.eta, Landing: p.at,
			Delay: p.at.Sub(p.eta), Fixed: p.a.Fixed, DistanceToGoNM: p.a.DistanceToGoNM}
		if i > 0 {
			e.Leader = planned[i-1].a.Callsign
			_, e.SpacingNM, e.SpacingWhy = s.gap(planned[i-1].a, p.a, c)
		}
		out[i] = e
	}
	s.report(out)
	return out
}

// report remembers the sequence and reports what changed.
func (s *ApproachSequencer) report(seq []SequenceEntry) {
	s.mu.Lock()
	var changes []SequenceChange
	now := map[string]bool{}
	for _, e := range seq {
		now[e.Callsign] = true
		prev, had := s.last[e.Callsign]
		switch {
		case !had:
			changes = append(changes, SequenceChange{Runway: s.runway, Entry: e})
		case prev.Number != e.Number || absDuration(prev.Delay-e.Delay) >= s.opts.DelayStep:
			changes = append(changes, SequenceChange{Runway: s.runway, Entry: e, Previous: prev.Number})
		default:
			continue // unchanged: keep the reported values
		}
		s.last[e.Callsign] = e
	}
	for cs, e := range s.last {
		if !now[cs] {
			delete(s.last, cs)
			delete(s.first, cs)
			changes = append(changes, SequenceChange{Runway: s.runway, Entry: e, Previous: e.Number, Gone: true})
		}
	}
	s.seq = seq
	on := s.opts.OnChange
	s.mu.Unlock()
	if on != nil {
		for _, c := range changes {
			on(c)
		}
	}
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// DistanceToGo is the track distance in NM from pos along route (the
// points still to fly, in order) to the threshold: from pos to the point
// of the route it is heading for — the one after the leg it is nearest —
// then on along the route and to the threshold.
func DistanceToGo(pos airport.LatLon, route []airport.LatLon, threshold airport.LatLon) float64 {
	nm := func(a, b airport.LatLon) float64 { return calc.HaversineNM(a.Lat, a.Lon, b.Lat, b.Lon) }
	pts := append(append([]airport.LatLon(nil), route...), threshold)
	// The leg it is on: the one it is least off (the detour via pos is
	// shortest); before the first leg, it flies to the first point.
	next, best := 0, math.Inf(1)
	for i := 0; i+1 < len(pts); i++ {
		excess := nm(pts[i], pos) + nm(pos, pts[i+1]) - nm(pts[i], pts[i+1])
		if excess < best {
			best, next = excess, i+1
		}
	}
	if next == 1 && nm(pos, pts[1]) > nm(pts[0], pts[1]) {
		next = 0 // not at the route yet: to its first point
	}
	d := nm(pos, pts[next])
	for i := next; i+1 < len(pts); i++ {
		d += nm(pts[i], pts[i+1])
	}
	return d
}
