package airport

import (
	"fmt"
	"strings"
	"sync"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Default SimConnect IDs of a ProcedureLoader: three definition IDs and
// three request IDs per airport being loaded (procedureSlots at a time).
const (
	DefaultProcedureDefinitionBase uint32 = 8400
	DefaultProcedureRequestBase    uint32 = 8500
	procedureSlots                        = 8
	procedureParts                        = 3
)

// ProcedureLoader loads airports' departures, arrivals and approaches
// through the facility API (#312). Call Request, feed every message to
// Handle; it returns the Procedures once all three parts have arrived.
type ProcedureLoader struct {
	client           FacilityClient
	defBase, reqBase uint32
	registered       bool

	mu    sync.Mutex
	slots [procedureSlots]*procState
}

type procState struct {
	icao    string
	pending int
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
	return &ProcedureLoader{client: client, defBase: defBase, reqBase: reqBase}
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
		return fmt.Errorf("airport: %d procedure loads already pending", procedureSlots)
	}
	l.slots[slot] = &procState{icao: icao, pending: procedureParts, procs: map[uint32]*Procedure{}, isDep: map[uint32]bool{},
		approx: map[uint32]*Approach{}, trans: map[uint32]*Transition{}}
	for part := 0; part < procedureParts; part++ {
		if err := l.client.RequestFacilityData(l.defBase+uint32(part), l.reqBase+uint32(slot*procedureParts+part), icao, ""); err != nil {
			l.slots[slot] = nil
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
		if st, _, _, ok := l.lookup(uint32(m.UserRequestId)); ok {
			n := int(m.DwSize) - int(unsafe.Offsetof(m.Data))
			if n > 0 {
				st.add(m, unsafe.Slice((*byte)(unsafe.Pointer(&m.Data)), n))
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
		return st.finish(), true
	}
	return Procedures{}, false
}

// add stores one record; legs go straight to their parent, transitions
// are linked to theirs in finish.
func (s *procState) add(m *types.SIMCONNECT_RECV_FACILITY_DATA, b []byte) {
	r := recordReader{b: b}
	id, parent := uint32(m.UniqueRequestId), uint32(m.ParentUniqueRequestId)
	switch m.Type {
	case types.SIMCONNECT_FACILITY_DATA_AIRPORT:
		if len(b) >= 4 {
			s.magVar = r.f32() // MAGVAR, on the departures request
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
