package airport

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Default SimConnect IDs of a ProcedureLoader: three definition IDs and
// three request IDs per airport being loaded (procedureSlots at a time).
const (
	DefaultProcedureDefinitionBase uint32 = 8400
	DefaultProcedureRequestBase    uint32 = 8500
	// DefaultProcedureTimeout ends a procedure load the simulator never
	// finished (an airport it does not know sends nothing).
	DefaultProcedureTimeout = 30 * time.Second
	procedureSlots          = 8
	procedureParts          = 3
)

// ProcedureLoader loads airports' departures, arrivals and approaches
// through the facility API (#312). Call Request, feed every message to
// Handle; it returns the Procedures once all three parts have arrived.
// Call Expire now and then to end loads the simulator never finishes
// (Request also frees their slots, so they never block new loads).
type ProcedureLoader struct {
	client           FacilityClient
	defBase, reqBase uint32
	timeout          time.Duration
	registered       bool

	mu    sync.Mutex
	slots [procedureSlots]*procState
	// timedOut are the airports ended inside Request, for the next Expire.
	timedOut []string
}

type procState struct {
	icao    string
	pending int
	// deadline: when the load is given up; for an expired one, when its
	// slot is free again even without the late replies.
	deadline time.Time
	// expired: given up while replies may still come; its request IDs stay
	// out of use until they have, so a late reply never lands in another
	// airport's load (#45).
	expired bool
	// Records by unique request ID (children name their parent's), and the
	// order they came in.
	procs     map[uint32]*Procedure
	isDep     map[uint32]bool
	procOrder []uint32
	approx    map[uint32]*Approach
	apprOrder []uint32
	trans     map[uint32]*Transition
	links     []procLink
	magVar    float64
}

// procLink ties a transition to its procedure or approach.
type procLink struct {
	child, parent uint32
	runway        bool // a runway transition (else enroute, or an approach transition)
}

// NewProcedureLoader creates a loader sending its requests through client.
func NewProcedureLoader(client FacilityClient) *ProcedureLoader {
	return NewProcedureLoaderWithIDs(client, DefaultProcedureDefinitionBase, DefaultProcedureRequestBase)
}

// NewProcedureLoaderWithIDs is NewProcedureLoader on its own definition and
// request ID bases, for two loaders on one connection (an application and
// the traffic World, #710). It uses defBase to defBase+2 and reqBase to
// reqBase+23.
func NewProcedureLoaderWithIDs(client FacilityClient, defBase, reqBase uint32) *ProcedureLoader {
	return &ProcedureLoader{client: client, defBase: defBase, reqBase: reqBase, timeout: DefaultProcedureTimeout}
}

// SetTimeout sets how long a load may take before it is ended.
func (l *ProcedureLoader) SetTimeout(d time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.timeout = d
}

// Reset forgets registered definitions and loads in flight, e.g. after the
// simulator reconnects. A non-nil client replaces the current one.
func (l *ProcedureLoader) Reset(client FacilityClient) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if client != nil {
		l.client = client
	}
	l.registered = false
	l.slots = [procedureSlots]*procState{}
	l.timedOut = nil
}

// Pending returns the airports whose procedures are loading.
func (l *ProcedureLoader) Pending() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, s := range l.slots {
		if s != nil && !s.expired {
			out = append(out, s.icao)
		}
	}
	return out
}

// Expire ends the loads not finished by their deadline and returns their
// airports (also those Request ended to make room). A slot stays out of
// use until the late replies are in, or for another timeout.
func (l *ProcedureLoader) Expire(now time.Time) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.expire(now)
	out := l.timedOut
	l.timedOut = nil
	return out
}

func (l *ProcedureLoader) expire(now time.Time) {
	for i, s := range l.slots {
		switch {
		case s == nil || !now.After(s.deadline):
		case s.expired:
			l.slots[i] = nil
		default:
			// Bounded for a caller that never calls Expire.
			if len(l.timedOut) >= 64 {
				l.timedOut = l.timedOut[1:]
			}
			l.timedOut = append(l.timedOut, s.icao)
			s.expired, s.deadline = true, now.Add(l.timeout)
		}
	}
}

// procedureDefinitions are the three facility definitions: departures,
// arrivals and approaches with their transitions and legs.
func procedureDefinitions() [][]string {
	legs := func(kind string) []string {
		return append(append([]string{"OPEN " + kind}, legFields...), "CLOSE "+kind)
	}
	proc := func(kind string) []string {
		d := []string{"OPEN AIRPORT", "MAGVAR", "OPEN " + kind, "NAME", "N_RUNWAY_TRANSITIONS", "N_ENROUTE_TRANSITIONS", "N_APPROACH_LEGS"}
		d = append(d, legs("APPROACH_LEG")...)
		d = append(d, "OPEN RUNWAY_TRANSITION", "RUNWAY_NUMBER", "RUNWAY_DESIGNATOR", "N_APPROACH_LEGS")
		d = append(d, legs("APPROACH_LEG")...)
		d = append(d, "CLOSE RUNWAY_TRANSITION", "OPEN ENROUTE_TRANSITION", "NAME", "N_APPROACH_LEGS")
		d = append(d, legs("APPROACH_LEG")...)
		return append(d, "CLOSE ENROUTE_TRANSITION", "CLOSE "+kind, "CLOSE AIRPORT")
	}
	appr := []string{"OPEN AIRPORT", "OPEN APPROACH", "TYPE", "SUFFIX", "RUNWAY_NUMBER", "RUNWAY_DESIGNATOR",
		"N_TRANSITIONS", "N_FINAL_APPROACH_LEGS", "N_MISSED_APPROACH_LEGS", "OPEN APPROACH_TRANSITION", "TYPE", "NAME", "N_APPROACH_LEGS"}
	appr = append(appr, legs("APPROACH_LEG")...)
	appr = append(appr, "CLOSE APPROACH_TRANSITION")
	appr = append(appr, legs("FINAL_APPROACH_LEG")...)
	appr = append(appr, legs("MISSED_APPROACH_LEG")...)
	appr = append(appr, "CLOSE APPROACH", "CLOSE AIRPORT")
	return [][]string{proc("DEPARTURE"), proc("ARRIVAL"), appr}
}

// Request starts loading an airport's procedures.
func (l *ProcedureLoader) Request(icao string) error {
	icao = strings.ToUpper(strings.TrimSpace(icao))
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.registered {
		for i, def := range procedureDefinitions() {
			for _, f := range def {
				if err := l.client.AddToFacilityDefinition(l.defBase+uint32(i), f); err != nil {
					return err
				}
			}
		}
		l.registered = true
	}
	slot := -1
	for i, s := range l.slots {
		if s == nil {
			slot = i
			break
		}
	}
	if slot < 0 {
		// Loads the simulator never finished must not block new ones (#46).
		l.expire(time.Now())
		for i, s := range l.slots {
			if s == nil {
				slot = i
				break
			}
		}
	}
	if slot < 0 {
		return fmt.Errorf("airport: %d procedure loads already pending", procedureSlots)
	}
	st := &procState{icao: icao, pending: procedureParts, deadline: time.Now().Add(l.timeout), procs: map[uint32]*Procedure{},
		isDep: map[uint32]bool{}, approx: map[uint32]*Approach{}, trans: map[uint32]*Transition{}}
	l.slots[slot] = st
	for part := 0; part < procedureParts; part++ {
		if err := l.client.RequestFacilityData(l.defBase+uint32(part), l.reqBase+uint32(slot*procedureParts+part), icao, ""); err != nil {
			l.slots[slot] = nil
			if part > 0 {
				// The parts already sent may still be answered.
				st.pending, st.expired = part, true
				l.slots[slot] = st
			}
			return err
		}
	}
	return nil
}

func (l *ProcedureLoader) lookup(req uint32) (*procState, int, int, bool) {
	if req < l.reqBase || req >= l.reqBase+procedureSlots*procedureParts {
		return nil, 0, 0, false
	}
	n := int(req - l.reqBase)
	s := l.slots[n/procedureParts]
	return s, n / procedureParts, n % procedureParts, s != nil
}

// Handle consumes the loader's facility messages; when an airport's three
// parts are complete it returns its Procedures and true.
func (l *ProcedureLoader) Handle(msg engine.Message) (Procedures, bool) {
	if msg.SIMCONNECT_RECV == nil {
		return Procedures{}, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	switch types.SIMCONNECT_RECV_ID(msg.DwID) {
	case types.SIMCONNECT_RECV_ID_FACILITY_DATA:
		m := msg.AsFacilityData()
		if st, _, part, ok := l.lookup(uint32(m.UserRequestId)); ok && !st.expired {
			n := int(m.DwSize) - int(unsafe.Offsetof(m.Data))
			if n > 0 {
				st.add(m, part, unsafe.Slice((*byte)(unsafe.Pointer(&m.Data)), n))
			}
		}
	case types.SIMCONNECT_RECV_ID_FACILITY_DATA_END:
		m := msg.AsFacilityDataEnd()
		st, slot, _, ok := l.lookup(uint32(m.RequestId))
		if !ok {
			return Procedures{}, false
		}
		if st.pending--; st.pending > 0 {
			return Procedures{}, false
		}
		l.slots[slot] = nil
		if st.expired {
			return Procedures{}, false // the late replies are in: the slot is free again
		}
		return st.finish(), true
	}
	return Procedures{}, false
}

// add stores one record; legs go straight to their parent, transitions
// are linked to theirs in finish.
func (s *procState) add(m *types.SIMCONNECT_RECV_FACILITY_DATA, part int, b []byte) {
	r := recordReader{b: b}
	id, parent := uint32(m.UniqueRequestId), uint32(m.ParentUniqueRequestId)
	switch m.Type {
	case types.SIMCONNECT_FACILITY_DATA_AIRPORT:
		// MAGVAR from the departures request only: the approaches one opens
		// AIRPORT without fields, whatever bytes follow it (E17).
		if part == 0 && len(b) >= 4 {
			s.magVar = r.f32()
		}
	case types.SIMCONNECT_FACILITY_DATA_DEPARTURE, types.SIMCONNECT_FACILITY_DATA_ARRIVAL:
		s.procs[id] = &Procedure{Name: r.str(8), Legs: []Leg{}, RunwayTransitions: []Transition{}, EnrouteTransitions: []Transition{}}
		s.isDep[id] = m.Type == types.SIMCONNECT_FACILITY_DATA_DEPARTURE
		s.procOrder = append(s.procOrder, id)
	case types.SIMCONNECT_FACILITY_DATA_RUNWAY_TRANSITION:
		s.trans[id] = &Transition{Runway: runwayName(r.i32(), r.i32()), Legs: []Leg{}}
		s.links = append(s.links, procLink{id, parent, true})
	case types.SIMCONNECT_FACILITY_DATA_ENROUTE_TRANSITION:
		s.trans[id] = &Transition{Name: r.str(8), Legs: []Leg{}}
		s.links = append(s.links, procLink{id, parent, false})
	case types.SIMCONNECT_FACILITY_DATA_APPROACH:
		t := types.SIMCONNECT_FACILITY_APPROACH_TYPE(r.i32())
		suffix := ""
		if c := r.i32(); c >= 'A' && c <= 'Z' {
			suffix = string(rune(c))
		}
		rwy := runwayName(r.i32(), r.i32())
		s.approx[id] = &Approach{Type: t, Runway: rwy, Suffix: suffix, Name: approachName(t, rwy, suffix),
			Transitions: []Transition{}, Final: []Leg{}, Missed: []Leg{}}
		s.apprOrder = append(s.apprOrder, id)
	case types.SIMCONNECT_FACILITY_DATA_APPROACH_TRANSITION:
		_ = r.i32() // TYPE
		s.trans[id] = &Transition{Name: r.str(8), Legs: []Leg{}}
		s.links = append(s.links, procLink{id, parent, false})
	case types.SIMCONNECT_FACILITY_DATA_APPROACH_LEG, types.SIMCONNECT_FACILITY_DATA_FINAL_APPROACH_LEG, types.SIMCONNECT_FACILITY_DATA_MISSED_APPROACH_LEG:
		leg := decodeLeg(b)
		switch {
		case s.trans[parent] != nil:
			s.trans[parent].Legs = append(s.trans[parent].Legs, leg)
		case s.procs[parent] != nil:
			s.procs[parent].Legs = append(s.procs[parent].Legs, leg)
		case s.approx[parent] != nil && m.Type == types.SIMCONNECT_FACILITY_DATA_MISSED_APPROACH_LEG:
			s.approx[parent].Missed = append(s.approx[parent].Missed, leg)
		case s.approx[parent] != nil:
			s.approx[parent].Final = append(s.approx[parent].Final, leg)
		}
	}
}

// finish links the transitions to their parents and returns the airport's
// procedures in the order the simulator sent them.
func (s *procState) finish() Procedures {
	for _, k := range s.links {
		t := *s.trans[k.child]
		switch {
		case s.procs[k.parent] != nil && k.runway:
			s.procs[k.parent].RunwayTransitions = append(s.procs[k.parent].RunwayTransitions, t)
		case s.procs[k.parent] != nil:
			s.procs[k.parent].EnrouteTransitions = append(s.procs[k.parent].EnrouteTransitions, t)
		case s.approx[k.parent] != nil:
			s.approx[k.parent].Transitions = append(s.approx[k.parent].Transitions, t)
		}
	}
	out := Procedures{ICAO: s.icao, MagVar: s.magVar, Departures: []Procedure{}, Arrivals: []Procedure{}, Approaches: []Approach{}}
	for _, id := range s.procOrder {
		if s.isDep[id] {
			out.Departures = append(out.Departures, *s.procs[id])
		} else {
			out.Arrivals = append(out.Arrivals, *s.procs[id])
		}
	}
	for _, id := range s.apprOrder {
		out.Approaches = append(out.Approaches, *s.approx[id])
	}
	return out
}
