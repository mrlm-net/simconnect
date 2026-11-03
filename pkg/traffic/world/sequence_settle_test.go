package world

import (
	"testing"
	"time"
)

// A new place in the sequence is said once it has held numberSettle; swings
// in between are not said (live, LKPR: DFVKB told 2, 3, 1, 3 within a minute
// around a go-around).
func TestSequenceNumberSettles(t *testing.T) {
	q := &sequences{seqSaid: map[string]seqSaid{}}
	t0 := time.Date(2026, 11, 3, 13, 20, 0, 0, time.UTC)
	at := func(s int) time.Time { return t0.Add(time.Duration(s) * time.Second) }

	if say, n := q.sequenceCall(at(0), "DFVKB", 2, 0); !say || n != 2 {
		t.Fatalf("first number: say %v n %d, want 2", say, n)
	}
	for _, c := range []struct{ s, number int }{{10, 3}, {20, 1}, {30, 3}, {40, 2}} {
		if say, n := q.sequenceCall(at(c.s), "DFVKB", c.number, 0); say {
			t.Errorf("+%ds number %d said (n %d) before it held", c.s, c.number, n)
		}
	}
	// Back at 2: nothing new. Then 3 for good.
	q.sequenceCall(at(50), "DFVKB", 3, 0)
	if say, _ := q.sequenceCall(at(80), "DFVKB", 3, 0); say {
		t.Error("3 said after 30 s")
	}
	if say, n := q.sequenceCall(at(96), "DFVKB", 3, 0); !say || n != 3 {
		t.Errorf("3 held 46 s: say %v n %d, want 3", say, n)
	}
	// A new speed during a swing goes out without the number.
	q.sequenceCall(at(100), "DFVKB", 4, 0)
	if say, n := q.sequenceCall(at(110), "DFVKB", 4, 180); !say || n != 0 {
		t.Errorf("new speed during a swing: say %v n %d, want speed only", say, n)
	}
}

// A number found stale when the call goes on air is forgotten, so the real
// place is said next.
func TestSequenceUnsayNumber(t *testing.T) {
	q := &sequences{seqSaid: map[string]seqSaid{}}
	t0 := time.Date(2026, 11, 3, 13, 20, 0, 0, time.UTC)
	q.sequenceCall(t0, "DFVKB", 1, 0)
	q.unsayNumber("DFVKB", 1)
	if say, n := q.sequenceCall(t0.Add(time.Second), "DFVKB", 3, 0); !say || n != 3 {
		t.Errorf("after a stale call: say %v n %d, want 3 at once", say, n)
	}
}
