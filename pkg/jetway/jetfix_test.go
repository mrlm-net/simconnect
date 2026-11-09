package jetway

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

// fixHelpers is a backend with gates 4, 5, 7 and 9 at EDDM, a placeholder
// on each but those given in docked (helper 500+stand, docked an hour).
func fixHelpers(t *testing.T, docked ...int) (*fakeSim, *Helpers, *[]string, time.Time) {
	t.Helper()
	f := &fakeSim{}
	var logs []string
	h := NewHelpers(f, HelperOptions{Types: []string{"A320", "B738"}, RetractPlan: func(string) (string, error) { return "out.pln", nil },
		Log: func(format string, a ...any) { logs = append(logs, fmt.Sprintf(format, a...)) }})
	now := time.Now()
	h.Update(now)
	h.see(jw(4, 0, 0), jw(5, 0, 0), jw(7, 0, 0), jw(9, 0, 0))
	h.mu.Lock()
	for _, s := range []int{4, 5, 7, 9} {
		g := h.gate("EDDM", s)
		if slices.Contains(docked, s) {
			obj := uint32(500 + s)
			g.helper, g.status, g.since, g.model = obj, StatusDocked, now.Add(-time.Hour), "B738"
			h.byObj[obj] = &pending{icao: "EDDM", stand: s, helper: true, object: obj}
		} else {
			g.placeholder, g.status = uint32(100+s), StatusReserved
		}
		if h.accepted["EDDM"] == nil {
			h.accepted["EDDM"] = map[int]bool{}
		}
		h.accepted["EDDM"][s] = true
	}
	h.mu.Unlock()
	return f, h, &logs, now
}

// newHelper answers the last creation, a helper, with object obj.
func (h *Helpers) newObject(t *testing.T, f *fakeSim, obj uint32) {
	t.Helper()
	if len(f.created) == 0 {
		t.Fatal("nothing created")
	}
	h.assign(f, obj)
}

// G: undocking a gate still docking retires its helper (the retract plan,
// removed RetractWait later), never removes it with the jetway moving to
// it (live, LKPR 13, 15, 17 to 21: stuck half way).
func TestFixUndockWhileDockingRetires(t *testing.T) {
	f, h, _, now := fixHelpers(t, 4)
	if err := h.Dock("EDDM", 9, "B738"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(HelperDelay)
	h.Update(now)
	h.newObject(t, f, 909)
	h.see(jw(9, 2, 909)) // its jetway on the way
	if h.Status("EDDM", 9) != StatusDocking {
		t.Fatalf("stand 9 %s", h.Status("EDDM", 9))
	}
	h.Undock("EDDM", 9)
	if slices.Contains(f.removed, 909) || !slices.Contains(f.plans, 909) {
		t.Fatalf("removed %v, plans %v: want the helper retired, not removed", f.removed, f.plans)
	}
	h.Update(now.Add(RetractWait + time.Second))
	if !slices.Contains(f.removed, 909) {
		t.Fatalf("not removed after its retract: %v", f.removed)
	}
}

// A/B: a helper the sim puts on a docked gate: the robbed gate is docked
// again first, its own helper made once the stray is gone, and no refill
// of the airport (live, LKPR 2026-11-08: twelve jetways pulled off parked
// aircraft by refills after steals).
func TestFixStealRedocksRobbedGate(t *testing.T) {
	f, h, _, now := fixHelpers(t, 4)
	if err := h.Dock("EDDM", 9, "A320"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(HelperDelay)
	h.Update(now)
	h.newObject(t, f, 909)
	h.see(jw(4, 1, 909)) // stand 9's helper on docked 4
	if h.Filling("EDDM") {
		t.Fatal("the whole airport refilled after a steal")
	}
	if h.Status("EDDM", 4) != StatusDocking || h.Status("EDDM", 9) != StatusFailed {
		t.Fatalf("stand 4 %s (want docking again), stand 9 %s (want failed)", h.Status("EDDM", 4), h.Status("EDDM", 9))
	}
	// Other docks wait for the robbed gate.
	if err := h.Dock("EDDM", 7, "A320"); err != ErrFilling {
		t.Fatalf("dock 7 with 4 docking again: %v", err)
	}
	n := len(f.created)
	for i := 0; i < int(RetractWait/time.Second); i++ {
		now = now.Add(time.Second)
		h.Update(now)
	}
	if len(f.created) != n {
		t.Fatalf("created %v before the stray left stand 4", f.created[n:])
	}
	now = now.Add(time.Second)
	h.Update(now)
	if !slices.Contains(f.removed, 909) {
		t.Fatalf("stray not removed: %v", f.removed)
	}
	now = now.Add(HelperDelay)
	h.Update(now)
	if len(f.created) != n+1 || f.created[n].title != HelperTitle("B738", false) {
		t.Fatalf("created %v, want stand 4's B738 helper alone", f.created[n:])
	}
	h.newObject(t, f, 404)
	h.see(jw(4, 7, 404))
	if h.Status("EDDM", 4) != StatusDocked {
		t.Fatalf("stand 4 %s, want docked again", h.Status("EDDM", 4))
	}
}

// A/B: a refill's placeholder on a docked gate stops the fill at once and
// docks that gate again from it, before anything else is made.
func TestFixPlaceholderStealStopsFill(t *testing.T) {
	f, h, _, now := fixHelpers(t, 4, 5)
	h.mu.Lock()
	h.filling["EDDM"] = true
	h.mu.Unlock()
	h.Update(now)
	h.newObject(t, f, 800)
	h.see(jw(4, 1, 800)) // on docked 4
	if h.Filling("EDDM") {
		t.Fatal("the fill goes on after a placeholder took a docked gate")
	}
	if h.Status("EDDM", 4) != StatusDocking {
		t.Fatalf("stand 4 %s, want docking again", h.Status("EDDM", 4))
	}
	n := len(f.created)
	now = now.Add(RetractWait + time.Second)
	h.Update(now)
	if !slices.Contains(f.removed, 800) {
		t.Fatalf("its placeholder not taken away for the dock: %v", f.removed)
	}
	now = now.Add(HelperDelay)
	h.Update(now)
	if len(f.created) != n+1 || f.created[n].title != HelperTitle("B738", false) {
		t.Fatalf("created %v, want stand 4's helper alone", f.created[n:])
	}
}

// A refill after a failed dock: not at once (each new ParkedATC another
// chance at a docked gate), and only until the empty gates are held again,
// no unclaimed placeholder after them.
func TestFixRefillOnlyEmptyGates(t *testing.T) {
	f, h, _, now := fixHelpers(t, 4)
	if err := h.Dock("EDDM", 9, "A320"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(HelperDelay)
	h.Update(now)
	h.newObject(t, f, 909)
	now = now.Add(ClaimTimeout + time.Second) // on a ramp: no jetway took it
	h.Update(now)
	if h.Status("EDDM", 9) != StatusFailed || h.Filling("EDDM") {
		t.Fatalf("stand 9 %s, filling %v: want failed, no refill yet", h.Status("EDDM", 9), h.Filling("EDDM"))
	}
	// The next dock elsewhere refills first: one placeholder for 9.
	if err := h.Dock("EDDM", 7, "A320"); err != ErrFilling || !h.Filling("EDDM") {
		t.Fatalf("dock 7: %v, filling %v", err, h.Filling("EDDM"))
	}
	n := len(f.created)
	h.Update(now)
	if len(f.created) != n+1 {
		t.Fatalf("created %d, want one placeholder", len(f.created)-n)
	}
	h.newObject(t, f, 199)
	h.see(jw(9, 1, 199))
	now = now.Add(10 * time.Second)
	h.Update(now)
	if h.Filling("EDDM") || len(f.created) != n+1 {
		t.Fatalf("filling %v, created %d: want done once 9 is held", h.Filling("EDDM"), len(f.created)-n)
	}
}

// Load: the jetway list read every pollEvery while a dock is coming, every
// pollSlow while only undocking (each read stalls all traffic).
func TestFixPollSlowWhileUndocking(t *testing.T) {
	f, h, logs, now := fixHelpers(t, 4)
	h.Undock("EDDM", 4)
	f.asked = 0
	for i := 0; i < 20; i++ {
		now = now.Add(time.Second)
		h.Update(now)
	}
	if f.asked > 2 {
		t.Fatalf("list read %d times in 20 s while only undocking", f.asked)
	}
	// Home: logged with the time it took.
	now = now.Add(RetractWait)
	h.Update(now)
	h.see(jw(4, 0, 0))
	if h.Status("EDDM", 4) != StatusReserved {
		t.Fatalf("stand 4 %s", h.Status("EDDM", 4))
	}
	home := false
	for _, l := range *logs {
		home = home || strings.Contains(l, "stand 4: home")
	}
	if !home {
		t.Errorf("no home log: %q", *logs)
	}
	// A dock coming: every pollEvery.
	if err := h.Dock("EDDM", 9, "A320"); err != nil {
		t.Fatal(err)
	}
	f.asked = 0
	for i := 0; i < 20; i++ {
		now = now.Add(time.Second)
		h.Update(now)
	}
	if f.asked < 8 {
		t.Fatalf("list read %d times in 20 s with a dock coming", f.asked)
	}
}

// F: docks asked together are made together, and helpers of one title
// landing on each other's gates dock there (the gates swap); a helper of
// another title on a batch gate still fails (live, BATC at LKPR: nine
// jetways together; ours one at a time, two minutes apart).
func TestFixBatchSwap(t *testing.T) {
	f, h, _, now := fixHelpers(t)
	for _, s := range []int{4, 5, 7} {
		if err := h.Dock("EDDM", s, "B738"); err != nil {
			t.Fatalf("dock %d: %v", s, err)
		}
	}
	now = now.Add(HelperDelay)
	h.Update(now)
	if len(f.created) != 3 {
		t.Fatalf("created %d helpers, want three together", len(f.created))
	}
	// The sim's order: 4's helper on 5, 5's on 7, 7's on 4.
	objs := map[int]uint32{}
	for i, s := range []int{4, 5, 7} {
		obj := uint32(600 + s)
		h.mu.Lock()
		p := h.byReq[f.created[i].req]
		p.object = obj
		h.byObj[obj] = p
		h.mu.Unlock()
		objs[s] = obj
	}
	h.see(jw(5, 1, objs[4]), jw(7, 1, objs[5]), jw(4, 1, objs[7]))
	h.see(jw(5, 7, objs[4]), jw(7, 7, objs[5]), jw(4, 7, objs[7]))
	for _, s := range []int{4, 5, 7} {
		if st := h.Status("EDDM", s); st != StatusDocked {
			t.Errorf("stand %d %s, want docked", s, st)
		}
	}
	if len(f.removed) != 3 { // the three placeholders, no helper
		t.Errorf("removed %v", f.removed)
	}
}
