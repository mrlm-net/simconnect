package worldtest

import (
	"maps"
	"math"
	"slices"
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
	// Its step from the placement before (from: that one): metres north
	// and east, degrees turned (signed), metres; none on the first.
	step step
	from struct {
		by   string
		kts  float64
		step step
	}
}

type step struct {
	dn, de, turn, d float64
	gap             time.Duration // the frame it took
	ok              bool
}

// liveFrame is the longest frame the jump trace's rule is made for: the
// simulator's 30 to 60 Hz. Its floors (a metre, 3°) are per frame of
// that length.
const liveFrame = time.Second / 30

// jumpTraceGap is the jump trace's: placements farther apart are not judged.
const jumpTraceGap = 1500 * time.Millisecond

// ktsPerMS converts VELOCITY BODY Z (m/s) to knots.
const ktsPerMS = 1 / 0.514444

// metresPerDeg: a degree of latitude, metres.
const metresPerDeg = 111320.0

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

// place takes a placement of objectID and judges the one before it, now
// that what came after it is known (its speed written after it, the step
// after it). l.mu held.
func (l *Loopback) place(objectID uint32, lat, lon, hdg, kts float64, by string) {
	g := &l.calls
	g.init()
	now := l.clock()
	last, ok := g.last[objectID]
	cur := placement{at: now, lat: lat, lon: lon, hdg: hdg, kts: kts, by: by}
	if ok {
		cur.gap = now.Sub(last.at)
		cur.step = step{
			dn:   (lat - last.lat) * metresPerDeg,
			de:   (lon - last.lon) * metresPerDeg * math.Cos(lat*math.Pi/180),
			turn: math.Remainder(hdg-last.hdg, 360),
			d:    calc.HaversineMeters(last.lat, last.lon, lat, lon),
			gap:  cur.gap, ok: true,
		}
		cur.from.by, cur.from.kts, cur.from.step = last.by, last.kts, last.step
		if j, jumped := g.judge(objectID, last, &cur); jumped {
			g.jumps = append(g.jumps, j)
		}
	}
	g.last[objectID] = cur
}

// judge reports whether p is a placement the jump trace would report:
// farther from the one before than the speed explains, or turned more than
// it may (traffic.JumpTraced). next is the placement after it, nil when
// none came (yet).
//
// On frames longer than a live one (Options.FrameRate under 30) the rule's
// per-frame floors no longer hold: a tug turning a steady 90°/s, smooth at
// 60 Hz (1.5° a frame), turns 18° in a 5 Hz frame. There p is a jump only
// when its step also breaks from the step before (that motion gone on,
// scaled to p's frame) and the step after breaks from it, each by more than
// the rule allows: an outlier, a step aside or a snap, not a steady turn,
// its start or end, or a reversal sampled coarsely.
func (g *ledger) judge(objectID uint32, p placement, next *placement) (Jump, bool) {
	s := p.step
	if !s.ok {
		return Jump{}, false
	}
	speed := math.Max(p.kts, p.from.kts)
	if !traffic.JumpTraced(speed, s.gap, p.from.step.gap, s.d, math.Abs(s.turn)) {
		return Jump{}, false
	}
	j := Jump{At: p.at, ObjectID: objectID, Tail: g.tails[objectID], By: p.by, From: p.from.by, Metres: s.d, TurnDeg: math.Abs(s.turn), Dt: s.gap, Kts: speed}
	if s.gap <= liveFrame {
		return j, true
	}
	if p.from.step.ok && !breaks(p.from.step, s, speed) {
		return Jump{}, false
	}
	if next != nil && next.step.gap <= jumpTraceGap && !breaks(s, next.step, math.Max(speed, next.kts)) {
		return Jump{}, false
	}
	return j, true
}

// breaks reports whether step b breaks from step a, a's motion gone on for
// b's frame, by more than the jump trace's rule allows.
func breaks(a, b step, kts float64) bool {
	if a.gap <= 0 {
		return true
	}
	k := float64(b.gap) / float64(a.gap)
	rd := math.Hypot(b.dn-a.dn*k, b.de-a.de*k)
	rt := math.Abs(b.turn - a.turn*k)
	return traffic.JumpTraced(kts, b.gap, a.gap, rd, rt)
}

// unplace ends objectID's placements (removed): its last one is judged
// with none after it. l.mu held.
func (l *Loopback) unplace(objectID uint32) {
	g := &l.calls
	g.init()
	if p, ok := g.last[objectID]; ok {
		if j, jumped := g.judge(objectID, p, nil); jumped {
			g.jumps = append(g.jumps, j)
		}
	}
	delete(g.last, objectID)
}

// bodyZ takes a VELOCITY BODY Z written for objectID (m/s): travelling
// backwards the World places it with INITPOSITION Airspeed 0 (a DWORD, no
// negative speeds) and writes minus its speed here with every placement
// (traffic wheels.go), so its last placement moved at |v|. l.mu held.
func (l *Loopback) bodyZ(objectID uint32, v float64) {
	g := &l.calls
	g.init()
	if p, ok := g.last[objectID]; ok {
		p.kts = math.Max(p.kts, math.Abs(v)*ktsPerMS)
		g.last[objectID] = p
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

// Jumps are the placements so far the jump trace would report, in order;
// on frames longer than a live one, outliers only (judge). The last
// placement of each object, judged with none after it yet, counts too.
func (l *Loopback) Jumps() []Jump {
	l.mu.Lock()
	defer l.mu.Unlock()
	g := &l.calls
	g.init()
	js := append([]Jump(nil), g.jumps...)
	for id, p := range g.last {
		if j, jumped := g.judge(id, p, nil); jumped {
			js = append(js, j)
		}
	}
	slices.SortStableFunc(js, func(a, b Jump) int { return a.At.Compare(b.At) })
	return js
}
