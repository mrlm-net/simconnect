package world

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Scheduled traffic on the map (#367, #368): a schedule for the loaded
// airport (traffic.Schedule) run by a traffic.TrafficManager, which spawns
// departures on their stands before their STD and arrivals at their STAR
// entry before their STA through this spawner. Off until switched on.
//
//	GET  /api/schedule             — on/off, settings, every managed flight
//	POST /api/schedule             — {"enabled":true,"icao":"LKPR","density":1,"maxAircraft":12,"seed":1}
//	GET  /api/boards?icao=LKPR     — departures and arrivals of an airport

type scheduler struct {
	// held: the traffic held for a replay (hold.go); heldEnabled the
	// schedule as it was, given back.
	held, heldEnabled bool

	st  *state
	cc  *controlCenter
	cfg traffic.ScheduleConfig
	mgr *traffic.TrafficManager

	created time.Time // wall time, for weatherWait

	mu      sync.Mutex
	density float64
	seed    uint64
	// noIFR and noVFR switch the airline (and overflying) and the light
	// aircraft flights off; both run by default.
	noIFR, noVFR bool
	// noGen: no generated flights, only those added (POST /api/flights,
	// #738).
	noGen bool
	// offset: the airline timetable of this much later (or earlier) is
	// flown now (#738): a morning wave in the evening.
	offset   time.Duration
	airlines map[string]traffic.Airline
	focus    []string // the managed airports (the overflights avoid them)
	// Enroute aircraft (#369): by call sign once created, by request ID
	// while the simulator creates them.
	enroute map[string]*enrouteAC
	// corridor: the traffic along the user's route (#740), looked after
	// at corridorAt.
	corridor   *CorridorSettings
	corridorAt time.Time
	pending    map[uint32]*enrouteAC
	nextReq    uint32
	// realOn: real-world traffic (#841), the generator off; real its
	// aircraft by their ID.
	realOn  bool
	pacedAt time.Time // paceEnroute's last look
	// usedCS: the schedule's call signs by the hour they were made (#98).
	usedCS map[string]time.Time
	real   map[string]*realID
}

// ScheduleTiming times the scheduled traffic (Options.Schedule, #741);
// zero: the default in brackets.
type ScheduleTiming struct {
	// Horizon: how far ahead the timetable is planned (2 h).
	Horizon time.Duration
	// DepartureLead: a departure appears on its stand this long before its
	// STD (10 min); ArrivalLead: an arrival appears this long before its
	// STA to fly the STAR and approach (25 min); VFRLead: a VFR arrival,
	// near the airport (8 min).
	DepartureLead, ArrivalLead, VFRLead time.Duration
	// DepartureLate: a departure not started by STD plus this is cancelled
	// (15 min); ArrivalLate: an arrival not by STA-ArrivalLead plus this
	// (10 min).
	DepartureLate, ArrivalLate time.Duration
}

func newScheduler(st *state, cc *controlCenter) *scheduler {
	s := &scheduler{st: st, cc: cc, created: time.Now(), cfg: traffic.DefaultScheduleConfig(), density: 1, seed: uint64(time.Now().Unix()), airlines: map[string]traffic.Airline{},
		enroute: map[string]*enrouteAC{}, pending: map[uint32]*enrouteAC{}}
	for _, a := range s.cfg.Airlines {
		s.airlines[a.ICAO] = a
	}
	t := st.core.hooks.Schedule
	s.mgr = traffic.NewTrafficManager(s, traffic.ManagerOptions{Source: s.source, MaxAircraft: 12, MaxPerAirport: 12, OnEvent: s.event,
		Horizon: t.Horizon, DepartureLead: t.DepartureLead, ArrivalLead: t.ArrivalLead, VFRLead: t.VFRLead,
		DepartureLate: t.DepartureLate, ArrivalLate: t.ArrivalLate,
		Picture: cc.world, Overflights: s.overflights, Conditions: s.conditions}) // other traffic respected by default
	s.mgr.SetEnabled(false)
	return s
}

// conditions are the weather on final of an airport's arrival runway: the
// weather at the user aircraft (SimConnect reports no other), on the runway
// in use (#389).
func (s *scheduler) conditions(icao string) (traffic.ApproachConditions, bool) {
	if s.cc.weather == nil {
		return traffic.ApproachConditions{}, false
	}
	wx := s.cc.weather()
	g, err := s.st.cache.Graph(icao)
	if wx == nil || err != nil {
		return traffic.ApproachConditions{}, false
	}
	_, end, ok := g.Layout.RunwayEnd(s.cc.activeRunway(g, true))
	if !ok {
		return traffic.ApproachConditions{}, false
	}
	return traffic.ConditionsFrom(*wx, end.Heading), true
}

// source is the schedule of an hour for the managed airports; the seed per
// hour keeps it the same when asked again.
// airports are the managed airports.
func (s *scheduler) airports() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.focus...)
}

func (s *scheduler) source(from, to time.Time, focus []string) []traffic.Flight {
	s.mu.Lock()
	density, seed, noIFR, noVFR, noGen, off := s.density, s.seed, s.noIFR, s.noVFR, s.noGen, s.offset
	s.mu.Unlock()
	if noGen {
		return nil
	}
	opts := traffic.ScheduleOptions{Focus: focus, Density: density, Seed: seed ^ uint64(from.Unix()/3600)}
	for _, icao := range focus {
		if l, ok := s.st.cache.Layout(icao); ok {
			if opts.Layouts == nil {
				opts.Layouts = map[string]*airport.Layout{}
			}
			opts.Layouts[icao] = l
		}
	}
	var flights []traffic.Flight
	if !noIFR {
		// The timetable of from+offset, flown now (#738).
		// Call signs of the hours before kept out (#98).
		s.mu.Lock()
		opts.Used = map[string]bool{}
		for cs, at := range s.usedCS {
			if from.Sub(at) > usedCallsignFor {
				delete(s.usedCS, cs)
				continue
			}
			opts.Used[cs] = true
		}
		s.mu.Unlock()
		ifr := traffic.Schedule(s.cfg, opts, from.Add(off), to.Add(off))
		s.mu.Lock()
		if s.usedCS == nil {
			s.usedCS = map[string]time.Time{}
		}
		for _, f := range ifr {
			s.usedCS[f.Callsign] = from
		}
		s.mu.Unlock()
		// Business jets and turboprops at the large airports, IFR between
		// airports (#619).
		ifr = append(ifr, traffic.BusinessFlights(s.cfg, traffic.BusinessOptions{Focus: focus, Layouts: opts.Layouts, Density: density,
			Seed: opts.Seed ^ 0xb1}, from.Add(off), to.Add(off))...)
		for i := range ifr {
			ifr[i].STD, ifr[i].STA = ifr[i].STD.Add(-off), ifr[i].STA.Add(-off)
		}
		flights = ifr
	}
	// Light aircraft through the circuit, by day in visual conditions (#568).
	// Their lead is the manager's default VFRLead: Source runs under the
	// manager's lock, so its options are not asked for.
	if !noVFR {
		// On the schedule's clock like the others (E31): by day is the
		// simulator's day, not the wall clock's.
		vfr := traffic.VFRFlights(traffic.VFROptions{Focus: focus, Layouts: opts.Layouts, Density: density, Seed: opts.Seed ^ 0x7f,
			Visual: s.visual}, from.Add(off), to.Add(off))
		for i := range vfr {
			vfr[i].STD, vfr[i].STA = vfr[i].STD.Add(-off), vfr[i].STA.Add(-off)
		}
		flights = append(flights, vfr...)
	}
	return flights
}

// visual reports whether the weather at an airport allows VFR flights: a
// visibility of VFRVisibilityM and a ceiling of VFRCeilingFt or better
// (unknown counts as good). The weather is the user's (conditions).
func (s *scheduler) visual(icao string) bool {
	c, ok := s.conditions(icao)
	if !ok {
		return true
	}
	return (c.VisibilityM == 0 || c.VisibilityM >= vfrVisibilityM) && (c.CeilingFt == 0 || c.CeilingFt >= vfrCeilingFt)
}

// VFR flights need at least this visibility and ceiling (the usual VFR
// limits in a control zone).
const (
	vfrVisibilityM = 5000
	vfrCeilingFt   = 1500
)

// Overflights cross the scan range around the picture's centre (SimConnect
// sees 108 NM): six in the peak hour at density 1.
const overflightRadiusNM = 100

// overflights are the flights crossing the area in an hour (#369).
func (s *scheduler) overflights(from, to time.Time) []traffic.Flight {
	c, ok := s.cc.world.Centre()
	if !ok {
		return nil
	}
	s.mu.Lock()
	density, seed, focus, noIFR := s.density, s.seed, s.focus, s.noIFR || s.noGen
	s.mu.Unlock()
	if noIFR {
		return nil // airliners crossing the area are IFR traffic too
	}
	return traffic.Overflights(s.cfg, traffic.OverflightOptions{Centre: c, RadiusNM: overflightRadiusNM, Density: density,
		Seed: seed ^ uint64(from.Unix()/3600) ^ 0x0f, Exclude: focus}, from, to)
}

// Spawn puts a managed flight into the simulator (traffic.Spawner): the
// model of its airline and type, a stand, the runway in use, a flight plan
// from or to the other end (else a SID or STAR of the runway). A
// turnaround departure adopts its parked arrival.
func (s *scheduler) Spawn(f traffic.ManagedFlight) {
	go func() {
		spawn := func() error { return s.spawnWith(f, nil, "") }
		switch f.Stage {
		case "enroute":
			spawn = func() error { return s.spawnEnroute(f) }
		case "observed":
			spawn = func() error { return s.spawnObserved(f) } // a real aircraft in the air (#841)
		}
		if err := spawn(); err != nil {
			s.cc.log.printf("%-6s schedule: %s %s → %s (attempt %d) failed: %v", f.Callsign, f.Kind, f.Origin, f.Destination, f.Attempts, err)
			s.mgr.Failed(f.Callsign, err, s.cc.clock.Now())
		}
	}()
}

// spawnWith spawns f on its stand or at its STAR entry; pre is the
// arrival's route already planned (a handover from en route) and model its
// model, "" to choose.
func (s *scheduler) spawnWith(f traffic.ManagedFlight, pre *planned, model string) error {
	cc, st := s.cc, s.st
	g, err := st.cache.Graph(f.Airport)
	if err != nil {
		return err
	}
	if cc.core.allClosed(g.Layout) {
		return fmt.Errorf("%s: all runways closed", f.Airport) // waits for one to open (closures.go)
	}
	req := SpawnRequest{Kind: f.Kind, ICAO: f.Airport, Stand: -1, Tail: f.Callsign, Tug: true, Fuel: true, Deice: "auto"}
	vfr := f.Rules == "VFR"
	switch {
	case vfr || f.Operator == "business":
		req.StandUse = standGA // a light aircraft or a business jet (#568, #619)
	case s.airlines[airlineOf(f.Callsign)].Cargo:
		req.StandUse = standCargo
	}
	if vfr {
		// A light aircraft joining the circuit (#568): no plan, no STAR.
		req.Circuit, req.Tug, req.Fuel, req.Deice = true, false, false, ""
		if !f.Departure() {
			// Its training circuits, by its operator (VFRFlights, #565).
			req.TouchAndGos, req.StopAndGo = f.TouchAndGos, f.StopAndGo
		}
		if !s.visual(f.Airport) {
			return fmt.Errorf("%w: no VFR in this weather", traffic.ErrSpawnBlocked)
		}
	}
	// Resolved by spawn (pickRunway); a planned arrival on its plan's.
	req.Runway = "active"
	if pre != nil && pre.plan != nil && pre.plan.Request.ArrivalRunway != "" && !f.Departure() {
		req.Runway = pre.plan.Request.ArrivalRunway
	}
	if pre != nil && pre.runway != "" && !f.Departure() {
		req.Runway = pre.runway
	}
	if pre != nil && pre.adopt != 0 && !f.Departure() {
		req.adopt = pre.adopt // the en route aircraft flies on (#643)
	}
	if f.Departure() {
		req.pushAt = f.STD
		// A real aircraft parked with no departure seen (#841).
		req.parked = f.Observed != nil && f.STD.Sub(s.cc.clock.Now()) > s.mgr.Options().DepartureLead
	} else if f.TurnTo != "" {
		// Its stand away from neighbours due off when its turnaround is.
		for _, d := range s.mgr.Flights() {
			if d.Callsign == f.TurnTo && d.Departure() {
				req.offBlock = d.STD
			}
		}
	}
	// A turnaround: the arrival's aircraft on its stand.
	var turnFrom *controlled // a turnaround's arrival, forgotten once the departure is up
	if f.TurnFrom != "" {
		arr := cc.byTail(f.TurnFrom)
		var id uint32
		if arr != nil {
			arr.mu.Lock()
			id, req.Stand, req.Model = arr.objectID, arr.stand, arr.view.Model
			arr.mu.Unlock()
		}
		if id == 0 {
			return fmt.Errorf("turnaround: %s is not on its stand", f.TurnFrom)
		}
		req.adopt = id
		// The stand passes to the departure: its aircraft, detected there, is
		// then its own and not in the way (#470). Back to the arrival if the
		// departure fails (#73: the stand was freed under the parked aircraft
		// and the arrival forgotten, its object orphaned).
		arr.stands.Transfer(arr.Tail, f.Callsign)
		arr.stands.SetOffBlock(f.Callsign, f.STD)
		turnFrom = arr
	} else if model != "" {
		req.Model = model
	} else {
		a := s.airlines[f.Airline]
		models := traffic.ModelsForFlight(cc.modelList(), f.Airline, a.Name, f.Type, f.Callsign, 6)
		if len(models) == 0 {
			return fmt.Errorf("no model of a %s", f.Type)
		}
		req.Model = models[(f.Attempts-1)%len(models)] // another one on each attempt
	}
	// A departure's stand first: with parallel runways used together its
	// runway is the one nearest the stand, and the flight plan's SID follows
	// the runway. An arrival takes the less busy runway (its stand comes
	// near it). Released again if the spawn fails.
	assigned := false
	if f.Departure() && req.Stand < 0 {
		m, _, _ := strings.Cut(req.Model, liverySep)
		sr := traffic.StandRequirements{Owner: f.Callsign, Airline: airlineOf(f.Callsign), HalfSpan: traffic.ProfileFor(m).Motion.SpanMeters / 2, OffBlock: f.STD}
		// A real aircraft on the stand where it is seen (#841).
		s, err := -1, traffic.ErrNoStand
		if o := f.Observed; o != nil && o.OnGround {
			s, err = cc.allocator(g).AssignNear(sr, o.Position, realStandNearM)
		}
		// By its use (#833): gates for an airliner, GA ramps for a light
		// aircraft or a business jet (#568, #619), cargo stands for a
		// freighter; then another that fits.
		if err != nil {
			s, err = assignStand(cc.allocator(g), sr, req.StandUse)
		}
		if err != nil {
			return fmt.Errorf("%w: %v", traffic.ErrSpawnBlocked, err) // no stand free now: tried again
		}
		req.Stand, assigned = s, true
	}
	spawned := false
	defer func() {
		if turnFrom != nil && !spawned {
			turnFrom.stands.Transfer(f.Callsign, turnFrom.Tail)
		}
		if assigned && !spawned {
			cc.allocator(g).ReleaseOwner(f.Callsign)
		}
	}()
	if req.Runway == "active" {
		req.Runway = cc.pickRunway(g, !f.Departure(), req.Stand)
	}
	// The flight plan from or to the other end, else the runway's procedure.
	req.Other = f.Destination
	if !f.Departure() {
		req.Other = f.Origin
	}
	var entry []airport.NavPoint
	p, err := pre, error(nil)
	if vfr {
		// Through the circuit: no flight plan, no procedure.
	} else if pre == nil && req.Other == "" {
		err = errNoOther // a real aircraft's other end is not known (#841)
	} else if pre == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		p, err = planFor(ctx, st, g, req)
		cancel()
		if err == nil && f.Departure() && f.Observed != nil {
			flyGiven(p, f.Observed.Route) // its route after the SID (#845)
		}
		if err == nil && f.Departure() && len(f.Path) > 0 {
			flyGiven(p, f.Path) // a recorded day: its path after the SID
		}
	}
	if errors.Is(err, errNoOther) {
		// The runway's procedure, said as such.
	} else if err != nil {
		s.cc.log.printf("%-6s schedule: no flight plan with %s (%v): the runway's procedure", f.Callsign, req.Other, err)
	}
	if err != nil {
		req.Other, req.Procedure = "", true
		// Picked here, to see where it starts; the spawn flies the same one.
		if pts, name, _, err := cc.procedureFor(g, req); err == nil {
			entry, req.ProcName = pts, name
		}
	} else if p != nil {
		req.planned, entry = p, p.route
	}
	// Nobody appears on top of other traffic: an arrival waits while an
	// aircraft is near its STAR entry (the manager tries again, no attempt).
	if !f.Departure() && len(entry) > 0 && pre == nil {
		if who := s.nearEntry(entry[0]); who != "" {
			return fmt.Errorf("%w: %s near %s", traffic.ErrSpawnBlocked, who, entry[0].Ident)
		}
	}
	var it *controlled
	if err := cc.do(func() (e error) { it, e = cc.spawn(g, req); return e }); err != nil && req.Procedure && !f.Departure() {
		// No STAR for the runway: a straight-in approach.
		req.Procedure = false
		if err2 := cc.do(func() (e error) { it, e = cc.spawn(g, req); return e }); err2 != nil {
			return err
		}
	} else if errors.Is(err, traffic.ErrNoStand) || errors.Is(err, traffic.ErrStandTaken) {
		return fmt.Errorf("%w: %v", traffic.ErrSpawnBlocked, err)
	} else if err != nil {
		return err
	}
	spawned = true
	if turnFrom != nil {
		cc.forget(turnFrom)
	}
	it.mu.Lock()
	it.managed = s.mgr
	it.observed = f.Observed
	stand, runway := it.view.Stand, it.view.Runway
	it.mu.Unlock()
	s.mgr.Describe(f.Callsign, req.Model, stand, runway)
	when := "STD " + f.STD.Local().Format("15:04")
	if !f.Departure() {
		when = "STA " + f.STA.Local().Format("15:04")
	}
	s.cc.log.printf("%-6s schedule: %s %s → %s, %s, %s at %s", f.Callsign, f.Kind, orUnknown(f.Origin), orUnknown(f.Destination), f.Type, when, stand)
	return nil
}

// nearEntry names an airborne aircraft near an arrival's first point, ""
// when it is clear.
func (s *scheduler) nearEntry(p airport.NavPoint) string {
	alt := p.AltMax / 0.3048
	if alt == 0 {
		alt = p.AltMin / 0.3048
	}
	return s.nearPoint(p.Position, alt)
}

// Hold keeps a boarding departure on its stand or releases it
// (traffic.Holder): the manager's ground stop.
func (s *scheduler) Hold(f traffic.ManagedFlight, on bool) {
	if it := s.cc.byTail(f.Callsign); it != nil && it.dep != nil {
		it.dep.HoldPushback(on)
	}
}

// event logs the manager's lifecycle (traffic.ManagerOptions.OnEvent):
// what the app reacts to.
func (s *scheduler) event(e traffic.ManagerEvent) {
	f := e.Flight
	switch e.Kind {
	case traffic.EventHeld:
		s.cc.log.printf("%-6s schedule: held on the stand — %s", f.Callsign, e.Reason)
	case traffic.EventReleased:
		s.cc.log.printf("%-6s schedule: hold released", f.Callsign)
	case traffic.EventDelayed:
		s.cc.log.printf("%-6s schedule: %s delayed — %s", f.Callsign, f.Kind, e.Reason)
	case traffic.EventEstimated:
		if f.Estimated.IsZero() {
			s.cc.log.printf("%-6s schedule: on time", f.Callsign)
		} else {
			s.cc.log.printf("%-6s schedule: estimated %s (%s)", f.Callsign, f.Estimated.Local().Format("15:04"), e.Reason)
		}
	case traffic.EventBlocked:
		s.cc.log.printf("%-6s schedule: waiting — %s", f.Callsign, e.Reason)
	case traffic.EventTurnaround:
		s.cc.log.printf("schedule: turnaround %s", e.Reason)
	case traffic.EventStatus:
		switch f.Status {
		case traffic.FlightCancelled:
			s.cc.log.printf("%-6s schedule: cancelled — %s", f.Callsign, f.Err)
		case traffic.FlightDone:
			s.cc.log.printf("%-6s schedule: done", f.Callsign)
		}
	case traffic.EventEnabled, traffic.EventDisabled:
		s.cc.log.printf("schedule: %s", e.Kind)
	}
}

// Remove takes a managed flight's aircraft out (traffic.Spawner).
func (s *scheduler) Remove(f traffic.ManagedFlight) {
	if s.removeEnroute(f.Callsign) {
		s.cc.log.printf("%-6s schedule: removed (%s%s)", f.Callsign, f.Status, map[bool]string{true: ", " + f.Note}[f.Note != ""])
		return
	}
	it := s.cc.byTail(f.Callsign)
	if it == nil {
		return
	}
	s.cc.do(func() error { return s.cc.remove(it) })
	s.cc.log.printf("%-6s schedule: removed (%s)", f.Callsign, f.Status)
}

// managedStatus is the manager's status of a controller event; ok false
// when the event says nothing new.
func managedStatus(ev TaxiOrArrival) (traffic.FlightStatus, bool, error) {
	if e := ev.dep; e != nil {
		switch s := e.State; {
		case s == traffic.TaxiFailed || s == traffic.TaxiCancelled:
			return 0, false, orErr(e.Err, s.String())
		case s == traffic.TaxiAwaitingPushback:
			return traffic.FlightBoarding, true, nil
		case s >= traffic.TaxiPushback && s <= traffic.TaxiLinedUp:
			return traffic.FlightTaxiing, true, nil
		case s == traffic.TaxiDeparting:
			return traffic.FlightDeparting, true, nil
		case s == traffic.TaxiComplete:
			return traffic.FlightDeparted, true, nil
		}
	}
	if e := ev.arr; e != nil {
		switch s := e.State; {
		case s == traffic.ArrivalFailed || s == traffic.ArrivalCancelled:
			return 0, false, orErr(e.Err, s.String())
		case s == traffic.ArrivalApproaching || s == traffic.ArrivalLanding:
			return traffic.FlightApproaching, true, nil
		case s >= traffic.ArrivalRollout && s < traffic.ArrivalParked:
			return traffic.FlightLanded, true, nil
		case s == traffic.ArrivalParked:
			return traffic.FlightParked, true, nil
		}
	}
	return 0, false, nil
}

func orErr(err error, state string) error {
	if err != nil {
		return err
	}
	return errors.New(state)
}

// weatherWait is how long the schedule waits for the first weather sample
// before spawning without it (a simulator that gives none).
const weatherWait = 30 * time.Second

func (s *scheduler) tick(now time.Time) {
	// No spawn before the weather is known: the runway in use comes from it,
	// and a flight spawned on the fallback (the preferred runway) keeps that
	// runway (#458; LKPR, live: the first flights took 24, then 06).
	if s.cc.weather != nil && s.cc.weather() == nil && time.Since(s.created) < weatherWait {
		s.handovers(now)
		s.corridorTick(now)
		return
	}
	s.mgr.Tick(now)
	s.expirePending()
	s.paceEnroute(now) // no catching up the one ahead en route
	s.handovers(now)
	s.corridorTick(now)
}

type scheduleView struct {
	Enabled     bool                     `json:"enabled"`
	Airports    []string                 `json:"airports"`
	Density     float64                  `json:"density"`
	MaxAircraft int                      `json:"maxAircraft"`
	Others      traffic.OtherTrafficMode `json:"others"`
	// IFR and VFR: the airline flights (and overflights) and the light
	// aircraft flights run.
	IFR bool `json:"ifr"`
	VFR bool `json:"vfr"`
	// Generator: the generated timetable runs; false only flights added.
	Generator bool `json:"generator"`
	// OffsetMin: the airline timetable this many minutes later is flown
	// now (#738).
	OffsetMin float64                 `json:"offsetMin,omitempty"`
	Seed      uint64                  `json:"seed"`
	Active    int                     `json:"active"`
	Flights   []traffic.ManagedFlight `json:"flights"`
	// Now is the traffic time the flights' times are in (#413).
	Now time.Time `json:"now"`
}

func registerSchedule(mux *http.ServeMux, st *state) {
	sched := func(w http.ResponseWriter) *scheduler {
		st.mu.Lock()
		s := st.schedule
		st.mu.Unlock()
		if s == nil {
			http.Error(w, "simulator not connected", http.StatusServiceUnavailable)
		}
		return s
	}
	mux.HandleFunc("GET /api/schedule", func(w http.ResponseWriter, r *http.Request) {
		s := sched(w)
		if s == nil {
			return
		}
		s.mu.Lock()
		density, seed, ifr, vfr, gen, off := s.density, s.seed, !s.noIFR, !s.noVFR, !s.noGen, s.offset.Minutes()
		s.mu.Unlock() // never held while calling the manager (its Source takes it)
		v := scheduleView{Enabled: s.mgr.Enabled(), Airports: s.mgr.Airports(), Density: density, Seed: seed, IFR: ifr, VFR: vfr, Generator: gen, OffsetMin: off,
			MaxAircraft: s.mgr.Options().MaxAircraft, Others: s.mgr.Options().Others, Active: s.mgr.Active(), Flights: s.mgr.Flights(), Now: s.cc.clock.Now()}
		writeJSON(w, v)
	})
	mux.HandleFunc("POST /api/schedule", func(w http.ResponseWriter, r *http.Request) {
		s := sched(w)
		if s == nil {
			return
		}
		var req struct {
			Enabled     *bool    `json:"enabled"`
			ICAO        string   `json:"icao"`
			Airports    []string `json:"airports"` // several (#371); icao is one
			Density     float64  `json:"density"`
			MaxAircraft int      `json:"maxAircraft"`
			Seed        *uint64  `json:"seed"`
			Others      string   `json:"others"`    // respect | ignore
			IFR         *bool    `json:"ifr"`       // airline flights and overflights
			VFR         *bool    `json:"vfr"`       // light aircraft through the circuit
			Generator   *bool    `json:"generator"` // false: only flights added (POST /api/flights, #738)
			OffsetMin   *float64 `json:"offsetMin"` // the airline timetable this many minutes later flown now (#738)
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		if req.Density > 0 {
			s.density = req.Density
		}
		if req.Seed != nil {
			s.seed = *req.Seed
		}
		if req.IFR != nil {
			s.noIFR = !*req.IFR
		}
		if req.VFR != nil {
			s.noVFR = !*req.VFR
		}
		if req.Generator != nil {
			s.noGen = !*req.Generator
		}
		if req.OffsetMin != nil {
			s.offset = time.Duration(*req.OffsetMin * float64(time.Minute))
		}
		s.mu.Unlock()
		// Switched off: its flights not yet in the simulator go at once (the
		// hours already planned would otherwise still bring them); those
		// flying finish their flight.
		if req.IFR != nil && !*req.IFR || req.VFR != nil && !*req.VFR {
			now := s.cc.clock.Now()
			for _, f := range s.mgr.Flights() {
				vfr := f.Rules == "VFR"
				if f.Status == traffic.FlightScheduled && (vfr && req.VFR != nil && !*req.VFR || !vfr && req.IFR != nil && !*req.IFR) {
					s.mgr.Remove(f.Callsign, now)
				}
			}
		}
		if req.MaxAircraft > 0 {
			s.mgr.SetLimits(req.MaxAircraft, req.MaxAircraft)
		}
		if req.ICAO != "" {
			req.Airports = append(req.Airports, req.ICAO)
		}
		var airports []string
		for _, a := range req.Airports {
			icao := strings.ToUpper(strings.TrimSpace(a))
			if icao == "" || slices.Contains(airports, icao) {
				continue
			}
			// Loaded now if not yet: its stands, runways and procedures.
			if _, err := st.cache.Graph(icao); err != nil {
				ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
				_, err = st.load(ctx, icao, false, st.requests)
				cancel()
				if err != nil {
					http.Error(w, "loading "+icao+": "+err.Error(), http.StatusUnprocessableEntity)
					return
				}
			}
			airports = append(airports, icao)
		}
		if len(airports) > 0 {
			s.mgr.SetAirports(airports...)
			s.mu.Lock()
			s.focus = airports
			s.mu.Unlock()
		}
		switch req.Others {
		case "respect":
			s.mgr.SetOthers(traffic.OtherRespect)
		case "ignore":
			s.mgr.SetOthers(traffic.OtherIgnore)
		}
		if req.Enabled != nil {
			s.mgr.SetEnabled(*req.Enabled)
		}
		o := s.mgr.Options()
		s.mu.Lock()
		density, ifr, vfr := s.density, !s.noIFR, !s.noVFR
		s.mu.Unlock()
		onOff := map[bool]string{true: "on", false: "off"}
		tlog.printf("schedule: %v at %s, density %.1f, max %d aircraft, IFR %s, VFR %s, other traffic: %s", onOff[s.mgr.Enabled()],
			strings.Join(s.mgr.Airports(), ","), density, o.MaxAircraft, onOff[ifr], onOff[vfr], o.Others)
		w.WriteHeader(http.StatusNoContent)
	})
	// POST /api/flights [traffic.Flight] — flights at a chosen time (#737):
	// a callsign, origin and destination (one of them a scheduled airport),
	// the STD and STA in traffic time (GET /api/schedule's now), and an
	// airline or type; the manager spawns them as it does the timetable's.
	// Answers the flights taken; 422 when one cannot be.
	mux.HandleFunc("POST /api/flights", func(w http.ResponseWriter, r *http.Request) {
		s := sched(w)
		if s == nil {
			return
		}
		var flights []traffic.Flight
		if err := json.NewDecoder(r.Body).Decode(&flights); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		managed, now := s.mgr.Airports(), s.cc.clock.Now()
		for i := range flights {
			f := &flights[i]
			f.Callsign, f.Origin, f.Destination = strings.ToUpper(f.Callsign), strings.ToUpper(f.Origin), strings.ToUpper(f.Destination)
			if f.Airline == "" && len(f.Callsign) > 3 {
				f.Airline = f.Callsign[:3]
			}
			switch {
			case f.Callsign == "" || f.Origin == "" || f.Destination == "":
				http.Error(w, fmt.Sprintf("flight %d: callsign, origin and destination are needed", i), http.StatusUnprocessableEntity)
				return
			case !slices.Contains(managed, f.Origin) && !slices.Contains(managed, f.Destination):
				http.Error(w, fmt.Sprintf("%s: neither %s nor %s is a scheduled airport (%s)", f.Callsign, f.Origin, f.Destination, strings.Join(managed, ",")), http.StatusUnprocessableEntity)
				return
			case slices.Contains(managed, f.Origin) && f.STD.IsZero(), slices.Contains(managed, f.Destination) && f.STA.IsZero():
				http.Error(w, f.Callsign+": an STD from a scheduled airport and an STA to one are needed", http.StatusUnprocessableEntity)
				return
			}
			if f.Type == "" {
				f.Type = "A320"
			}
			// An arrival appears ArrivalLead before its STA to fly the STAR
			// and approach: one added later than that (and ArrivalLate) is
			// refused here rather than cancelled at once.
			o := s.mgr.Options()
			lead := o.ArrivalLead
			if f.Rules == "VFR" {
				lead = o.VFRLead
			}
			if slices.Contains(managed, f.Destination) && now.After(f.STA.Add(-lead).Add(o.ArrivalLate)) {
				http.Error(w, fmt.Sprintf("%s: STA %s is too soon: an arrival is added at least %v before its STA (it appears %v before, to fly the STAR and approach)",
					f.Callsign, f.STA.In(now.Location()).Format("15:04"), lead-o.ArrivalLate, lead), http.StatusUnprocessableEntity)
				return
			}
		}
		s.mgr.Add(flights)
		for _, f := range flights {
			tlog.printf("%-6s schedule: added %s → %s, %s, STD %s, STA %s", f.Callsign, f.Origin, f.Destination, f.Type, f.STD.In(now.Location()).Format("15:04"), f.STA.In(now.Location()).Format("15:04"))
		}
		writeJSON(w, flights)
	})
	// GET /api/flights — the manager's flights (as GET /api/schedule's).
	mux.HandleFunc("GET /api/flights", func(w http.ResponseWriter, r *http.Request) {
		s := sched(w)
		if s == nil {
			return
		}
		writeJSON(w, s.mgr.Flights())
	})
	mux.HandleFunc("GET /api/boards", func(w http.ResponseWriter, r *http.Request) {
		s := sched(w)
		if s == nil {
			return
		}
		deps, arrs := s.mgr.Board(strings.ToUpper(r.URL.Query().Get("icao")))
		if deps == nil {
			deps = []traffic.ManagedFlight{}
		}
		if arrs == nil {
			arrs = []traffic.ManagedFlight{}
		}
		writeJSON(w, map[string]any{"departures": deps, "arrivals": arrs})
	})
}

// usedCallsignFor: a scheduled call sign is not given again for this long.
const usedCallsignFor = 12 * time.Hour
