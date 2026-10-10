package traffic

import (
	"maps"
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
	// Runway is the runway end it lands on: "" the sequencer's own. On
	// dependent parallel approaches the arrivals of the adjacent final are
	// given too (Fixed, with their runway): only DiagonalNM is kept to them.
	Runway string
	// Fixes are the named fixes ahead on its route, in order, with the
	// track distance to each: arrivals sharing one are put in trail there
	// (MergeSpacingNM), not only on the final.
	Fixes []FixAhead
}

// FixAhead is a named fix on an arrival's route and its track distance.
type FixAhead struct {
	Name string
	NM   float64
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
	// MinimumNM is the spacing it must not land closer than: SpacingNM
	// without the compression buffer (worked off with speed; live, RYR270
	// sent around 6.2 NM behind a PC-12 for the 7 NM with its buffer).
	MinimumNM float64 `json:"minimumNM,omitempty"`
	// ETA is when it would land flying on as it is; Landing when it lands
	// in the sequence; Delay the difference it must absorb.
	ETA     time.Time     `json:"eta"`
	Landing time.Time     `json:"landing"`
	Delay   time.Duration `json:"delay"`
	Fixed   bool          `json:"fixed,omitempty"`
	// ShortBy is how much sooner than its spacing it would land behind its
	// leader: a fixed arrival (established, inside FreezeNM) keeps its
	// predicted time, so two of them closing up on the final show here
	// before they meet (0: spaced).
	ShortBy time.Duration `json:"shortBy,omitempty"`
	// DistanceToGoNM as given.
	DistanceToGoNM float64 `json:"distanceToGoNM"`
	// Runway: as given (ApproachAircraft.Runway); not "" for an arrival on
	// the adjacent final of dependent parallel approaches.
	Runway string `json:"runway,omitempty"`
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
	// TacticalSwapGain: two arrivals not yet fixed swap when that cuts
	// their delay by this or more (default 60 s; negative: never), and
	// TacticalSwapMaxCost is the most the one moved back may lose (3 min).
	TacticalSwapGain    time.Duration
	TacticalSwapMaxCost time.Duration
	// TacticalSwapHold: an arrival swapped is not swapped again for this
	// long (default 3 min): the one moved back is given its delay, and its
	// new prediction must not swap it straight back (live, LKPR: RYR730,
	// CSA1119 and CSA1009 traded places every few seconds).
	TacticalSwapHold time.Duration
	// CompressionMaxNM caps the compression buffer behind a slower leader
	// (0: DefaultCompressionMaxNM, 2 NM; negative: none).
	CompressionMaxNM float64
	// MinSpacingNM is the least spacing on final whatever the wake (0: the
	// minimum radar separation, 3 NM); a unit may keep more, e.g. 5 NM.
	MinSpacingNM float64
	// AllowReduced uses the reduced radar separation (2.5 NM) where the
	// conditions allow it (and the airport is approved for it).
	AllowReduced bool
	// TimeBased keeps the spacing's time instead of its distance: in a
	// headwind the distance shrinks (time-based separation, TBS); the
	// default keeps the distance, which takes longer to fly into the wind.
	TimeBased bool
	// DiagonalNM is the spacing to an arrival on the adjacent final of
	// dependent parallel approaches (0: 2 NM, AN-Conf/11-IP/3 2.3.2.2 b).
	DiagonalNM float64
	// DepartureGapNM is the spacing on final that lets one departure go
	// between two arrivals on the same runway (mixed mode; 0: 6 NM). With
	// SetDepartureSlots, that many gaps open in front of the next arrivals
	// not yet established.
	DepartureGapNM float64
	// MergeSpacingNM: two arrivals whose routes merge at a fix outside the
	// final pass it in trail, the second at least this far behind the
	// first (0: DefaultMergeSpacingNM; negative: landings only). Live,
	// LKPR: THY319 and TVS979 from two STARs met at PR574 at 0.7 NM,
	// spaced for landing only.
	MergeSpacingNM float64
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
	// first is each arrival's prediction when it joined the sequence; keys
	// its place in the order at the last Update, and manual the place a
	// controller gave it (Move), which it keeps.
	first  map[string]time.Time
	keys   map[string]time.Time
	manual map[string]time.Time
	// behind: arrivals told to follow another (Behind), by call sign, the
	// one they follow.
	behind map[string]string
	// swappedAt: when each arrival last changed places in a tactical swap.
	swappedAt map[string]time.Time
	cond      ApproachConditions
	// depSlots: departures waiting for the runway, each to get a gap.
	depSlots int
}

// SetDepartureSlots asks for n departure gaps (DepartureGapNM) in front of
// the next arrivals not yet established: the departures waiting at the
// runway (holding short, lining up, lined up) go between them. 0: none.
func (s *ApproachSequencer) SetDepartureSlots(n int) {
	s.mu.Lock()
	s.depSlots = max(0, n)
	s.mu.Unlock()
}

// departureGap is the time a departure gap takes on final between lead
// and follow, and its spacing: DepartureGapNM, and at least what the tower
// needs (RunwayController): the leader off the runway, then the follower
// still DepartureGapArrivalNM out as the departure rolls.
func (s *ApproachSequencer) departureGap(lead, follow ApproachAircraft, c ApproachConditions, departures int) (time.Duration, float64) {
	nm := s.opts.DepartureGapNM
	if nm == 0 {
		nm = DefaultDepartureGapNM
	}
	nm += float64(max(departures, 1)-1) * DoubleGapExtraNM // each departure more in the gap
	kts := c.FinalGroundKts(follow.FinalKts)
	g := SeparationTime(nm, kts)
	if need := RunwayOccupancyIn(lead.Wake, true, c.Surface) + SeparationTime(DepartureGapArrivalNM, kts); need > g {
		g = need
		nm = g.Hours() * kts
	}
	return g, math.Round(nm*10) / 10
}

const (
	// DefaultDepartureGapNM is the least gap on final for one departure
	// in mixed mode.
	DefaultDepartureGapNM = 6.0
	// DepartureGapArrivalNM is how far out the next arrival still is as
	// the departure in the gap starts its roll: the tower's MinArrivalNM
	// (4 NM by default) and half a mile to spare.
	DepartureGapArrivalNM = 4.5
	// DoubleGapQueue: with this many departures waiting for a runway, its
	// gaps fit two departures each (a double gap): DoubleGapExtraNM more
	// for the second, about two minutes of final, the departure interval
	// on one route. Two departures then cost 10.5 NM of arrival spacing,
	// not 12, and a long queue drains while the arrivals keep coming.
	DoubleGapQueue   = 3
	DoubleGapExtraNM = 4.5
)

// NewApproachSequencer creates the sequencer of a runway end ("24").
func NewApproachSequencer(runway string, opts SequencerOptions) *ApproachSequencer {
	if opts.FreezeNM == 0 {
		opts.FreezeNM = 8
	}
	if opts.FinalNM == 0 {
		opts.FinalNM = 10
	}
	if opts.MergeSpacingNM == 0 {
		opts.MergeSpacingNM = DefaultMergeSpacingNM
	}
	if opts.DelayStep == 0 {
		opts.DelayStep = 30 * time.Second
	}
	if opts.SwapMargin == 0 {
		opts.SwapMargin = 90 * time.Second
	}
	if opts.TacticalSwapGain == 0 {
		opts.TacticalSwapGain = time.Minute
	}
	if opts.TacticalSwapGain < 0 {
		opts.TacticalSwapGain = time.Duration(math.MaxInt64) // never
	}
	if opts.TacticalSwapHold == 0 {
		opts.TacticalSwapHold = 3 * time.Minute
	}
	if opts.TacticalSwapMaxCost == 0 {
		opts.TacticalSwapMaxCost = 3 * time.Minute
	}
	return &ApproachSequencer{runway: runway, opts: opts, last: map[string]SequenceEntry{}, first: map[string]time.Time{},
		keys: map[string]time.Time{}, manual: map[string]time.Time{}, behind: map[string]string{}, swappedAt: map[string]time.Time{}}
}

// Rejoin puts an arrival back into the sequence afresh, by its prediction
// from now on — after a go-around it is sequenced again like a newcomer
// instead of keeping the place its first approach had (#394).
func (s *ApproachSequencer) Rejoin(callsign string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.first, callsign)
	delete(s.last, callsign)
	delete(s.manual, callsign)
	delete(s.behind, callsign)
}

// Behind keeps callsign landing after lead, a spacing behind it, close in
// or not, as the tower told it ("number 2, follow …"): ordered by their
// predicted times alone, a VFR arrival turning in from a short circuit was
// put in front of the jet it had been told to follow, and cleared to land
// (live, OKVUV and CSA549). It holds until lead lands or callsign rejoins.
func (s *ApproachSequencer) Behind(callsign, lead string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if lead == "" || lead == callsign {
		delete(s.behind, callsign)
		return
	}
	s.behind[callsign] = lead
}

// Move moves an arrival places on in the landing order (negative: earlier)
// among those not yet established, as a controller would; it keeps its new
// place (until Rejoin). It returns ErrNotSequenced when the arrival is not
// in the sequence and ErrEstablished when it is inside FreezeNM.
func (s *ApproachSequencer) Move(callsign string, places int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var order []SequenceEntry
	at := -1
	for _, e := range s.seq {
		if e.Callsign == callsign && e.Fixed {
			return ErrEstablished
		}
		if !e.Fixed {
			if e.Callsign == callsign {
				at = len(order)
			}
			order = append(order, e)
		}
	}
	if at < 0 {
		return ErrNotSequenced
	}
	to := max(0, min(len(order)-1, at+places))
	if to == at {
		return nil
	}
	// Past the one at its new place, by more than the swap margin.
	margin := s.opts.SwapMargin + time.Second
	if to < at {
		margin = -margin
	}
	s.manual[callsign] = s.keys[order[to].Callsign].Add(margin)
	return nil
}

// Runway is the sequencer's runway end.
func (s *ApproachSequencer) Runway() string { return s.runway }

// Sequence is the last sequence, first to land first.
func (s *ApproachSequencer) Sequence() []SequenceEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]SequenceEntry(nil), s.seq...)
}

// TerminalNM before the final an arrival is predicted at most at
// TerminalKts (250 kt below 10,000 ft).
const (
	TerminalNM  = 40.0
	TerminalKts = 250.0
)

// eta predicts when an arrival lands flying on as it is.
func (s *ApproachSequencer) eta(now time.Time, a ApproachAircraft, c ApproachConditions) time.Time {
	final := c.FinalGroundKts(a.FinalKts) // on final, into the wind
	gs := math.Max(a.GroundKts, final)
	if a.GroundKts < 1 && a.DistanceToGoNM > s.opts.FinalNM {
		// No speed yet (just created or adopted: the first look reports
		// 0 kt): not predicted at the final speed all the way (live,
		// TVS440 adopted at LOMKI went from number 2 to 3, delay 8 min, and
		// back a second later).
		gs = TerminalKts
	}
	// The terminal area is flown at most at TerminalKts whatever the speed
	// now: a jet at cruise is not predicted at 460 kt down to the final
	// (live, TVS440 at FL410 took number 2 from ENT1816, told it a
	// minute before, on such a prediction).
	term := math.Max(math.Min(gs, TerminalKts), final)
	far := math.Max(0, a.DistanceToGoNM-s.opts.FinalNM-TerminalNM)
	outer := math.Max(0, math.Min(a.DistanceToGoNM-s.opts.FinalNM, TerminalNM))
	inner := math.Min(a.DistanceToGoNM, s.opts.FinalNM)
	h := far/gs + outer/term + inner/((term+final)/2)
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
	// On adjacent finals of dependent parallel approaches: the diagonal
	// spacing, and no runway occupancy (another runway).
	if lead.Runway != follow.Runway {
		nm := s.opts.DiagonalNM
		if nm == 0 {
			nm = 2
		}
		return SeparationTime(nm, c.FinalGroundKts(follow.FinalKts)), nm, "adjacent final"
	}
	nm, why := ArrivalSpacing(lead.Wake, follow.Wake, s.opts.Scheme, c, s.opts.AllowReduced && s.opts.MinSpacingNM == 0)
	if s.opts.MinSpacingNM > nm {
		nm, why = s.opts.MinSpacingNM, ""
	}
	// Compression: a follower faster on final than its leader closes on it
	// all the way down, and any error in either prediction comes off the
	// spacing; it is given CompressionNMPer30Kts per 30 kt of difference,
	// at most CompressionMaxNM (live, LKPR: a B738 behind a PC-24 at 108 kt
	// was 31 s short on the final and went around).
	if diff := finalKtsOf(follow) - finalKtsOf(lead); diff > 0 && s.opts.CompressionMaxNM >= 0 {
		limit := s.opts.CompressionMaxNM
		if limit == 0 {
			limit = DefaultCompressionMaxNM
		}
		if extra := math.Min(limit, diff/30*CompressionNMPer30Kts); extra >= 0.1 {
			nm += extra
			if why == "" {
				why = "compression"
			}
		}
	}
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

// minimumNM is follow's spacing behind lead without the compression
// buffer: the diagonal on adjacent finals, else the wake or radar minimum
// in the conditions (at least MinSpacingNM).
func (s *ApproachSequencer) minimumNM(lead, follow ApproachAircraft, c ApproachConditions) float64 {
	if lead.Runway != follow.Runway {
		if s.opts.DiagonalNM > 0 {
			return s.opts.DiagonalNM
		}
		return 2
	}
	nm, _ := ArrivalSpacing(lead.Wake, follow.Wake, s.opts.Scheme, c, s.opts.AllowReduced && s.opts.MinSpacingNM == 0)
	return math.Max(nm, s.opts.MinSpacingNM)
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
	// Fixed arrivals keep the order they had: one established on the final
	// is not passed by another fixed later, closer in by its prediction
	// (live, LKPR: a DA62 from SIERRA on a short base, number 2, took number
	// 1 from AUA529 on a 5 NM final, which was sent around). The one behind
	// shows the spacing it lacks (ShortBy) instead.
	s.mu.Lock()
	had := map[string]int{}
	for cs, e := range s.last {
		had[cs] = e.Number
	}
	s.mu.Unlock()
	for range len(fixed) {
		changed := false
		for i := range fixed {
			for j := range fixed {
				pi, oki := had[fixed[i].a.Callsign]
				pj, okj := had[fixed[j].a.Callsign]
				// i was ahead of j: j lands after it, however close in.
				if oki && okj && pi < pj && !fixed[j].at.After(fixed[i].at) {
					fixed[j].at, changed = fixed[i].at.Add(time.Second), true
				}
			}
		}
		if !changed {
			break
		}
	}
	byLanding := func(l []slot) {
		sort.SliceStable(l, func(i, j int) bool { return l[i].at.Before(l[j].at) })
	}
	byLanding(fixed)
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
		if m, ok := s.manual[f.a.Callsign]; ok {
			first = m // where a controller put it
		}
		free[i].key = first
		s.keys[f.a.Callsign] = first
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
	// A newcomer on a route merging with one already sequenced goes ahead
	// of it only when it passes their merge fix a full spacing ahead;
	// level with it or behind there, it goes behind, whatever its time to
	// the runway (live, OKGOZ on GOLO4S slotted ahead of EZY131 on
	// LOMK8S, side by side at FL100: they met at 0.4 NM).
	for range len(free) {
		changed := false
		for i := 0; i+1 < len(free); i++ {
			n, e := &free[i], &free[i+1]
			// n new, or moving up past e (behind it the last time: OKGOZ
			// was number 5 behind EZY131 its first second, number 4 the
			// next).
			pn, nw := prev[n.a.Callsign]
			pe, ew := prev[e.a.Callsign]
			s.mu.Lock()
			_, placed := s.manual[n.a.Callsign] // a controller put it there
			s.mu.Unlock()
			if !ew || nw && pn < pe || placed || n.a.Runway != e.a.Runway {
				continue
			}
			// Behind it at their merge fix, or ahead only by costing it more
			// than a tactical swap may (live, OKUFC, a DA62 joining 11 NM
			// out, moved up past TVS220 and TVS1568: minutes for each).
			tn, te, merges := firstMerge(n.a, e.a, s.opts.FinalNM)
			atMerge := merges && tn+SeparationTime(s.opts.MergeSpacingNM, mergeSpeed(n.a)) > te
			g, _, _ := s.gap(n.a, e.a, c)
			costly := n.eta.Add(g).Sub(e.eta) > s.opts.TacticalSwapMaxCost
			if !atMerge && !costly {
				continue
			}
			n.key = e.key.Add(time.Second)
			s.mu.Lock()
			s.keys[n.a.Callsign], s.first[n.a.Callsign] = n.key, n.key
			s.mu.Unlock()
			free[i], free[i+1] = free[i+1], free[i]
			changed = true
		}
		if !changed {
			break
		}
	}
	// Tactical swaps: two arrivals not yet fixed change places when that
	// cuts their delay by TacticalSwapGain or more and costs the one moved
	// back no more than TacticalSwapMaxCost; their keys change too, so the
	// next look keeps the new order (live, LKPR: OKYDV could land before
	// TVS223 turning base, which had room to extend). Not for an arrival a
	// controller placed (Move) or told to follow another (Behind), nor a
	// newcomer: it joins at the back first (first come, first served).
	s.mu.Lock()
	for i := 0; i+1 < len(free); i++ {
		a, b := &free[i], &free[i+1]
		_, am := s.manual[a.a.Callsign]
		_, bm := s.manual[b.a.Callsign]
		_, ab := s.behind[a.a.Callsign]
		_, bb := s.behind[b.a.Callsign]
		_, aw := prev[a.a.Callsign]
		_, bw := prev[b.a.Callsign]
		recent := now.Sub(s.swappedAt[a.a.Callsign]) < s.opts.TacticalSwapHold || now.Sub(s.swappedAt[b.a.Callsign]) < s.opts.TacticalSwapHold
		if !aw || !bw || am || bm || ab || bb || recent || !b.eta.Before(a.eta) {
			continue
		}
		// Not past one it meets at their merge fix level or behind.
		if tb, ta, ok := firstMerge(b.a, a.a, s.opts.FinalNM); ok && tb+SeparationTime(s.opts.MergeSpacingNM, mergeSpeed(b.a)) > ta {
			continue
		}
		gAB, _, _ := s.gap(a.a, b.a, c)
		gBA, _, _ := s.gap(b.a, a.a, c)
		// The pair alone: as ordered, A then B; swapped, B then A.
		delayAB := max(0, a.eta.Add(gAB).Sub(b.eta))
		delayBA := max(0, b.eta.Add(gBA).Sub(a.eta))
		if delayAB-delayBA < s.opts.TacticalSwapGain || delayBA > s.opts.TacticalSwapMaxCost {
			continue
		}
		a.key, b.key = b.key, a.key
		s.keys[a.a.Callsign], s.keys[b.a.Callsign] = a.key, b.key
		s.first[a.a.Callsign], s.first[b.a.Callsign] = a.key, b.key
		s.swappedAt[a.a.Callsign], s.swappedAt[b.a.Callsign] = now, now
		free[i], free[i+1] = free[i+1], free[i]
		i++ // a pair at a time
	}
	s.mu.Unlock()
	planned := fixed // sorted by landing time
	var lastFree *slot
	s.mu.Lock()
	slots := s.depSlots
	s.mu.Unlock()
	gapped := map[string]float64{} // arrivals behind a departure gap: its NM
	for k := range free {
		f := free[k]
		at := f.eta
		// Never before the one ahead of it in the order.
		ahead := lastFree
		if ahead == nil && len(fixed) > 0 {
			ahead = &fixed[len(fixed)-1] // the last established one
		}
		if ahead != nil {
			g, _, _ := s.gap(ahead.a, f.a, c)
			// A departure gap in front of it, while departures wait (a wide
			// enough gap already is one).
			if slots > 0 && ahead.a.Runway == f.a.Runway {
				// A queue at the runway: two departures in one wider gap (a
				// double gap), less room off the arrivals than two single ones.
				n := 1
				if slots >= DoubleGapQueue {
					n = 2
				}
				if dg, nm := s.departureGap(ahead.a, f.a, c, n); dg > g {
					g = dg
					gapped[f.a.Callsign] = nm
				}
				slots -= n
			}
			if at.Before(ahead.at.Add(g)) {
				at = ahead.at.Add(g)
			}
		}
		// In trail where its route merges with one landing before it.
		for _, p := range planned {
			if need := s.mergeDelay(p.a, p.at.Sub(p.eta), f.a); f.eta.Add(need).After(at) {
				at = f.eta.Add(need)
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
	// Told to follow another (Behind): a spacing after it, whatever the
	// predictions; dropped once the one followed is out of the sequence.
	s.mu.Lock()
	for cs, lead := range s.behind {
		in := false
		for _, p := range planned {
			in = in || p.a.Callsign == lead
		}
		if !in {
			delete(s.behind, cs)
		}
	}
	behind := maps.Clone(s.behind)
	s.mu.Unlock()
	for range len(behind) + 1 {
		changed := false
		for i := range planned {
			lead, ok := behind[planned[i].a.Callsign]
			if !ok {
				continue
			}
			for j := range planned {
				if planned[j].a.Callsign != lead {
					continue
				}
				g, _, _ := s.gap(planned[j].a, planned[i].a, c)
				if earliest := planned[j].at.Add(g); planned[i].at.Before(earliest) {
					planned[i].at, changed = earliest, true
				}
			}
		}
		if !changed {
			break
		}
		sort.SliceStable(planned, func(i, j int) bool { return planned[i].at.Before(planned[j].at) })
	}
	out := make([]SequenceEntry, len(planned))
	for i, p := range planned {
		e := SequenceEntry{Callsign: p.a.Callsign, Number: i + 1, Wake: p.a.Wake, ETA: p.eta, Landing: p.at,
			Delay: p.at.Sub(p.eta), Fixed: p.a.Fixed, DistanceToGoNM: p.a.DistanceToGoNM, Runway: p.a.Runway}
		if i > 0 {
			e.Leader = planned[i-1].a.Callsign
			var g time.Duration
			g, e.SpacingNM, e.SpacingWhy = s.gap(planned[i-1].a, p.a, c)
			e.MinimumNM = s.minimumNM(planned[i-1].a, p.a, c)
			if short := g - p.at.Sub(planned[i-1].at); short > 0 {
				e.ShortBy = short
			}
			if nm, ok := gapped[p.a.Callsign]; ok && planned[i-1].a.Callsign != "" {
				e.SpacingNM, e.SpacingWhy = nm, "departure gap"
			}
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
			delete(s.swappedAt, cs)
			delete(s.first, cs)
			// Its place moved by hand and its key go too (#97: the call sign
			// again later jumped to number 1).
			delete(s.manual, cs)
			delete(s.keys, cs)
			delete(s.behind, cs)
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

// DistanceVia is the track distance in NM from pos to each point of route
// in turn, then to the threshold: for a route that starts at the point the
// aircraft flies to, as ArrivalController.ProcedureRoute gives it. Unlike
// DistanceToGo it does not look for the leg the aircraft is on, which a
// go-around's circuit — looping back past the final — would mislead.
func DistanceVia(pos airport.LatLon, route []airport.LatLon, threshold airport.LatLon) float64 {
	d, at := 0.0, pos
	for _, p := range append(append([]airport.LatLon(nil), route...), threshold) {
		d += calc.HaversineNM(at.Lat, at.Lon, p.Lat, p.Lon)
		at = p
	}
	return d
}

// RouteAhead is the part of route (the points of a route, in order) still
// to fly from pos: from the point after the leg it is nearest (the one it
// is heading for), as DistanceToGo counts it; all of it when pos is not
// at the route yet. For ConflictOptions.Route.
func RouteAhead(pos airport.LatLon, route []airport.LatLon) []airport.LatLon {
	return route[nextOnRoute(pos, route):]
}

// ProfileAhead is RouteAhead for a route with altitudes: the points still
// to fly from pos. For ConflictOptions.Profile.
func ProfileAhead(pos airport.LatLon, route []RoutePoint) []RoutePoint {
	pts := make([]airport.LatLon, len(route))
	for i, p := range route {
		pts[i] = p.Position
	}
	return route[nextOnRoute(pos, pts):]
}

// nextOnRoute is the index of the point of pts pos is heading for.
func nextOnRoute(pos airport.LatLon, pts []airport.LatLon) int {
	nm := func(a, b airport.LatLon) float64 { return calc.HaversineNM(a.Lat, a.Lon, b.Lat, b.Lon) }
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
	return next
}

// DistanceToGo is the track distance in NM from pos along route (the
// points still to fly, in order) to the threshold: from pos to the point
// of the route it is heading for — the one after the leg it is nearest —
// then on along the route and to the threshold.
func DistanceToGo(pos airport.LatLon, route []airport.LatLon, threshold airport.LatLon) float64 {
	nm := func(a, b airport.LatLon) float64 { return calc.HaversineNM(a.Lat, a.Lon, b.Lat, b.Lon) }
	pts := append(append([]airport.LatLon(nil), route...), threshold)
	next := nextOnRoute(pos, pts)
	d := nm(pos, pts[next])
	for i := next; i+1 < len(pts); i++ {
		d += nm(pts[i], pts[i+1])
	}
	return d
}

// Compression buffer (ApproachSequencer.gap): CompressionNMPer30Kts of
// extra spacing per 30 kt a follower is faster on final than its leader,
// at most DefaultCompressionMaxNM (SequencerOptions.CompressionMaxNM).
const (
	CompressionNMPer30Kts   = 1.0
	DefaultCompressionMaxNM = 2.0
)

// DefaultMergeSpacingNM is the spacing at a merge point: the 5 NM radar
// minimum away from the terminal area (ConflictOptions.MinNM) and a mile
// for the turns onto the common route.
const DefaultMergeSpacingNM = 6.0

// mergeKts is the least speed a time to a merge point is flown at.
const mergeKts = 150.0

// mergeDelay is the delay follow must lose before the first fix its route
// shares with lead, so it passes it MergeSpacingNM behind: lead at its
// speed now, late by leadDelay (absorbed on the way). A fix on the final
// is the final's spacing, and one follow reaches first plainly ahead is
// no merge it trails in.
func (s *ApproachSequencer) mergeDelay(lead ApproachAircraft, leadDelay time.Duration, follow ApproachAircraft) time.Duration {
	sep := s.opts.MergeSpacingNM
	if sep <= 0 || lead.Runway != follow.Runway {
		return 0
	}
	// Every fix they share outside the final, not only the first: a faster
	// one behind closes all along the common route (live, LKPR: RYR270 at
	// 210 kt 20 NM behind OKZWR, a PC-12 at 170, on LOMK8S).
	kl, kf := mergeSpeed(lead), mergeSpeed(follow)
	var need time.Duration
	first := true
	for _, ff := range follow.Fixes {
		if follow.DistanceToGoNM-ff.NM <= s.opts.FinalNM {
			break // on the final from here on
		}
		for _, lf := range lead.Fixes {
			if lf.Name != ff.Name {
				continue
			}
			tl := time.Duration(lf.NM/kl*float64(time.Hour)) + max(0, leadDelay)
			tf := time.Duration(ff.NM / kf * float64(time.Hour))
			if first && tf+SeparationTime(sep, kf) <= tl {
				return 0 // at the merge well before it: not in trail behind it
			}
			first = false
			need = max(need, tl+SeparationTime(sep, kl)-tf)
			break
		}
	}
	return need
}

// firstMerge is the time a and b take to the first fix their routes share
// outside the final (finalNM): false when they share none.
func firstMerge(a, b ApproachAircraft, finalNM float64) (time.Duration, time.Duration, bool) {
	ka, kb := mergeSpeed(a), mergeSpeed(b)
	for _, fa := range a.Fixes {
		if a.DistanceToGoNM-fa.NM <= finalNM {
			return 0, 0, false
		}
		for _, fb := range b.Fixes {
			if fb.Name == fa.Name && b.DistanceToGoNM-fb.NM > finalNM {
				return time.Duration(fa.NM / ka * float64(time.Hour)), time.Duration(fb.NM / kb * float64(time.Hour)), true
			}
		}
	}
	return 0, 0, false
}

// mergeSpeed is a's speed to a merge point: its ground speed now, at
// least mergeKts (TerminalKts before it reports one).
func mergeSpeed(a ApproachAircraft) float64 {
	if a.GroundKts < 1 {
		return TerminalKts
	}
	return max(mergeKts, a.GroundKts)
}

// finalKtsOf is a's approach speed on final (140 when not given).
func finalKtsOf(a ApproachAircraft) float64 {
	if a.FinalKts > 0 {
		return a.FinalKts
	}
	return 140
}
