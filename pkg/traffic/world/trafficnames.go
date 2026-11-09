package world

import (
	"github.com/mrlm-net/simconnect/pkg/jetway"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// The live traffic scan reads, every second, every aircraft within
// trafficRadius: positions, flight values and the AI state. An aircraft's
// title and ATC ID never change, so they are read by a scan of their own
// only when the second's scan holds an aircraft not named yet, not every
// second (each read is the simulator's main thread's, for every aircraft
// within 200 km, MSFS AI and online traffic included).
//
// An aircraft not named yet is left out of the second's scan until its
// names arrive: the scan waits for them (a frame or two) and is handed on
// with them. Were they lost, the next second's scan goes on without the
// unnamed ones and asks again.
const (
	defTrafficNames uint32 = 2030
	reqTrafficNames uint32 = 2031
)

// defineTrafficScan defines the live traffic scan: AI state, position and
// flight values every second (defTraffic, trafficRaw); title, tail and
// wing span once per aircraft (defTrafficNames, trafficNameRaw).
func defineTrafficScan(client interface {
	AddToDataDefinition(definitionID uint32, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID uint32) error
}) {
	client.AddToDataDefinition(defTrafficNames, "TITLE", "", types.SIMCONNECT_DATATYPE_STRING256, 0, 0)
	client.AddToDataDefinition(defTrafficNames, "ATC ID", "", types.SIMCONNECT_DATATYPE_STRING32, 0, 1)
	client.AddToDataDefinition(defTrafficNames, "WING SPAN", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 2)
	client.AddToDataDefinition(defTraffic, "AI TRAFFIC STATE", "", types.SIMCONNECT_DATATYPE_STRING256, 0, 2)
	for i, v := range []struct{ name, unit string }{
		{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"}, {"PLANE ALT ABOVE GROUND", "feet"},
		{"GROUND VELOCITY", "knots"}, {"PLANE HEADING DEGREES TRUE", "degrees"}, {"VERTICAL SPEED", "feet per minute"},
		{"SIM ON GROUND", "bool"}, {"GEAR TOTAL PCT EXTENDED", "percent"}, // native 0–1; "percent" returns it unscaled
		{"LIGHT LANDING", "bool"}, {"LIGHT TAXI", "bool"}, {"LIGHT STROBE", "bool"}, {"LIGHT BEACON", "bool"}, {"LIGHT NAV", "bool"},
		{"PLANE ALTITUDE", "feet"},
	} {
		client.AddToDataDefinition(defTraffic, v.name, v.unit, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i+3))
	}
}

// trafficNameRaw matches the defTrafficNames data definition: what never
// changes about an aircraft (its wing span too).
type trafficNameRaw struct {
	Title  [256]byte
	AtcID  [32]byte
	SpanFt float64
}

// trafficNames keeps the names of the aircraft scanned, by object ID.
type trafficNames struct {
	names map[uint32]trafficName
	// stash: a second's scan waiting for its names; waiting: a names scan
	// asked for and not answered yet.
	stash   []Traffic
	waiting bool
}

type trafficName struct {
	title, tail string
	spanM       float64
}

func newTrafficNames() *trafficNames { return &trafficNames{names: map[uint32]trafficName{}} }

// fill names the aircraft of scan known by name, leaving out the jetway
// helpers (#572: invisible, not traffic) and those not named yet.
func (n *trafficNames) fill(scan []Traffic) (out []Traffic, unknown int) {
	out = make([]Traffic, 0, len(scan))
	for _, t := range scan {
		nm, ok := n.names[t.ObjectID]
		if !ok {
			unknown++
			continue
		}
		if jetway.IsHelper(nm.title) {
			continue
		}
		t.Title, t.Tail, t.Span = nm.title, nm.tail, nm.spanM
		out = append(out, t)
	}
	return out, unknown
}

// complete takes a second's whole scan: what to hand on now (nil: wait for
// the names), and whether to ask for the names.
func (n *trafficNames) complete(scan []Traffic) (feed []Traffic, ask bool) {
	out, unknown := n.fill(scan)
	switch {
	case unknown == 0:
		n.stash, n.waiting = nil, false
		return out, false
	case n.waiting:
		// The last names scan never answered: on without the unnamed ones.
		n.stash = nil
		return out, true
	}
	n.stash, n.waiting = scan, true
	return nil, true
}

// name records one answer of the names scan.
func (n *trafficNames) name(id uint32, title, tail string, spanM float64) {
	n.names[id] = trafficName{title, tail, spanM}
}

// namesDone ends a names scan: the scan that waited for it, named, if any.
func (n *trafficNames) namesDone() ([]Traffic, bool) {
	n.waiting = false
	if n.stash == nil {
		return nil, false
	}
	out, _ := n.fill(n.stash)
	n.stash = nil
	return out, true
}

// forget drops an aircraft gone from the scans.
func (n *trafficNames) forget(id uint32) { delete(n.names, id) }
