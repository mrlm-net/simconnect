package traffic

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"sync"
	"time"
)

// The runway controller (#393) is the tower of one runway: it clears
// departures to line up and take off, and ground traffic to cross, by what
// the runway is doing — the wake and route interval after the previous
// departure, the runway free, and the next arrival far enough out that
// the departure is off the runway before it lands (mixed-mode use: the
// departures go in the gaps between arrivals). An arrival on short final
// with the runway not free is sent around (#394).

// RunwayPhase is what a user of the runway is doing.
type RunwayPhase uint8

const (
	RunwayHoldingShort RunwayPhase = iota // at the holding point: waits to line up (a departure) or to cross
	RunwayLinedUp                         // on the runway, waiting for the take-off clearance
	RunwayRolling                         // take-off roll, landing roll, or crossing: on the runway
	RunwayAirborne                        // just departed
	RunwayFinal                           // arriving, not yet on the runway
)

// RunwayUser is an aircraft using, or about to use, the runway.
type RunwayUser struct {
	Callsign string
	Wake     Wake
	Phase    RunwayPhase
	// Departure: its SID or route (the same one waits longer); Crossing: it
	// only crosses; Arrival: landing (on final, or on its landing roll).
	Route    string
	Crossing bool
	Arrival  bool
	// An arrival on final: distance to the threshold and ground speed;
	// Established on the final approach (not still on its STAR or downwind:
	// only then is it cleared to land, #486).
	DistanceNM, GroundKts float64
	Established           bool
	// ClimbKts: a departure's climb speed (TAS), for the interval and the
	// order behind a slower one on the same route; 0 unknown.
	ClimbKts float64
	// AlongM: lined up, how far down the runway it stands from its
	// take-off threshold (an intersection's way in); 0 at the threshold. A
	// take-off waits for no one lined up behind it.
	AlongM float64
	// End: the runway end it lands on or takes off from ("24"); "" unknown
	// (a crossing, other traffic). Traffic using the other end of the
	// runway holds a take-off or a landing toward it (see Decide).
	End string
	// GoingAround: an arrival climbing out ahead of the runway after its
	// go-around (RunwayAirborne): no take-off below it (live, TVS648
	// released behind EZY1672 going around on 06, 4.1 NM and 996 ft).
	GoingAround bool
	// Other traffic: counted, never cleared.
	Other bool
	// Host: the host's aircraft (the user's, cleared by the host's own
	// ATC, #739): holding short, it takes its place in the queue, first
	// come, and those behind it wait for it; it is never cleared here.
	Host bool
	// LineUpCleared: holding short, told to line up already and not
	// moving yet. It goes first: no other departure is lined up past it.
	LineUpCleared bool
	// LiningUp: RunwayLinedUp but still turning onto the runway. Not "the
	// departing" yet for a conditional line-up behind it (offline LKPR:
	// two lining up at once, a DA-62 at B taking 80 s).
	LiningUp bool
	// LineUpIn: a departure holding short or lining up, how long it still
	// takes to line up (TaxiController.LineUpLeft); 0 not known: LineUpTime
	// from the holding point, half of it lining up.
	LineUpIn time.Duration
}

// OppositeArrivalNM: an arrival to the other end of the runway this close
// holds a take-off toward it.
const OppositeArrivalNM = 12.0

// RunwayClearances are the controller's decisions this time.
type RunwayClearances struct {
	LineUp  []string `json:"lineUp,omitempty"`  // line up and wait
	Takeoff []string `json:"takeoff,omitempty"` // cleared for take-off (lined up, or holding short: a rolling take-off)
	Cross   []string `json:"cross,omitempty"`   // cross the runway
	// GoAround: arrivals on short final with the runway not free (the
	// reason is in Waiting).
	GoAround []string `json:"goAround,omitempty"`
	// Land: the next arrival, within ClearToLandNM with nothing in the way
	// on the runway: cleared to land (Doc 4444 12.3.4.16).
	Land []string `json:"land,omitempty"`
	// LineUpBehind: the first departure at the holding points waiting only
	// for the next arrival, by call sign, and that arrival: line up and
	// wait behind it once it has passed (a conditional line-up).
	LineUpBehind map[string]string `json:"lineUpBehind,omitempty"`
	// CrossBehind: aircraft holding short of a crossing waiting only for
	// the next arrival (the runway otherwise free): a conditional crossing
	// behind it ("behind the landing A320, cross runway 12, behind"), by
	// crossing aircraft, the arrival's call sign.
	CrossBehind map[string]string `json:"crossBehind,omitempty"`
	// LineUpBehindDeparting: when the runway is busy (RushQueue departures
	// waiting, or an arrival within RushArrivalWithin), the first departure
	// at the holding points behind ours on its take-off roll, by call sign,
	// and that departure: "behind the departing A320, line up and wait",
	// lined up as it rolls, not once it is airborne.
	LineUpBehindDeparting map[string]string `json:"lineUpBehindDeparting,omitempty"`
	// LineUpAhead: those in LineUpBehindDeparting whose departure is down
	// the runway past their way in already (an intersection departure ahead
	// of them). It never passes in front of them, so no condition: "line up
	// and wait" (Doc 4444 12.2.7; live, TVS737 at the threshold told
	// "behind the departing Cirrus SR22" rolling from B).
	LineUpAhead map[string]bool `json:"lineUpAhead,omitempty"`
	// NoDelay: departures in Takeoff with the next arrival established
	// within NoDelayNM, by call sign, that arrival's distance (NM): "cleared
	// for take-off, no delay, traffic on 5 mile final".
	NoDelay map[string]float64 `json:"noDelay,omitempty"`
	// NextArrival is the next arrival to land ("" none).
	NextArrival string `json:"nextArrival,omitempty"`
	// Why each departure or crossing still waits.
	Waiting map[string]string `json:"waiting,omitempty"`
}

// RunwayControllerOptions tune a RunwayController.
type RunwayControllerOptions struct {
	// MinArrivalNM: a departure takes off only with the next arrival
	// farther out than this (default 4 NM), and at least its runway
	// occupancy plus Margin away in time.
	MinArrivalNM float64
	Margin       time.Duration // default 30 s
	// CrossTime: how long a crossing takes (default 40 s); a crossing needs
	// the next arrival that long plus Margin away.
	CrossTime time.Duration
	// Surface: the runway state (longer occupancy wet or contaminated).
	Surface RunwaySurface
	// GoAroundAt: an arrival this long before the threshold with the
	// runway not free goes around (default 30 s, about 1.2 NM at 140 kt).
	// A departure rolling is not in the way: it is airborne before then.
	GoAroundAt time.Duration
	// ClearToLandNM: the next arrival is cleared to land within this of
	// the threshold, once nothing is in the way (default 6 NM).
	ClearToLandNM float64
	// LineUpTime: how long a departure at a holding point takes to line up
	// (default 60 s); cleared to line up and take off in one, it needs the
	// next arrival that much farther away.
	LineUpTime time.Duration
	// LineUpWaitTime: how long lining up to a stop takes (line up and wait,
	// a conditional line-up; default 85 s, measured live): a departure lined
	// up behind a landing aircraft goes only once that is done.
	LineUpWaitTime time.Duration
	// MinArrivalLinedUpNM: a departure already lined up takes off with the
	// next arrival farther out than this (default 3 NM) and its time rule
	// (its roll and Margin) kept; one still to line up needs MinArrivalNM.
	MinArrivalLinedUpNM float64
	// NoDelayNM: a take-off with the next arrival established within this
	// of the threshold is told so (RunwayClearances.NoDelay; default 8 NM,
	// about 3.5 minutes at 140 kt).
	NoDelayNM float64
	// LandingClearanceNM: a departure or crossing ahead of an arrival
	// leaves the runway with the arrival still this far out (default 2 NM;
	// the user, live: OKJTX, a DA42 cleared "no delay" with CSA1312 on a 5
	// mile final, airborne with it on short final, a conflict alert). It was
	// 3 NM: the gaps on final that asked for, 10 NM before nearly every
	// arrival, kept the arrivals 15-25 min late (offline LKPR, the user).
	LandingClearanceNM float64
}

// landMargin is how long before arrival a lands the runway must be free
// again: Margin, at least the time it flies LandingClearanceNM (at
// landingNominalKts when its speed is not known).
func (r *RunwayController) landMargin(a *RunwayUser) time.Duration {
	kts := landingNominalKts
	if a != nil && a.GroundKts > 0 {
		kts = max(a.GroundKts, 100)
	}
	return max(r.opts.Margin, SeparationTime(r.opts.LandingClearanceNM, kts))
}

// landingNominalKts: an arrival's speed on final when its own is unknown.
const landingNominalKts = 140.0

// lineUpGoSlack: a take-off with its line-up still to come is given this
// much more: the spread of the line-up and the roll from it (offline LKPR:
// TVS837 with the arrival 7 NM out lined up in 64 s and was airborne 56 s
// later, against 60 and 45). One lined up already needs none, nor one
// whose own line-up time is known (RunwayUser.LineUpIn).
const lineUpGoSlack = 20 * time.Second

// takeoffSaidIn: a take-off clearance ahead of an arrival is said some 15 s
// after it is decided (the tower's answer and the frequency; offline LKPR:
// CSA493 decided with TVS1843 8.6 NM out, said at 8.0). 25 s, with the
// landing clearance's few seconds, held the runway idle and cost a
// fifth of the departures.
const takeoffSaidIn = 15 * time.Second

// liningUpLeft: one still lining up is given half the line-up time still
// to go before its roll (live, OKJTX at B cleared for take-off 36 s into
// an 80 s line-up, counted as lined up).
func (r *RunwayController) liningUpLeft(u RunwayUser) time.Duration {
	if u.LiningUp {
		if u.LineUpIn > 0 {
			return u.LineUpIn
		}
		return r.opts.LineUpTime / 2
	}
	return 0
}

// lineUpFrom: one at its holding point takes its own line-up time
// (LineUpIn) if known and longer, else LineUpTime.
func (r *RunwayController) lineUpFrom(u RunwayUser) time.Duration {
	return max(u.LineUpIn, r.opts.LineUpTime)
}

// RunwayController clears the users of one runway.
type RunwayController struct {
	opts RunwayControllerOptions

	mu      sync.Mutex
	lastDep *RunwayUser // the last departure that started its roll
	lastAt  time.Time
	queue   map[string]time.Time // when each departure started waiting
	// granted: departures lined up and cleared for take-off at the last
	// decision; kept down to grantedArrivalNM.
	granted map[string]bool
	// wayIn: each departure's way in, how far down the runway (m), from its
	// holding point; kept through its roll (LineUpAhead).
	wayIn map[string]float64
}

// RollPastIn is how long a departure on its take-off roll at kts takes to
// cover m more meters, accelerating at RollAccelMS2 from its speed now; 0
// for none left.
func RollPastIn(m, kts float64) time.Duration {
	if m <= 0 {
		return 0
	}
	v := kts * 1852 / 3600
	t := (math.Sqrt(v*v+2*RollAccelMS2*m) - v) / RollAccelMS2
	return time.Duration(t * float64(time.Second))
}

// RollAccelMS2: a take-off roll's mean acceleration (m/s²), about 0.2 g.
const RollAccelMS2 = 2.0

// wayInApartM: two ways in this far apart down the runway or more are
// different ones (an intersection ahead of the threshold).
const wayInApartM = 150.0

// grantedArrivalNM: a take-off already granted to one lined up stands with
// the next arrival this close, the time rule (its roll and the margin)
// kept. Withdrawn at MinArrivalLinedUpNM while the frequency was busy, it
// left the departure on the runway (live, LKPR: AUA511 granted, the call
// not yet said, KLM1683 came inside 3 NM and went around).
const grantedArrivalNM = 2.0

// grantedSlack: a take-off granted already keeps its time rule this much
// short, so a decision a second later does not withdraw it.
const grantedSlack = 15 * time.Second

// behindRoom reports whether a departure u lining up behind the next
// arrival (landing in next seconds) can take off before the one after it
// (then seconds): the first off the runway, the departure's roll and the
// margin, all before the second lands.
func (r *RunwayController) behindRoom(next float64, first RunwayUser, then float64, u RunwayUser) bool {
	// Lined up once the first is off the runway and its own line-up is done
	// (live, TVS158 took 84 s and QTR1788 behind had to go around).
	free := max(RunwayOccupancyIn(first.Wake, true, r.opts.Surface), r.opts.LineUpWaitTime)
	need := next + (free + takeoffSaidIn + RunwayOccupancyIn(u.Wake, false, r.opts.Surface) + r.landMargin(nil)).Seconds()
	return then >= need
}

// BehindRoom is behindRoom for the map: whether departure u, lined up
// behind arrival first now landing, still has time before the next
// arrival (landing in then).
func (r *RunwayController) BehindRoom(first RunwayUser, then time.Duration, u RunwayUser) bool {
	return r.behindRoom(0, first, then.Seconds(), u)
}

// LineUpRoom is for a departure u lined up behind one departing: whether
// it still has time to line up and go before the next arrival (landing in
// then). The same room it was given its conditional line-up with: its
// line-up, its time on the runway and the margin; the one departing ahead
// is not landing, so no landing roll of it to wait out (live, KLM1616
// cleared behind the departing 737, then told to hold position).
func (r *RunwayController) LineUpRoom(then time.Duration, u RunwayUser) bool {
	return then >= takeoffSaidIn+r.opts.LineUpTime+RunwayOccupancyIn(u.Wake, false, r.opts.Surface)+r.landMargin(nil)
}

// NewRunwayController creates the controller of one runway.
func NewRunwayController(opts RunwayControllerOptions) *RunwayController {
	if opts.MinArrivalNM == 0 {
		opts.MinArrivalNM = 4
	}
	if opts.Margin == 0 {
		opts.Margin = 30 * time.Second
	}
	if opts.CrossTime == 0 {
		opts.CrossTime = 40 * time.Second
	}
	if opts.GoAroundAt == 0 {
		opts.GoAroundAt = 30 * time.Second
	}
	if opts.ClearToLandNM == 0 {
		opts.ClearToLandNM = 6
	}
	if opts.LineUpTime == 0 {
		opts.LineUpTime = 60 * time.Second
	}
	if opts.LineUpWaitTime == 0 {
		opts.LineUpWaitTime = 85 * time.Second
	}
	if opts.MinArrivalLinedUpNM == 0 {
		opts.MinArrivalLinedUpNM = 3
	}
	if opts.NoDelayNM == 0 {
		opts.NoDelayNM = 8
	}
	if opts.LandingClearanceNM == 0 {
		opts.LandingClearanceNM = 2
	}
	return &RunwayController{opts: opts, queue: map[string]time.Time{}}
}

// SetSurface changes the runway state.
func (r *RunwayController) SetSurface(s RunwaySurface) {
	r.mu.Lock()
	r.opts.Surface = s
	r.mu.Unlock()
}

// Decide gives this time's clearances for the users at now.
func (r *RunwayController) Decide(now time.Time, users []RunwayUser) RunwayClearances {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := RunwayClearances{Waiting: map[string]string{}, LineUpBehind: map[string]string{}, CrossBehind: map[string]string{}, LineUpBehindDeparting: map[string]string{}, LineUpAhead: map[string]bool{}, NoDelay: map[string]float64{}}
	prevGranted := r.granted
	r.granted = map[string]bool{}

	// What the runway is doing: who is on it, who of ours is lined up, when
	// the next arrival lands, who waits at the holding points.
	// occupied: who is on the runway (the first seen); onRunway: all of
	// them — a take-off waits for every one but itself.
	occupied, linedUp := "", ""
	var ourLinedUp []RunwayUser // ours lined up (one, unless an intersection's)
	along := map[string]float64{}
	rollingDep := "" // ours on its take-off roll
	rollingKts := 0.0
	var onRunway []string
	nextArr, nextArrName := math.Inf(1), ""
	var nextArrUser RunwayUser
	// thenArr: when the arrival after the next lands (seconds).
	thenArr := math.Inf(1)
	var holding []RunwayUser
	ourDep := map[string]bool{} // ours departing: lined up, or holding short to go
	seen := map[string]bool{}
	for _, u := range users {
		seen[u.Callsign] = true
		switch u.Phase {
		case RunwayRolling:
			occupied = u.Callsign
			onRunway = append(onRunway, u.Callsign)
			if !u.Arrival && !u.Crossing && !u.Other && !u.Host {
				rollingDep, rollingKts = u.Callsign, u.GroundKts
				along[u.Callsign] = u.AlongM // how far down the runway it rolls now
			}
			if !u.Arrival && !u.Crossing && (r.lastDep == nil || r.lastDep.Callsign != u.Callsign) {
				uu := u // a departure started its roll: the interval runs from here
				r.lastDep, r.lastAt = &uu, now
			}
		case RunwayLinedUp:
			occupied = u.Callsign
			onRunway = append(onRunway, u.Callsign)
			along[u.Callsign] = u.AlongM
			if !u.Arrival && !u.Crossing && !u.Other && !u.Host {
				ourDep[u.Callsign] = true
			}
			if !u.Other && !u.Host {
				linedUp = u.Callsign
				ourLinedUp = append(ourLinedUp, u)
			}
		case RunwayFinal:
			t := u.DistanceNM / math.Max(u.GroundKts, 100) * 3600
			if t < nextArr {
				thenArr = nextArr
				nextArr, nextArrName, nextArrUser = t, u.Callsign, u
			} else if t < thenArr {
				thenArr = t
			}
		case RunwayHoldingShort:
			if !u.Other {
				if !u.Crossing && !u.Host {
					along[u.Callsign], ourDep[u.Callsign] = u.AlongM, true // its way in
					if r.wayIn == nil {
						r.wayIn = map[string]float64{}
					}
					r.wayIn[u.Callsign] = u.AlongM
				}
				holding = append(holding, u)
				if _, ok := r.queue[u.Callsign]; !ok {
					r.queue[u.Callsign] = now
				}
			}
		}
	}
	for cs := range r.queue {
		if !seen[cs] {
			delete(r.queue, cs)
		}
	}
	for cs := range r.wayIn {
		if !seen[cs] {
			delete(r.wayIn, cs)
		}
	}
	// The next arrival lands after occ and the margin, and is not inside
	// MinArrivalNM.
	arrivalClear := func(occ time.Duration, minNM float64) string {
		for _, u := range users {
			if u.Phase == RunwayFinal && u.DistanceNM < minNM {
				return fmt.Sprintf("%s on a %.1f NM final", u.Callsign, u.DistanceNM)
			}
		}
		if nextArr*float64(time.Second) < float64(occ+r.landMargin(&nextArrUser)) {
			return fmt.Sprintf("%s lands in %s", nextArrName, time.Duration(nextArr*float64(time.Second)).Round(time.Second))
		}
		return ""
	}
	// The interval after the last departure.
	interval := func(u RunwayUser) string {
		if r.lastDep == nil || r.lastDep.Callsign == u.Callsign {
			return ""
		}
		iv := DepartureIntervalSpeeds(r.lastDep.Wake, u.Wake, r.lastDep.Route != "" && r.lastDep.Route == u.Route, r.lastDep.ClimbKts, u.ClimbKts)
		if left := iv - now.Sub(r.lastAt); left > 0 {
			return fmt.Sprintf("%s behind %s", left.Round(time.Second), r.lastDep.Callsign)
		}
		return ""
	}
	// The other end of the runway in use at once (a runway change, its last
	// arrivals landing on the old end; live LKPR: DLH1582 cleared for
	// take-off on 24 toward PHDYS on the 06 approach, 0.6 NM). A take-off
	// waits for any arrival or departure using the other end; a landing for
	// a departure from it, or an arrival to it that is closer (the nearer
	// lands first: the two ends never hold each other).
	otherEnd := func(u, v RunwayUser) bool {
		return u.End != "" && v.End != "" && v.End != u.End && v.Callsign != u.Callsign && !v.Crossing
	}
	departureBlocked := func(u RunwayUser) string {
		for _, v := range users {
			if !otherEnd(u, v) {
				continue
			}
			switch {
			// Close in only: farther out it is given the gap for the departures
			// (a runway change, ApproachSequencer.SetNotBefore), and both waiting
			// on each other nobody would go.
			case v.Arrival && (v.Phase == RunwayRolling || v.Phase == RunwayFinal && v.DistanceNM < OppositeArrivalNM):
				return v.Callsign + " landing on " + v.End
			case !v.Arrival && (v.Phase == RunwayLinedUp || v.Phase == RunwayRolling || v.Phase == RunwayAirborne):
				return v.Callsign + " departing from " + v.End
			}
		}
		return ""
	}
	landingBlocked := func(u RunwayUser) bool {
		for _, v := range users {
			if !otherEnd(u, v) {
				continue
			}
			if !v.Arrival && (v.Phase == RunwayLinedUp || v.Phase == RunwayRolling || v.Phase == RunwayAirborne) ||
				v.Arrival && (v.Phase == RunwayRolling || v.Phase == RunwayFinal && v.DistanceNM < u.DistanceNM) {
				return true
			}
		}
		return false
	}
	takeoffWhy := func(u RunwayUser, lineUp time.Duration) string {
		if why := departureBlocked(u); why != "" {
			return why
		}
		for _, v := range users {
			if v.GoingAround && v.Callsign != u.Callsign {
				return v.Callsign + " going around"
			}
		}
		// Anyone else on it: one lining up as another rolls out after
		// landing was cleared, the arrival listed first (live: BAW1272).
		// Not one of ours lined up behind it (further up the runway): two
		// lined up, the one ahead goes first (live, LKPR: TVS1663 at the
		// threshold, PHVTA at B, each waiting for the other).
		// As far down: the first lined up goes.
		behindU := func(cs string) bool {
			i := slices.IndexFunc(ourLinedUp, func(o RunwayUser) bool { return o.Callsign == cs })
			j := slices.IndexFunc(ourLinedUp, func(o RunwayUser) bool { return o.Callsign == u.Callsign })
			if i < 0 || j < 0 {
				return false
			}
			a, b := along[cs], along[u.Callsign]
			return a < b || a == b && i > j
		}
		for _, cs := range onRunway {
			if cs != u.Callsign && !behindU(cs) {
				return cs + " on the runway"
			}
		}
		if occupied != "" && occupied != u.Callsign && !behindU(occupied) {
			return occupied + " on the runway"
		}
		if why := interval(u); why != "" {
			return why
		}
		minNM := r.opts.MinArrivalNM
		need := takeoffSaidIn + lineUp + RunwayOccupancyIn(u.Wake, false, r.opts.Surface)
		if lineUp > 0 && u.LineUpIn == 0 {
			need += lineUpGoSlack
		}
		if lineUp == 0 {
			minNM = r.opts.MinArrivalLinedUpNM // lined up: the time rule does the rest
			if prevGranted[u.Callsign] {
				minNM = min(minNM, grantedArrivalNM)
			}
		}
		// Granted already: it stands a second later, not withdrawn as the
		// arrival's time crosses the limit while the call waits for the
		// frequency (live, LKPR: TVS850 "line up and take off" withdrawn
		// with the arrival 2m14s out, a second inside its need).
		if prevGranted[u.Callsign] {
			need -= grantedSlack
		}
		return arrivalClear(need, minNM)
	}

	out.NextArrival = nextArrName
	// The next arrival on short final with the runway not free: around.
	// In the way: anyone lined up, crossing, still on it after landing, or
	// other traffic on it; not our departure rolling.
	blocker := ""
	for _, u := range users {
		inWay := u.Phase == RunwayLinedUp || u.Phase == RunwayRolling && (u.Arrival || u.Crossing || u.Other)
		if inWay && blocker == "" {
			blocker = u.Callsign
		}
	}
	// Only an arrival established on the final: one still on its procedure
	// passing near the threshold is not landing (live, a circuit downwind).
	if blocker != "" && nextArrName != "" && nextArrName != blocker && nextArrUser.Established && nextArr*float64(time.Second) <= float64(r.opts.GoAroundAt) {
		out.GoAround = append(out.GoAround, nextArrName)
		out.Waiting[nextArrName] = blocker + " on the runway"
	}
	// The next arrival, near enough, with the runway free: cleared to land.
	// Free means nobody on it, a departure on its roll included (no
	// reduced runway separation): it is cleared once that one is airborne.
	if nextArrName != "" && !nextArrUser.Other && nextArrUser.Established && occupied == "" && nextArrUser.DistanceNM <= r.opts.ClearToLandNM && !landingBlocked(nextArrUser) {
		out.Land = append(out.Land, nextArrName)
	}

	// Ours lined up: take-off when it may.
	for _, u := range ourLinedUp {
		if why := takeoffWhy(u, r.liningUpLeft(u)); why == "" {
			out.Takeoff = append(out.Takeoff, u.Callsign)
			r.granted[u.Callsign] = true
		} else {
			out.Waiting[u.Callsign] = why
		}
	}
	// The holding points, first come first.
	sort.SliceStable(holding, func(i, j int) bool { return r.queue[holding[i].Callsign].Before(r.queue[holding[j].Callsign]) })
	// A faster one on the same route, at the holding points about as long,
	// goes first: behind the slower it would wait the catch-up as well
	// (DepartureIntervalSpeeds) and still close on it after take-off.
	for i := 0; i+1 < len(holding); i++ {
		a, b := holding[i], holding[i+1]
		if a.Route != "" && a.Route == b.Route && a.ClimbKts > 0 && b.ClimbKts-a.ClimbKts >= DepartureFirstKts &&
			r.queue[b.Callsign].Sub(r.queue[a.Callsign]) <= DepartureFirstWithin {
			holding[i], holding[i+1] = b, a
			i++
		}
	}
	// A rush: departures queueing, or an arrival soon (LineUpBehindDeparting).
	rush := len(holding) >= RushQueue || nextArrName != "" && nextArr*float64(time.Second) <= float64(RushArrivalWithin)
	number := 1
	// One told to line up and still at its holding point: the next on the
	// runway, from wherever the others wait (live, TVS1608 at A and RYR257
	// at Z both told to line up behind the departing 737, 16 s apart).
	cleared := ""
	if i := slices.IndexFunc(holding, func(u RunwayUser) bool { return u.LineUpCleared && !u.Crossing && !u.Host }); i >= 0 {
		cleared = holding[i].Callsign
	}
	for _, u := range holding {
		if cleared != "" && u.Callsign != cleared && !u.Crossing && !u.Host {
			number++
			out.Waiting[u.Callsign] = cleared + " lining up"
			continue
		}
		if u.Host {
			// Its turn: ours behind it are the next numbers; behind ours, it
			// is a number too.
			if occupied == "" {
				occupied, linedUp = u.Callsign, u.Callsign
			} else {
				number++
			}
			out.Waiting[u.Callsign] = "cleared by its own ATC"
			continue
		}
		if u.Crossing {
			switch {
			case occupied != "" && crossingOnly(users, onRunway, out.Cross, occupied) && arrivalClear(r.opts.CrossTime, 0) == "":
				// Only crossings on it: across together, side by side or in
				// turn (the user: crossings keep the flow).
				out.Cross = append(out.Cross, u.Callsign)
			case occupied != "":
				out.Waiting[u.Callsign] = occupied + " on the runway"
			case departureReady(holding, u.Callsign, takeoffWhy, r.opts.LineUpTime) != "":
				// The runway's own traffic first: a departure that can go now
				// goes before a crossing (the user: handling the active runway
				// has priority).
				out.Waiting[u.Callsign] = departureReady(holding, u.Callsign, takeoffWhy, r.opts.LineUpTime) + " departing first"
			default:
				if why := arrivalClear(r.opts.CrossTime, 0); why != "" {
					out.Waiting[u.Callsign] = why
					// Only the next arrival in the way: across behind it.
					if nextArrName != "" && !nextArrUser.Other && nextArrUser.Established {
						out.CrossBehind[u.Callsign] = nextArrName
					}
					continue
				}
				out.Cross = append(out.Cross, u.Callsign)
				occupied = u.Callsign // one at a time
			}
			continue
		}
		// A departure: one on the runway at a time — behind the one of ours
		// lined up, it is the next number; else it waits for who is on it.
		if occupied != "" {
			// Busy: the first behind ours on its take-off roll lines up as
			// it rolls, if the arrivals leave it time on the runway until its
			// own take-off (the user: lined up earlier in a rush).
			// Without a rush too where the one rolling is past its way in
			// already (an intersection departure behind it: live, LKPR 06 at
			// E rolling, one at F lines up as it passes; the user).
			passed := u.AlongM > 0 && along[rollingDep] > u.AlongM+IntersectionPassed(u.Callsign)
			// Behind the one about to go: its way in behind that one's (the
			// one at E cleared for take-off, this one at F lines up and waits
			// as it goes: the user). Ours only, a departure, not yet gone,
			// and the only one of ours lined up or lining up: never a second
			// behind it (live, LKPR: OKFZG told to line up behind OKFXC at B
			// while CSA958 was lining up from the same holding point; then
			// cleared for take-off with CSA958 on the runway).
			// About to go: cleared for take-off, not one waiting lined up for
			// an arrival (live, RYR1416 told to line up behind the DA-42
			// lined up at B waiting for AFR765: two on the runway, AFR765
			// sent around).
			if occ, ok := along[occupied]; ok && occupied != rollingDep && ourDep[occupied] && r.granted[occupied] && number == 1 && len(out.LineUpBehindDeparting) == 0 &&
				len(ourLinedUp) == 1 && ourLinedUp[0].Callsign == occupied && !ourLinedUp[0].LiningUp &&
				occ > u.AlongM+IntersectionPassed(u.Callsign) && departureBlocked(u) == "" &&
				arrivalClear(r.intervalLeft(now, u)+r.opts.LineUpTime+RunwayOccupancyIn(u.Wake, false, r.opts.Surface)+takeoffSaidIn+r.opts.Margin, r.opts.MinArrivalNM) == "" {
				out.LineUpBehindDeparting[u.Callsign] = occupied
				out.LineUpAhead[u.Callsign] = true // down the runway past its way in
				out.Waiting[u.Callsign] = occupied + " departing ahead"
				continue
			}
			// Never with one of ours lined up or lining up already: that one
			// goes next, and one lined up further down would be in its way
			// (live, OKDIS lined up at an intersection ahead of CSA443 lined up
			// at the threshold, both behind the one rolling).
			if occupied == rollingDep && number == 1 && (rush || passed) && len(out.LineUpBehindDeparting) == 0 && len(ourLinedUp) == 0 && departureBlocked(u) == "" &&
				// Its line-up too, while the interval runs: the room the map
				// checks again once the one rolling has passed (LineUpRoom;
				// offline LKPR, WZZ1040 told to line up, then to hold position).
				// Its line-up starts once the one rolling has passed its way in
				// (live, OKSZG at B waited 31 s for CSA813, lined up 75 s, and
				// CSA236 was sent around).
				arrivalClear(max(r.intervalLeft(now, u), RollPastIn(u.AlongM+IntersectionPassed(u.Callsign)-along[rollingDep], rollingKts)+r.opts.LineUpTime)+RunwayOccupancyIn(u.Wake, false, r.opts.Surface)+takeoffSaidIn+r.opts.Margin, r.opts.MinArrivalNM) == "" {
				out.LineUpBehindDeparting[u.Callsign] = rollingDep
				// In down the runway from an intersection ahead of this one's
				// way in, or past it already (at the threshold too): it never
				// passes in front of this one.
				if in, ok := r.wayIn[rollingDep]; ok && in >= u.AlongM+wayInApartM || along[rollingDep] > u.AlongM+IntersectionPassed(u.Callsign) {
					out.LineUpAhead[u.Callsign] = true
				}
				out.Waiting[u.Callsign] = rollingDep + " on the runway"
				occupied = u.Callsign
				continue
			}
			if occupied == linedUp || slices.Contains(out.LineUp, occupied) || out.LineUpBehindDeparting[occupied] != "" {
				number++
				out.Waiting[u.Callsign] = fmt.Sprintf("number %d for departure", number)
			} else {
				out.Waiting[u.Callsign] = occupied + " on the runway"
			}
			continue
		}
		// From the holding point: lining up takes its time too.
		why := takeoffWhy(u, r.lineUpFrom(u))
		switch {
		case why == "":
			// Line up and go.
			out.LineUp = append(out.LineUp, u.Callsign)
			out.Takeoff = append(out.Takeoff, u.Callsign)
			r.granted[u.Callsign] = true
			occupied = u.Callsign
		// Lined up only if it can still go before the next arrival once the
		// interval has run (live, LKPR: AUA511 lined up 28 s behind a
		// Phenom, KLM1683 came inside 3 NM meanwhile and went around).
		// Lining up to a stop takes LineUpWaitTime, not the line-up-and-go
		// LineUpTime (live, OKWMV at B lined up 100 s after its clearance,
		// AFR765 on final went around).
		case departureBlocked(u) == "" && r.intervalLeft(now, u) <= r.opts.LineUpTime+LineUpWaitMax &&
			arrivalClear(max(r.intervalLeft(now, u), r.opts.LineUpWaitTime)+RunwayOccupancyIn(u.Wake, false, r.opts.Surface)+takeoffSaidIn+r.opts.Margin, r.opts.MinArrivalNM) == "":
			// Only the interval runs: line up and wait.
			out.LineUp = append(out.LineUp, u.Callsign)
			out.Waiting[u.Callsign] = why
			occupied = u.Callsign
		default:
			out.Waiting[u.Callsign] = why
			// Waiting for the next arrival only, first in turn: behind it.
			// Only behind one established on the final (one still on its STAR
			// or downwind may be many minutes away), and with time to go
			// before the arrival after it: lined up as the first lands, it
			// goes once that one is off the runway — live, TVS1124 lined up
			// behind CSA1232 and LOT775 behind it had to go around.
			// Never behind one landing toward it from the other end.
			if number == 1 && nextArrName != "" && nextArrUser.Established && len(out.LineUpBehind) == 0 && interval(u) == "" && nextArrUser.End == u.End &&
				r.behindRoom(nextArr, nextArrUser, thenArr, u) {
				out.LineUpBehind[u.Callsign] = nextArrName
			}
		}
	}
	// Cleared with the next arrival close behind: told so, to roll at once.
	if nextArrName != "" && !nextArrUser.Other && nextArrUser.Established && nextArrUser.DistanceNM <= r.opts.NoDelayNM {
		for _, cs := range out.Takeoff {
			out.NoDelay[cs] = nextArrUser.DistanceNM
		}
	}
	return out
}

// A runway is in a rush with RushQueue or more departures at its holding
// points, or an arrival landing within RushArrivalWithin: the next
// departure lines up behind the one rolling (LineUpBehindDeparting).
var (
	RushQueue         = 2
	RushArrivalWithin = 5 * time.Minute
)

// LineUpWaitMax: a departure waiting out its interval is told to line up
// and wait only when, lined up, it waits no longer than this for its
// take-off; earlier it holds short (live, EZY562 lined up 3 minutes behind
// a TBM on the same SID, the runway blocked for nothing).
const LineUpWaitMax = 45 * time.Second

// intervalLeft is how long u still waits for the departure interval behind
// the last departure (0 none).
func (r *RunwayController) intervalLeft(now time.Time, u RunwayUser) time.Duration {
	if r.lastDep == nil || r.lastDep.Callsign == u.Callsign {
		return 0
	}
	iv := DepartureIntervalSpeeds(r.lastDep.Wake, u.Wake, r.lastDep.Route != "" && r.lastDep.Route == u.Route, r.lastDep.ClimbKts, u.ClimbKts)
	return max(0, iv-now.Sub(r.lastAt))
}

// Clone is a copy of r with its state now (the last departure, the queue
// at the holding points): deciding on it leaves r as it is — for a "what
// if" (a player's ATC asking before it clears the user aircraft).
func (r *RunwayController) Clone() *RunwayController {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := &RunwayController{opts: r.opts, lastAt: r.lastAt, queue: make(map[string]time.Time, len(r.queue))}
	if r.lastDep != nil {
		d := *r.lastDep
		c.lastDep = &d
	}
	for k, v := range r.queue {
		c.queue[k] = v
	}
	return c
}

// departureReady names a departure among holding (not a crossing, not
// other traffic) that could take off now; "" none.
func departureReady(holding []RunwayUser, except string, takeoffWhy func(RunwayUser, time.Duration) string, lineUp time.Duration) string {
	for _, d := range holding {
		if d.Crossing || d.Other || d.Host || d.Callsign == except {
			continue
		}
		if takeoffWhy(d, lineUp) == "" {
			return d.Callsign
		}
	}
	return ""
}

// crossingOnly reports whether everyone on the runway now (onRunway) and
// the one holding it (occupied, cleared across already: crossed) are
// crossing it, nobody landing or taking off.
func crossingOnly(users []RunwayUser, onRunway, crossed []string, occupied string) bool {
	crossing := map[string]bool{}
	for _, u := range users {
		if u.Crossing {
			crossing[u.Callsign] = true
		}
	}
	for _, cs := range onRunway {
		if !crossing[cs] {
			return false
		}
	}
	return crossing[occupied] || slices.Contains(crossed, occupied)
}

// IntersectionPassed is how far a departure must be past another's way in
// (its intersection) for that one to line up behind it: 400 to 800 m, the
// same for one call sign each time (the user: 200 m too little; a
// controller's judgement varies).
func IntersectionPassed(cs string) float64 {
	h := 0
	for _, c := range cs {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return 400 + float64(h%401)
}
