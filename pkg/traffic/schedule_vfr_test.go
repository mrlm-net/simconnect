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
	l := lkprGraph(t).Layout
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
