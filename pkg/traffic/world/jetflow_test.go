package world

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/jetway"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// The jetway flow offline (LKPR 2026-11-08 log audit): the world's
// jetwayTick against the real helper backend on a fake simulator, the
// jetway list and assigned objects fed as the simulator sends them.

type jwCreated struct {
	title, tail string
	req         uint32
}

type jwSim struct {
	created         []jwCreated
	removed, plans  []uint32
	jetwayListsRead int
}

func (f *jwSim) AICreateParkedATCAircraft(title, tail, icao string, req uint32) error {
	f.created = append(f.created, jwCreated{title, tail, req})
	return nil
}
func (f *jwSim) AISetAircraftFlightPlan(obj uint32, path string, req uint32) error {
	f.plans = append(f.plans, obj)
	return nil
}
func (f *jwSim) AIReleaseControl(obj, req uint32) error { return nil }
func (f *jwSim) AIRemoveObject(obj, req uint32) error {
	f.removed = append(f.removed, obj)
	return nil
}
func (f *jwSim) RequestJetwayData(icao string, n uint32, idx *int32) error {
	f.jetwayListsRead++
	return nil
}
func (f *jwSim) AddToDataDefinition(uint32, string, string, types.SIMCONNECT_DATATYPE, float32, uint32) error {
	return nil
}
func (f *jwSim) RequestDataOnSimObject(uint32, uint32, uint32, types.SIMCONNECT_PERIOD, types.SIMCONNECT_DATA_REQUEST_FLAG, uint32, uint32, uint32) error {
	return nil
}
func (f *jwSim) SetDataOnSimObject(uint32, uint32, types.SIMCONNECT_DATA_SET_FLAG, uint32, uint32, unsafe.Pointer) error {
	return nil
}

type jwBed struct {
	t    *testing.T
	cc   *controlCenter
	svc  *jetway.Helpers
	sim  *jwSim
	g    *airport.Graph
	now  time.Time
	obj  uint32
	seen int // creations handled
	logs []string
	// claim: the stand the next placeholder takes (-1: none, left
	// unclaimed).
	claim []int
}

// jwState is a jetway entry of the list: stand, status, attached object.
type jwState struct {
	stand       int
	status, att uint32
}

func newJWBed(t *testing.T, types ...string) *jwBed {
	t.Helper()
	jetwaysOn.mu.Lock()
	wasOn := jetwaysOn.on
	jetwaysOn.on = true
	jetwaysOn.mu.Unlock()
	dir := t.TempDir()
	gateLessons.load(filepath.Join(dir, "jetway-gates.json"))
	t.Cleanup(func() {
		jetwaysOn.mu.Lock()
		jetwaysOn.on = wasOn
		jetwaysOn.mu.Unlock()
		gateLessons.saving.Wait()
		gateLessons.load("")
	})
	w := New(Options{DataDir: dir})
	b := &jwBed{t: t, sim: &jwSim{}, obj: 1000, now: traffic.Now()}
	b.cc = newControlCenter(nil, w.st.core)
	b.cc.fuelTitles.stairsStock = []string{"ASO_Boarding_Stairs"}
	b.cc.fuelTitles.busesStock = []string{"Bus Apron 02"}
	b.cc.fuelTitles.gpusStock = []string{"ASO_GPU"}
	if len(types) == 0 {
		types = []string{"A320", "B738"}
	}
	b.svc = jetway.NewHelpers(b.sim, jetway.HelperOptions{Types: types, RetractPlan: func(string) (string, error) { return "out.pln", nil },
		RequestBase: 50_000, OnDock: b.cc.dockResult,
		Log: func(f string, a ...any) { b.logs = append(b.logs, fmt.Sprintf(f, a...)) }})
	b.cc.jw.mu.Lock()
	b.cc.jw.svc = b.svc
	b.cc.jw.prepared, b.cc.jw.docked = map[string]bool{"LKPR": true}, map[string]bool{}
	b.cc.jw.mu.Unlock()
	g, err := airport.BuildGraph(lkprLayout(t))
	if err != nil {
		t.Fatal(err)
	}
	b.g = g
	return b
}

func (b *jwBed) stand(label string) int {
	b.t.Helper()
	s, err := b.g.Layout.ParkingIndex(label)
	if err != nil || !b.g.Layout.Parking[s].IsGate() {
		b.t.Fatalf("no gate %s", label)
	}
	return s
}

// list feeds the simulator's jetway list.
func (b *jwBed) list(js ...jwState) {
	const header = 28
	buf := make([]byte, header+len(js)*types.JetwayDataSize)
	le := binary.LittleEndian
	le.PutUint32(buf[0:], uint32(len(buf)))
	le.PutUint32(buf[8:], uint32(types.SIMCONNECT_RECV_ID_JETWAY_DATA))
	le.PutUint32(buf[16:], uint32(len(js)))
	le.PutUint32(buf[24:], 1)
	for i, j := range js {
		e := buf[header+i*types.JetwayDataSize:]
		copy(e[0:8], "LKPR")
		le.PutUint32(e[8:], uint32(j.stand))
		le.PutUint32(e[48:], j.status)
		le.PutUint32(e[156:], j.att)
	}
	b.svc.Handle(engine.Message{SIMCONNECT_RECV: (*types.SIMCONNECT_RECV)(unsafe.Pointer(&buf[0]))})
}

// assign answers creation c with a new object, returned.
func (b *jwBed) assign(c jwCreated) uint32 {
	b.obj++
	buf := make([]byte, 20)
	le := binary.LittleEndian
	le.PutUint32(buf[0:], 20)
	le.PutUint32(buf[8:], uint32(types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID))
	le.PutUint32(buf[12:], c.req)
	le.PutUint32(buf[16:], b.obj)
	b.svc.Handle(engine.Message{SIMCONNECT_RECV: (*types.SIMCONNECT_RECV)(unsafe.Pointer(&buf[0]))})
	return b.obj
}

// tick runs the world's jetway tick d later. New placeholders are given
// objects: on b.claim's next stand, else nowhere (unclaimed). New helpers
// are returned for the test to place.
func (b *jwBed) tick(d time.Duration) (helpers []jwCreated) {
	b.now = b.now.Add(d)
	b.cc.jetwayTick(b.now)
	for ; b.seen < len(b.sim.created); b.seen++ {
		c := b.sim.created[b.seen]
		if strings.HasPrefix(c.tail, "JW") {
			helpers = append(helpers, c)
			continue
		}
		obj := b.assign(c)
		if len(b.claim) > 0 {
			s := b.claim[0]
			b.claim = b.claim[1:]
			if s >= 0 {
				b.list(jwState{s, 1, obj})
			}
		}
	}
	return helpers
}

// ticks runs n one-second ticks, collecting new helpers.
func (b *jwBed) ticks(n int) (helpers []jwCreated) {
	for i := 0; i < n; i++ {
		helpers = append(helpers, b.tick(time.Second)...)
	}
	return helpers
}

// fill has placeholders take stands (the jetway gates listed are all of
// jetways), then one unclaimed: the fill done, the placeholders released.
func (b *jwBed) fill(jetways []int, hold ...int) {
	b.t.Helper()
	var l []jwState
	for _, s := range jetways {
		l = append(l, jwState{s, 0, 0})
	}
	b.list(l...)
	if err := b.svc.Prepare("LKPR"); err != nil {
		b.t.Fatal(err)
	}
	b.claim = append(append([]int{}, hold...), -1)
	b.ticks(len(hold) + 1)
	b.ticks(int((jetway.ClaimTimeout + jetway.RetractWait) / time.Second))
	if b.svc.Filling("LKPR") || len(b.svc.Dockable("LKPR")) != len(hold) {
		b.t.Fatalf("filling %v, dockable %v, want %v", b.svc.Filling("LKPR"), b.svc.Dockable("LKPR"), hold)
	}
}

type jwArr struct {
	arrivalCtl
}

func (a *jwArr) State() traffic.ArrivalState { return traffic.ArrivalParked }

type jwDep struct {
	departureCtl
	added int
}

func (d *jwDep) State() traffic.TaxiState { return traffic.TaxiAwaitingPushback }
func (d *jwDep) AddStandServices(stairs, gpu traffic.FuelService, buses []traffic.FuelService) bool {
	d.added++
	return true
}

type jwStairs struct{ traffic.FuelService }

// park puts our aircraft id on stand: an arrival parked, else a departure
// waiting for its push (stand services from mk, nil none).
func (b *jwBed) park(id int, tail, model string, stand int, obj uint32, arrival bool) *controlled {
	it := &controlled{ID: id, Tail: tail, ICAO: "LKPR", cc: b.cc, graph: b.g, stand: stand, objectID: obj, removed: make(chan struct{}), spoken: map[string]bool{}}
	it.view.Model = model
	if arrival {
		it.arr = &jwArr{}
	} else {
		it.dep = &jwDep{}
		it.standServices = func() (traffic.FuelService, traffic.FuelService, []traffic.FuelService) {
			return &jwStairs{}, nil, nil
		}
	}
	b.cc.mu.Lock()
	b.cc.items[id] = it
	b.cc.mu.Unlock()
	return it
}

func (b *jwBed) leave(id int) {
	b.cc.mu.Lock()
	delete(b.cc.items, id)
	b.cc.mu.Unlock()
}

func (b *jwBed) fails(stand int) int {
	b.cc.jw.mu.Lock()
	defer b.cc.jw.mu.Unlock()
	return b.cc.jw.fails[gateKey("LKPR", stand)]
}

const jwA320 = "FSLTL_FAIB_A320_SmartWings"

// C: a dock refused tick after tick (the airport refilling, another gate
// docking) after one real failure counts that one failure, never one per
// refused second, and leaves no "no jetway for this type" lesson (live,
// LKPR 2026-11-08: 3 refused seconds rested the gate, 10 false lessons).
func TestJetflowRefusedTicksNoFalseFailures(t *testing.T) {
	b := newJWBed(t)
	x, s, u := b.stand("B15"), b.stand("B14"), b.stand("C24")
	b.fill([]int{x, s, u}, x, s, u)
	b.park(1, "SWR662", jwA320, x, 7001, true)
	b.tick(time.Second) // its dock asked
	h := b.tick(jetway.HelperDelay)
	if len(h) != 1 {
		t.Fatalf("helpers %v", h)
	}
	b.list(jwState{s, 1, b.assign(h[0])}) // put on B14: a real failure
	b.tick(time.Second)
	if b.svc.Status("LKPR", x) != jetway.StatusFailed {
		t.Fatalf("B15 %s", b.svc.Status("LKPR", x))
	}
	// Another aircraft at C24: the refill and its dock keep every dock
	// refused from now on (placeholders never claimed).
	b.park(2, "CSA871", jwA320, u, 7002, true)
	b.ticks(90)
	if n := b.fails(x); n > 1 {
		t.Errorf("B15 counted %d failed docks, one happened", n)
	}
	if gateLessons.noJetway("LKPR", x, "A320", b.now) {
		t.Error("a 'no jetway for A320' lesson saved from refused ticks")
	}
}

// D: a gate whose failed dock was followed by a refill placeholder
// (status reserved, not failed) is asked again for the aircraft on it
// (live, LKPR: gates never asked again after a refill).
func TestJetflowAskAgainAfterRefill(t *testing.T) {
	b := newJWBed(t)
	x, s, u := b.stand("B15"), b.stand("B14"), b.stand("C24")
	b.fill([]int{x, s, u}, x, s, u)
	b.park(1, "SWR662", jwA320, x, 7001, true)
	b.tick(time.Second)
	h := b.tick(jetway.HelperDelay)
	b.list(jwState{s, 1, b.assign(h[0])})
	// A refill: its placeholder takes B15 again.
	b.svc.Prepare("LKPR")
	b.claim = []int{x, -1}
	b.ticks(40)
	if st := b.svc.Status("LKPR", x); st != jetway.StatusReserved {
		t.Fatalf("B15 %s after the refill", st)
	}
	b.ticks(40)
	if st := b.svc.Status("LKPR", x); st != jetway.StatusDocking {
		t.Errorf("B15 %s a minute after the refill, want asked again (docking)", st)
	}
}

// D: a turnaround's departure (a new entry, the same aircraft) whose
// arrival used up its tries is asked a dock of its own (live, LKPR: the
// asked mark and tries kept by gate, the departure inherited them).
func TestJetflowTurnaroundAsksAgain(t *testing.T) {
	b := newJWBed(t)
	x, s, u := b.stand("B15"), b.stand("B14"), b.stand("C24")
	b.fill([]int{x, s, u}, x, s, u)
	b.park(1, "SWR662", jwA320, x, 7001, true)
	tries := 0
	for i := 0; i < 400 && tries < JetwayDockTries; i++ {
		for _, c := range b.tick(time.Second) {
			tries++
			b.list(jwState{s, 1, b.assign(c)}) // each put on B14
		}
	}
	if tries < JetwayDockTries {
		t.Fatalf("%d docks tried", tries)
	}
	b.ticks(70)
	b.leave(1)
	b.park(2, "SWR662", jwA320, x, 7001, false)
	n := len(b.sim.created)
	b.ticks(70)
	asked := false
	for _, c := range b.sim.created[n:] {
		asked = asked || strings.HasPrefix(c.tail, "JW")
	}
	if !asked && b.svc.Status("LKPR", x) != jetway.StatusDocking {
		t.Errorf("the turnaround's departure never asked for its jetway (B15 %s)", b.svc.Status("LKPR", x))
	}
}

// E: a gate the simulator gives no helper (ErrNoGate) hands the departure
// on it stairs, a GPU and buses at once; the stairs are made for it.
func TestJetflowNoGateGetsStairs(t *testing.T) {
	b := newJWBed(t)
	x, s, u, v := b.stand("B15"), b.stand("B14"), b.stand("C24"), b.stand("C22")
	b.fill([]int{x, s, u, v}, s, u) // B15 and C22 free: no helper for B15
	it := b.park(1, "SWR662", jwA320, x, 7001, false)
	b.ticks(3)
	if d := it.dep.(*jwDep); d.added != 1 {
		t.Fatalf("stand services given %d times, want once", d.added)
	}
	r := SpawnRequest{Kind: "departure", Stand: x, Model: jwA320, StandUse: standAirline}
	if b.cc.stairs(r, b.g, 0, traffic.ProfileFor(jwA320).Motion) == nil {
		t.Error("no stairs made for it")
	}
}

// E: a dock refused for about a minute (another gate docking that never
// docks) gives the departure stairs instead.
func TestJetflowRefusedMinuteGetsStairs(t *testing.T) {
	b := newJWBed(t)
	x, s, u := b.stand("B15"), b.stand("B14"), b.stand("C24")
	b.fill([]int{x, s, u}, x, s, u)
	b.park(2, "CSA871", jwA320, u, 7002, true)
	b.ticks(5) // C24 docking, its helper made and never placed (on its way 90 s)
	it := b.park(1, "SWR662", jwA320, x, 7001, false)
	b.ticks(30)
	d := it.dep.(*jwDep)
	if d.added != 0 {
		t.Fatal("stairs after 30 s")
	}
	b.ticks(40)
	if d.added != 1 {
		t.Errorf("stand services given %d times after 70 s refused, want once", d.added)
	}
}

// E: an arrival on a gate that gives it no helper has no jetway coming:
// its turnaround departure is made with stairs.
func TestJetflowArrivalNoGate(t *testing.T) {
	b := newJWBed(t)
	x, s, u, v := b.stand("B15"), b.stand("B14"), b.stand("C24"), b.stand("C22")
	b.fill([]int{x, s, u, v}, s, u)
	b.park(1, "SWR662", jwA320, x, 7001, true)
	b.ticks(3)
	if !b.cc.gateWithoutJetway("LKPR", x, jwA320) {
		t.Error("its turnaround would wait for a jetway that never comes")
	}
}

// E: stairs not to be had yet are asked for again, not given up.
func TestJetflowStairsQueued(t *testing.T) {
	b := newJWBed(t)
	x, s, u, v := b.stand("B15"), b.stand("B14"), b.stand("C24"), b.stand("C22")
	b.fill([]int{x, s, u, v}, s, u)
	it := b.park(1, "SWR662", jwA320, x, 7001, false)
	calls := 0
	it.standServices = func() (traffic.FuelService, traffic.FuelService, []traffic.FuelService) {
		calls++
		if calls == 1 {
			return nil, nil, nil
		}
		return &jwStairs{}, nil, nil
	}
	b.ticks(5)
	if d := it.dep.(*jwDep); d.added != 1 {
		t.Errorf("given %d times after none to be had once, want once", d.added)
	}
}

// E: a departure whose jetway never came is not let go after
// JetwayWaitMax with nothing at its door: it waits for stairs.
func TestJetflowBoardingWaitsForStairs(t *testing.T) {
	b := newJWBed(t)
	x, s, u := b.stand("B15"), b.stand("B14"), b.stand("C24")
	b.fill([]int{x, s, u}, x, s, u)
	it := b.park(1, "SWR662", jwA320, x, 7001, false)
	b.svc.Prepare("LKPR") // filling: its jetway still to come
	it.mu.Lock()
	it.boardWaitSince = b.cc.clock.Now().Add(-JetwayWaitMax - time.Minute)
	left := it.boardingLeft()
	it.mu.Unlock()
	if left == 0 {
		t.Error("let go with no jetway and no stairs")
	}
}

// A/B: a helper of a wider type than the aircraft's (the gate's learned
// helper: LKPR C24 took the A332's) is not made while a gate it fits is
// docked: the simulator gives it that gate, taking its jetway (live, LKPR
// 2026-11-08: C24's A332 helper for a B738 went to docked C22).
func TestJetflowWideHelperWaitsForDockedGate(t *testing.T) {
	b := newJWBed(t, "A320", "B738", "A332")
	c24, c22, b14 := b.stand("C24"), b.stand("C22"), b.stand("B14")
	b.svc.Learn("LKPR", c24, jetway.HelperTitle("B738", false), false)
	b.svc.Learn("LKPR", c24, jetway.HelperTitle("A332", false), true)
	b.fill([]int{c24, c22, b14}, c24, c22, b14)
	b.park(1, "QTR291", "FSLTL_A332_QTR", c22, 7001, true)
	b.tick(time.Second)
	h := b.tick(jetway.HelperDelay)
	if len(h) != 1 {
		t.Fatalf("helpers %v", h)
	}
	obj := b.assign(h[0])
	b.list(jwState{c22, 1, obj})
	b.list(jwState{c22, 7, obj})
	if b.svc.Status("LKPR", c22) != jetway.StatusDocked {
		t.Fatalf("C22 %s", b.svc.Status("LKPR", c22))
	}
	b.ticks(int(jetway.DockSettle/time.Second) + 5)
	b.park(2, "RYR1", "FSLTL_FAIB_B738_RYR", c24, 7002, true)
	b.ticks(3)
	if st := b.svc.Status("LKPR", c24); st == jetway.StatusDocking {
		t.Error("C24's A332 helper made with C22 docked")
	}
}

// H: the jetway starts home as the crew asks for the push, not once ground
// answers; home is logged with the time it took.
func TestJetflowUndockAtPushRequest(t *testing.T) {
	b := newJWBed(t)
	b.now = traffic.Now().Add(-time.Hour) // boarding long done by the clock
	x, s, u := b.stand("B15"), b.stand("B14"), b.stand("C24")
	b.fill([]int{x, s, u}, x, s, u)
	it := b.park(1, "SWR662", jwA320, x, 7001, false)
	b.tick(time.Second)
	h := b.tick(jetway.HelperDelay)
	if len(h) != 1 {
		t.Fatalf("helpers %v", h)
	}
	obj := b.assign(h[0])
	b.list(jwState{x, 1, obj})
	b.list(jwState{x, 7, obj})
	if b.svc.Status("LKPR", x) != jetway.StatusDocked {
		t.Fatalf("B15 %s", b.svc.Status("LKPR", x))
	}
	it.gates.Store(true) // the user answers: ground says nothing
	it.mu.Lock()
	it.delivered = true
	it.request = "pushback"
	it.onRequest("pushback")
	it.mu.Unlock()
	if st := b.svc.Status("LKPR", x); st != jetway.StatusUndocking {
		t.Fatalf("B15 %s after the crew asked for the push, want undocking", st)
	}
	b.ticks(int(jetway.RetractWait/time.Second) + 2)
	b.list(jwState{x, 0, 0})
	home := false
	for _, l := range b.logs {
		home = home || strings.Contains(l, fmt.Sprintf("stand %d: home, ", x))
	}
	if !home {
		t.Errorf("home not logged: %q", b.logs)
	}
}

// F: aircraft of several types parked at once get their jetways together,
// in seconds, not one gate at a time two minutes apart (live, BATC at
// LKPR: nine jetways docked together about 65 s after spawn). Two of one
// type whose helpers the sim swaps dock all the same.
func TestJetflowParallelDocks(t *testing.T) {
	b := newJWBed(t, "A320", "B738", "A321")
	b15, b14, c24, c22 := b.stand("B15"), b.stand("B14"), b.stand("C24"), b.stand("C22")
	b.fill([]int{b15, b14, c24, c22}, b15, b14, c24, c22)
	at := map[string][]int{}
	for i, a := range []struct {
		model string
		stand int
	}{{jwA320, b15}, {"FSLTL_FAIB_B738_RYR", b14}, {"FSLTL_A321_SWR", c24}, {"FSLTL_A320_EZY", c22}} {
		b.park(i+1, fmt.Sprint("T", i), a.model, a.stand, uint32(7001+i), true)
	}
	// Where the sim puts each title's helpers: the two A320s swapped.
	at[jetway.HelperTitle("A320", false)] = []int{c22, b15}
	at[jetway.HelperTitle("B738", false)] = []int{b14}
	at[jetway.HelperTitle("A321", false)] = []int{c24}
	for i := 0; i < 30; i++ {
		for _, c := range b.tick(time.Second) {
			l := at[c.title]
			if len(l) == 0 {
				t.Fatalf("an extra %s", c.title)
			}
			obj := b.assign(c)
			b.list(jwState{l[0], 1, obj})
			b.list(jwState{l[0], 7, obj})
			at[c.title] = l[1:]
		}
	}
	for _, s := range []int{b15, b14, c24, c22} {
		if st := b.svc.Status("LKPR", s); st != jetway.StatusDocked {
			t.Errorf("%s %s 30 s after all parked, want docked", b.g.Layout.Parking[s].Label(), st)
		}
	}
}

// C: "no jetway for this type" lessons a gate's rest saved (no "why") are
// dropped at load; those of a timed-out dock kept.
func TestJetflowFalseLessonsDropped(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "jetway-gates.json")
	now := traffic.Now().Unix()
	body := fmt.Sprintf(`[{"icao":"LKPR","stand":20,"type":"B738","noJetwayAt":%d},{"icao":"LKPR","stand":21,"type":"A321","noJetwayAt":%d,"why":"timeout"},{"icao":"LKPR","stand":11,"helper":"MyCrew Jetway Helper BCS3"}]`, now, now)
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { gateLessons.saving.Wait(); gateLessons.load("") })
	gateLessons.load(file)
	if gateLessons.noJetway("LKPR", 20, "B738", traffic.Now()) {
		t.Error("a rest's lesson kept")
	}
	if !gateLessons.noJetway("LKPR", 21, "A321", traffic.Now()) {
		t.Error("a timed-out dock's lesson dropped")
	}
	gateLessons.saving.Wait()
	gateLessons.load(file)
	gateLessons.mu.Lock()
	n := len(gateLessons.items)
	gateLessons.mu.Unlock()
	if n != 2 {
		t.Errorf("%d lessons after a restart, want 2", n)
	}
}
