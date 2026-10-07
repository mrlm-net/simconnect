package traffic

import (
	"errors"
	"sort"
	"sync"
	"time"
)

// The traffic manager (#368) turns a schedule into traffic: departures
// appear on their stand before their STD and push at it, arrivals appear
// in time to fly their STAR and approach for their STA, an arrival and a
// later departure of the same airline and type at the same airport are one
// aircraft turning around, and aircraft that are done leave. It decides
// what happens when; a Spawner does it in the simulator (it knows the
// stands, models, runways and controllers) and reports back.

// FlightStatus is where a managed flight is.
type FlightStatus uint8

const (
	FlightScheduled   FlightStatus = iota // not in the simulator yet
	FlightSpawning                        // handed to the Spawner
	FlightBoarding                        // departure: on its stand before the STD
	FlightTaxiing                         // departure: pushback, taxi, line-up
	FlightDeparting                       // departure: take-off and climb-out, still controlled
	FlightDeparted                        // departure: airborne and on its way; removed once it leaves the area
	FlightEnroute                         // arrival or overflight: airborne on its flight plan (MSFS AI), before the STAR entry
	FlightApproaching                     // arrival: airborne, STAR and approach
	FlightLanded                          // arrival: on the runway or taxiing in
	FlightParked                          // arrival: on its stand
	FlightDone                            // gone: removed, or turned into its next flight
	FlightCancelled                       // never flew or ended early; see Err
)

var flightStatusNames = [...]string{"scheduled", "spawning", "boarding", "taxiing", "departing", "departed", "enroute", "approaching", "landed", "parked", "done", "cancelled"}

func (s FlightStatus) String() string {
	if int(s) < len(flightStatusNames) {
		return flightStatusNames[s]
	}
	return "unknown"
}

// MarshalText makes the status its name in JSON.
func (s FlightStatus) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// active reports whether the flight has an aircraft in the simulator.
func (s FlightStatus) active() bool { return s >= FlightSpawning && s <= FlightParked }

// ManagedFlight is a scheduled flight and what became of it.
type ManagedFlight struct {
	Flight
	// Kind is "departure" or "arrival" at Airport, the managed airport the
	// flight is at (a flight between two managed airports is one of each).
	Kind    string       `json:"kind"`
	Airport string       `json:"airport"`
	Status  FlightStatus `json:"status"`
	// TurnFrom is the arrival whose aircraft this departure is (it adopts
	// the aircraft on its stand); TurnTo the departure an arrival becomes.
	TurnFrom string `json:"turnFrom,omitempty"`
	TurnTo   string `json:"turnTo,omitempty"`
	// Set by the Spawner (Describe): the model, the stand, the runway.
	Model  string `json:"model,omitempty"`
	Stand  string `json:"stand,omitempty"`
	Runway string `json:"runway,omitempty"`
	// Attempts is how often it was spawned; a Spawner tries another model
	// or stand on each attempt.
	Attempts int    `json:"attempts,omitempty"`
	Err      string `json:"error,omitempty"`
	// Since is when the status last changed.
	Since time.Time `json:"since"`
	// Estimated is the expected departure or arrival time when the
	// situation checks see it late (zero: on time); Note says why, or
	// what holds it; Held: kept on its stand (AdviceHold).
	Estimated time.Time `json:"estimated,omitempty"`
	Note      string    `json:"note,omitempty"`
	Held      bool      `json:"held,omitempty"`
	// Stage "enroute": the spawn is the enroute part of an arrival (MSFS AI
	// on its flight plan, handed to the arrival controller at the STAR
	// entry) or an overflight (#369).
	Stage string `json:"stage,omitempty"`
	// ObjectID is the aircraft, once the Spawner attached it (Attach).
	ObjectID uint32 `json:"objectId,omitempty"`

	retryAt   time.Time
	seenAt    time.Time // last seen in the picture
	noEnroute bool      // the enroute spawn failed: straight to the STAR entry
	dropped   bool      // a real flight the feed dropped: removed once parked (#841)
}

// Departure reports whether the flight departs from its managed airport.
func (f *ManagedFlight) Departure() bool { return f.Kind == "departure" }

// Arrival reports whether the flight arrives at its managed airport.
func (f *ManagedFlight) Arrival() bool { return f.Kind == "arrival" }

// Overflight reports whether the flight only crosses the area (#369).
func (f *ManagedFlight) Overflight() bool { return f.Kind == "overflight" }

// key identifies a managed flight: a call sign can be both an arrival at
// one managed airport and a departure from another.
func (f *ManagedFlight) key() string { return f.Kind + " " + f.Callsign }

// Spawner puts managed flights into the simulator.
type Spawner interface {
	// Spawn starts f: a departure on a stand at f.Airport (adopting the
	// aircraft of f.TurnFrom when set; the manager starts it only once that
	// arrival is parked), or an arrival at f.Airport. It must not block:
	// report with the manager's Update, Describe and Failed.
	Spawn(f ManagedFlight)
	// Remove takes f's aircraft out of the simulator.
	Remove(f ManagedFlight)
}

// Holder is a Spawner that can keep a boarding departure on its stand
// (AdviceHold): on until released.
type Holder interface {
	Hold(f ManagedFlight, on bool)
}

// ManagerOptions tune a TrafficManager; zero fields take the defaults.
type ManagerOptions struct {
	// Source gives the flights of a time window at the managed airports
	// (e.g. Schedule with a seed per window); the manager asks for Horizon
	// ahead, an hour at a time. It runs under the manager's lock: it must
	// not call the manager.
	Source  func(from, to time.Time, airports []string) []Flight
	Horizon time.Duration // default 2 h
	// DepartureLead is how long before its STD a departure appears on its
	// stand (default 10 min); ArrivalLead how long before its STA an arrival
	// appears to fly its STAR and approach (default 25 min).
	DepartureLead, ArrivalLead time.Duration
	// VFRLead: a VFR flight (Flight.Rules) appears this long before its
	// STA near the airport to join the circuit (default 8 min).
	VFRLead time.Duration
	// A departure not started by STD+DepartureLate (default 15 min), or an
	// arrival not by STA-ArrivalLead+ArrivalLate (default 10 min), is
	// cancelled: too late.
	DepartureLate, ArrivalLate time.Duration
	// MaxAircraft in the simulator at once (default 24), MaxPerAirport at
	// or around one airport (default 16).
	MaxAircraft, MaxPerAirport int
	// Spacing between spawns at one airport: arrivals (default 3 min, they
	// share the STAR entries) and departures (default 1 min).
	ArrivalSpacing, DepartureSpacing time.Duration
	// MaxAttempts per flight (default 3), RetryAfter between them (30 s),
	// SpawnTimeout before an unanswered spawn counts as failed (2 min).
	MaxAttempts              int
	RetryAfter, SpawnTimeout time.Duration
	// RemoveDepartedAfter: a departed aircraft is removed this long after
	// it left the controllers (default 30 min; with a Picture it goes once it
	// leaves the area, LeftAfter); RemoveParkedAfter: a parked
	// arrival not turning around (default 20 min).
	RemoveDepartedAfter, RemoveParkedAfter time.Duration
	// Turnarounds: an arrival turns into a departure of the same airline
	// and type from its airport between MinTurn (default 40 min) and
	// MaxTurn (default 3 h) after its STA. Negative MinTurn: none.
	MinTurn, MaxTurn time.Duration
	// Keep: done and cancelled flights stay on the boards this long
	// (default 1 h).
	Keep time.Duration
	// EnrouteLead: an arrival appears this long before ArrivalLead, en
	// route on its flight plan, and is handed to the arrival controller at
	// its STAR entry (default 20 min; negative: arrivals appear at the STAR
	// entry) (#369).
	EnrouteLead time.Duration
	// Overflights gives the flights crossing the area in a time window
	// (e.g. traffic.Overflights); they appear when they enter it (Flight.Enter)
	// and leave with it. MaxOverflights at once (default 4).
	Overflights    func(from, to time.Time) []Flight
	MaxOverflights int
	// LeftAfter: an airborne aircraft of ours the picture has not seen for
	// this long has left the area and is removed (default 1 min).
	LeftAfter time.Duration
	// Checks look at each airport every Tick and advise (situation.go);
	// nil means DefaultChecks, an empty slice none.
	Checks []SituationCheck
	// Picture is the traffic picture: with Others OtherRespect (the
	// default) the checks see the traffic that is not ours — MSFS AI, other
	// add-ons, the user — at each managed airport (Situation.Others).
	Picture *TrafficPicture
	Others  OtherTrafficMode
	// Conditions gives the weather on final of an airport's arrival runway
	// (ConditionsFrom): the checks space arrivals by it (#389). nil: calm,
	// good visibility, dry.
	Conditions func(icao string) (ApproachConditions, bool)
	// OnEvent is called with every lifecycle event, in order, outside the
	// manager's lock (it may call the manager); see also Events.
	OnEvent func(ManagerEvent)
}

func (o *ManagerOptions) defaults() {
	def := func(d *time.Duration, v time.Duration) {
		if *d == 0 {
			*d = v
		}
	}
	def(&o.Horizon, 2*time.Hour)
	def(&o.DepartureLead, 10*time.Minute)
	def(&o.ArrivalLead, 25*time.Minute)
	def(&o.VFRLead, 8*time.Minute)
	def(&o.DepartureLate, 15*time.Minute)
	def(&o.ArrivalLate, 10*time.Minute)
	def(&o.ArrivalSpacing, 3*time.Minute)
	def(&o.DepartureSpacing, time.Minute)
	def(&o.RetryAfter, 30*time.Second)
	def(&o.SpawnTimeout, 2*time.Minute)
	def(&o.RemoveDepartedAfter, 30*time.Minute)
	def(&o.EnrouteLead, 20*time.Minute)
	def(&o.LeftAfter, time.Minute)
	def(&o.RemoveParkedAfter, 20*time.Minute)
	def(&o.MinTurn, 40*time.Minute)
	def(&o.MaxTurn, 3*time.Hour)
	def(&o.Keep, time.Hour)
	if o.MaxAircraft == 0 {
		o.MaxAircraft = 24
	}
	if o.MaxPerAirport == 0 {
		o.MaxPerAirport = 16
	}
	if o.MaxAttempts == 0 {
		o.MaxAttempts = 3
	}
	if o.MaxOverflights == 0 {
		o.MaxOverflights = 4
	}
	if o.Checks == nil {
		o.Checks = DefaultChecks()
	}
}

// OtherTrafficMode is how a manager treats the traffic that is not ours.
type OtherTrafficMode uint8

const (
	// OtherRespect: other traffic counts — it fills taxiways, takes landing
	// slots, blocks spawn points.
	OtherRespect OtherTrafficMode = iota
	// OtherIgnore: the manager plans as if it were not there.
	OtherIgnore
)

func (o OtherTrafficMode) String() string {
	if o == OtherIgnore {
		return "ignore"
	}
	return "respect"
}

// MarshalText makes the mode its name in JSON.
func (o OtherTrafficMode) MarshalText() ([]byte, error) { return []byte(o.String()), nil }

// SetOthers changes how other traffic is treated.
func (m *TrafficManager) SetOthers(mode OtherTrafficMode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.opts.Others = mode
}

// Others is the traffic not ours at a managed airport the manager
// respects: on its ground, or arriving to or departing from it; nil when
// it ignores other traffic or has no Picture.
func (m *TrafficManager) Others(icao string) []TrackedAircraft {
	m.mu.Lock()
	mode, p := m.opts.Others, m.opts.Picture
	m.mu.Unlock()
	return othersAt(p, mode, icao)
}

func othersAt(p *TrafficPicture, mode OtherTrafficMode, icao string) []TrackedAircraft {
	if p == nil || mode == OtherIgnore {
		return nil
	}
	var out []TrackedAircraft
	for _, a := range p.Aircraft() {
		if !a.Ours && a.Airport == icao {
			out = append(out, a)
		}
	}
	return out
}

// TrafficManager runs the managed airports' schedules: Tick it (every
// second or so), feed it the Spawner's reports.
type TrafficManager struct {
	spawner Spawner

	mu        sync.Mutex
	opts      ManagerOptions
	airports  map[string]bool
	flights   map[string]*ManagedFlight
	until     time.Time            // the Source was asked up to here
	overUntil time.Time            // the Overflights source too
	last      map[string]time.Time // last spawn by kind and airport
	enabled   bool

	// Lifecycle events (#368): collected under mu, delivered after it.
	pending []ManagerEvent
	events  chan ManagerEvent
	onEvent func(ManagerEvent)
}

// NewTrafficManager creates a manager for the airports (ICAO) that puts
// its flights into the simulator with spawner. It starts enabled.
func NewTrafficManager(spawner Spawner, opts ManagerOptions, airports ...string) *TrafficManager {
	opts.defaults()
	m := &TrafficManager{spawner: spawner, opts: opts, airports: map[string]bool{}, flights: map[string]*ManagedFlight{}, last: map[string]time.Time{}, enabled: true,
		events: make(chan ManagerEvent, 256), onEvent: opts.OnEvent}
	for _, a := range airports {
		m.airports[a] = true
	}
	return m
}

// SetAirports changes the managed airports. Flights at airports no longer
// managed that have not started are dropped; the rest play out.
func (m *TrafficManager) SetAirports(airports ...string) {
	m.mu.Lock()
	defer m.unlock()
	m.airports = map[string]bool{}
	for _, a := range airports {
		m.airports[a] = true
	}
	for k, f := range m.flights {
		if !m.airports[f.Airport] && f.Status == FlightScheduled {
			delete(m.flights, k)
		}
	}
	m.until = time.Time{} // ask the Source again: new airports
}

// Airports are the managed airports.
func (m *TrafficManager) Airports() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.airports))
	for a := range m.airports {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

// SetEnabled starts or stops spawning; aircraft already there play out.
func (m *TrafficManager) SetEnabled(on bool) {
	m.mu.Lock()
	if m.enabled != on {
		k := EventDisabled
		if on {
			k = EventEnabled
		}
		m.pending = append(m.pending, ManagerEvent{Kind: k, Time: time.Now()})
	}
	m.enabled = on
	m.unlock()
}

// Enabled reports whether the manager spawns.
func (m *TrafficManager) Enabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled
}

// Options returns the options in use.
func (m *TrafficManager) Options() ManagerOptions {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.opts
}

// Add adds flights (a schedule); a flight already known by kind and call
// sign keeps its state. Each flight is managed at the managed airport(s)
// it departs from or arrives at; others are ignored.
func (m *TrafficManager) Add(flights []Flight) {
	m.mu.Lock()
	defer m.unlock()
	m.add(flights)
}

// addAt adds flights, leaving out those already too late at now to fly
// (a schedule started mid-day does not show the morning as cancelled).
func (m *TrafficManager) addAt(flights []Flight, now time.Time) {
	o := m.opts
	var keep []Flight
	for _, f := range flights {
		depLate := m.airports[f.Origin] && now.After(f.STD.Add(o.DepartureLate))
		arrLate := m.airports[f.Destination] && now.After(f.STA.Add(-m.arrivalLead(&f)).Add(o.ArrivalLate))
		if (depLate || !m.airports[f.Origin]) && (arrLate || !m.airports[f.Destination]) {
			continue
		}
		keep = append(keep, f)
	}
	m.add(keep)
}

func (m *TrafficManager) add(flights []Flight) {
	for _, f := range flights {
		for _, kind := range []string{"departure", "arrival"} {
			at := f.Origin
			if kind == "arrival" {
				at = f.Destination
			}
			if !m.airports[at] {
				continue
			}
			mf := &ManagedFlight{Flight: f, Kind: kind, Airport: at}
			if _, ok := m.flights[mf.key()]; !ok {
				m.flights[mf.key()] = mf
				m.emit(EventAdded, mf, time.Time{}, "")
			}
		}
	}
	m.pair()
}

// pair links arrivals and departures that are one aircraft turning around:
// each arrival still scheduled takes the first free departure of its
// airline and type from its airport in the turn window.
func (m *TrafficManager) pair() {
	if m.opts.MinTurn < 0 {
		return
	}
	var arrs, deps []*ManagedFlight
	for _, f := range m.flights {
		switch {
		case f.Observed != nil:
			// A real aircraft turns around as the feed says (Turn), not by
			// its airline and type.
		case f.Kind == "arrival" && f.TurnTo == "" && f.Status <= FlightParked:
			arrs = append(arrs, f)
		case f.Kind == "departure" && f.TurnFrom == "" && f.Status == FlightScheduled:
			deps = append(deps, f)
		}
	}
	sort.Slice(arrs, func(i, j int) bool { return lessFlight(arrs[i].STA, arrs[i].Callsign, arrs[j].STA, arrs[j].Callsign) })
	sort.Slice(deps, func(i, j int) bool { return lessFlight(deps[i].STD, deps[i].Callsign, deps[j].STD, deps[j].Callsign) })
	for _, a := range arrs {
		for _, d := range deps {
			if d.TurnFrom != "" || d.Airport != a.Airport || d.Airline != a.Airline || d.Type != a.Type {
				continue
			}
			if gap := d.STD.Sub(a.STA); gap >= m.opts.MinTurn && gap <= m.opts.MaxTurn {
				// A GA aircraft (no airline: its registration is its call sign)
				// flies on under its registration, not the departure's own.
				if a.Airline == "" && d.Callsign != a.Callsign && m.flights["departure "+a.Callsign] == nil {
					delete(m.flights, d.key())
					d.Callsign = a.Callsign
					m.flights[d.key()] = d
				}
				a.TurnTo, d.TurnFrom = d.Callsign, a.Callsign
				m.emit(EventTurnaround, d, time.Time{}, a.Callsign+" → "+d.Callsign)
				break
			}
		}
	}
}

func lessFlight(t1 time.Time, c1 string, t2 time.Time, c2 string) bool {
	if !t1.Equal(t2) {
		return t1.Before(t2)
	}
	return c1 < c2
}

// Tick runs the manager at now: it asks the Source ahead, spawns what is
// due within the limits, removes what is done and cancels what is late.
func (m *TrafficManager) Tick(now time.Time) {
	m.mu.Lock()
	var spawn, remove []ManagedFlight
	m.extend(now)
	m.expire(now, &remove)
	holds := m.check(now, &remove)
	if m.enabled {
		m.due(now, &spawn)
	}
	m.unlock()
	// Outside the lock: the Spawner may report back at once.
	for _, f := range remove {
		m.removeNow(f)
	}
	if h, ok := m.spawner.(Holder); ok {
		for _, f := range holds {
			h.Hold(f, f.Held)
		}
	}
	for _, f := range spawn {
		m.spawner.Spawn(f)
	}
}

// extend asks the Source for the hours up to now+Horizon not asked yet.
func (m *TrafficManager) extend(now time.Time) {
	m.extendOverflights(now)
	if m.opts.Source == nil || len(m.airports) == 0 {
		return
	}
	from := now.Truncate(time.Hour)
	if m.until.After(from) {
		from = m.until
	}
	var airports []string
	for a := range m.airports {
		airports = append(airports, a)
	}
	sort.Strings(airports)
	for ; from.Before(now.Add(m.opts.Horizon)); from = from.Add(time.Hour) {
		m.addAt(m.opts.Source(from, from.Add(time.Hour), airports), now)
		m.until = from.Add(time.Hour)
	}
}

// expire handles time running out: late flights are cancelled, spawns
// unanswered fail, departed and parked aircraft are removed, old flights
// leave the boards.
func (m *TrafficManager) expire(now time.Time, remove *[]ManagedFlight) {
	o := m.opts
	for k, f := range m.flights {
		switch f.Status {
		case FlightScheduled:
			if f.Observed != nil {
				break // a real aircraft goes when the feed drops it (#841)
			}
			if f.Departure() && now.After(later(f.STD, f.Estimated).Add(o.DepartureLate)) && !m.waitsForTurn(f) ||
				f.Arrival() && now.After(later(f.STA, f.Estimated).Add(-m.arrivalLead(&f.Flight)).Add(o.ArrivalLate)) ||
				f.Overflight() && now.After(f.Exit.Add(-5*time.Minute)) {
				f.Err = "too late"
				m.set(f, FlightCancelled, now)
				m.unpair(f)
			}
		case FlightSpawning:
			if now.Sub(f.Since) > o.SpawnTimeout {
				m.failed(f, errors.New("the simulator did not create it"), now, remove)
			}
		case FlightDeparted, FlightEnroute:
			m.leaving(f, now, remove)
		case FlightParked:
			if f.TurnTo == "" && (now.Sub(f.Since) >= o.RemoveParkedAfter || f.dropped) {
				m.set(f, FlightDone, now)
				*remove = append(*remove, *f)
			}
		case FlightDone, FlightCancelled:
			if now.Sub(f.Since) > o.Keep {
				delete(m.flights, k)
			}
		}
	}
}

// waitsForTurn reports whether a departure still waits for its arrival
// (landing late delays it, as it does in life).
func (m *TrafficManager) waitsForTurn(d *ManagedFlight) bool {
	a := m.turnFrom(d)
	return a != nil && a.Status.active()
}

func (m *TrafficManager) turnFrom(d *ManagedFlight) *ManagedFlight {
	if d.TurnFrom == "" {
		return nil
	}
	return m.flights["arrival "+d.TurnFrom]
}

func (m *TrafficManager) turnTo(a *ManagedFlight) *ManagedFlight {
	if a.TurnTo == "" {
		return nil
	}
	return m.flights["departure "+a.TurnTo]
}

// unpair breaks f's turnaround link.
func (m *TrafficManager) unpair(f *ManagedFlight) {
	if d := m.turnTo(f); d != nil {
		d.TurnFrom = ""
	}
	if a := m.turnFrom(f); a != nil {
		a.TurnTo = ""
	}
	f.TurnTo, f.TurnFrom = "", ""
}

// due spawns the flights whose time has come, in time order, within the
// limits and spacing.
func (m *TrafficManager) due(now time.Time, spawn *[]ManagedFlight) {
	o := m.opts
	total, at := 0, map[string]int{}
	var ready []*ManagedFlight
	for _, f := range m.flights {
		if f.Status.active() {
			total++
			if f.Status != FlightDeparted {
				at[f.Airport]++
			}
		}
		if f.Status != FlightScheduled || now.Before(f.retryAt) {
			continue
		}
		if start, _ := m.start(f, now); !now.Before(start) {
			ready = append(ready, f)
		}
	}
	overflying := 0
	for _, f := range m.flights {
		if f.Overflight() && f.Status.active() {
			overflying++
		}
	}
	sort.Slice(ready, func(i, j int) bool {
		return lessFlight(ready[i].focusTime(), ready[i].Callsign, ready[j].focusTime(), ready[j].Callsign)
	})
	for _, f := range ready {
		adopt := false
		if a := m.turnFrom(f); a != nil {
			switch {
			case a.Status == FlightParked:
				adopt = true
			case a.Status.active() || a.Status == FlightScheduled || a.Status == FlightSpawning:
				continue // not on the stand yet
			default:
				m.unpair(f) // the arrival never made it: a fresh aircraft
			}
		}
		if f.Overflight() {
			if overflying >= o.MaxOverflights || total >= o.MaxAircraft {
				continue
			}
			overflying++
			total++
			f.Attempts++
			f.Stage = "enroute"
			m.set(f, FlightSpawning, now)
			*spawn = append(*spawn, *f)
			continue
		}
		// The adopted aircraft is already counted, and on its stand.
		if !adopt {
			if total >= o.MaxAircraft || at[f.Airport] >= o.MaxPerAirport {
				continue
			}
			spacing := o.ArrivalSpacing
			if f.Departure() {
				spacing = o.DepartureSpacing
			}
			// A real aircraft is where it is: no spacing (#841).
			if last, ok := m.last[f.key()[:1]+f.Airport]; ok && now.Sub(last) < spacing && f.Observed == nil {
				continue
			}
			m.last[f.key()[:1]+f.Airport] = now
			total++
			at[f.Airport]++
		}
		f.Attempts++
		_, f.Stage = m.start(f, now)
		m.set(f, FlightSpawning, now)
		*spawn = append(*spawn, *f)
	}
}

func (f *ManagedFlight) focusTime() time.Time {
	switch {
	case f.Departure():
		return f.STD
	case f.Overflight():
		return f.Enter
	}
	return f.STA
}

func (m *TrafficManager) set(f *ManagedFlight, s FlightStatus, now time.Time) {
	if f.Status != s {
		prev := f.Status
		f.Status, f.Since = s, now
		m.emitPrev(EventStatus, f, prev, now, "")
	}
}

// find returns the managed flight of a call sign: the active one when it
// is both an arrival and a departure.
func (m *TrafficManager) find(callsign string) *ManagedFlight {
	d, a := m.flights["departure "+callsign], m.flights["arrival "+callsign]
	if d == nil && a == nil {
		return m.flights["overflight "+callsign]
	}
	switch {
	case d == nil:
		return a
	case a == nil:
		return d
	case a.Status.active() && !d.Status.active():
		return a
	}
	return d
}

// Update reports a flight's progress (the Spawner's view of its
// controller). A departure boarding in a turnaround ends its arrival.
func (m *TrafficManager) Update(callsign string, s FlightStatus, now time.Time) {
	m.mu.Lock()
	defer m.unlock()
	f := m.find(callsign)
	if f == nil || f.Status == FlightDone || f.Status == FlightCancelled {
		return
	}
	m.set(f, s, now)
	f.Err = ""
	if s >= FlightApproaching && f.Arrival() {
		f.Stage = "" // handed over at the STAR entry
	}
	if f.Departure() && s >= FlightBoarding {
		if a := m.turnFrom(f); a != nil && a.Status == FlightParked {
			m.set(a, FlightDone, now) // the same aircraft flies on
		}
	}
}

// Describe records what the Spawner chose for a flight.
func (m *TrafficManager) Describe(callsign, model, stand, runway string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if f := m.find(callsign); f != nil {
		f.Model, f.Stand, f.Runway = model, stand, runway
	}
}

// Failed reports that a flight could not be spawned or its controller
// ended in an error. A spawn is tried again (another model or stand) up to
// MaxAttempts; a flight already flying is cancelled and removed.
func (m *TrafficManager) Failed(callsign string, err error, now time.Time) {
	m.mu.Lock()
	var remove []ManagedFlight
	if f := m.find(callsign); f != nil {
		m.failed(f, err, now, &remove)
	}
	m.unlock()
	for _, f := range remove {
		m.removeNow(f)
	}
}

func (m *TrafficManager) failed(f *ManagedFlight, err error, now time.Time, remove *[]ManagedFlight) {
	f.Err = err.Error()
	switch {
	case f.Status == FlightSpawning && f.Arrival() && f.Stage == "enroute":
		// No enroute part: the arrival appears at its STAR entry instead,
		// no attempt lost.
		m.set(f, FlightScheduled, now)
		f.noEnroute, f.Stage, f.Attempts = true, "", f.Attempts-1
		f.retryAt = now.Add(m.opts.RetryAfter)
		m.emit(EventRetry, f, now, "enroute: "+err.Error()+"; at the STAR entry instead")
	case f.Status == FlightSpawning && errors.Is(err, ErrSpawnBlocked):
		// Its place is taken: wait, it is no failed attempt.
		m.set(f, FlightScheduled, now)
		f.retryAt, f.Note, f.Err = now.Add(m.opts.RetryAfter), err.Error(), ""
		f.Attempts--
		delete(m.last, f.key()[:1]+f.Airport) // nothing appeared: no spacing
		m.emit(EventBlocked, f, now, err.Error())
	case f.Status == FlightSpawning && errors.Is(err, ErrSpawnImpossible):
		// Never possible: cancelled now (live, DLH112's overflight tried
		// again with a plan that never enters the area).
		m.set(f, FlightCancelled, now)
		m.unpair(f)
	case f.Status == FlightSpawning && f.Attempts < m.opts.MaxAttempts:
		m.set(f, FlightScheduled, now)
		f.retryAt = now.Add(m.opts.RetryAfter)
		m.emit(EventRetry, f, now, err.Error())
	case f.Status == FlightSpawning:
		m.set(f, FlightCancelled, now)
		m.unpair(f)
	case f.Status.active():
		m.set(f, FlightCancelled, now)
		m.unpair(f)
		*remove = append(*remove, *f)
	}
}

// Remove takes a flight out now (e.g. removed on the map): its aircraft is
// removed and it is done; a turnaround departure then starts fresh.
func (m *TrafficManager) Remove(callsign string, now time.Time) {
	m.mu.Lock()
	var remove []ManagedFlight
	if f := m.find(callsign); f != nil && f.Status != FlightDone && f.Status != FlightCancelled {
		if f.Status.active() {
			remove = append(remove, *f)
		}
		m.set(f, FlightDone, now)
		m.unpair(f)
	}
	m.unlock()
	for _, f := range remove {
		m.removeNow(f)
	}
}

// Flights returns the managed flights, by their time at their airport.
func (m *TrafficManager) Flights() []ManagedFlight {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ManagedFlight, 0, len(m.flights))
	for _, f := range m.flights {
		out = append(out, *f)
	}
	sort.Slice(out, func(i, j int) bool {
		return lessFlight(out[i].focusTime(), out[i].Key(), out[j].focusTime(), out[j].Key())
	})
	return out
}

// Key identifies a managed flight: its kind and call sign.
func (f ManagedFlight) Key() string { return f.key() }

// Board is an airport's departures and arrivals, by time.
func (m *TrafficManager) Board(icao string) (departures, arrivals []ManagedFlight) {
	for _, f := range m.Flights() {
		if f.Airport != icao {
			continue
		}
		if f.Departure() {
			departures = append(departures, f)
		} else {
			arrivals = append(arrivals, f)
		}
	}
	return departures, arrivals
}

// Active counts the flights with an aircraft in the simulator.
func (m *TrafficManager) Active() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, f := range m.flights {
		if f.Status.active() {
			n++
		}
	}
	return n
}

// SetLimits changes MaxAircraft and MaxPerAirport (0 keeps one).
func (m *TrafficManager) SetLimits(maxAircraft, maxPerAirport int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if maxAircraft > 0 {
		m.opts.MaxAircraft = maxAircraft
	}
	if maxPerAirport > 0 {
		m.opts.MaxPerAirport = maxPerAirport
	}
}

// check runs the situation checks at every managed airport and applies
// their advice; it returns the departures whose hold changed.
func (m *TrafficManager) check(now time.Time, remove *[]ManagedFlight) []ManagedFlight {
	if len(m.opts.Checks) == 0 {
		return nil
	}
	by := map[string][]ManagedFlight{}
	for _, f := range m.flights {
		by[f.Airport] = append(by[f.Airport], *f)
	}
	hold := map[string]string{}
	estimated := map[string]bool{}
	for icao, fs := range by {
		sort.Slice(fs, func(i, j int) bool { return lessFlight(fs[i].focusTime(), fs[i].Key(), fs[j].focusTime(), fs[j].Key()) })
		sit := Situation{Now: now, Airport: icao, Flights: fs, Options: m.opts, Others: othersAt(m.opts.Picture, m.opts.Others, icao)}
		if m.opts.Conditions != nil && icao != "" {
			sit.Conditions, _ = m.opts.Conditions(icao)
		}
		if p := m.opts.Picture; p != nil {
			for _, a := range p.Airports() {
				if a.ICAO == icao {
					sit.Position = a.Position
				}
			}
		}
		for _, c := range m.opts.Checks {
			for _, a := range c(sit) {
				f := m.flights[a.Key]
				if f == nil {
					continue
				}
				switch a.Action {
				case AdviceDelay:
					// A real arrival is in the air already: the sequence absorbs it (#841).
					if f.Status == FlightScheduled && a.Until.After(f.retryAt) && !(f.Observed != nil && f.Arrival()) {
						// Announced when it moves a minute or more (the prediction
						// drifts by seconds each tick).
						if a.Until.Sub(f.retryAt) >= time.Minute || f.Note != a.Reason {
							defer m.emit(EventDelayed, f, now, a.Reason)
						}
						f.retryAt, f.Note = a.Until, a.Reason
					}
				case AdviceHold:
					if f.Status == FlightBoarding {
						hold[a.Key] = a.Reason
					}
				case AdviceEstimate:
					// An estimate moves by whole minutes: a prediction that drifts
					// by seconds each tick is not news.
					if f.Estimated.IsZero() || absDuration(f.Estimated.Sub(a.Until)) >= time.Minute {
						f.Estimated = a.Until.Truncate(time.Minute)
						defer m.emit(EventEstimated, f, now, a.Reason) // with the new time
					}
					estimated[a.Key] = true
					if a.Reason != "" {
						f.Note = a.Reason
					}
				case AdviceRemove:
					if f.Status.active() {
						f.Err = a.Reason
						m.set(f, FlightCancelled, now)
						m.unpair(f)
						*remove = append(*remove, *f)
					}
				}
			}
		}
	}
	var changed []ManagedFlight
	for k, f := range m.flights {
		reason, held := hold[k]
		if held != f.Held {
			f.Held = held
			if held {
				f.Note = reason
				m.emit(EventHeld, f, now, reason)
			} else {
				f.Note = ""
				m.emit(EventReleased, f, now, "")
			}
			changed = append(changed, *f)
		}
		if !estimated[k] && f.Status == FlightScheduled && !f.Estimated.IsZero() {
			f.Estimated = time.Time{} // on time again
			m.emit(EventEstimated, f, now, "on time")
		}
	}
	return changed
}

func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
