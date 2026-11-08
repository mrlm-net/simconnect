package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/jetway"
)

// The names (TITLE, ATC ID) are read once per aircraft, not every second:
// ten seconds of the same aircraft ask for them once, the first second's
// scan handed on with them.
func TestTrafficNamesOncePerAircraft(t *testing.T) {
	n := newTrafficNames()
	scan := func() []Traffic { return []Traffic{{ObjectID: 1, State: "taxi"}, {ObjectID: 2}} }
	asks, fed := 0, 0
	for sec := 0; sec < 10; sec++ {
		out, ask := n.complete(scan())
		if ask {
			asks++
			n.name(1, "FSLTL A320 Air France SL", "CSA1")
			n.name(2, "FSLTL B738 Ryanair", "RYR2")
			out, _ = n.namesDone()
		}
		if len(out) != 2 || out[0].Title == "" || out[1].Tail != "RYR2" || out[0].State != "taxi" {
			t.Fatalf("second %d: fed %+v", sec, out)
		}
		fed++
	}
	if asks != 1 || fed != 10 {
		t.Errorf("names asked %d times in 10 s, %d scans fed; want once, 10", asks, fed)
	}
	// A new aircraft: asked again, the scan waits for it.
	out, ask := n.complete(append(scan(), Traffic{ObjectID: 3}))
	if !ask || out != nil {
		t.Fatalf("new aircraft: ask %v, fed %v", ask, out)
	}
	n.name(3, "FSLTL A20N Wizz", "WZZ3")
	if out, ok := n.namesDone(); !ok || len(out) != 3 {
		t.Fatalf("after its names: %v %+v", ok, out)
	}
}

// Jetway helpers are left out once named; names lost: the next second goes
// on without the unnamed ones and asks again.
func TestTrafficNamesHelpersAndLost(t *testing.T) {
	n := newTrafficNames()
	if _, ask := n.complete([]Traffic{{ObjectID: 1}, {ObjectID: 9}}); !ask {
		t.Fatal("not asked")
	}
	n.name(1, "FSLTL A320 Air France SL", "CSA1")
	n.name(9, jetway.HelperTitle("A320", false), "JW1")
	if out, _ := n.namesDone(); len(out) != 1 || out[0].ObjectID != 1 {
		t.Fatalf("helper not left out: %+v", out)
	}
	if _, ask := n.complete([]Traffic{{ObjectID: 1}, {ObjectID: 5}}); !ask {
		t.Fatal("new aircraft not asked")
	}
	out, ask := n.complete([]Traffic{{ObjectID: 1}, {ObjectID: 5}}) // names never came
	if !ask || len(out) != 1 || out[0].ObjectID != 1 {
		t.Fatalf("names lost: ask %v, fed %+v; want the named one and asked again", ask, out)
	}
	n.forget(1)
	if _, ask := n.complete([]Traffic{{ObjectID: 1}}); !ask {
		t.Error("forgotten aircraft not asked again")
	}
}
