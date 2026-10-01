//go:build windows
// +build windows

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
	// Other traffic: counted, never cleared.
	Other bool
}

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
}

// RunwayController clears the users of one runway.
type RunwayController struct {
	opts RunwayControllerOptions

	mu      sync.Mutex
	lastDep *RunwayUser // the last departure that started its roll
	lastAt  time.Time
	queue   map[string]time.Time // when each departure started waiting
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
	out := RunwayClearances{Waiting: map[string]string{}, LineUpBehind: map[string]string{}}

	// What the runway is doing: who is on it, who of ours is lined up, when
	// the next arrival lands, who waits at the holding points.
	// occupied: who is on the runway (the first seen); onRunway: all of
	// them — a take-off waits for every one but itself.
	occupied, linedUp := "", ""
	var onRunway []string
	nextArr, nextArrName := math.Inf(1), ""
	var nextArrUser RunwayUser
	var holding []RunwayUser
	seen := map[string]bool{}
	for _, u := range users {
		seen[u.Callsign] = true
		switch u.Phase {
		case RunwayRolling:
			occupied = u.Callsign
			onRunway = append(onRunway, u.Callsign)
			if !u.Arrival && !u.Crossing && (r.lastDep == nil || r.lastDep.Callsign != u.Callsign) {
				uu := u // a departure started its roll: the interval runs from here
				r.lastDep, r.lastAt = &uu, now
			}
		case RunwayLinedUp:
			occupied = u.Callsign
			onRunway = append(onRunway, u.Callsign)
			if !u.Other {
				linedUp = u.Callsign
			}
		case RunwayFinal:
			if t := u.DistanceNM / math.Max(u.GroundKts, 100) * 3600; t < nextArr {
				nextArr, nextArrName, nextArrUser = t, u.Callsign, u
			}
		case RunwayHoldingShort:
			if !u.Other {
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
	// The next arrival lands after occ and the margin, and is not inside
	// MinArrivalNM.
	arrivalClear := func(occ time.Duration, minNM float64) string {
		for _, u := range users {
			if u.Phase == RunwayFinal && u.DistanceNM < minNM {
				return fmt.Sprintf("%s on a %.1f NM final", u.Callsign, u.DistanceNM)
			}
		}
		if nextArr*float64(time.Second) < float64(occ+r.opts.Margin) {
			return fmt.Sprintf("%s lands in %s", nextArrName, time.Duration(nextArr*float64(time.Second)).Round(time.Second))
		}
		return ""
	}
	// The interval after the last departure.
	interval := func(u RunwayUser) string {
		if r.lastDep == nil || r.lastDep.Callsign == u.Callsign {
			return ""
		}
		iv := DepartureInterval(r.lastDep.Wake, u.Wake, r.lastDep.Route != "" && r.lastDep.Route == u.Route)
		if left := iv - now.Sub(r.lastAt); left > 0 {
			return fmt.Sprintf("%s behind %s", left.Round(time.Second), r.lastDep.Callsign)
		}
		return ""
	}
	takeoffWhy := func(u RunwayUser, lineUp time.Duration) string {
		// Anyone else on it: one lining up as another rolls out after
		// landing was cleared, the arrival listed first (live: BAW1272).
		for _, cs := range onRunway {
			if cs != u.Callsign {
				return cs + " on the runway"
			}
		}
		if occupied != "" && occupied != u.Callsign {
			return occupied + " on the runway"
		}
		if why := interval(u); why != "" {
			return why
		}
		return arrivalClear(lineUp+RunwayOccupancyIn(u.Wake, false, r.opts.Surface), r.opts.MinArrivalNM)
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
	if nextArrName != "" && !nextArrUser.Other && nextArrUser.Established && occupied == "" && nextArrUser.DistanceNM <= r.opts.ClearToLandNM {
		out.Land = append(out.Land, nextArrName)
	}

	// Ours lined up: take-off when it may.
	for _, u := range users {
		if u.Callsign != linedUp {
			continue
		}
		if why := takeoffWhy(u, 0); why == "" {
			out.Takeoff = append(out.Takeoff, u.Callsign)
		} else {
			out.Waiting[u.Callsign] = why
		}
	}
	// The holding points, first come first.
	sort.SliceStable(holding, func(i, j int) bool { return r.queue[holding[i].Callsign].Before(r.queue[holding[j].Callsign]) })
	number := 1
	for _, u := range holding {
		if u.Crossing {
			switch {
			case occupied != "":
				out.Waiting[u.Callsign] = occupied + " on the runway"
			default:
				if why := arrivalClear(r.opts.CrossTime, 0); why != "" {
					out.Waiting[u.Callsign] = why
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
			if occupied == linedUp || slices.Contains(out.LineUp, occupied) {
				number++
				out.Waiting[u.Callsign] = fmt.Sprintf("number %d for departure", number)
			} else {
				out.Waiting[u.Callsign] = occupied + " on the runway"
			}
			continue
		}
		// From the holding point: lining up takes its time too.
		why := takeoffWhy(u, r.opts.LineUpTime)
		switch {
		case why == "":
			// Line up and go.
			out.LineUp = append(out.LineUp, u.Callsign)
			out.Takeoff = append(out.Takeoff, u.Callsign)
			occupied = u.Callsign
		case arrivalClear(r.opts.LineUpTime+RunwayOccupancyIn(u.Wake, false, r.opts.Surface)+r.opts.Margin, r.opts.MinArrivalNM) == "":
			// Only the interval runs: line up and wait.
			out.LineUp = append(out.LineUp, u.Callsign)
			out.Waiting[u.Callsign] = why
			occupied = u.Callsign
		default:
			out.Waiting[u.Callsign] = why
			// Waiting for the next arrival only, first in turn: behind it.
			if number == 1 && nextArrName != "" && len(out.LineUpBehind) == 0 && interval(u) == "" {
				out.LineUpBehind[u.Callsign] = nextArrName
			}
		}
	}
	return out
}
