package traffic

import (
	"slices"
	"testing"
)

// TestFleetAbandon: an aircraft created after its creation was abandoned is
// removed once; a creation that never answered does not take the aircraft of
// whoever gets its request ID next (#1396).
func TestFleetAbandon(t *testing.T) {
	c := &fakeClient{}
	f := NewFleet(c)
	const req = 77
	ask := func() {
		t.Helper()
		if err := f.RequestNonATC(NonATCOpts{Model: "A320", Tail: "ABC"}, req); err != nil {
			t.Fatal(err)
		}
	}
	ask()
	if _, ok, _ := f.HandleAbandoned(assignedMsg(req, 5)); ok {
		t.Fatal("a creation still wanted was taken as abandoned")
	}
	f.Abandon(req)
	p, ok, err := f.HandleAbandoned(assignedMsg(req, 5))
	if !ok || err != nil || p.Tail != "ABC" || !slices.Equal(c.removed, []uint32{5}) {
		t.Fatalf("late answer: %+v, %v, %v, removed %v", p, ok, err, c.removed)
	}
	if _, ok, _ := f.HandleAbandoned(assignedMsg(req, 6)); ok {
		t.Fatal("handled twice")
	}

	// Never answered, then the request ID is used again.
	ask()
	f.Abandon(req)
	ask()
	if _, ok, _ := f.HandleAbandoned(assignedMsg(req, 7)); ok || slices.Contains(c.removed, 7) {
		t.Fatalf("the new owner's aircraft was removed: %v", c.removed)
	}
	if a, ok := f.Acknowledge(req, 7); !ok || a.ObjectID != 7 {
		t.Fatalf("the new owner's answer: %+v, %v", a, ok)
	}
	// A request not pending: nothing to abandon.
	f.Abandon(999)
	if _, ok, _ := f.HandleAbandoned(assignedMsg(999, 8)); ok {
		t.Fatal("a request never made was taken as abandoned")
	}
}
