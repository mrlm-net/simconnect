package worldtest

import (
	"maps"
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// The loopback counts the SimConnect calls the World makes, by kind and by
// the aircraft they are for (its tail), and checks every placement of an
// aircraft against the last one by the jump trace's rule
// (traffic.JumpTraced): what traffic costs the simulator, and whether it
// moves smoothly, measured offline (#1481).

// Kinds of calls counted (Calls, CallsOf).
const (
	CallCreate    = "create"    // AICreate*
	CallRemove    = "remove"    // AIRemoveObject
	CallWaypoints = "waypoints" // SetDataOnSimObject of a waypoint list
	CallPosition  = "position"  // SetDataOnSimObject of a position
	CallSetData   = "setData"   // any other SetDataOnSimObject
	CallRequest   = "request"   // RequestDataOnSimObject
	CallScan      = "scan"      // RequestDataOnSimObjectType
	CallEvent     = "event"     // TransmitClientEvent
	// Not the World's calls: the simulator's work for them.
	CallData   = "data"   // data sent for an object: a request due, a scan's entry
	CallAIMove = "aiMove" // a position SimlessAI wrote, as the simulator's AI moves it
)

// Placers of an aircraft (Jump.By).
const (
	PlacedCreate = "create" // where it was created
	PlacedWorld  = "world"  // the World wrote its position
	PlacedAI     = "ai"     // SimlessAI flew it there
)

// Jump is a placement the jump trace would report: farther from the last
// one than the aircraft's speed explains, or turned more than it may.
type Jump struct {
	At       time.Time
	ObjectID uint32
	Tail     string
	By, From string // who placed it, and the one before
	Metres   float64
	TurnDeg  float64
	Dt       time.Duration
	Kts      float64
}

type placement struct {
	at       time.Time
	gap      time.Duration
	lat, lon float64
	hdg, kts float64
	by       string
}

// ledger is the loopback's count; under Loopback.mu.
type ledger struct {
	total  map[string]int
	byTail map[string]map[string]int
	tails  map[uint32]string // every object's tail, removed ones kept
	last   map[uint32]placement
	jumps  []Jump
}

func (g *ledger) init() {
	if g.total == nil {
		g.total, g.byTail = map[string]int{}, map[string]map[string]int{}
		g.tails, g.last = map[uint32]string{}, map[uint32]placement{}
	}
}

// count counts a call of kind for objectID (0: none). l.mu held.
func (l *Loopback) count(kind string, objectID uint32) {
	g := &l.calls
	g.init()
	g.total[kind]++
	if tail := g.tails[objectID]; tail != "" {
		m := g.byTail[tail]
		if m == nil {
			m = map[string]int{}
			g.byTail[tail] = m
		}
		m[kind]++
	}
}

// place checks a placement of objectID against its last one. l.mu held.
func (l *Loopback) place(objectID uint32, lat, lon, hdg, kts float64, by string) {
	g := &l.calls
	g.init()
	now := l.clock()
	last, ok := g.last[objectID]
	dt := now.Sub(last.at)
	g.last[objectID] = placement{at: now, gap: dt, lat: lat, lon: lon, hdg: hdg, kts: kts, by: by}
	if !ok {
		return
	}
	d := calc.HaversineMeters(last.lat, last.lon, lat, lon)
	turn := math.Abs(math.Remainder(hdg-last.hdg, 360))
	if traffic.JumpTraced(math.Max(kts, last.kts), dt, last.gap, d, turn) {
		g.jumps = append(g.jumps, Jump{At: now, ObjectID: objectID, Tail: g.tails[objectID], By: by, From: last.by,
			Metres: d, TurnDeg: turn, Dt: dt, Kts: math.Max(kts, last.kts)})
	}
}

// setPosition takes a position written for o by. l.mu held.
func (l *Loopback) setPosition(o *Object, p types.SIMCONNECT_DATA_INITPOSITION, by string) {
	if p.Latitude == 0 && p.Longitude == 0 {
		return
	}
	o.Lat, o.Lon, o.AltFt = p.Latitude, p.Longitude, p.Altitude
	o.Pitch, o.Bank, o.Heading = p.Pitch, p.Bank, p.Heading
	o.OnGround = p.OnGround != 0
	o.GroundKts = float64(p.Airspeed)
	l.place(o.ID, o.Lat, o.Lon, o.Heading, o.GroundKts, by)
}

// SetAIPosition takes a position SimlessAI flew an aircraft to
// (traffic.AIPositionWriter): kept as any position, counted apart from
// the World's calls.
func (l *Loopback) SetAIPosition(definitionID, objectID uint32, p types.SIMCONNECT_DATA_INITPOSITION) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.count(CallAIMove, objectID)
	if o := l.objects[objectID]; o != nil {
		l.setPosition(o, p, PlacedAI)
	}
	return nil
}

// Calls are the calls counted so far, by kind.
func (l *Loopback) Calls() map[string]int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return maps.Clone(l.calls.total)
}

// CallsOf are the calls counted so far for the aircraft tail, by kind.
func (l *Loopback) CallsOf(tail string) map[string]int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return maps.Clone(l.calls.byTail[tail])
}

// Jumps are the placements so far the jump trace would report, in order.
func (l *Loopback) Jumps() []Jump {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Jump(nil), l.calls.jumps...)
}
