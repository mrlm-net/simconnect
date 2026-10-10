package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestOffersIntersection: a queue of IntersectionQueue for the full length
// gives a light or medium departure an intersection, never a heavy.
func TestOffersIntersection(t *testing.T) {
	for _, c := range []struct {
		model string
		queue int
		want  bool
	}{
		{"A320", 1, false}, {"A320", 2, true}, {"C172", 3, true}, {"B77W", 5, false}, {"A388", 5, false},
	} {
		if got := offersIntersection(c.model, c.queue); got != c.want {
			t.Errorf("%s with %d queuing: %v, want %v", c.model, c.queue, got, c.want)
		}
	}
}

// TestFullLengthQueue: departures for the runway's full length on the
// ground count; one from an intersection, airborne, for another runway or
// the asking one do not.
func TestFullLengthQueue(t *testing.T) {
	cc := &controlCenter{items: map[int]*controlled{}}
	add := func(id int, v ControlView) *controlled {
		it := &controlled{dep: &remoteDep{}}
		it.view = v
		cc.items[id] = it
		return it
	}
	me := add(1, ControlView{ICAO: "LKPR", Runway: "24", OnGround: true, State: "taxiing"})
	add(2, ControlView{ICAO: "LKPR", Runway: "24", OnGround: true, State: "holding short"})
	add(3, ControlView{ICAO: "LKPR", Runway: "24", OnGround: true, State: "taxiing"})
	add(4, ControlView{ICAO: "LKPR", Runway: "24", OnGround: true, State: "holding short", Entry: "B"})
	add(5, ControlView{ICAO: "LKPR", Runway: "24", OnGround: false, State: "climbing"})
	add(6, ControlView{ICAO: "LKPR", Runway: "30", OnGround: true, State: "taxiing"})
	if n := cc.fullLengthQueue("lkpr", "24", me); n != 2 {
		t.Errorf("queue %d, want 2", n)
	}
}

// TestNearestEntryLKPR: LKPR 24 has a named intersection to offer.
func TestNearestEntryLKPR(t *testing.T) {
	l := lkprLayout(t)
	g, err := airport.BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	_, end, ok := l.RunwayEnd("24")
	if !ok {
		t.Fatal("no 24")
	}
	if e := nearestEntry(g, "24", end.Threshold); e == "" {
		t.Error("no intersection of 24")
	} else {
		t.Logf("24: intersection %s nearest its threshold", e)
	}
}
