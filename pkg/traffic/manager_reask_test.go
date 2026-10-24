package traffic

import (
	"fmt"
	"testing"
	"time"
)

// The schedule posted again with the same airports must not ask the Source
// again: a Source keeping call signs unique (the World's) gives other
// flights for the same hours, on top of those there (live, LKPR: 580
// arrivals in two hours, 45 at one STA).
func TestManagerSetAirportsAgainKeepsTimetable(t *testing.T) {
	n := 0
	asked := map[string]int{}
	m := NewTrafficManager(&fakeSpawner{}, ManagerOptions{Source: func(from, to time.Time, airports []string) []Flight {
		var out []Flight
		for _, a := range airports {
			asked[a]++
			for i := 0; i < 10; i++ {
				n++
				out = append(out, Flight{Callsign: fmt.Sprintf("TST%d", n), Type: "A320", Origin: "EGLL", Destination: a,
					STD: from.Add(-2 * time.Hour), STA: from.Add(time.Duration(i) * 5 * time.Minute)})
			}
		}
		return out
	}}, "LKPR")
	m.SetEnabled(false)
	m.Tick(t0)
	before := len(m.Flights())
	if before == 0 {
		t.Fatal("no flights from the source")
	}
	for i := 0; i < 5; i++ {
		m.SetAirports("LKPR")
		m.Tick(t0)
	}
	if got := len(m.Flights()); got != before {
		t.Fatalf("%d flights after posting the same airports five times, want %d", got, before)
	}
	m.SetAirports("LKPR", "EDDM")
	m.Tick(t0)
	if asked["LKPR"] != asked["EDDM"] {
		t.Fatalf("asked LKPR %d times and the new EDDM %d: only the new airport is asked", asked["LKPR"], asked["EDDM"])
	}
}
