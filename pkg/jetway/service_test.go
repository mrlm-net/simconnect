package jetway

import (
	"fmt"
	"math"
	"slices"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/dict"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type fakeSim struct {
	created []struct {
		title string
		req   uint32
	}
	removed  []uint32
	released []uint32
	plans    []uint32
	asked    int
}

func (f *fakeSim) AICreateParkedATCAircraft(title, tail, icao string, req uint32) error {
	f.created = append(f.created, struct {
		title string
		req   uint32
	}{title, req})
	return nil
}
func (f *fakeSim) AISetAircraftFlightPlan(obj uint32, path string, req uint32) error {
	f.plans = append(f.plans, obj)
	return nil
}
func (f *fakeSim) AIReleaseControl(obj, req uint32) error {
	f.released = append(f.released, obj)
	return nil
}
func (f *fakeSim) AIRemoveObject(obj, req uint32) error {
	f.removed = append(f.removed, obj)
	return nil
}
func (f *fakeSim) RequestJetwayData(icao string, n uint32, idx *int32) error {
	f.asked++
	return nil
}
func (f *fakeSim) AddToDataDefinition(uint32, string, string, types.SIMCONNECT_DATATYPE, float32, uint32) error {
	return nil
}
func (f *fakeSim) RequestDataOnSimObject(uint32, uint32, uint32, types.SIMCONNECT_PERIOD, types.SIMCONNECT_DATA_REQUEST_FLAG, uint32, uint32, uint32) error {
	return nil
}
func (f *fakeSim) SetDataOnSimObject(uint32, uint32, types.SIMCONNECT_DATA_SET_FLAG, uint32, uint32, unsafe.Pointer) error {
	return nil
}

// The offset round trip, and a type's door offset by title.
func TestDoorOffset(t *testing.T) {
	o := placePos{Lat: 50.1, Lon: 14.26, Hdg: 214}
	lat, lon := fromOffset(o, 2.5, -0.4)
	if a, r := offsetM(o, lat, lon); math.Abs(a-2.5) > 0.01 || math.Abs(r+0.4) > 0.01 {
		t.Errorf("round trip: %.3f ahead, %.3f right", a, r)
	}
	if err := dict.Use("jetway.doorOffsets", []byte(`[{"type":"B738","aheadM":1.5},{"type":"A321","aheadM":0}]`)); err != nil {
		t.Fatal(err)
	}
	defer dict.Reset("jetway.doorOffsets")
	if d, ok := DoorOffsetFor("FSLTL_FAIB_B738_TVS-Smartwings"); !ok || d.AheadM != 1.5 {
		t.Errorf("B738: %+v %v", d, ok)
	}
	if _, ok := DoorOffsetFor("FSLTL A20N WZZ"); ok {
		t.Error("A20N has none")
	}
}

func jw(stand int, status, attached uint32) types.SIMCONNECT_JETWAY_DATA {
	var j types.SIMCONNECT_JETWAY_DATA
	copy(j.AirportIcao[:], "EDDM")
	j.ParkingIndex, j.Status, j.AttachedObjectId = int32(stand), status, types.DWORD(attached)
	return j
}

// assign answers the last creation with object obj (the sim's
// ASSIGNED_OBJECT_ID).
func (h *Helpers) assign(f *fakeSim, obj uint32) {
	h.mu.Lock()
	defer h.mu.Unlock()
	p := h.byReq[f.created[len(f.created)-1].req]
	p.object = obj
	h.byObj[obj] = p
}

func (h *Helpers) see(js ...types.SIMCONNECT_JETWAY_DATA) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, j := range js {
		h.jetway(j)
	}
}

// Gates filled with placeholders one at a time (each retracted by its
// flight plan, released after), a helper docked on a gate, undocked
// before the push, the placeholder back; a helper the sim puts on another
// gate fails.
func TestHelpersFlow(t *testing.T) {
	f := &fakeSim{}
	h := NewHelpers(f, HelperOptions{Types: []string{"A320", "B738"}, RetractPlan: func(string) (string, error) { return "out.pln", nil }})
	now := time.Now()
	if err := h.Prepare("EDDM"); err != nil {
		t.Fatal(err)
	}
	h.see(jw(7, 0, 0), jw(9, 0, 0)) // two jetway gates
	h.Update(now)
	if len(f.created) != 1 {
		t.Fatalf("placeholders created %d, want 1 at a time", len(f.created))
	}
	h.assign(f, 101)
	h.see(jw(7, 1, 101)) // the jetway claims it
	if err := h.Dock("EDDM", 7, "A320"); err != ErrFilling {
		t.Fatalf("dock while filling: %v, want ErrFilling", err)
	}
	h.Update(now)
	if h.Status("EDDM", 7) != StatusReserved || len(f.plans) != 1 || len(f.created) != 2 {
		t.Fatalf("stand 7 %s, plans %d, created %d", h.Status("EDDM", 7), len(f.plans), len(f.created))
	}
	h.assign(f, 102)
	h.see(jw(9, 1, 102))
	now = now.Add(RetractWait + time.Second)
	h.Update(now)
	if len(f.released) != 2 {
		t.Fatalf("released %v", f.released)
	}
	h.Update(now) // a third finds no gate
	h.assign(f, 103)
	now = now.Add(ClaimTimeout + time.Second)
	h.Update(now)
	if len(f.removed) != 1 || f.removed[0] != 103 {
		t.Fatalf("the extra placeholder not removed: %v", f.removed)
	}

	// Dock at 9: its placeholder goes, the B738 helper comes and docks.
	if err := h.Dock("EDDM", 9, "FSLTL_FAIB_B738_RYR-Ryanair"); err != nil {
		t.Fatal(err)
	}
	nc := len(f.created)
	h.Update(now) // not yet: the gate freed first
	if len(f.created) != nc {
		t.Fatal("helper created with its placeholder just gone")
	}
	now = now.Add(HelperDelay)
	h.Update(now)
	if got := f.created[len(f.created)-1].title; got != "MyCrew Jetway Helper B738" || f.removed[len(f.removed)-1] != 102 {
		t.Fatalf("helper %q, removed %v", got, f.removed)
	}
	h.assign(f, 201)
	h.see(jw(9, 1, 201))
	if h.Status("EDDM", 9) != StatusDocking {
		t.Fatalf("stand 9 %s", h.Status("EDDM", 9))
	}
	h.see(jw(9, 7, 201))
	if h.Status("EDDM", 9) != StatusDocked {
		t.Fatalf("stand 9 %s, want docked", h.Status("EDDM", 9))
	}
	// Undock: the helper stays as the gate's placeholder, its jetway sent
	// home by the retract plan; no new placeholder, which the sim could put
	// on a docked gate.
	h.Undock("EDDM", 9)
	if h.Status("EDDM", 9) != StatusUndocking || slices.Contains(f.removed, 201) || f.plans[len(f.plans)-1] != 201 {
		t.Fatalf("stand 9 %s, removed %v, plans %v", h.Status("EDDM", 9), f.removed, f.plans)
	}
	n := len(f.created)
	// The list says home early: still undocking until RetractWait.
	now = now.Add(16 * time.Second)
	h.Update(now)
	h.see(jw(9, 0, 0))
	if h.Status("EDDM", 9) != StatusUndocking || len(f.created) != n {
		t.Fatalf("stand 9 %s after 16 s, want undocking", h.Status("EDDM", 9))
	}
	now = now.Add(RetractWait)
	h.Update(now)
	h.see(jw(9, 0, 0))
	if len(f.created) != n || h.Status("EDDM", 9) != StatusReserved || !slices.Contains(f.released, 201) {
		t.Fatalf("home: created %d (want %d), stand 9 %s, released %v", len(f.created), n, h.Status("EDDM", 9), f.released)
	}

	// A helper the sim puts on another gate: the dock failed, the helper
	// sent off with the retract plan (its jetway home first) and removed
	// RetractWait later.
	if err := h.Dock("EDDM", 7, "A320"); err != nil {
		t.Fatalf("dock 7: %v", err)
	}
	now = now.Add(HelperDelay)
	h.Update(now)
	h.assign(f, 301)
	h.see(jw(5, 1, 301))
	if h.Status("EDDM", 7) != StatusFailed || slices.Contains(f.removed, 301) {
		t.Fatalf("stand 7 %s, removed %v: want failed, the helper retracting", h.Status("EDDM", 7), f.removed)
	}

	// Stand 7, a gate the sim gives placeholders, empty now: filled again
	// before any other dock (refill first).
	if err := h.Dock("EDDM", 9, "B738"); err != ErrFilling || !h.Filling("EDDM") {
		t.Fatalf("dock with stand 7 empty: %v, filling %v; want ErrFilling, a refill", err, h.Filling("EDDM"))
	}
	// A helper no jetway takes (put on a ramp): removed after ClaimTimeout.
	h.mu.Lock()
	delete(h.filling, "EDDM")
	h.gate("EDDM", 7).placeholder = 777 // the refill's placeholder on it
	h.mu.Unlock()
	h.Dock("EDDM", 9, "B738")
	now = now.Add(HelperDelay)
	h.Update(now)
	h.assign(f, 401)
	now = now.Add(ClaimTimeout + time.Second)
	h.Update(now)
	if h.Status("EDDM", 9) != StatusFailed {
		t.Fatalf("stand 9 %s, want failed", h.Status("EDDM", 9))
	}
	// Both helpers removed once their jetways had time to go home.
	now = now.Add(RetractWait)
	h.Update(now)
	if !slices.Contains(f.removed, 301) || !slices.Contains(f.removed, 401) {
		t.Fatalf("helpers not removed after their retract: %v", f.removed)
	}
}

// A jetway gate no placeholder holds (next to an occupied one): no helper,
// unless it is the only free jetway gate, where the simulator puts the
// next ParkedATC (live, LKPR B15: the helpers of other gates went there).
func TestHelpersNoPlaceholder(t *testing.T) {
	f := &fakeSim{}
	h := NewHelpers(f, HelperOptions{Types: []string{"B738"}, RetractPlan: func(string) (string, error) { return "out.pln", nil }})
	h.see(jw(11, 0, 0), jw(12, 0, 0))
	if err := h.Dock("EDDM", 11, "B738"); err != ErrNoGate || len(f.created) != 0 {
		t.Fatalf("dock with two free gates: %v, created %d; want ErrNoGate, none", err, len(f.created))
	}
	h.mu.Lock()
	h.gate("EDDM", 12).placeholder = 101
	h.mu.Unlock()
	if err := h.Dock("EDDM", 11, "B738"); err != nil {
		t.Fatalf("dock at the only free gate: %v", err)
	}
	h.Update(time.Now().Add(HelperDelay))
	if len(f.created) == 0 || f.created[0].title != "MyCrew Jetway Helper B738" {
		t.Fatalf("created %v, want the B738 helper", f.created)
	}
}

func TestHelpersUnavailable(t *testing.T) {
	h := NewHelpers(&fakeSim{}, HelperOptions{})
	if ok, why := h.Available(); ok || why == "" {
		t.Errorf("available %v %q without a package", ok, why)
	}
	if err := h.Prepare("EDDM"); err == nil {
		t.Error("prepared without a package")
	}
}

// One helper on its way at a time: the second dock waits for the first's
// gate (two at once took each other's).
func TestHelpersOneAtATime(t *testing.T) {
	f := &fakeSim{}
	h := NewHelpers(f, HelperOptions{Types: []string{"B738"}, RetractPlan: func(string) (string, error) { return "out.pln", nil }})
	h.see(jw(7, 0, 0), jw(9, 0, 0))
	h.mu.Lock()
	h.gate("EDDM", 7).placeholder, h.gate("EDDM", 9).placeholder = 101, 102
	h.mu.Unlock()
	if err := h.Dock("EDDM", 7, "B738"); err != nil {
		t.Fatal(err)
	}
	if err := h.Dock("EDDM", 9, "B738"); err != ErrFilling {
		t.Fatalf("second dock: %v, want ErrFilling", err)
	}
}

// A docked gate a new helper or placeholder takes is docked again.
func TestHelpersDockedGateTaken(t *testing.T) {
	f := &fakeSim{}
	h := NewHelpers(f, HelperOptions{Types: []string{"B738"}, RetractPlan: func(string) (string, error) { return "out.pln", nil }})
	h.see(jw(7, 0, 0), jw(9, 0, 0))
	h.mu.Lock()
	h.gate("EDDM", 7).helper, h.gate("EDDM", 7).status = 500, StatusDocked
	r := h.req()
	p := &pending{icao: "EDDM", stand: -1, object: 600}
	h.byReq[r], h.byObj[600] = p, p
	h.mu.Unlock()
	h.see(jw(7, 1, 600)) // a placeholder on the docked gate
	if h.Status("EDDM", 7) != StatusFailed || !slices.Contains(f.removed, 500) {
		t.Fatalf("stand 7 %s, removed %v", h.Status("EDDM", 7), f.removed)
	}
	if h.Dockable("EDDM")[0] != 7 {
		t.Error("the placeholder should hold stand 7 for the next dock")
	}
}

// A fill whose placeholders no jetway takes, try after try, ends with no
// gate, and is tried again RefillEvery later instead of never.
func TestHelpersRefillAfterEmpty(t *testing.T) {
	f := &fakeSim{}
	h := NewHelpers(f, HelperOptions{Types: []string{"A320"}, RetractPlan: func(string) (string, error) { return "out.pln", nil }})
	now := time.Now()
	if err := h.Prepare("EDDM"); err != nil {
		t.Fatal(err)
	}
	h.see(jw(7, 0, 0))
	obj := uint32(500)
	for i := 0; h.Filling("EDDM") && i < 2*EmptyFillTries+10; i++ { // each placeholder unclaimed
		n := len(f.created)
		h.Update(now)
		if len(f.created) > n {
			obj++
			h.assign(f, obj)
		}
		now = now.Add(ClaimTimeout + time.Second)
	}
	if h.Filling("EDDM") {
		t.Fatal("still filling after every try")
	}
	n := len(f.created)
	h.Update(now.Add(RefillEvery / 2))
	if len(f.created) != n {
		t.Fatal("tried again before RefillEvery")
	}
	h.Update(now.Add(RefillEvery + time.Second))
	if !h.Filling("EDDM") || len(f.created) != n+1 {
		t.Fatalf("filling %v, created %d (was %d): want a new fill", h.Filling("EDDM"), len(f.created), n)
	}
}

// A gate docked moments ago counts as free to the sim: no other helper is
// made until it has settled (DockSettle).
func TestHelpersDockSettle(t *testing.T) {
	f := &fakeSim{}
	h := NewHelpers(f, HelperOptions{Types: []string{"B738"}, RetractPlan: func(string) (string, error) { return "out.pln", nil }})
	now := time.Now()
	h.see(jw(7, 0, 0), jw(9, 0, 0))
	h.mu.Lock()
	h.now = now
	g7 := h.gate("EDDM", 7)
	g7.helper, g7.status, g7.since = 201, StatusDocked, now
	h.gate("EDDM", 9).placeholder = 102
	h.mu.Unlock()
	if err := h.Dock("EDDM", 9, "B738"); err != ErrFilling {
		t.Fatalf("dock right after 7 docked: %v, want ErrFilling", err)
	}
	h.Update(now.Add(DockSettle + time.Second))
	if err := h.Dock("EDDM", 9, "B738"); err != nil {
		t.Fatalf("dock once 7 settled: %v", err)
	}
}

// A dock whose jetway never docks keeps its helper on the gate as the
// placeholder: no refill, whose placeholders the sim put on docked gates.
func TestHelpersDockTimeoutKeepsHelper(t *testing.T) {
	f := &fakeSim{}
	h := NewHelpers(f, HelperOptions{Types: []string{"B738"}, RetractPlan: func(string) (string, error) { return "out.pln", nil }})
	now := time.Now()
	h.Update(now)
	h.see(jw(7, 0, 0), jw(9, 0, 0))
	h.mu.Lock()
	h.gate("EDDM", 7).helper, h.gate("EDDM", 7).status = 500, StatusDocked
	h.gate("EDDM", 9).placeholder, h.gate("EDDM", 9).status = 600, StatusReserved
	h.gate("EDDM", 7).since = now.Add(-time.Hour)
	h.mu.Unlock()
	if err := h.Dock("EDDM", 9, "B738"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(HelperDelay)
	h.Update(now)
	h.assign(f, 700)
	h.see(jw(9, 1, 700)) // on its gate, never docks
	h.Update(now)
	n := len(f.created)
	now = now.Add(DockTimeout + time.Second)
	h.Update(now)
	if h.Status("EDDM", 9) != StatusFailed || slices.Contains(f.removed, 700) || !slices.Contains(f.plans, 700) {
		t.Fatalf("stand 9 %s, removed %v, plans %v: want failed, the helper kept and retracting", h.Status("EDDM", 9), f.removed, f.plans)
	}
	if h.Filling("EDDM") || len(f.created) != n {
		t.Fatal("refilled after the timeout")
	}
	now = now.Add(RetractWait + time.Second)
	h.Update(now)
	if !slices.Contains(f.released, 700) {
		t.Fatalf("released %v, want the kept helper", f.released)
	}
}

// A gate that never takes a helper type (the sim puts it on another gate,
// taking that one's jetway) is docked next time with one it does take: the
// last docked there, else the placeholders' type; the host hears both and
// gives them back at the next start (live, LKPR 2026-10-20 to 22: stand 11
// refused BCS3 five times of five).
func TestHelpersGateRefusesType(t *testing.T) {
	f := &fakeSim{}
	var heard []string
	h := NewHelpers(f, HelperOptions{Types: []string{"A320", "BCS3", "B738"}, RetractPlan: func(string) (string, error) { return "out.pln", nil },
		OnLearn: func(icao string, stand int, helper string, docked bool) {
			heard = append(heard, fmt.Sprint(icao, stand, helper, docked))
		}})
	now := time.Now()
	h.see(jw(7, 0, 0), jw(9, 0, 0))
	h.mu.Lock()
	h.gate("EDDM", 7).placeholder, h.gate("EDDM", 9).placeholder = 101, 102
	h.mu.Unlock()
	dock := func(model string, obj uint32, at int) string {
		t.Helper()
		h.mu.Lock()
		delete(h.filling, "EDDM")
		g := h.gate("EDDM", 7)
		g.status, g.helper = StatusReserved, 0
		if g.placeholder == 0 {
			g.placeholder = 101
		}
		h.mu.Unlock()
		h.Update(now) // the clock
		if err := h.Dock("EDDM", 7, model); err != nil {
			t.Fatalf("dock %s: %v", model, err)
		}
		n := len(f.created)
		now = now.Add(HelperDelay)
		h.Update(now)
		h.mu.Lock()
		var p *pending
		var title string
		for _, c := range f.created[n:] { // the helper (a refill may come too)
			if q := h.byReq[c.req]; q != nil && q.helper {
				p, title = q, c.title
			}
		}
		if p == nil {
			h.mu.Unlock()
			t.Fatalf("dock %s: no helper created (%v)", model, f.created[n:])
		}
		p.object = obj
		h.byObj[obj] = p
		h.mu.Unlock()
		h.see(jw(at, 1, obj))
		if at == 7 {
			h.see(jw(7, 7, obj))
		}
		return title
	}
	if got := dock("FSLTL_SBAI_BCS3_CSA-Lines", 301, 9); got != HelperTitle("BCS3", false) {
		t.Fatalf("first helper %q", got)
	}
	// Refused: the placeholders' type instead.
	if got := dock("FSLTL_SBAI_BCS3_CSA-Lines", 302, 7); got != HelperTitle("A320", false) {
		t.Fatalf("after a refusal %q, want the placeholders' type", got)
	}
	// A B738 docked there: the gate's helper from then on for a refused type.
	h.mu.Lock()
	h.gate("EDDM", 7).status = StatusReserved
	h.mu.Unlock()
	if got := dock("FSLTL_FAIB_B738_RYR-Ryanair", 303, 7); got != HelperTitle("B738", false) {
		t.Fatalf("B738 at 7: %q", got)
	}
	if got := dock("FSLTL_SBAI_BCS3_CSA-Lines", 304, 7); got != HelperTitle("B738", false) {
		t.Fatalf("a refused type at 7: %q, want the B738 that docked", got)
	}
	want := []string{"EDDM7" + HelperTitle("BCS3", false) + "false", "EDDM7" + HelperTitle("A320", false) + "true", "EDDM7" + HelperTitle("B738", false) + "true"}
	if !slices.Equal(heard, want) {
		t.Errorf("heard %v, want %v", heard, want)
	}
	// Given back at the next start.
	h2 := NewHelpers(f, HelperOptions{Types: []string{"A320", "BCS3"}})
	h2.Learn("eddm", 7, HelperTitle("BCS3", false), false)
	h2.mu.Lock()
	got := h2.gateTakes("EDDM", 7, HelperTitle("BCS3", false))
	h2.mu.Unlock()
	if got != HelperTitle("A320", false) {
		t.Errorf("learned refusal not kept: %q", got)
	}
}

// A helper the sim puts on another, docked gate: that gate docked again,
// its own failed and counted a miss. Placeholders of a fill landing on
// docked gates FillStrays times stop the fill a while. A gate whose
// helpers go elsewhere more than they dock (GateUnfitMin) gets no jetway,
// tried again once each GateRetest.
func TestHelpersStraysOnDockedGates(t *testing.T) {
	f := &fakeSim{}
	var results []bool
	h := NewHelpers(f, HelperOptions{Types: []string{"B738"}, RetractPlan: func(string) (string, error) { return "out.pln", nil },
		OnGateResult: func(icao string, stand int, docked bool) { results = append(results, docked) }})
	now := time.Now()
	h.Update(now)
	h.see(jw(4, 0, 0), jw(5, 0, 0), jw(7, 0, 0), jw(9, 0, 0))
	h.mu.Lock()
	for _, s := range []int{4, 5, 7} {
		h.gate("EDDM", s).helper, h.gate("EDDM", s).status = uint32(500+s), StatusDocked
		h.byObj[uint32(500+s)] = &pending{icao: "EDDM", stand: s, helper: true, object: uint32(500 + s)}
	}
	h.gate("EDDM", 9).status = StatusDocking
	stray := &pending{icao: "EDDM", stand: 9, helper: true, object: 700}
	h.byReq[h.req()], h.byObj[700] = stray, stray
	h.mu.Unlock()
	h.see(jw(7, 1, 700)) // stand 9's helper on docked 7
	if !slices.Contains(f.removed, 507) || h.Status("EDDM", 7) != StatusFailed || h.Status("EDDM", 9) != StatusFailed || !slices.Equal(results, []bool{false}) {
		t.Fatalf("removed %v, stand 7 %s, stand 9 %s, results %v", f.removed, h.Status("EDDM", 7), h.Status("EDDM", 9), results)
	}
	// The fill: its placeholders land on docked 4, then 5.
	for i, s := range []int{4, 5} {
		obj := uint32(800 + i)
		p := &pending{icao: "EDDM", stand: -1, object: obj}
		h.mu.Lock()
		h.byReq[h.req()], h.byObj[obj] = p, p
		h.mu.Unlock()
		h.see(jw(s, 1, obj))
	}
	if h.Filling("EDDM") {
		t.Errorf("still filling after %d placeholders on docked gates", FillStrays)
	}
	// Stand 9: missed more than docked: no jetway until GateRetest.
	h2 := NewHelpers(&fakeSim{}, HelperOptions{Types: []string{"B738"}})
	h2.Update(now)
	h2.see(jw(9, 0, 0))
	h2.mu.Lock()
	h2.gate("EDDM", 9).placeholder = 909
	h2.mu.Unlock()
	h2.GateRecord("eddm", 9, 1, GateUnfitMin)
	if err := h2.Dock("EDDM", 9, "B738"); err != nil {
		t.Fatalf("first try of an unfit gate: %v, want tried once", err)
	}
	h2.mu.Lock()
	g := h2.gate("EDDM", 9)
	g.status, g.helper, g.placeholder = StatusReserved, 0, 910
	h2.mu.Unlock()
	if err := h2.Dock("EDDM", 9, "B738"); err != ErrNoGate {
		t.Errorf("unfit gate again within GateRetest: %v, want ErrNoGate", err)
	}
	h2.Update(now.Add(GateRetest + time.Minute))
	if err := h2.Dock("EDDM", 9, "B738"); err != nil {
		t.Errorf("unfit gate after GateRetest: %v, want tried again", err)
	}
}
