package world

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestStartStandKeepsStandsForDue: a start stand or night stop is placed
// only where the departures due within DepartureLead plus DepartureLate
// still find a stand each (#1677).
func TestStartStandKeepsStandsForDue(t *testing.T) {
	l := &airport.Layout{ICAO: "ZZZZ", Latitude: 50, Longitude: 14}
	for i := range 3 {
		p := airport.Parking{Index: i, Type: 9, Radius: 25, Position: airport.LatLon{Lat: 50, Lon: 14 + float64(i)*0.01}}
		l.Parking = append(l.Parking, p)
	}
	alloc := traffic.NewStandAllocator(nil, &airport.Graph{Layout: l})
	if err := alloc.Occupy(0, "START", 0); err != nil {
		t.Fatal(err)
	}
	a320 := standNeed{half: traffic.DefaultHalfSpanMeters}
	if !keepsStandsFor(alloc, l, []standNeed{a320, a320}) {
		t.Error("two due, two stands free: not kept")
	}
	if keepsStandsFor(alloc, l, []standNeed{a320, a320, a320}) {
		t.Error("three due, two stands free: kept")
	}
	if keepsStandsFor(alloc, l, []standNeed{{half: 40}}) {
		t.Error("a 80 m span due, no stand that large: kept")
	}

	now := time.Date(2026, 11, 22, 14, 3, 0, 0, time.UTC)
	o := traffic.ManagerOptions{DepartureLead: 10 * time.Minute, DepartureLate: 15 * time.Minute}
	dep := func(cs string, std time.Duration, mod func(*traffic.ManagedFlight)) traffic.ManagedFlight {
		f := traffic.ManagedFlight{Kind: "departure", Airport: "ZZZZ", Status: traffic.FlightScheduled,
			Flight: traffic.Flight{Callsign: cs, Type: "A320", STD: now.Add(std)}}
		mod(&f)
		return f
	}
	none := func(*traffic.ManagedFlight) {}
	needs := dueStandNeeds([]traffic.ManagedFlight{
		dep("JBU1065", 2*time.Minute, none),
		dep("DAL925", 24*time.Minute, none),
		dep("JBU1701", 147*time.Minute, none),
		dep("SELF", 2*time.Minute, none),
		dep("TURN", 2*time.Minute, func(f *traffic.ManagedFlight) { f.TurnFrom = "ARR" }),
		dep("NIGHT", 2*time.Minute, func(f *traffic.ManagedFlight) { f.NightStop = true }),
		dep("UP", 2*time.Minute, func(f *traffic.ManagedFlight) { f.Status = traffic.FlightBoarding }),
		dep("ELSE", 2*time.Minute, func(f *traffic.ManagedFlight) { f.Airport = "YYYY" }),
	}, "ZZZZ", "SELF", now, o)
	if len(needs) != 2 {
		t.Errorf("%d departures need a stand, want 2 (JBU1065, DAL925)", len(needs))
	}
}
