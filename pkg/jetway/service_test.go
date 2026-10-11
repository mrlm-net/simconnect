package jetway

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/types"
)

type fakeSim struct {
	created  []struct{ title string; req uint32 }
	removed  []uint32
	released []uint32
	plans    []uint32
	asked    int
}

func (f *fakeSim) AICreateParkedATCAircraft(title, tail, icao string, req uint32) error {
	f.created = append(f.created, struct{ title string; req uint32 }{title, req})
	return nil
}
func (f *fakeSim) AISetAircraftFlightPlan(obj uint32, path string, req uint32) error {
	f.plans = append(f.plans, obj)
	return nil
}
func (f *fakeSim) AIReleaseControl(obj, req uint32) error { f.released = append(f.released, obj); return nil }
func (f *fakeSim) AIRemoveObject(obj, req uint32) error    { f.removed = append(f.removed, obj); return nil }
func (f *fakeSim) RequestJetwayData(icao string, n uint32, idx *int32) error {
	f.asked++
	return nil
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
	// Undock: the helper goes; home, a placeholder again.
	h.Undock("EDDM", 9)
	if h.Status("EDDM", 9) != StatusUndocking || f.removed[len(f.removed)-1] != 201 {
		t.Fatalf("stand 9 %s, removed %v", h.Status("EDDM", 9), f.removed)
	}
	n := len(f.created)
	h.see(jw(9, 0, 0))
	if len(f.created) != n+1 {
		t.Fatal("no placeholder back once the jetway was home")
	}

	// A helper the sim puts on another gate: removed, the dock failed.
	h.Dock("EDDM", 7, "A320")
	h.assign(f, 301)
	h.see(jw(5, 1, 301))
	if h.Status("EDDM", 7) != StatusFailed || f.removed[len(f.removed)-1] != 301 {
		t.Fatalf("stand 7 %s, removed %v", h.Status("EDDM", 7), f.removed)
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
