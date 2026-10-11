package jetway

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Status is where a gate's jetway is for our traffic.
type Status string

const (
	StatusNone      Status = ""          // not ours: no placeholder, no helper
	StatusReserved  Status = "reserved"  // a placeholder holds the gate, jetway home
	StatusDocking   Status = "docking"   // a helper is on it, the jetway moving in
	StatusDocked    Status = "docked"    // the jetway at the door
	StatusUndocking Status = "undocking" // the helper gone, the jetway moving home
	StatusFailed    Status = "failed"    // the sim put the helper elsewhere, or no jetway came
)

// Service docks jetways to our traffic. The engine asks it, never needs
// it: with no service, or one not Available, traffic parks and pushes as
// without jetways. Today's backend is Helpers (an invisible ParkedATC
// aircraft per docked gate); a direct API, once the simulator has one,
// is another backend behind the same calls.
type Service interface {
	// Available reports whether the backend can work here, and why not.
	Available() (bool, string)
	// Prepare readies icao's jetway gates for our traffic; Release frees
	// them (every helper and placeholder removed, the jetways home).
	Prepare(icao string) error
	Release(icao string) error
	// Dock brings stand's jetway to the aircraft parked there (model is its
	// ICAO type or title); Undock sends it home before the push.
	Dock(icao string, stand int, model string) error
	Undock(icao string, stand int) error
	// Status is a stand's jetway now.
	Status(icao string, stand int) Status
	// Handle takes the simulator's messages for the service; Update drives
	// its timers, every frame or so.
	Handle(msg engine.Message) bool
	Update(now time.Time)
}

// Client is what Helpers needs of the simulator connection.
type Client interface {
	AICreateParkedATCAircraft(title, tail, icao string, requestID uint32) error
	AISetAircraftFlightPlan(objectID uint32, path string, requestID uint32) error
	AIReleaseControl(objectID uint32, requestID uint32) error
	AIRemoveObject(objectID uint32, requestID uint32) error
	RequestJetwayData(icao string, arrayCount uint32, indexes *int32) error
}

// Timings of the helper backend, as measured live at EDDM (2026-10-10).
const (
	// RetractWait: a placeholder's flight plan retracts the jetway its
	// creation claimed; released this long after (earlier, the jetway
	// stayed; later, the sim pushes it back).
	RetractWait = 25 * time.Second
	// ClaimTimeout: a new ParkedATC that no jetway takes in this long sits
	// on a gate without one, or none: removed.
	ClaimTimeout = 12 * time.Second
	// DockTimeout: a helper's jetway not docked in this long has failed.
	DockTimeout = 90 * time.Second
	// HelperDelay: the helper is created this long after its gate's
	// placeholder went (created at once, the sim had not freed the gate
	// yet: live, LKPR C20's helper went to GA stand 56).
	HelperDelay = 3 * time.Second
	// pollEvery: the jetway data asked for while anything moves.
	pollEvery = 2 * time.Second
)

// HelperOptions configure Helpers.
type HelperOptions struct {
	// Types are the ICAO types with a helper in the installed package
	// (WritePackage); a model of another type takes FallbackType's.
	Types        []string
	FallbackType string
	// RetractPlan is a flight plan file (.pln) out of icao, given to each
	// placeholder to retract the jetway its creation claimed.
	RetractPlan func(icao string) (string, error)
	// RequestBase is the first request ID of the 1000 the backend uses.
	RequestBase uint32
	// Log, when set, is told what the backend does.
	Log func(format string, args ...any)
}

type pending struct {
	icao       string
	stand      int // a helper's gate; -1 a placeholder
	helper     bool
	createdAt  time.Time
	object     uint32
	retractAt  time.Time // placeholder: release after the flight plan
	retracting bool
}

type gate struct {
	placeholder, helper uint32
	status              Status
	since               time.Time
	model               string
	createAt            time.Time // docking: the helper created then (zero: done)
}

// Helpers is the invisible-helper backend (Service).
type Helpers struct {
	client Client
	opt    HelperOptions

	mu         sync.Mutex
	now        time.Time
	nextReq    uint32
	byReq      map[uint32]*pending
	byObj      map[uint32]*pending
	gates      map[string]map[int]*gate // icao → stand → gate
	jetways    map[string]map[int]bool  // icao → stands with a jetway
	filling    map[string]bool          // icao → placeholders being made
	emptyTries map[string]int           // icao → fills that found no gate yet
	lastPoll   time.Time
}

// NewHelpers makes the helper backend on client.
func NewHelpers(client Client, o HelperOptions) *Helpers {
	if o.RequestBase == 0 {
		o.RequestBase = 920_000_000
	}
	if o.FallbackType == "" {
		o.FallbackType = "A320"
	}
	return &Helpers{client: client, opt: o, byReq: map[uint32]*pending{}, byObj: map[uint32]*pending{},
		gates: map[string]map[int]*gate{}, jetways: map[string]map[int]bool{}, filling: map[string]bool{}, emptyTries: map[string]int{}}
}

var _ Service = (*Helpers)(nil)

// ErrFilling: Dock asked while the airport's placeholders are still taking
// their gates; ask again later.
var ErrFilling = errors.New("jetway: gates still filling")

// ErrNoGate: the stand holds no placeholder (next to an occupied gate), so
// the sim would not put the helper there.
var ErrNoGate = errors.New("jetway: the sim gives this gate to no helper")

func (h *Helpers) logf(f string, a ...any) {
	if h.opt.Log != nil {
		h.opt.Log(f, a...)
	}
}

func (h *Helpers) Available() (bool, string) {
	if len(h.opt.Types) == 0 {
		return false, "jetway helper package missing"
	}
	if h.opt.RetractPlan == nil {
		return false, "no retract plan"
	}
	return true, ""
}

func (h *Helpers) req() uint32 {
	r := h.opt.RequestBase + h.nextReq%1000
	h.nextReq++
	return r
}

func (h *Helpers) gate(icao string, stand int) *gate {
	icao = strings.ToUpper(icao)
	m := h.gates[icao]
	if m == nil {
		m = map[int]*gate{}
		h.gates[icao] = m
	}
	g := m[stand]
	if g == nil {
		g = &gate{}
		m[stand] = g
	}
	return g
}

// Prepare asks for icao's jetways; the placeholders follow, one at a
// time, once the list is in (Handle).
func (h *Helpers) Prepare(icao string) error {
	if ok, why := h.Available(); !ok {
		return errors.New(why)
	}
	h.mu.Lock()
	h.filling[strings.ToUpper(icao)] = true
	h.mu.Unlock()
	return h.client.RequestJetwayData(strings.ToUpper(icao), 0, nil)
}

// Release removes every placeholder and helper at icao.
func (h *Helpers) Release(icao string) error {
	icao = strings.ToUpper(icao)
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.filling, icao)
	var errs []error
	for _, g := range h.gates[icao] {
		for _, obj := range []uint32{g.placeholder, g.helper} {
			if obj != 0 {
				errs = append(errs, h.client.AIRemoveObject(obj, h.req()))
				delete(h.byObj, obj)
			}
		}
	}
	for r, p := range h.byReq {
		if p.icao == icao {
			if p.object != 0 {
				errs = append(errs, h.client.AIRemoveObject(p.object, h.req()))
				delete(h.byObj, p.object)
			}
			delete(h.byReq, r)
		}
	}
	delete(h.gates, icao)
	return errors.Join(errs...)
}

// Dock takes stand's placeholder away and puts the helper of model's type
// there: the sim gives it the one free gate, and its jetway docks.
func (h *Helpers) Dock(icao string, stand int, model string) error {
	icao = strings.ToUpper(icao)
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.jetways[icao][stand] {
		return fmt.Errorf("no jetway at stand %d", stand)
	}
	// While placeholders are still taking gates, other gates are free and
	// the sim gives the helper the first of them (live, stand 0 for 14):
	// asked again once they hold them.
	if h.filling[icao] || h.creating(icao) {
		return ErrFilling
	}
	for s, g := range h.gates[icao] {
		if s != stand && g.status == StatusUndocking {
			return ErrFilling // its gate free until a placeholder is back
		}
		// One helper on its way at a time: two at once, each took the
		// other's gate (live, LKPR 11 and 25).
		if s != stand && g.status == StatusDocking && g.helper == 0 {
			return ErrFilling
		}
	}
	g := h.gate(icao, stand)
	if g.helper != 0 || g.status == StatusDocking || g.status == StatusDocked {
		return nil
	}
	// Only a gate a placeholder holds: the sim puts no ParkedATC next to an
	// occupied gate, so the others (every other one) cannot be given to a
	// helper; asked, it went to another gate or a ramp (live, LKPR 5, 7,
	// 14, 16).
	if g.placeholder == 0 {
		return ErrNoGate
	}
	if err := h.client.AIRemoveObject(g.placeholder, h.req()); err != nil {
		return err
	}
	delete(h.byObj, g.placeholder)
	g.placeholder = 0
	// The helper once the gate is free (Update).
	g.createAt = h.now.Add(HelperDelay)
	g.status, g.since, g.model = StatusDocking, h.now, model
	h.logf("jetway %s stand %d: docking (%s)", icao, stand, h.helperTitle(model))
	return nil
}

// createHelper creates stand's helper. h.mu held.
func (h *Helpers) createHelper(icao string, stand int, g *gate) error {
	g.createAt = time.Time{}
	r := h.req()
	h.byReq[r] = &pending{icao: icao, stand: stand, helper: true, createdAt: h.now}
	return h.client.AICreateParkedATCAircraft(h.helperTitle(g.model), "JW"+fmt.Sprint(r%10000), icao, r)
}

// dropHelper removes stand's helper, created or on its way. h.mu held.
func (h *Helpers) dropHelper(icao string, stand int, g *gate) {
	g.createAt = time.Time{}
	if g.helper != 0 {
		h.client.AIRemoveObject(g.helper, h.req())
		delete(h.byObj, g.helper)
		g.helper = 0
	}
	for r, p := range h.byReq {
		if p.helper && p.icao == icao && p.stand == stand {
			if p.object != 0 {
				h.client.AIRemoveObject(p.object, h.req())
				delete(h.byObj, p.object)
			}
			delete(h.byReq, r)
		}
	}
}

func (h *Helpers) helperTitle(model string) string {
	m := strings.ToUpper(model)
	for _, t := range h.opt.Types {
		if m == strings.ToUpper(t) || strings.Contains(m, strings.ToUpper(t)) {
			return HelperTitle(t, false)
		}
	}
	return HelperTitle(h.opt.FallbackType, false)
}

// Undock removes stand's helper: the jetway goes home, and a placeholder
// holds the gate again once it is there.
func (h *Helpers) Undock(icao string, stand int) error {
	icao = strings.ToUpper(icao)
	h.mu.Lock()
	defer h.mu.Unlock()
	g := h.gate(icao, stand)
	if g.status != StatusDocking && g.status != StatusDocked {
		return nil
	}
	h.dropHelper(icao, stand, g)
	g.status, g.since = StatusUndocking, h.now
	h.logf("jetway %s stand %d: undocking", icao, stand)
	return nil
}

// Dockable are icao's gates a helper can be given now: those a placeholder
// holds (the sim puts none next to an occupied gate, so about every other
// jetway gate).
func (h *Helpers) Dockable(icao string) []int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.dockable(icao)
}

// dockable is Dockable. h.mu held.
func (h *Helpers) dockable(icao string) []int {
	var out []int
	for stand, g := range h.gates[strings.ToUpper(icao)] {
		if g.placeholder != 0 {
			out = append(out, stand)
		}
	}
	sort.Ints(out)
	return out
}

// Since is when stand's jetway status last changed (docked: when it
// docked); zero none.
func (h *Helpers) Since(icao string, stand int) time.Time {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m := h.gates[strings.ToUpper(icao)]; m != nil && m[stand] != nil {
		return m[stand].since
	}
	return time.Time{}
}

func (h *Helpers) Status(icao string, stand int) Status {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m := h.gates[strings.ToUpper(icao)]; m != nil && m[stand] != nil {
		return m[stand].status
	}
	return StatusNone
}

// Handle takes the backend's created objects and the jetway lists.
func (h *Helpers) Handle(msg engine.Message) bool {
	if msg.SIMCONNECT_RECV == nil {
		return false
	}
	switch types.SIMCONNECT_RECV_ID(msg.DwID) {
	case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
		m := msg.AsAssignedObjectID()
		h.mu.Lock()
		p, ok := h.byReq[uint32(m.DwRequestID)]
		if !ok {
			h.mu.Unlock()
			return false
		}
		p.object = uint32(m.DwObjectID)
		h.byObj[p.object] = p
		h.mu.Unlock()
		return true
	case types.SIMCONNECT_RECV_ID_JETWAY_DATA:
		l := msg.AsJetwayData()
		h.mu.Lock()
		defer h.mu.Unlock()
		for _, j := range l.Entries() {
			h.jetway(j)
		}
		return false // others may read the list too
	}
	return false
}

// jetway takes one jetway's state. h.mu held.
func (h *Helpers) jetway(j types.SIMCONNECT_JETWAY_DATA) {
	icao := strings.ToUpper(strings.TrimRight(string(j.AirportIcao[:]), "\x00 "))
	stand := int(j.ParkingIndex)
	if h.jetways[icao] == nil {
		h.jetways[icao] = map[int]bool{}
	}
	h.jetways[icao][stand] = true
	att := uint32(j.AttachedObjectId)
	if p := h.byObj[att]; att != 0 && p != nil && p.icao == icao {
		g := h.gate(icao, stand)
		switch {
		case !p.helper && g.placeholder != att:
			// A placeholder took this gate: retract its claim (flight plan),
			// release it a while after (RetractWait).
			g.placeholder, g.status, g.since = att, StatusReserved, h.now
			if path, err := h.opt.RetractPlan(icao); err == nil {
				h.client.AISetAircraftFlightPlan(att, path, h.req())
				p.retractAt, p.retracting = h.now.Add(RetractWait), true
			}
		case p.helper && p.stand != stand:
			// The sim put the helper on another gate: not ours.
			h.logf("jetway %s: helper for stand %d took stand %d: removed", icao, p.stand, stand)
			h.client.AIRemoveObject(att, h.req())
			delete(h.byObj, att)
			if w := h.gate(icao, p.stand); w.status == StatusDocking {
				w.status, w.since = StatusFailed, h.now
			}
			// Gates free again (its own, and maybe others): filled before the
			// next dock.
			h.filling[icao] = true
		case p.helper:
			g.helper = att
			if j.Status == 7 {
				if g.status != StatusDocked {
					h.logf("jetway %s stand %d: docked", icao, stand)
					g.since = h.now
				}
				g.status = StatusDocked
			}
		}
	}
	if g := h.gates[icao][stand]; g != nil && g.status == StatusUndocking && j.Status == 0 && att == 0 && h.now.Sub(g.since) >= RetractWait {
		// Home (the list says so before it is: live, 16 s after the helper
		// went, still moving; it takes about 24 s): a placeholder holds the
		// gate again.
		g.status = StatusNone
		h.spawnPlaceholder(icao)
	}
}

// spawnPlaceholder creates one placeholder at icao. h.mu held.
func (h *Helpers) spawnPlaceholder(icao string) {
	r := h.req()
	h.byReq[r] = &pending{icao: icao, stand: -1, createdAt: h.now}
	h.client.AICreateParkedATCAircraft(HelperTitle(h.opt.FallbackType, false), "JP"+fmt.Sprint(r%10000), icao, r)
}

// Update drives the timers: placeholders released after their retract,
// claims and dockings timed out, the next placeholder while filling, and
// the jetway list asked for while anything moves.
func (h *Helpers) Update(now time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.now = now
	busy := false
	for r, p := range h.byReq {
		if p.helper {
			g := h.gate(p.icao, p.stand)
			switch {
			case p.object != 0 && g.helper == p.object:
				delete(h.byReq, r) // on its gate
			case p.object != 0 && now.Sub(p.createdAt) > ClaimTimeout:
				// No jetway took it: put on a gate without one, or a ramp.
				h.logf("jetway %s stand %d: helper not on a jetway gate: removed", p.icao, p.stand)
				h.dropHelper(p.icao, p.stand, g)
				g.status, g.since = StatusFailed, now
				h.filling[p.icao] = true
			default:
				busy = true
			}
			continue
		}
		switch {
		case p.retracting && !now.Before(p.retractAt):
			h.client.AIReleaseControl(p.object, h.req())
			p.retracting = false
			delete(h.byReq, r) // done: it holds its gate
		case p.retracting:
			busy = true
		case p.object != 0 && !p.helper && h.placeholderGate(p) < 0 && now.Sub(p.createdAt) > ClaimTimeout:
			// No jetway took it: no free jetway gate left for it.
			h.client.AIRemoveObject(p.object, h.req())
			delete(h.byObj, p.object)
			delete(h.byReq, r)
			// None taken yet: the sim not ready (live, the first one after
			// a sim start went unclaimed): tried again, a few times.
			if len(h.dockable(p.icao)) == 0 && h.emptyTries[p.icao] < 5 {
				h.emptyTries[p.icao]++
				break
			}
			delete(h.filling, p.icao)
			h.logf("jetway %s: gates filled (%d)", p.icao, len(h.dockable(p.icao)))
		default:
			busy = true
		}
	}
	for icao, m := range h.gates {
		for stand, g := range m {
			if g.status == StatusDocking && !g.createAt.IsZero() && !now.Before(g.createAt) {
				h.createHelper(icao, stand, g)
			}
			if (g.status == StatusDocking || g.status == StatusUndocking) && now.Sub(g.since) > DockTimeout {
				h.logf("jetway %s stand %d: %s timed out", icao, stand, g.status)
				h.dropHelper(icao, stand, g)
				g.status = StatusFailed
				h.filling[icao] = true // its gate free: a placeholder again
			}
			if g.status == StatusDocking || g.status == StatusUndocking {
				busy = true
			}
		}
	}
	// Filling: one placeholder at a time (its gate known before the next).
	for icao := range h.filling {
		busy = true
		if !h.creating(icao) && len(h.jetways[icao]) > 0 {
			h.spawnPlaceholder(icao)
		}
	}
	if busy && now.Sub(h.lastPoll) >= pollEvery {
		h.lastPoll = now
		asked := map[string]bool{}
		for _, p := range h.byReq {
			asked[p.icao] = true
		}
		for icao := range h.filling {
			asked[icao] = true
		}
		for icao, m := range h.gates {
			for _, g := range m {
				if g.status == StatusDocking || g.status == StatusUndocking {
					asked[icao] = true
				}
			}
		}
		for icao := range asked {
			h.client.RequestJetwayData(icao, 0, nil)
		}
	}
}

// creating reports a placeholder at icao on its way to a gate. h.mu held.
func (h *Helpers) creating(icao string) bool {
	for _, p := range h.byReq {
		if p.icao == icao && !p.helper && !p.retracting {
			return true
		}
	}
	return false
}

// placeholderGate is the stand p holds, -1 none yet. h.mu held.
func (h *Helpers) placeholderGate(p *pending) int {
	for stand, g := range h.gates[p.icao] {
		if g.placeholder == p.object {
			return stand
		}
	}
	return -1
}
