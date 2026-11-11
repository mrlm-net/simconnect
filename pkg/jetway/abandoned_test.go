package jetway

import (
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// assigned is the sim's ASSIGNED_OBJECT_ID for request req: object obj.
func assigned(req, obj uint32) engine.Message {
	m := &types.SIMCONNECT_RECV_ASSIGNED_OBJECT_ID{DwRequestID: types.DWORD(req), DwObjectID: types.DWORD(obj)}
	m.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID)
	return engine.Message{SIMCONNECT_RECV: &m.SIMCONNECT_RECV}
}

// An object the sim creates after its request was dropped (the airport
// released, or a dock undone, before its ID came) is removed as it comes,
// not left on a gate until the connection closes.
func TestHelpersLateObjectRemoved(t *testing.T) {
	f := &fakeSim{}
	h := NewHelpers(f, HelperOptions{Types: []string{"A320"}, RetractPlan: func(string) (string, error) { return "out.pln", nil }})
	now := time.Now()
	if err := h.Prepare("EDDM"); err != nil {
		t.Fatal(err)
	}
	h.see(jw(7, 0, 0))
	h.Update(now)
	if len(f.created) != 1 {
		t.Fatalf("created %d", len(f.created))
	}
	req := f.created[0].req
	if err := h.Release("EDDM"); err != nil {
		t.Fatal(err)
	}
	if !h.Handle(assigned(req, 501)) {
		t.Error("late object not taken")
	}
	if !slices.Contains(f.removed, 501) {
		t.Errorf("late object not removed: %v", f.removed)
	}
	// Not ours: left alone.
	if h.Handle(assigned(req+999, 502)) || slices.Contains(f.removed, 502) {
		t.Error("another client's object taken")
	}
}

// A placeholder the sim never creates (refused, its answer lost) is
// dropped after CreateTimeout: the fill goes on and docks are not held
// "still filling" for good; created after all, it is removed.
func TestHelpersCreateTimeout(t *testing.T) {
	f := &fakeSim{}
	h := NewHelpers(f, HelperOptions{Types: []string{"A320"}, RetractPlan: func(string) (string, error) { return "out.pln", nil }})
	now := time.Now()
	if err := h.Prepare("EDDM"); err != nil {
		t.Fatal(err)
	}
	h.see(jw(7, 0, 0), jw(9, 0, 0))
	h.Update(now)
	if len(f.created) != 1 {
		t.Fatalf("created %d", len(f.created))
	}
	lost := f.created[0].req
	now = now.Add(CreateTimeout / 2)
	h.Update(now)
	if len(f.created) != 1 {
		t.Fatalf("a second placeholder before the first timed out: %d", len(f.created))
	}
	now = now.Add(CreateTimeout)
	h.Update(now)
	h.Update(now)
	if len(f.created) < 2 {
		t.Fatalf("fill stuck on a placeholder never created: %d", len(f.created))
	}
	h.mu.Lock()
	_, kept := h.byReq[lost]
	h.mu.Unlock()
	if kept {
		t.Error("the lost request still pending")
	}
	h.Handle(assigned(lost, 601))
	if !slices.Contains(f.removed, 601) {
		t.Errorf("lost placeholder created late not removed: %v", f.removed)
	}
}

// An empty gate crowded by a held one (SetCrowded) is not refilled while a
// gate is docked: its placeholder would go to the docked gate. A dock at
// another gate goes ahead instead of waiting for that refill (live, LKPR
// 2026-11-11: C23 empty between held C22 and C24, every refill on docked
// A1, every dock refused for an hour).
func TestHelpersCrowdedEmptyGateNotRefilled(t *testing.T) {
	f := &fakeSim{}
	h := NewHelpers(f, HelperOptions{Types: []string{"A320"}, RetractPlan: func(string) (string, error) { return "out.pln", nil }})
	now := time.Now()
	h.Update(now)
	h.see(jw(7, 0, 0), jw(9, 0, 0), jw(11, 0, 0), jw(13, 0, 0))
	h.SetCrowded("EDDM", map[int][]int{7: {9}, 9: {7}})
	h.mu.Lock()
	h.accepted["EDDM"] = map[int]bool{7: true, 9: true, 11: true, 13: true}
	h.gate("EDDM", 9).placeholder, h.gate("EDDM", 9).status = 109, StatusReserved
	h.gate("EDDM", 13).placeholder, h.gate("EDDM", 13).status = 113, StatusReserved
	h.gate("EDDM", 11).helper, h.gate("EDDM", 11).status = 511, StatusDocked
	h.gate("EDDM", 7).status = StatusFailed // emptied: its helper took another gate
	h.mu.Unlock()
	if err := h.Dock("EDDM", 13, "A320"); err != nil {
		t.Fatalf("dock at 13 with crowded 7 empty: %v", err)
	}
	if h.Filling("EDDM") {
		t.Error("a refill started for crowded 7")
	}
}
