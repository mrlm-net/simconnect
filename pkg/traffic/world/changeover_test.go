package world

import (
	"testing"
	"time"
)

// TestRunwayChangeGapFixed: a runway change's deadline for the old
// direction's departures is set once, not slid on with every look (live,
// LKPR: ENT1744, HACEH and OKWOU went from 06 over ten minutes while the
// 24 arrivals came on); the first landing comes changeGapMargin after the
// last has gone.
func TestRunwayChangeGapFixed(t *testing.T) {
	q := &sequences{}
	t0 := time.Date(2026, 10, 19, 18, 20, 0, 0, time.UTC)
	k := "LKPR 24"
	first := q.runwayChangeGap(t0, k, 3)
	if want := t0.Add(3*changeDepEvery + changeGapMargin); !first.Equal(want) {
		t.Fatalf("first look: not before %s, want %s", first.Format("15:04:05"), want.Format("15:04:05"))
	}
	later := q.runwayChangeGap(t0.Add(4*time.Minute), k, 2)
	if !later.Equal(first) {
		t.Errorf("4 min on, two left: not before %s, want it kept at %s", later.Format("15:04:05"), first.Format("15:04:05"))
	}
	by, ok := q.changeDeadline(k)
	if !ok || !by.Equal(t0.Add(3*changeDepEvery)) {
		t.Errorf("deadline %s %v, want %s", by.Format("15:04:05"), ok, t0.Add(3*changeDepEvery).Format("15:04:05"))
	}
	gone := t0.Add(9 * time.Minute)
	q.runwayChangeGap(gone, k, 1) // the last one still climbing out
	if got := q.runwayChangeGap(gone.Add(time.Second), k, 0); !got.Equal(gone.Add(changeGapMargin)) {
		t.Errorf("all gone: not before %s, want %s", got.Format("15:04:05"), gone.Add(changeGapMargin).Format("15:04:05"))
	}
	// A look counting none for a moment (a state between two): the same
	// deadline, not a new one later (live, OKBWJ's delay up 40 s a look).
	if got := q.runwayChangeGap(gone.Add(2*time.Second), k, 1); !got.Equal(gone.Add(2*time.Second).Add(changeGapMargin)) && !got.Equal(first) {
		if by, _ := q.changeDeadline(k); !by.Equal(t0.Add(3 * changeDepEvery)) {
			t.Errorf("after a look counting none: deadline %s, want it kept", by.Format("15:04:05"))
		}
	}
	end := gone.Add(2*time.Second + changeGapMargin + time.Second)
	if got := q.runwayChangeGap(end, k, 0); !got.IsZero() {
		t.Errorf("all gone, margin passed: not before %s, want none", got.Format("15:04:05"))
	}
	if _, ok := q.changeDeadline(k); ok {
		t.Error("deadline kept after the changeover")
	}
}

// TestHeldSlots: the departures waiting count at their most for
// depSlotsHold, so the gaps they get do not come and go (live, TVS555's
// gap 6.4 and 10.5 NM by turns).
func TestHeldSlots(t *testing.T) {
	q := &sequences{}
	t0 := time.Date(2026, 10, 19, 20, 47, 0, 0, time.UTC)
	k := "LKPR 06"
	for i, c := range []struct {
		after time.Duration
		n     int
		want  int
	}{{0, 3, 3}, {10 * time.Second, 2, 3}, {40 * time.Second, 3, 3}, {90 * time.Second, 2, 3}, {40*time.Second + depSlotsHold, 2, 2}, {41*time.Second + depSlotsHold, 0, 2}} {
		if got := q.heldSlots(t0.Add(c.after), k, c.n); got != c.want {
			t.Errorf("look %d (%s, %d waiting): %d, want %d", i, c.after, c.n, got, c.want)
		}
	}
}
