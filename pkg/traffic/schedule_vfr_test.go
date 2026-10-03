//go:build windows
// +build windows

package traffic

import (
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestVFRFlights: light aircraft fly in to and out of LKPR by day only, as
// OK- registrations, as many as asked for; none in poor weather.
func TestVFRFlights(t *testing.T) {
	l := smallField(lkprGraph(t).Layout)
	opts := VFROptions{Focus: []string{"LKPR"}, Layouts: map[string]*airport.Layout{"LKPR": l}, PerHour: 2, Seed: 7}
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	fs := VFRFlights(opts, day, day.Add(24*time.Hour))
	if len(fs) < 30 || len(fs) > 60 {
		t.Fatalf("%d VFR flights in a day at 2 an hour each way by day, want about 45", len(fs))
	}
	deps := 0
	pos := airport.LatLon{Lat: l.Latitude, Lon: l.Longitude}
	for _, f := range fs {
		dep := f.Origin == "LKPR"
		if dep {
			deps++
		}
		if f.Rules != "VFR" || !dep && (f.Destination != "LKPR" || f.Origin != "") || dep && f.Destination != "" || !strings.HasPrefix(f.Callsign, "OK") || len(f.Callsign) != 5 {
			t.Errorf("flight %+v", f)
		}
		if when := f.STA; dep && !Daylight(pos, f.STD) || !dep && (!Daylight(pos, when.Add(-8*time.Minute)) || !Daylight(pos, when)) {
			t.Errorf("%s at %v: not by day", f.Callsign, f.STA)
		}
		if p := ProfileFor(f.Type); p.Category != CategoryPiston {
			t.Errorf("%s: type %s is no light aircraft", f.Callsign, f.Type)
		}
	}
	if deps == 0 || deps == len(fs) {
		t.Errorf("%d departures of %d flights", deps, len(fs))
	}
	t.Logf("%d flights, %d departures", len(fs), deps)
	opts.Visual = func(string) bool { return false }
	if n := len(VFRFlights(opts, day, day.Add(24*time.Hour))); n != 0 {
		t.Errorf("%d VFR flights in poor weather", n)
	}
}

// TestManagerVFRLead: a VFR arrival appears VFRLead (8 min) before its STA,
// near the airport, never en route.
func TestManagerVFRLead(t *testing.T) {
	sp := &fakeSpawner{}
	m := NewTrafficManager(sp, ManagerOptions{MinTurn: -1}, "LKPR")
	m.Add([]Flight{{Callsign: "OKABC", Type: "C172", Destination: "LKPR", STA: t0.Add(time.Hour), STD: t0.Add(52 * time.Minute), Rules: "VFR"}})
	m.Tick(t0.Add(51 * time.Minute))
	if len(sp.spawned) != 0 {
		t.Fatalf("spawned at 51 min, stage %q", sp.spawned[0].Stage)
	}
	m.Tick(t0.Add(52 * time.Minute))
	if len(sp.spawned) != 1 || sp.spawned[0].Stage != "" || sp.spawned[0].Kind != "arrival" {
		t.Fatalf("spawned %+v, want the VFR arrival near the airport", sp.spawned)
	}
}

// smallField is l as a small field: its gates left out (LargeAirport), for
// the light aircraft and their circuits at LKPR's position.
func smallField(l *airport.Layout) *airport.Layout {
	s := *l
	s.Parking = nil
	for _, p := range l.Parking {
		if !p.IsGate() {
			s.Parking = append(s.Parking, p)
		}
	}
	return &s
}

// TestVFRFlightsLargeAirport: at a large airport (LKPR) VFR flights are
// fewer, mid-size aircraft flying between fields, privately, with no
// training circuits (#619).
func TestVFRFlightsLargeAirport(t *testing.T) {
	l := lkprGraph(t).Layout
	if !LargeAirport(l) || LargeAirport(smallField(l)) {
		t.Fatal("LKPR should be large, without its gates small")
	}
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	large := VFRFlights(VFROptions{Focus: []string{"LKPR"}, Layouts: map[string]*airport.Layout{"LKPR": l}, PerHour: 2, Seed: 7}, day, day.Add(24*time.Hour))
	small := VFRFlights(VFROptions{Focus: []string{"LKPR"}, Layouts: map[string]*airport.Layout{"LKPR": smallField(l)}, PerHour: 2, Seed: 7}, day, day.Add(24*time.Hour))
	if len(large) == 0 || len(large) > len(small)*3/4 {
		t.Errorf("%d VFR flights at the large airport, %d at the small: want about half", len(large), len(small))
	}
	mid := map[string]bool{}
	for _, x := range VFRLargeTypes {
		mid[x.Type] = true
	}
	for _, f := range large {
		if !mid[f.Type] || f.TouchAndGos > 0 || f.StopAndGo || f.Operator != "private" {
			t.Errorf("flight %+v at a large airport", f)
		}
	}
}

// TestBusinessFlights: business jets and turboprops in to and out of a
// large airport, IFR, from and to other airports in their range, none at
// a small field (#619).
func TestBusinessFlights(t *testing.T) {
	l := lkprGraph(t).Layout
	cfg := DefaultScheduleConfig()
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	fs := BusinessFlights(cfg, BusinessOptions{Focus: []string{"LKPR"}, Layouts: map[string]*airport.Layout{"LKPR": l}, Seed: 3}, day, day.Add(24*time.Hour))
	if len(fs) < 15 || len(fs) > 80 {
		t.Fatalf("%d business flights in a day, want some 40", len(fs))
	}
	kinds := map[string]bool{}
	for _, x := range BusinessTypes {
		kinds[x.Type] = true
	}
	arr := 0
	for _, f := range fs {
		if f.Destination == "LKPR" {
			arr++
		} else if f.Origin != "LKPR" {
			t.Errorf("%+v neither to nor from LKPR", f)
		}
		if !kinds[f.Type] || f.Rules != "" || f.Operator != "business" || f.Origin == f.Destination || f.DistanceNM < businessMinNM || !f.STA.After(f.STD) {
			t.Errorf("flight %+v", f)
		}
		if p := ProfileFor(f.Type); p.Type != f.Type {
			t.Errorf("%s: no profile of its own (got %s)", f.Type, p.Type)
		}
	}
	if arr == 0 || arr == len(fs) {
		t.Errorf("%d arrivals of %d", arr, len(fs))
	}
	if n := len(BusinessFlights(cfg, BusinessOptions{Focus: []string{"LKPR"}, Layouts: map[string]*airport.Layout{"LKPR": smallField(l)}, Seed: 3}, day, day.Add(24*time.Hour))); n != 0 {
		t.Errorf("%d business flights at a small field", n)
	}
}
