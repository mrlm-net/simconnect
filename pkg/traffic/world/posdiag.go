package world

// POSDIAG: a temporary diagnostic build, NOT TO SHIP (posdiag overlay only).
// Log-only, for the airborne position jumps (still frames, then a
// catch-up): per aircraft, a gap over posDiagGap in a stream that came
// every frame, of the simulator's data (each object and request) and of our
// position writes (the injector's Initial Position); and every message
// whose handling here took over posDiagSlow, with the handler that took it.
// Every line starts "posdiag".

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

const (
	posDiagGap   = 30 * time.Millisecond // a gap reported
	posDiagFrame = 25 * time.Millisecond // the gap before under this: a stream every frame
	posDiagLong  = 3 * time.Second       // over this: a pause, a new stream, not reported
	posDiagSlow  = 20 * time.Millisecond // a message's handling reported
)

// posGap times one stream: report on a gap over posDiagGap after one under
// posDiagFrame.
type posGap struct {
	last    time.Time
	lastGap time.Duration
}

func (g *posGap) step(now time.Time) (gap, before time.Duration, report bool) {
	if !g.last.IsZero() {
		gap, before = now.Sub(g.last), g.lastGap
		report = gap > posDiagGap && gap < posDiagLong && before > 0 && before < posDiagFrame
		g.lastGap = gap
	}
	g.last = now
	return gap, before, report
}

type posDiagState struct {
	mu     sync.Mutex
	data   map[[2]uint32]*posGap // by object, request
	writes map[uint32]*posGap    // by object
	logf   func(string, ...any)
}

var posDiag = &posDiagState{data: map[[2]uint32]*posGap{}, writes: map[uint32]*posGap{}}

func (p *posDiagState) setLog(logf func(string, ...any)) {
	p.mu.Lock()
	p.logf = logf
	p.mu.Unlock()
}

func (p *posDiagState) printf(f string, a ...any) {
	p.mu.Lock()
	logf := p.logf
	p.mu.Unlock()
	if logf != nil {
		logf(f, a...)
	}
}

// arrived notes a SIMOBJECT_DATA message; tail names its object ("" not ours).
func (p *posDiagState) arrived(msg engine.Message, tail func(uint32) string) {
	if msg.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA {
		return
	}
	d := msg.AsSimObjectData()
	k := [2]uint32{uint32(d.DwObjectID), uint32(d.DwRequestID)}
	now := time.Now()
	p.mu.Lock()
	g := p.data[k]
	if g == nil {
		g = &posGap{}
		p.data[k] = g
	}
	gap, before, report := g.step(now)
	p.mu.Unlock()
	if report {
		p.printf("posdiag data gap: object %d (%s) request %d: %s, the one before %s", k[0], tail(k[0]), k[1], gap.Round(time.Millisecond), before.Round(time.Millisecond))
	}
}

// wrote notes our position write on obj. Called under the injector's lock:
// nothing here takes the World's locks.
func (p *posDiagState) wrote(obj uint32) {
	now := time.Now()
	p.mu.Lock()
	g := p.writes[obj]
	if g == nil {
		g = &posGap{}
		p.writes[obj] = g
	}
	gap, before, report := g.step(now)
	p.mu.Unlock()
	if report {
		p.printf("posdiag write gap: object %d: %s, the one before %s", obj, gap.Round(time.Millisecond), before.Round(time.Millisecond))
	}
}

// posDiagClient is the injector's client: its position writes noted.
type posDiagClient struct {
	engine.Client
	def uint32 // the injector's Initial Position definition
}

func (c *posDiagClient) SetDataOnSimObject(definitionID uint32, objectID uint32, flags types.SIMCONNECT_DATA_SET_FLAG, arrayCount uint32, cbUnitSize uint32, data unsafe.Pointer) error {
	if definitionID == c.def {
		posDiag.wrote(objectID)
	}
	return c.Client.SetDataOnSimObject(definitionID, objectID, flags, arrayCount, cbUnitSize, data)
}

// posDiagWrap wraps the injector's client (nil stays nil: tests).
func posDiagWrap(client engine.Client, def uint32) engine.Client {
	if client == nil {
		return nil
	}
	return &posDiagClient{Client: client, def: def}
}

// posDiagTimer times one message through controlCenter.handle, stage by
// stage (mark after each); done reports it over posDiagSlow.
type posDiagTimer struct {
	msg    engine.Message
	start  time.Time
	at     time.Time
	stages []string
}

func newPosDiagTimer(msg engine.Message) *posDiagTimer {
	now := time.Now()
	return &posDiagTimer{msg: msg, start: now, at: now}
}

func (t *posDiagTimer) mark(stage string) {
	now := time.Now()
	if d := now.Sub(t.at); d >= time.Millisecond {
		t.stages = append(t.stages, fmt.Sprintf("%s %s", stage, d.Round(time.Millisecond)))
	}
	t.at = now
}

func (t *posDiagTimer) done(handledBy string) {
	d := time.Since(t.start)
	if d <= posDiagSlow {
		return
	}
	id := uint32(0)
	if t.msg.SIMCONNECT_RECV != nil {
		id = uint32(t.msg.DwID)
	}
	posDiag.printf("posdiag slow: message %d took %s, handled by %s (%s)", id, d.Round(time.Millisecond), handledBy, strings.Join(t.stages, ", "))
}
