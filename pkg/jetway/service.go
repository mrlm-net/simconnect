package jetway

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

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
	AddToDataDefinition(definitionID uint32, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID uint32) error
	RequestDataOnSimObject(requestID uint32, definitionID uint32, objectID uint32, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error
	SetDataOnSimObject(definitionID uint32, objectID uint32, flags types.SIMCONNECT_DATA_SET_FLAG, arrayCount uint32, cbUnitSize uint32, data unsafe.Pointer) error
}

// placePos is an aircraft's position as read for a helper's place.
type placePos struct{ Lat, Lon, AltFt, Hdg float64 }

// placing is a helper being put by our aircraft: both positions asked for.
type placing struct {
	icao  string
	stand int
	model string
	// title: the gate's helper (a variant's, DockAs), "" the model's type's.
	title         string
	ours, helper  uint32
	oursAt, helAt *placePos
	// placeholder: only its position is read, for OnPlaceholder.
	placeholder bool
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
	// EmptyFillTries: placeholders tried again while none has taken a gate
	// (ClaimTimeout each): about 5 min for a simulator still loading its
	// airport (live, a map started with the simulator: five tries in a
	// minute all unclaimed, no jetway all evening).
	EmptyFillTries = 25
	// RefillEvery: a fill that took no gate is tried again this long after.
	RefillEvery = 2 * time.Minute
	// DockSettle: how long a gate docked was once left before the next
	// helper was made. No longer waited (it held each arrival two minutes,
	// and the sim counts docked gates free at any age, 13 s to 50 min
	// live): a helper given a docked gate is undone by docking that gate
	// again first (redock). Kept for callers.
	DockSettle = 2 * time.Minute
	// DockTimeout: a helper's jetway not docked in this long has failed.
	DockTimeout = 90 * time.Second
	// FillStrays: placeholders of one fill that land on docked gates
	// before the fill stops (tried again RefillEvery later): each new
	// ParkedATC is another chance at a docked gate; the first stops it
	// (live, LKPR 2026-11-08: twelve jetways pulled off parked aircraft by
	// refills).
	FillStrays = 1
	// StrayWaitMax: the longest wait of a fill stopped again and again.
	StrayWaitMax = 30 * time.Minute
	// HelperDelay: the helper is created this long after its gate's
	// placeholder went (created at once, the sim had not freed the gate
	// yet: live, LKPR C20's helper went to GA stand 56).
	HelperDelay = 3 * time.Second
	// pollEvery: the jetway data asked for while a dock is coming (a gate
	// docking, a helper on its way); pollSlow while only undocking,
	// retracting or filling. Each read stalls all traffic about 100 ms
	// (live, LKPR 2026-11-08).
	pollEvery = 2 * time.Second
	pollSlow  = 10 * time.Second
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
	// AircraftAt is our aircraft parked on icao's stand (its object, 0
	// none): each helper is measured against it (logged), and a type with
	// a DoorOffset is put there. DefineBase is the first of the two data
	// definitions that takes (RequestBase when 0).
	AircraftAt func(icao string, stand int) uint32
	DefineBase uint32
	// Offset, when set, is where helper (a helper title) goes from our
	// aircraft of model so the jetway meets its door, meters ahead and
	// right: our model's jetway door minus the helper's. A DoorOffset in
	// the table wins; neither: the sim's place.
	Offset func(model, helper string) (ahead, right float64, ok bool)
	// MoveHelpers puts each helper at its door offset; off (the default),
	// the offset is only logged. Live, LKPR 2026-10-12: a helper moved
	// after the sim placed it had its jetway report docked without the
	// jetway moving at all (A4, C22, C30); unmoved, they docked.
	MoveHelpers bool
	// OnPlaceholder, when set, hears where the simulator parked each
	// placeholder or helper (title) on its gate: how it parks on that gate,
	// for the host to park its own aircraft the same way.
	OnPlaceholder func(icao string, stand int, title string, lat, lon, hdg float64)
	// Log, when set, is told what the backend does.
	Log func(format string, args ...any)
	// OnLearn, when set, hears what a gate taught: helper (a title) docked
	// there (docked true), or the sim put it on another gate instead, a
	// type that gate does not take. Kept by the host and given back with
	// Learn at the next start.
	OnLearn func(icao string, stand int, helper string, docked bool)
	// OnGateResult, when set, hears each dock at a gate: docked, or its
	// helper put on another gate. Counted by the host and given back with
	// GateRecord at the next start (GateUnfit).
	OnGateResult func(icao string, stand int, docked bool)
	// OnDock, when set, hears how a dock for our aircraft of model ended
	// at the gate it was given: docked, or its jetway never came (the
	// docking timed out, the helper on the gate). A helper the sim put on
	// another gate is not told here (OnGateResult, OnLearn). Called with
	// the backend's lock held: it must not call back.
	OnDock func(icao string, stand int, model string, docked bool)
}

// gateLearning is what a gate has shown of the helpers it takes: the sim
// never puts some types on some gates, and a helper of such a type goes to
// another gate, taking its jetway from our aircraft there (live, LKPR
// 2026-10-20 to 22: stand 11 refused BCS3 five times of five, A20N twice;
// 19 BCS3 and A20N three times each; 20 B738 four times, but took A332
// twelve times).
type gateLearning struct {
	refused map[string]bool
	docked  string // the last helper that docked there
	// docks and aways: its docks done and its helpers put elsewhere, of
	// any type; tried: when an unfit gate was tried last (GateUnfit).
	docks, aways int
	tried        time.Time
}

// A gate whose helpers the sim puts elsewhere more often than on it, of
// any type, GateUnfitMin times or more, gets no jetway: each miss takes a
// docked gate's jetway (live, LKPR 2026-10-21 to 23: stand 11 docked 8
// times and missed 15, 19 docked 5 and missed 9; every other gate missed 2
// at most). It is tried again once every GateRetest.
const (
	GateUnfitMin = 3
	GateRetest   = 2 * time.Hour
)

// GateRecord gives back a gate's docks and misses (OnGateResult).
func (h *Helpers) GateRecord(icao string, stand int, docks, aways int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	l := h.learning(strings.ToUpper(icao), stand)
	l.docks, l.aways = docks, aways
}

// learning is icao's stand's record, made when new. h.mu held.
func (h *Helpers) learning(icao string, stand int) *gateLearning {
	k := fmt.Sprint(icao, " ", stand)
	l := h.learned[k]
	if l == nil {
		l = &gateLearning{refused: map[string]bool{}}
		h.learned[k] = l
	}
	return l
}

// gateResult counts a dock at icao's stand and tells the host. h.mu held.
func (h *Helpers) gateResult(icao string, stand int, docked bool) {
	l := h.learning(icao, stand)
	if docked {
		l.docks++
	} else {
		l.aways++
	}
	if h.opt.OnGateResult != nil {
		h.opt.OnGateResult(icao, stand, docked)
	}
}

// gateUnfit reports that icao's stand gets no jetway now (GateUnfit),
// marking a retest when its time has come. h.mu held.
func (h *Helpers) gateUnfit(icao string, stand int) bool {
	l := h.learned[fmt.Sprint(icao, " ", stand)]
	if l == nil || l.aways < GateUnfitMin || l.aways <= l.docks {
		return false
	}
	if !l.tried.IsZero() && h.now.Sub(l.tried) < GateRetest {
		return true
	}
	l.tried = h.now
	h.logf("jetway %s stand %d: its helpers went elsewhere %d times, docked %d: tried once more (no jetway otherwise, for %s)", icao, stand, l.aways, l.docks, GateRetest)
	return false
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
	title               string    // the helper's (a variant's), "" the model's type's
	createAt            time.Time // docking: the helper created then (zero: done)
	// redockAt: docking again after its jetway was taken (redock): what
	// sits on the gate now leaves then, its own helper HelperDelay after.
	redockAt time.Time
}

// Helpers is the invisible-helper backend (Service).
type Helpers struct {
	client Client
	opt    HelperOptions

	mu      sync.Mutex
	now     time.Time
	nextReq uint32
	byReq   map[uint32]*pending
	byObj   map[uint32]*pending
	gates   map[string]map[int]*gate // icao → stand → gate
	jetways map[string]map[int]bool  // icao → stands with a jetway
	filling map[string]bool          // icao → placeholders being made
	// refill: icao → the fill under way only holds the gates emptied again
	// (emptyGates), done once none is left: no placeholder past them, each
	// another chance at a docked gate.
	refill     map[string]bool
	emptyTries map[string]int       // icao → fills that found no gate yet
	refillAt   map[string]time.Time // icao → when a fill that found no gate is tried again
	// lastList is the last jetway list read (logList).
	lastList []types.SIMCONNECT_JETWAY_DATA
	// accepted: icao → the gates the sim has given a placeholder (refill first).
	accepted map[string]map[int]bool
	// learned: "ICAO stand" → the helpers that gate takes and refuses.
	learned map[string]*gateLearning
	// strays: icao → placeholders of the fill under way that landed on a
	// docked gate (FillStrays).
	strays map[string]int
	// strayWait: icao → the wait after the last fill stopped (FillStrays),
	// doubled each time up to StrayWaitMax; zero after a fill done.
	strayWait map[string]time.Duration
	lastPoll  time.Time
	placeDefs bool                // the place definitions made
	placings  map[uint32]*placing // request → helper being placed
	// retiring: helpers sent off with the retract plan, removed then: one
	// removed while its jetway still moved to it left that jetway stuck
	// half way for good (live, LKPR 13, 15, 17 to 21: "retract home"
	// with nothing attached, every later dock there timed out).
	retiring map[uint32]time.Time
	// rest: "ICAO stand" → its jetway's main handle as last listed at rest
	// with nothing attached, logged beside the handle when it docks (a
	// "docked" whose jetway never moved, live LKPR C21 2026-11-08: the
	// data to tell it by, once seen).
	rest map[string]types.SIMCONNECT_DATA_XYZ
}

// NewHelpers makes the helper backend on client.
func NewHelpers(client Client, o HelperOptions) *Helpers {
	if o.RequestBase == 0 {
		o.RequestBase = 920_000_000
	}
	if o.FallbackType == "" {
		o.FallbackType = "A320"
	}
	if o.DefineBase == 0 {
		o.DefineBase = o.RequestBase
	}
	return &Helpers{client: client, opt: o, byReq: map[uint32]*pending{}, byObj: map[uint32]*pending{},
		gates: map[string]map[int]*gate{}, jetways: map[string]map[int]bool{}, filling: map[string]bool{}, refill: map[string]bool{}, emptyTries: map[string]int{}, refillAt: map[string]time.Time{}, accepted: map[string]map[int]bool{}, placings: map[uint32]*placing{}, retiring: map[uint32]time.Time{},
		learned: map[string]*gateLearning{}, strays: map[string]int{}, strayWait: map[string]time.Duration{}, rest: map[string]types.SIMCONNECT_DATA_XYZ{}}
}

// Learn gives back what a gate taught (OnLearn): helper docked there, or
// was put on another gate instead.
func (h *Helpers) Learn(icao string, stand int, helper string, docked bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.learn(strings.ToUpper(icao), stand, helper, docked, false)
}

// learn notes what icao's stand taught; tell: the host hears it. h.mu held.
func (h *Helpers) learn(icao string, stand int, helper string, docked, tell bool) {
	l := h.learning(icao, stand)
	if docked {
		l.docked = helper
		delete(l.refused, helper)
	} else {
		l.refused[helper] = true
	}
	if tell && h.opt.OnLearn != nil {
		h.opt.OnLearn(icao, stand, helper, docked)
	}
}

// gateTakes is the helper icao's stand takes for title: title itself
// unless the gate has refused it, then the last helper docked there, else
// the placeholders' type (one sits there). h.mu held.
func (h *Helpers) gateTakes(icao string, stand int, title string) string {
	l := h.learned[fmt.Sprint(icao, " ", stand)]
	if l == nil || !l.refused[title] {
		return title
	}
	if l.docked != "" && !l.refused[l.docked] {
		return l.docked
	}
	return HelperTitle(h.opt.FallbackType, false)
}

// measure asks for our aircraft's and its helper's positions (place).
// h.mu held.
func (h *Helpers) measure(pl *placing) {
	read := h.opt.DefineBase
	if !h.placeDefs {
		h.placeDefs = true
		for i, v := range []struct{ n, u string }{{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"}, {"PLANE ALTITUDE", "feet"}, {"PLANE HEADING DEGREES TRUE", "degrees"}} {
			h.client.AddToDataDefinition(read, v.n, v.u, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i))
		}
		h.client.AddToDataDefinition(read+1, "Initial Position", "", types.SIMCONNECT_DATATYPE_INITPOSITION, 0, 0)
	}
	for _, obj := range []uint32{pl.ours, pl.helper} {
		if obj == 0 {
			continue
		}
		r := h.req()
		h.placings[r] = pl
		h.client.RequestDataOnSimObject(r, read, obj, types.SIMCONNECT_PERIOD_ONCE, 0, 0, 0, 0)
	}
}

// measurePlaceholder asks for the position of placeholder obj on stand
// (OnPlaceholder). h.mu held.
func (h *Helpers) measurePlaceholder(icao string, stand int, obj uint32) {
	h.measure(&placing{icao: icao, stand: stand, model: HelperTitle(h.opt.FallbackType, false), helper: obj, placeholder: true})
}

// place logs where the sim put the helper from our aircraft and, for a
// type with a DoorOffset, puts it there. h.mu held.
func (h *Helpers) place(pl *placing) {
	o, a := *pl.oursAt, *pl.helAt
	ahead, right := offsetM(o, a.Lat, a.Lon)
	// The table's (set by hand) first, else from both models' doors.
	off, ok := DoorOffsetFor(pl.model)
	if !ok && h.opt.Offset != nil {
		if a, r, k := h.opt.Offset(pl.model, h.placingTitle(pl)); k {
			off, ok = DoorOffset{Type: "from the models' doors", AheadM: a, RightM: r}, true
		}
	}
	if !ok {
		h.logf("jetway %s stand %d: helper %.2f m ahead, %.2f m right of %s (the sim's place; no door offset)", pl.icao, pl.stand, ahead, right, pl.model)
		return
	}
	if !h.opt.MoveHelpers {
		// The gap: where the jetway docks (the helper's door) from our
		// door, ahead and right (live, LKPR C22: CSA1894's 9 m ahead).
		h.logf("jetway %s stand %d: helper %.2f m ahead, %.2f m right of %s (the sim's place; %s: the door wants %.2f m ahead, %.2f m right; its jetway %.2f m ahead, %.2f m right of the door; not moved)", pl.icao, pl.stand, ahead, right, pl.model, h.placingTitle(pl), off.AheadM, off.RightM, ahead-off.AheadM, right-off.RightM)
		return
	}
	lat, lon := fromOffset(o, off.AheadM, off.RightM)
	pos := types.SIMCONNECT_DATA_INITPOSITION{Latitude: lat, Longitude: lon, Altitude: o.AltFt, Heading: o.Hdg, OnGround: 1}
	h.client.SetDataOnSimObject(h.opt.DefineBase+1, pl.helper, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(pos)), unsafe.Pointer(&pos))
	h.logf("jetway %s stand %d: helper %.2f m ahead, %.2f m right of %s, put %.2f m ahead, %.2f m right (door offset %s)", pl.icao, pl.stand, ahead, right, pl.model, off.AheadM, off.RightM, off.Type)
}

// placingTitle is the title of the helper pl places: the gate's (a variant,
// DockAs), else the model's type's. Live, LKPR B14: the offset logged from
// the B748's own door while its "B748 D-A320" variant stood there. h.mu held.
func (h *Helpers) placingTitle(pl *placing) string {
	if pl.title != "" {
		return pl.title
	}
	return h.helperTitle(pl.model)
}

// mPerDegLat is metres per degree of latitude.
const mPerDegLat = 111_320.0

// offsetM is where (lat, lon) is from o: metres ahead along its heading and
// to its right.
func offsetM(o placePos, lat, lon float64) (ahead, right float64) {
	n := (lat - o.Lat) * mPerDegLat
	e := (lon - o.Lon) * mPerDegLat * math.Cos(o.Lat*math.Pi/180)
	r := o.Hdg * math.Pi / 180
	return n*math.Cos(r) + e*math.Sin(r), -n*math.Sin(r) + e*math.Cos(r)
}

// fromOffset is the point ahead and right metres from o.
func fromOffset(o placePos, ahead, right float64) (lat, lon float64) {
	r := o.Hdg * math.Pi / 180
	n := ahead*math.Cos(r) - right*math.Sin(r)
	e := ahead*math.Sin(r) + right*math.Cos(r)
	return o.Lat + n/mPerDegLat, o.Lon + e/(mPerDegLat*math.Cos(o.Lat*math.Pi/180))
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
	delete(h.refill, icao)
	delete(h.emptyTries, icao) // a new fill tries again
	delete(h.refillAt, icao)
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
	for obj := range h.retiring {
		errs = append(errs, h.client.AIRemoveObject(obj, h.req()))
		delete(h.retiring, obj)
	}
	return errors.Join(errs...)
}

// Dock takes stand's placeholder away and puts the helper of model's type
// there: the sim gives it the one free gate, and its jetway docks.
func (h *Helpers) Dock(icao string, stand int, model string) error {
	return h.DockAs(icao, stand, model, "")
}

// DockAs is Dock with helper title (a variant, HelperVariantTitle) instead
// of model's type's; "" that one.
func (h *Helpers) DockAs(icao string, stand int, model, title string) error {
	icao = strings.ToUpper(icao)
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.jetways[icao][stand] {
		return fmt.Errorf("no jetway at stand %d", stand)
	}
	if h.gateUnfit(icao, stand) {
		return ErrNoGate // its helpers go elsewhere, taking other gates' jetways
	}
	// While placeholders are still taking gates, other gates are free and
	// the sim gives the helper the first of them (live, stand 0 for 14):
	// asked again once they hold them.
	if h.filling[icao] || h.creating(icao) {
		return ErrFilling
	}
	for s, g := range h.gates[icao] {
		if s != stand && g.status == StatusUndocking && g.placeholder == 0 {
			return ErrFilling // its gate free until a placeholder is back
		}
		// Docks go in batches: those asked before the batch's helpers are
		// made are made together (live, BATC at LKPR: nine jetways docked
		// together); a helper landing on another gate of its batch that
		// waits for the same title docks there (adopt). Once made, the next
		// batch waits until they are on their gates (live, LKPR 11 created
		// as 4 docked took 4), and a gate docked again first (redock).
		if s != stand && g.status == StatusDocking && (g.createAt.IsZero() || !g.redockAt.IsZero()) {
			return ErrFilling
		}
	}
	for s, g := range h.gates[icao] {
		// A gate the sim gives placeholders, left with nothing on it (its
		// jetway taken): the sim gives it the next helper too (live, LKPR
		// A1 took the helpers of B14, C26 and B16 one after another).
		// Filled again first.
		if s != stand && h.emptyGate(icao, s, g) {
			if h.now.Before(h.refillAt[icao]) || h.freeing(icao) {
				// A fill stopped a while (FillStrays), or helpers on their
				// way (a placeholder now could take their gates): later.
				return ErrFilling
			}
			h.filling[icao], h.refill[icao] = true, true
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
	// The one jetway gate no placeholder took (our aircraft on it as they
	// filled) is the simulator's only free gate: a helper made now goes
	// there (live, LKPR B15 never docked while the helpers of other gates
	// were given it).
	if g.placeholder == 0 {
		if free := h.freeGates(icao); len(free) != 1 || free[0] != stand {
			return ErrNoGate
		}
	} else {
		if err := h.client.AIRemoveObject(g.placeholder, h.req()); err != nil {
			return err
		}
		delete(h.byObj, g.placeholder)
		g.placeholder = 0
	}
	// The helper once the gate is free (Update), with the rest of its batch:
	// all made together once the last gate freed is.
	at := h.now.Add(HelperDelay)
	for _, o := range h.gates[icao] {
		if o.status == StatusDocking && !o.createAt.IsZero() && o.redockAt.IsZero() {
			o.createAt, o.since = at, h.now
		}
	}
	g.createAt = at
	if title == "" {
		title = h.helperTitle(model)
	}
	if takes := h.gateTakes(icao, stand, title); takes != title {
		h.logf("jetway %s stand %d: %s never taken here, its helper %s", icao, stand, title, takes)
		title = takes
	}
	g.status, g.since, g.model, g.title = StatusDocking, h.now, model, title
	h.logf("jetway %s stand %d: docking (%s)", icao, stand, title)
	return nil
}

// emptyGate reports that icao's stand g is a gate the sim gives
// placeholders, left with nothing of ours on it (its jetway taken, its
// dock failed). h.mu held.
//
// Not a failed gate whose helpers go elsewhere more than they dock
// (GateUnfitMin): the sim does not give it the next helper, and filled
// again its placeholder went to a docked gate, the fill stopped and
// tried again, every dock at the airport held up meanwhile (live LKPR
// 2026-11-09: stand 19, 11 misses and 3 docks, no jetway for over ten
// minutes).
func (h *Helpers) emptyGate(icao string, stand int, g *gate) bool {
	if g.status == StatusFailed && h.unfitKnown(icao, stand) {
		return false
	}
	return h.accepted[icao][stand] && g.placeholder == 0 && g.helper == 0 &&
		g.status != StatusDocking && g.status != StatusDocked && g.status != StatusUndocking
}

// unfitKnown reports a gate whose helpers went elsewhere more than they
// docked, GateUnfitMin times or more (gateUnfit, without its retest).
// h.mu held.
func (h *Helpers) unfitKnown(icao string, stand int) bool {
	l := h.learned[fmt.Sprint(icao, " ", stand)]
	return l != nil && l.aways >= GateUnfitMin && l.aways > l.docks
}

// emptyGates counts icao's empty gates (emptyGate). h.mu held.
func (h *Helpers) emptyGates(icao string) int {
	n := 0
	for s, g := range h.gates[icao] {
		if h.emptyGate(icao, s, g) {
			n++
		}
	}
	return n
}

// redock docks stand g, its jetway just taken by another of our objects
// now on it, again first: whatever sits there leaves (the stray retired,
// a placeholder taken away) RetractWait later, its own helper HelperDelay
// after (Update). No refill of the airport, whose placeholders the sim
// puts on docked gates (live, LKPR 2026-11-08: steals, refills, twelve
// jetways pulled off parked aircraft); other docks wait meanwhile (one
// helper on its way at a time). h.mu held.
func (h *Helpers) redock(icao string, stand int, g *gate) {
	g.status, g.since, g.createAt = StatusDocking, h.now, time.Time{}
	g.redockAt = h.now.Add(RetractWait)
	h.logf("jetway %s stand %d: its jetway taken: docked again first, in %s", icao, stand, RetractWait)
}

// freeing reports a gate at icao freed for a helper not on it yet (made
// or still to make, or docked again first): no placeholder is made
// meanwhile, the sim could give it that gate. h.mu held.
func (h *Helpers) freeing(icao string) bool {
	for _, g := range h.gates[icao] {
		if g.status == StatusDocking && g.helper == 0 {
			return true
		}
	}
	return false
}

// gateTitle is the helper g docks with. h.mu held.
func (h *Helpers) gateTitle(g *gate) string {
	if g.title != "" {
		return g.title
	}
	return h.helperTitle(g.model)
}

// adopt docks helper p (made for its gate) at stand, where the sim put
// it, when stand is a gate of its batch waiting for a helper of the same
// title: the two gates swap, stand's own helper (made or still to make)
// going to p's gate. Helpers made together land on the free gates in the
// sim's order, not ours (live, LKPR 11 and 25 took each other's).
// Reports whether it did. h.mu held.
func (h *Helpers) adopt(icao string, p *pending, stand int) bool {
	g, w := h.gates[icao][stand], h.gates[icao][p.stand]
	if g == nil || w == nil || g.status != StatusDocking || w.status != StatusDocking || g.helper != 0 || !g.redockAt.IsZero() {
		return false
	}
	if h.gateTitle(g) != h.gateTitle(w) {
		return false
	}
	var q *pending
	for _, o := range h.byReq {
		if o.helper && o != p && o.icao == icao && o.stand == stand {
			q = o
			break
		}
	}
	switch {
	case q != nil:
		q.stand = p.stand
	case !g.createAt.IsZero():
		w.createAt, g.createAt = g.createAt, time.Time{}
	default:
		return false
	}
	h.logf("jetway %s: helper for stand %d put on stand %d, of its batch, same helper: docks there, the gates swapped", icao, p.stand, stand)
	p.stand = stand
	return true
}

// createHelper creates stand's helper. h.mu held.
func (h *Helpers) createHelper(icao string, stand int, g *gate) error {
	g.createAt = time.Time{}
	r := h.req()
	h.byReq[r] = &pending{icao: icao, stand: stand, helper: true, createdAt: h.now}
	title := g.title
	if title == "" {
		title = h.helperTitle(g.model)
	}
	return h.client.AICreateParkedATCAircraft(title, "JW"+fmt.Sprint(r%10000), icao, r)
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

// retire sends helper obj at icao off with the retract plan, so the jetway
// moving to it goes home, and removes it RetractWait later (Update);
// removed at once without a plan. h.mu held.
func (h *Helpers) retire(icao string, obj uint32) {
	delete(h.byObj, obj)
	if _, ok := h.retiring[obj]; ok {
		return // on its way already
	}
	if h.opt.RetractPlan != nil {
		if path, err := h.opt.RetractPlan(icao); err == nil {
			h.client.AISetAircraftFlightPlan(obj, path, h.req())
			h.retiring[obj] = h.now.Add(RetractWait)
			return
		}
	}
	h.client.AIRemoveObject(obj, h.req())
}

// retireHelper is dropHelper with its helper retired (retire), for a
// jetway that may still be moving to it. h.mu held.
func (h *Helpers) retireHelper(icao string, stand int, g *gate) {
	g.createAt = time.Time{}
	if g.helper != 0 {
		h.retire(icao, g.helper)
		g.helper = 0
	}
	for r, p := range h.byReq {
		if p.helper && p.icao == icao && p.stand == stand {
			if p.object != 0 {
				h.retire(icao, p.object)
			}
			delete(h.byReq, r)
		}
	}
}

// TitleFor is the helper DockAs makes for model at icao's stand, asked
// with title ("" model's type's): the gate's learned one when it never
// takes that (gateTakes).
func (h *Helpers) TitleFor(icao string, stand int, model, title string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if title == "" {
		title = h.helperTitle(model)
	}
	return h.gateTakes(strings.ToUpper(icao), stand, title)
}

// HelperFor is the title of the helper model docks with.
func (h *Helpers) HelperFor(model string) string { return h.helperTitle(model) }

// Types are the ICAO types it has helpers of.
func (h *Helpers) Types() []string { return slices.Clone(h.opt.Types) }

// HasJetway reports whether icao's stand has a jetway (from the jetway
// list, once Prepare has had it).
func (h *Helpers) HasJetway(icao string, stand int) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.jetways[strings.ToUpper(icao)][stand]
}

// Jetways are icao's stands with a jetway, in order (from the jetway
// list, once Prepare has had it).
func (h *Helpers) Jetways(icao string) []int {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []int
	for stand, ok := range h.jetways[strings.ToUpper(icao)] {
		if ok {
			out = append(out, stand)
		}
	}
	slices.Sort(out)
	return out
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

// Undock sends stand's jetway home. A docked helper stays as the gate's
// placeholder: given the retract plan (its jetway goes home, its claim on
// the gate kept) and released RetractWait later, as a placeholder is. A
// helper removed instead left the gate to a new placeholder, which the sim
// could put on a docked gate it counts free (live, LKPR 6, 8 and 18 taken
// after undocks). A helper not docked yet is removed, and a placeholder
// holds the gate again once the jetway is home.
func (h *Helpers) Undock(icao string, stand int) error {
	icao = strings.ToUpper(icao)
	h.mu.Lock()
	defer h.mu.Unlock()
	g := h.gate(icao, stand)
	if g.status != StatusDocking && g.status != StatusDocked {
		return nil
	}
	if g.status == StatusDocked && g.helper != 0 && h.keepAsPlaceholder(icao, g) {
		g.status, g.since = StatusUndocking, h.now
		h.logf("jetway %s stand %d: undocking, its helper kept as the placeholder", icao, stand)
		return nil
	}
	// Retired, not removed: its jetway may still be moving to it (removed,
	// it stayed half way for good: retiring).
	h.retireHelper(icao, stand, g)
	g.status, g.since, g.redockAt = StatusUndocking, h.now, time.Time{}
	h.logf("jetway %s stand %d: undocking", icao, stand)
	return nil
}

// Filling reports whether icao's gates are still being filled with
// placeholders (Dockable not known yet).
func (h *Helpers) Filling(icao string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.filling[strings.ToUpper(icao)]
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
// freeGates are icao's jetway gates nothing of ours holds (no placeholder,
// no helper, not docking or docked), in order. h.mu held.
func (h *Helpers) freeGates(icao string) []int {
	icao = strings.ToUpper(icao)
	var out []int
	for stand, ok := range h.jetways[icao] {
		if !ok {
			continue
		}
		if g := h.gates[icao][stand]; g != nil && (g.placeholder != 0 || g.helper != 0 || g.status == StatusDocking || g.status == StatusDocked || g.status == StatusUndocking) {
			continue
		}
		out = append(out, stand)
	}
	sort.Ints(out)
	return out
}

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
		helper, icao, stand, obj := p.helper, p.icao, p.stand, p.object
		if helper {
			h.logf("jetway %s stand %d: helper is object %d", icao, stand, obj)
		}
		model, title := "", ""
		if g := h.gates[icao][stand]; g != nil {
			model, title = g.model, g.title
		}
		h.mu.Unlock()
		// Our aircraft asked for outside h.mu: AircraftAt takes the
		// caller's locks, which call Status (h.mu) in turn.
		if helper && h.opt.AircraftAt != nil {
			if ours := h.opt.AircraftAt(icao, stand); ours != 0 {
				h.mu.Lock()
				h.measure(&placing{icao: icao, stand: stand, model: model, title: title, ours: ours, helper: obj})
				h.mu.Unlock()
			}
		}
		return true
	case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
		d := msg.AsSimObjectData()
		h.mu.Lock()
		pl, ok := h.placings[uint32(d.DwRequestID)]
		if !ok {
			h.mu.Unlock()
			return false
		}
		delete(h.placings, uint32(d.DwRequestID))
		at := *engine.CastDataAs[placePos](&d.DwData)
		if pl.placeholder {
			h.mu.Unlock()
			if h.opt.OnPlaceholder != nil {
				h.opt.OnPlaceholder(pl.icao, pl.stand, pl.model, at.Lat, at.Lon, at.Hdg)
			}
			return true
		}
		if uint32(d.DwObjectID) == pl.ours {
			pl.oursAt = &at
		} else {
			pl.helAt = &at
		}
		if pl.oursAt != nil && pl.helAt != nil {
			h.place(pl)
		}
		// The model's type's title as before, not the gate's (placingTitle):
		// the host's gate measure takes that title's nose gear, and the
		// gate's would change where ours parks (to be decided).
		title := h.helperTitle(pl.model)
		h.mu.Unlock()
		if uint32(d.DwObjectID) != pl.ours && h.opt.OnPlaceholder != nil {
			// The helper parked the sim's way too: that gate's measure
			// where no placeholder held it.
			h.opt.OnPlaceholder(pl.icao, pl.stand, title, at.Lat, at.Lon, at.Hdg)
		}
		return true
	case types.SIMCONNECT_RECV_ID_JETWAY_DATA:
		l := msg.AsJetwayData()
		h.mu.Lock()
		defer h.mu.Unlock()
		entries := l.Entries()
		h.lastList = entries
		for _, j := range entries {
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
	if p := h.byObj[att]; att != 0 && p != nil && p.icao == icao && p.helper && p.stand != stand {
		h.adopt(icao, p, stand)
	}
	if p := h.byObj[att]; att != 0 && p != nil && p.icao == icao {
		g := h.gate(icao, stand)
		switch {
		case !p.helper && g.placeholder != att:
			// A placeholder took this gate: retract its claim (flight plan),
			// release it a while after (RetractWait).
			taken := g.status == StatusDocked || g.status == StatusDocking
			if taken {
				// On a docked gate (the sim counts docked gates free, at any
				// age: live, LKPR 13 s to 50 min after docking; removing the
				// placeholder at once did not bring the jetway back, live
				// LKPR 15): the jetway went with it; ours is dropped and the
				// gate docked again from this placeholder first (redock). The
				// fill stops a while (FillStrays): no cascade.
				h.logf("jetway %s stand %d: its jetway taken by a placeholder", icao, stand)
				h.dropHelper(icao, stand, g)
				if h.strays[icao]++; h.strays[icao] >= FillStrays && h.filling[icao] {
					// Each stopped fill waits twice as long (the sim keeps
					// giving docked gates: live LKPR, every 2 min a gate lost).
					h.strayWait[icao] = min(max(RefillEvery, 2*h.strayWait[icao]), StrayWaitMax)
					h.logf("jetway %s: %d placeholders on docked gates: filling again in %s", icao, h.strays[icao], h.strayWait[icao])
					delete(h.filling, icao)
					h.refillAt[icao] = h.now.Add(h.strayWait[icao])
					h.strays[icao] = 0
				}
			}
			g.placeholder, g.status, g.since = att, StatusReserved, h.now
			if h.accepted[icao] == nil {
				h.accepted[icao] = map[int]bool{}
			}
			h.accepted[icao][stand] = true // the sim gives this gate to a ParkedATC
			if h.opt.OnPlaceholder != nil {
				h.measurePlaceholder(icao, stand, att)
			}
			if taken {
				h.redock(icao, stand, g)
			}
			if path, err := h.opt.RetractPlan(icao); err == nil {
				h.client.AISetAircraftFlightPlan(att, path, h.req())
				p.retractAt, p.retracting = h.now.Add(RetractWait), true
			}
		case p.helper && p.stand != stand:
			// The sim put the helper on another gate: not ours.
			h.logf("jetway %s: helper for stand %d took stand %d: removed", icao, p.stand, stand)
			h.logList(icao)
			h.retire(icao, att)
			for r, q := range h.byReq {
				if q == p {
					delete(h.byReq, r) // done with
				}
			}
			h.gateResult(icao, p.stand, false)
			if w := h.gate(icao, p.stand); w.status == StatusDocking {
				w.status, w.since = StatusFailed, h.now
				// A type never docked there: the gate does not take it (a
				// type docked there before went elsewhere by chance, kept).
				if l := h.learned[fmt.Sprint(icao, " ", p.stand)]; w.title != "" && (l == nil || l.docked != w.title) {
					h.learn(icao, p.stand, w.title, false, true)
				}
			}
			// A docked gate taken (the sim does not always count a docked
			// helper's gate as taken; live, LKPR B9 1.5 min after it docked):
			// its jetway follows the new helper off, so that gate is cleared
			// too and docked again first, once the stray has gone (redock).
			// No refill: the next dock asked refills the emptied gates
			// first (DockAs).
			if g.status == StatusDocked || g.status == StatusDocking {
				h.dropHelper(icao, stand, g)
				h.redock(icao, stand, g)
			}
		case p.helper:
			g.helper = att
			if j.Status == 7 {
				if g.status != StatusDocked {
					h.logf("jetway %s stand %d: docked", icao, stand)
					h.logHandle(icao, stand, j)
					g.since = h.now
					h.gateResult(icao, stand, true)
					if h.opt.OnDock != nil {
						h.opt.OnDock(icao, stand, g.model, true)
					}
					if l := h.learned[fmt.Sprint(icao, " ", stand)]; g.title != "" && (l == nil || l.docked != g.title) {
						h.learn(icao, stand, g.title, true, true)
					}
				}
				g.status = StatusDocked
			}
		}
	}
	if j.Status == 0 && att == 0 {
		h.rest[fmt.Sprint(icao, " ", stand)] = j.MainHandlePos
	}
	if g := h.gates[icao][stand]; g != nil && g.status == StatusUndocking && j.Status == 0 && (att == 0 || att == g.placeholder) && h.now.Sub(g.since) >= RetractWait {
		// Home (the list says so before it is: live, 16 s after the helper
		// went, still moving; it takes about 24 s): a placeholder holds the
		// gate again: the helper kept (Undock), else a new one.
		h.logf("jetway %s stand %d: home, %s after the undock", icao, stand, h.now.Sub(g.since).Round(time.Second))
		if g.placeholder != 0 {
			g.status = StatusReserved
		} else {
			g.status = StatusNone
			h.spawnPlaceholder(icao)
		}
	}
}

// logHandle logs a docked jetway's main handle and exit door beside its
// handle at rest: the data to tell a "docked" whose jetway never moved
// (live, LKPR C21 2026-11-08) from a real dock, not judged on yet: what the
// fields hold at rest and docked was never read live. h.mu held.
func (h *Helpers) logHandle(icao string, stand int, j types.SIMCONNECT_JETWAY_DATA) {
	m, d := j.MainHandlePos, j.ExitDoorRelativePos
	r, ok := h.rest[fmt.Sprint(icao, " ", stand)]
	if !ok {
		h.logf("jetway %s stand %d: handle %.2f %.2f %.2f, door %.2f %.2f %.2f (none seen at rest)", icao, stand, m.X, m.Y, m.Z, d.X, d.Y, d.Z)
		return
	}
	moved := math.Sqrt((m.X-r.X)*(m.X-r.X) + (m.Y-r.Y)*(m.Y-r.Y) + (m.Z-r.Z)*(m.Z-r.Z))
	h.logf("jetway %s stand %d: handle %.2f %.2f %.2f, at rest %.2f %.2f %.2f (moved %.2f), door %.2f %.2f %.2f", icao, stand, m.X, m.Y, m.Z, r.X, r.Y, r.Z, moved, d.X, d.Y, d.Z)
}

// keepAsPlaceholder turns g's docked helper into its placeholder: the
// retract plan now, released RetractWait later (Update). False without a
// retract plan. h.mu held.
func (h *Helpers) keepAsPlaceholder(icao string, g *gate) bool {
	if h.opt.RetractPlan == nil {
		return false
	}
	path, err := h.opt.RetractPlan(icao)
	if err != nil {
		return false
	}
	obj := g.helper
	if err := h.client.AISetAircraftFlightPlan(obj, path, h.req()); err != nil {
		return false
	}
	p := h.byObj[obj]
	if p == nil {
		p = &pending{icao: icao, createdAt: h.now, object: obj}
		h.byObj[obj] = p
	}
	for r, q := range h.byReq {
		if q == p {
			delete(h.byReq, r) // its creation's entry: one entry, the placeholder's
		}
	}
	p.helper, p.stand = false, -1
	p.retractAt, p.retracting = h.now.Add(RetractWait), true
	h.byReq[h.req()] = p
	g.placeholder, g.helper, g.createAt = obj, 0, time.Time{}
	return true
}

// keepOnGate keeps stand's helper, created and put on stand by the sim
// (its jetway did not dock), as the gate's placeholder (keepAsPlaceholder).
// False when it has no object yet or was put on another gate. h.mu held.
func (h *Helpers) keepOnGate(icao string, stand int, g *gate) bool {
	if g.helper == 0 {
		for _, p := range h.byReq {
			if p.helper && p.icao == icao && p.stand == stand && p.object != 0 {
				g.helper = p.object
				break
			}
		}
	}
	if g.helper == 0 {
		return false
	}
	g.createAt = time.Time{}
	return h.keepAsPlaceholder(icao, g)
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
	busy, coming := false, false // coming: a dock on its way
	for r, p := range h.byReq {
		if p.helper {
			g := h.gate(p.icao, p.stand)
			switch {
			case p.object != 0 && g.helper == p.object:
				delete(h.byReq, r) // on its gate
			case p.object != 0 && now.Sub(p.createdAt) > ClaimTimeout:
				// No jetway took it: put on a gate without one, or a ramp.
				h.logf("jetway %s stand %d: helper not on a jetway gate: removed", p.icao, p.stand)
				// Its gate refilled when the next dock is asked (DockAs), not
				// at once: each placeholder another chance at a docked gate.
				h.retireHelper(p.icao, p.stand, g)
				g.status, g.since = StatusFailed, now
			default:
				busy, coming = true, true
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
			if len(h.dockable(p.icao)) == 0 && h.emptyTries[p.icao] < EmptyFillTries {
				h.emptyTries[p.icao]++
				break
			}
			delete(h.filling, p.icao)
			delete(h.refill, p.icao)
			h.strays[p.icao], h.strayWait[p.icao] = 0, 0
			h.logf("jetway %s: gates filled (%d)", p.icao, len(h.dockable(p.icao)))
			// None at all: tried again a while later, not left without jetways
			// until a restart (live, LKPR one evening: no gate taken for hours,
			// the next morning all 19 at once).
			if len(h.dockable(p.icao)) == 0 {
				h.refillAt[p.icao], h.emptyTries[p.icao] = now.Add(RefillEvery), 0
			}
		default:
			busy = true
		}
	}
	for obj, at := range h.retiring {
		if !now.Before(at) {
			h.client.AIRemoveObject(obj, h.req())
			delete(h.retiring, obj)
		}
	}
	for icao, m := range h.gates {
		for stand, g := range m {
			if g.status == StatusDocking && !g.redockAt.IsZero() && !now.Before(g.redockAt) {
				// Docked again (redock): what sat on it gone, its own helper
				// next.
				g.redockAt = time.Time{}
				if obj := g.placeholder; obj != 0 {
					h.client.AIRemoveObject(obj, h.req())
					delete(h.byObj, obj)
					for r, p := range h.byReq {
						if p.object == obj {
							delete(h.byReq, r)
						}
					}
					g.placeholder = 0
				}
				g.createAt, g.since = now.Add(HelperDelay), now
			}
			if g.status == StatusDocking && !g.createAt.IsZero() && !now.Before(g.createAt) {
				h.createHelper(icao, stand, g)
			}
			if (g.status == StatusDocking || g.status == StatusUndocking) && now.Sub(g.since) > DockTimeout {
				h.logf("jetway %s stand %d: %s timed out", icao, stand, g.status)
				if g.status == StatusDocking && h.opt.OnDock != nil {
					h.opt.OnDock(icao, stand, g.model, false) // its jetway never came
				}
				switch {
				case g.status == StatusUndocking && g.placeholder != 0:
					// Its helper kept (Undock) holds the gate.
					g.status = StatusReserved
				case g.status == StatusDocking && h.keepOnGate(icao, stand, g):
					// Its helper on the gate holds it: no refill, whose
					// placeholders the sim put on docked gates (live, LKPR 18
					// timed out, then 2, 15 and 17 lost their jetways).
					g.status, g.since = StatusFailed, now
				default:
					// Its gate free: refilled when the next dock is asked.
					h.retireHelper(icao, stand, g)
					g.status = StatusFailed
				}
			}
			if g.status == StatusDocking || g.status == StatusUndocking {
				busy = true
			}
			if g.status == StatusDocking {
				coming = true
			}
		}
	}
	for icao, at := range h.refillAt {
		if !now.Before(at) {
			delete(h.refillAt, icao)
			h.filling[icao] = true
		}
	}
	// Filling: one placeholder at a time (its gate known before the next).
	for icao := range h.filling {
		busy = true
		if h.creating(icao) || len(h.jetways[icao]) == 0 || h.freeing(icao) {
			continue
		}
		if h.refill[icao] && h.emptyGates(icao) == 0 {
			delete(h.filling, icao)
			delete(h.refill, icao)
			h.logf("jetway %s: emptied gates held again (%d)", icao, len(h.dockable(icao)))
			continue
		}
		h.spawnPlaceholder(icao)
	}
	every := pollSlow
	if coming {
		every = pollEvery
	}
	if busy && now.Sub(h.lastPoll) >= every {
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

// logList logs icao's jetways as the last list gave them (stand, status,
// attached object) beside what we hold there: when the simulator gives a
// gate to the wrong helper, which objects it saw where. h.mu held.
func (h *Helpers) logList(icao string) {
	var b strings.Builder
	for _, j := range h.lastList {
		if strings.ToUpper(strings.TrimRight(string(j.AirportIcao[:]), "\x00 ")) != icao {
			continue
		}
		stand := int(j.ParkingIndex)
		fmt.Fprintf(&b, " %d:s%d/a%d", stand, j.Status, j.AttachedObjectId)
		if g := h.gates[icao][stand]; g != nil {
			fmt.Fprintf(&b, "(ours p%d h%d %s)", g.placeholder, g.helper, g.status)
		}
	}
	h.logf("jetway %s list:%s", icao, b.String())
}
