package traffic

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// The situation checker (#368): each Tick the manager looks at each
// airport as the people there would — ATC watching the arrivals stream and
// the taxiways, the ramp watching the stands, dispatch watching the
// inbound aircraft of a turnaround — predicts what is coming and adjusts:
// an arrival is held back to keep the landing gaps, departures stay on
// their stands while the taxiways are full, a departure is estimated late
// when its aircraft is, and an aircraft that stopped making progress is
// taken out. The controllers do the same up close (give way, queue behind,
// hold for traffic behind the stand); this is the wider picture.

// SituationCheck looks at one airport and advises. Checks are pure: they
// see copies of the flights and change nothing themselves.
type SituationCheck func(s Situation) []Advice

// Situation is what a check sees: the flights at one managed airport (all
// statuses) at Now, and the manager's options.
type Situation struct {
	Now     time.Time
	Airport string
	Flights []ManagedFlight
	Options ManagerOptions
	// Others is the traffic not ours at the airport (on its ground,
	// arriving, departing) when the manager respects it; Position the
	// airport's, when the picture knows it.
	Others   []TrackedAircraft
	Position airport.LatLon
	// Conditions are the weather on final (ManagerOptions.Conditions).
	Conditions ApproachConditions
}

// AdviceAction is what a check advises for a flight.
type AdviceAction uint8

const (
	// AdviceDelay: a scheduled flight is not spawned before Until.
	AdviceDelay AdviceAction = iota
	// AdviceHold: a boarding departure does not push back while advised
	// (every Tick); once no check advises it any more it is released.
	AdviceHold
	// AdviceEstimate: the flight's estimated time (ETD or ETA) is Until.
	AdviceEstimate
	// AdviceRemove: the flight's aircraft is taken out and the flight
	// cancelled.
	AdviceRemove
)

// Advice is one check's advice for one flight (by Key).
type Advice struct {
	Key    string
	Action AdviceAction
	Until  time.Time
	Reason string
}

// ErrSpawnBlocked tells the manager (Failed) that a spawn found its place
// taken — other traffic at the STAR entry, on the stand: it waits
// RetryAfter and tries again without counting an attempt.
var ErrSpawnBlocked = errors.New("spawn point in use")

// ErrSpawnImpossible tells the manager (Failed) that a flight can never be
// spawned, e.g. an overflight whose plan never enters the area: it is
// cancelled at once, not tried again.
var ErrSpawnImpossible = errors.New("cannot be spawned")

// DefaultChecks are the checks a manager runs when ManagerOptions.Checks
// is nil.
func DefaultChecks() []SituationCheck {
	return []SituationCheck{CheckStuck(nil), CheckLandingFlow(0, 0), CheckGroundCongestion(0), CheckDepartureQueue(0), CheckTurnaround(0)}
}

// DefaultStuckAfter is how long a flight may stay in a status before
// CheckStuck takes it out.
var DefaultStuckAfter = map[FlightStatus]time.Duration{
	FlightBoarding:    75 * time.Minute, // lead, a late inbound, holds
	FlightTaxiing:     30 * time.Minute,
	FlightDeparting:   20 * time.Minute,
	FlightEnroute:     90 * time.Minute,
	FlightApproaching: 50 * time.Minute,
	FlightLanded:      25 * time.Minute,
}

// CheckStuck removes flights that stayed in one status longer than
// after[status] (nil: DefaultStuckAfter): an aircraft that stopped making
// progress blocks stands, taxiways and runways for everyone.
func CheckStuck(after map[FlightStatus]time.Duration) SituationCheck {
	if after == nil {
		after = DefaultStuckAfter
	}
	return func(s Situation) []Advice {
		var out []Advice
		for _, f := range s.Flights {
			max, ok := after[f.Status]
			if f.Held {
				continue // held by ATC (flow control): waiting, not stuck
			}
			if f.AtRunway {
				max *= 2 // in the runway queue: the tower's, it goes
			}
			if ok && s.Now.Sub(f.Since) > max {
				out = append(out, Advice{Key: f.Key(), Action: AdviceRemove, Reason: fmt.Sprintf("stuck %s for %s", f.Status, s.Now.Sub(f.Since).Round(time.Minute))})
			}
		}
		return out
	}
}

// Landing flow defaults: arrivals land LandingBeforeSTA before their STA
// (taxi-in), at least DefaultLandingGap apart; with departures waiting,
// the gap doubles to let them go between (DefaultDepartureGapQueue
// departures taxiing or lined up).
const (
	LandingBeforeSTA         = 5 * time.Minute
	DefaultLandingGap        = 3 * time.Minute
	DefaultDepartureGapQueue = 2
)

// landingETA predicts when an arrival lands: an approaching one at its
// scheduled landing or, if later, ArrivalLead after it appeared; a
// scheduled one ArrivalLead after it can appear.
func landingETA(f ManagedFlight, o ManagerOptions, now time.Time) time.Time {
	planned := f.STA.Add(-LandingBeforeSTA)
	fly := o.ArrivalLead - LandingBeforeSTA
	if f.Stage == "enroute" && (f.Status == FlightEnroute || f.Status == FlightSpawning) {
		// On its way to the STAR entry, reached EnrouteLead after it appeared.
		fly += o.EnrouteLead
	}
	switch f.Status {
	case FlightApproaching, FlightSpawning, FlightEnroute:
		// From when it appeared, not Since: Progress moves Since on with
		// every mile closer, and the landing slid along with the clock (live,
		// LKPR: every arrival behind pushed on, one landing in 15 minutes).
		appeared := f.statusAt
		if appeared.IsZero() {
			appeared = f.Since
		}
		if t := appeared.Add(fly); t.After(planned) {
			return t
		}
		return planned
	case FlightScheduled:
		appear := f.STA.Add(-o.ArrivalLead)
		if f.retryAt.After(appear) {
			appear = f.retryAt
		}
		if now.After(appear) {
			appear = now
		}
		if t := appear.Add(fly); t.After(planned) {
			return t
		}
		return planned
	}
	return time.Time{}
}

// CheckLandingFlow sequences the arrivals of an airport as approach
// control does: it predicts every landing and delays the spawn of a
// scheduled arrival that would land less than gap (0: DefaultLandingGap)
// behind the one before — one gap doubled while queue (0:
// DefaultDepartureGapQueue) or more departures wait for the runway — and
// estimates its ETA.
func CheckLandingFlow(gap time.Duration, queue int) SituationCheck {
	if gap == 0 {
		gap = DefaultLandingGap
	}
	if queue == 0 {
		queue = DefaultDepartureGapQueue
	}
	return func(s Situation) []Advice {
		waiting := 0
		type landing struct {
			f   ManagedFlight
			eta time.Time
		}
		var ls []landing
		for _, f := range s.Flights {
			if f.Departure() && f.Status == FlightTaxiing {
				waiting++
			}
			if f.Arrival() {
				if eta := landingETA(f, s.Options, s.Now); !eta.IsZero() {
					ls = append(ls, landing{f, eta})
				}
			}
		}
		// Other traffic arriving takes its slot: ETA from its distance and
		// speed (it cannot be delayed; it is not ours).
		for _, a := range s.Others {
			if (a.Phase == PhaseArriving || a.Phase == PhaseApproach) && s.Position != (airport.LatLon{}) {
				d := calc.HaversineNM(a.Position.Lat, a.Position.Lon, s.Position.Lat, s.Position.Lon)
				eta := s.Now.Add(time.Duration(d / math.Max(a.GroundKts, 120) * float64(time.Hour)))
				name := a.Tail
				if name == "" {
					name = a.Title
				}
				ls = append(ls, landing{ManagedFlight{Flight: Flight{Callsign: name}, Kind: "other", Status: FlightApproaching}, eta})
			}
		}
		sort.Slice(ls, func(i, j int) bool { return lessFlight(ls[i].eta, ls[i].f.Callsign, ls[j].eta, ls[j].f.Callsign) })
		// Gaps for the departures waiting, one for every three of them (one
		// goes between each two arrivals, two in a gap), the gaps already ahead counted: the queue sets
		// the balance (live, LKPR: one gap in front of every arrival, arrivals
		// 6 min apart; one gap at a time, eight departures queued).
		want := 0
		if waiting >= queue {
			want = (waiting + 2) / 3
		}
		gaps := 0
		// The weather stretches the gaps as it stretches the spacing on
		// final: twice in low visibility, a third more on a contaminated
		// runway.
		base := gap
		medium := Wake{WakeMedium, RecatD}
		if nm, _ := ArrivalSpacing(medium, medium, SchemeICAO, s.Conditions, false); nm > MinRadarSeparationNM {
			base = time.Duration(float64(gap) * nm / MinRadarSeparationNM)
		}
		var out []Advice
		var prev time.Time
		for _, l := range ls {
			eta := l.eta
			if !prev.IsZero() && eta.Sub(prev) >= 2*base && prev.After(s.Now) {
				gaps++ // one ahead already
			}
			need := base
			if gaps < want && l.f.Status == FlightScheduled && !prev.IsZero() {
				need = 2 * base
			}
			if !prev.IsZero() && eta.Sub(prev) < need && l.f.Status == FlightScheduled {
				shift := need - eta.Sub(prev)
				eta = eta.Add(shift)
				why := "landing flow"
				switch {
				case need > base:
					why = fmt.Sprintf("landing flow, gap for %d departures", waiting)
				case base > gap && s.Conditions.LowVisibility():
					why = "landing flow, low visibility procedures"
				case base > gap:
					why = "landing flow, " + s.Conditions.Surface.String() + " runway"
				}
				spawn := eta.Add(-(s.Options.ArrivalLead - LandingBeforeSTA))
				out = append(out, Advice{Key: l.f.Key(), Action: AdviceDelay, Until: spawn, Reason: why})
			}
			if need > base && eta.Sub(l.eta) > 0 && eta.Sub(prev) < 2*base+time.Second {
				gaps++ // opened
			}
			if l.f.Kind != "other" && (l.f.Status == FlightScheduled || l.f.Status == FlightApproaching) {
				if sta := eta.Add(LandingBeforeSTA); sta.Sub(l.f.STA) >= time.Minute {
					out = append(out, Advice{Key: l.f.Key(), Action: AdviceEstimate, Until: sta, Reason: "landing sequence"})
				}
			}
			prev = eta
		}
		return out
	}
}

// DefaultMaxTaxiing is how many aircraft may taxi at an airport before
// CheckGroundCongestion holds departures on their stands.
const DefaultMaxTaxiing = 8

// CheckGroundCongestion holds the boarding departures of an airport on
// their stands while max (0: DefaultMaxTaxiing) or more aircraft are
// taxiing there (departures out, arrivals in) — a ground stop, as a ground
// controller does rather than fill the taxiways.
func CheckGroundCongestion(max int) SituationCheck {
	if max == 0 {
		max = DefaultMaxTaxiing
	}
	return func(s Situation) []Advice {
		moving := 0
		for _, f := range s.Flights {
			// Not the runway queue: off the taxiways (live LKPR, a ground
			// stop with the taxiways empty, four at the holding point).
			if f.Status == FlightTaxiing && !f.AtRunway || f.Status == FlightLanded {
				moving++
			}
		}
		for _, a := range s.Others {
			if a.Phase == PhaseTaxiing || a.Phase == PhaseHolding || a.Phase == PhasePushback {
				moving++ // other traffic fills the taxiways too
			}
		}
		if moving < max {
			return nil
		}
		var out []Advice
		for _, f := range s.Flights {
			if f.Status == FlightBoarding {
				out = append(out, Advice{Key: f.Key(), Action: AdviceHold, Reason: fmt.Sprintf("ground stop, %d taxiing", moving)})
			}
		}
		return out
	}
}

// DefaultMaxDepartureQueue is how many departures may be taxiing out
// before CheckDepartureQueue holds the next on their stands, and
// DefaultMaxRunwayQueue how many may wait at the runway (in its line,
// holding short, lining up, rolling).
const (
	DefaultMaxDepartureQueue = 6
	DefaultMaxRunwayQueue    = 4
)

// CheckDepartureQueue holds the boarding departures of an airport on their
// stands while max (0: DefaultMaxDepartureQueue) or more departures are
// taxiing out, or its runway's queue is full (ManagerOptions.RunwayQueue,
// else DefaultMaxRunwayQueue), as departure management
// does: the queue at the runway stays short, and the arrivals are not
// spaced for a long one (live LKPR: nine departures at the holding point,
// every arrival behind a double gap, two of them held). The two are
// counted apart: one already at the runway, about to go, holds nobody on
// the taxiways back (live LKPR: departures held on their stands behind
// four at the holding point).
func CheckDepartureQueue(max int) SituationCheck {
	if max == 0 {
		max = DefaultMaxDepartureQueue
	}
	return func(s Situation) []Advice {
		taxiing := 0
		atRunway := map[string]int{} // by runway
		for _, f := range s.Flights {
			if f.Departure() && f.Status == FlightTaxiing {
				if f.AtRunway {
					atRunway[f.Runway]++
				} else {
					taxiing++
				}
			}
		}
		full := func(rwy string) bool {
			n := DefaultMaxRunwayQueue
			if q := s.Options.RunwayQueue; q != nil {
				if m := q(s.Airport, rwy); m > 0 {
					n = m
				}
			}
			return atRunway[rwy] >= n
		}
		var out []Advice
		for _, f := range s.Flights {
			if !f.Departure() || f.Status != FlightBoarding {
				continue
			}
			var reason string
			switch {
			case taxiing >= max:
				reason = fmt.Sprintf("departure queue, %d taxiing out", taxiing)
			case f.Runway != "" && full(f.Runway):
				reason = fmt.Sprintf("departure queue, %d at runway %s", atRunway[f.Runway], f.Runway)
			case f.Runway == "":
				// Its runway not known yet: held when any is full.
				for rwy := range atRunway {
					if full(rwy) {
						reason = fmt.Sprintf("departure queue, %d at runway %s", atRunway[rwy], rwy)
						break
					}
				}
			}
			if reason != "" {
				out = append(out, Advice{Key: f.Key(), Action: AdviceHold, Reason: reason})
			}
		}
		return out
	}
}

// DefaultMinGroundTime is the shortest turnaround on the stand.
const DefaultMinGroundTime = 25 * time.Minute

// CheckTurnaround estimates a turnaround departure late when its inbound
// aircraft is: parked, or predicted to land, less than minGround (0:
// DefaultMinGroundTime) before the STD. The manager already waits for the
// aircraft; this puts the delay on the board.
func CheckTurnaround(minGround time.Duration) SituationCheck {
	if minGround == 0 {
		minGround = DefaultMinGroundTime
	}
	return func(s Situation) []Advice {
		arr := map[string]ManagedFlight{}
		for _, f := range s.Flights {
			if f.Arrival() {
				arr[f.Callsign] = f
			}
		}
		var out []Advice
		for _, d := range s.Flights {
			a, ok := arr[d.TurnFrom]
			if !d.Departure() || !ok || d.Status != FlightScheduled {
				continue
			}
			in := a.Since // parked since
			if a.Status != FlightParked {
				in = landingETA(a, s.Options, s.Now).Add(LandingBeforeSTA)
				if in.IsZero() || in.Before(s.Now) {
					in = s.Now.Add(LandingBeforeSTA)
				}
			}
			if etd := in.Add(minGround); etd.Sub(d.STD) >= time.Minute {
				out = append(out, Advice{Key: d.Key(), Action: AdviceEstimate, Until: etd, Reason: "late inbound " + a.Callsign})
			}
		}
		return out
	}
}
