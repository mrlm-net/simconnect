package traffic

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/dict"
)

var flowsLKPR = airport.LatLon{Lat: 50.1008, Lon: 14.26}

// TestOverflightFlowsStreams: over 960 drawn hours (40 days) an area's streams
// keep their shares and hourly profiles (#1483), deterministic for a seed.
func TestOverflightFlowsStreams(t *testing.T) {
	t.Cleanup(func() { _ = dict.Reset("traffic.overflightFlows") })
	morning := make([]float64, 24)
	for h := 0; h < 12; h++ {
		morning[h] = 1
	}
	// Two streams with no region in common: Scandinavia to the
	// Mediterranean all day (weight 3), the British Isles to the Gulf in the
	// UTC morning only (weight 1).
	table := `[{"area": "TEST", "north": 50.5, "south": 49.5, "west": 13.5, "east": 15, "peakPerHour": 20, "streams": [
		{"name": "south", "from": ["EK", "EN", "ES"], "to": ["LG", "LI", "LD"], "weight": 3},
		{"name": "gulf", "from": ["EG", "EI"], "to": ["OM", "OT"], "weight": 1, "hoursUTC": [` + strings.TrimSuffix(strings.Repeat("1,", 12)+strings.Repeat("0,", 12), ",") + `]}]}]`
	if err := dict.Use("traffic.overflightFlows", []byte(table)); err != nil {
		t.Fatal(err)
	}
	area, ok := OverflightAreaAt(flowsLKPR)
	if !ok || area.Area != "TEST" {
		t.Fatalf("area at LKPR: %v %v, want TEST", area.Area, ok)
	}
	cfg := DefaultScheduleConfig()
	o := OverflightOptions{Centre: flowsLKPR, RadiusNM: 100, Seed: 7, Exclude: []string{"LKPR"}, Flows: true}
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	const hours = 960 // 40 days
	to := from.Add(hours * time.Hour)
	var flights []Flight
	for h := from; h.Before(to); h = h.Add(time.Hour) {
		flights = append(flights, Overflights(cfg, o, h, h.Add(time.Hour))...) // an hour at a time, as the manager asks
	}
	var morningN, afternoonN, south, gulf, gulfPM int
	var want float64
	for h := from; h.Before(to); h = h.Add(time.Hour) {
		want += OverflightRate(cfg, o, h)
	}
	for _, f := range flights {
		am := f.Enter.UTC().Hour() < 12
		if am {
			morningN++
		} else {
			afternoonN++
		}
		switch {
		case inRegion(f.Origin, []string{"EK", "EN", "ES"}):
			south++
		case inRegion(f.Origin, []string{"EG", "EI"}):
			gulf++
			if !am {
				gulfPM++
			}
		default:
			t.Errorf("%s %s → %s in no stream", f.Callsign, f.Origin, f.Destination)
		}
		dist := calc.HaversineNM(endPos(cfg, f.Origin).Lat, endPos(cfg, f.Origin).Lon, endPos(cfg, f.Destination).Lat, endPos(cfg, f.Destination).Lon)
		if _, _, ok := crossing(endPos(cfg, f.Origin), endPos(cfg, f.Destination), dist, flowsLKPR, 100); !ok || dist < 200 {
			t.Errorf("%s %s → %s does not cross the area (%.0f NM)", f.Callsign, f.Origin, f.Destination, dist)
		}
	}
	n := float64(len(flights))
	t.Logf("%d flights in %d h (mean %.0f): south %d, gulf %d (afternoon %d); morning %d, afternoon %d", len(flights), hours, want, south, gulf, gulfPM, morningN, afternoonN)
	// The volume: 20 × (3 + 1)/4 in the morning, 20 × 3/4 after.
	if math.Abs(want-hours/2*20-hours/2*15) > 1 {
		t.Errorf("mean %.0f, want %d", want, hours/2*20+hours/2*15)
	}
	if n < 0.9*want || n > 1.02*want {
		t.Errorf("%d drawn, want about %.0f", len(flights), want)
	}
	// The shares: morning 3:1, afternoon south only.
	if gulfPM > 0 {
		t.Errorf("%d of the morning-only stream in the afternoon", gulfPM)
	}
	if share, wantShare := float64(gulf)/n, 0.25*0.5*20/(want/hours); math.Abs(share-wantShare) > 0.03 {
		t.Errorf("the gulf stream's share %.3f, want %.3f", share, wantShare)
	}
	if r := float64(morningN) / float64(afternoonN); math.Abs(r-20.0/15) > 0.1 {
		t.Errorf("morning/afternoon %.2f, want %.2f", r, 20.0/15)
	}
	// Deterministic for a seed.
	again := drawHours(cfg, o, from, 48)
	first := drawHours(cfg, o, from, 48)
	if len(again) != len(first) {
		t.Fatalf("seeded twice: %d and %d flights", len(again), len(first))
	}
	for i := range again {
		if a, b := again[i], first[i]; a.Callsign != b.Callsign || a.Origin != b.Origin || a.Destination != b.Destination || !a.Enter.Equal(b.Enter) {
			t.Fatalf("seeded twice: %v and %v", again[i], first[i])
		}
	}
}

// endPos is airport icao's position in cfg.
func endPos(cfg ScheduleConfig, icao string) airport.LatLon {
	for _, a := range cfg.Airports {
		if a.ICAO == icao {
			return a.Position
		}
	}
	return airport.LatLon{}
}

// TestOverflightFlowsFallback: an area with no table entry still gets
// overflights crossing it between pairs at least 200 NM apart, never from
// or to a focus airport, its volume by the airports around; a remote
// place gets far fewer than central Europe.
func TestOverflightFlowsFallback(t *testing.T) {
	cfg := DefaultScheduleConfig()
	kosice := airport.LatLon{Lat: 48.6631, Lon: 21.2411}
	if _, ok := OverflightAreaAt(kosice); ok {
		t.Fatal("LZKZ has a table entry")
	}
	o := OverflightOptions{Centre: kosice, RadiusNM: 100, Seed: 3, Exclude: []string{"LZKZ"}, Flows: true}
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	flights := drawHours(cfg, o, from, 48)
	if len(flights) == 0 {
		t.Fatal("no overflights from the fallback")
	}
	for _, f := range flights {
		a, b := endPos(cfg, f.Origin), endPos(cfg, f.Destination)
		dist := calc.HaversineNM(a.Lat, a.Lon, b.Lat, b.Lon)
		if _, _, ok := crossing(a, b, dist, kosice, 100); !ok || dist < 200 || f.Origin == "LZKZ" || f.Destination == "LZKZ" {
			t.Errorf("%s %s → %s: crosses %v, %.0f NM", f.Callsign, f.Origin, f.Destination, ok, dist)
		}
	}
	prague := FallbackOverflightsPerHour(cfg, flowsLKPR, 100, []string{"LKPR"})
	east := FallbackOverflightsPerHour(cfg, kosice, 100, []string{"LZKZ"})
	remote := FallbackOverflightsPerHour(cfg, airport.LatLon{Lat: 62.0, Lon: -7.0}, 100, nil) // the Faroes
	pacific := FallbackOverflightsPerHour(cfg, airport.LatLon{Lat: -17.7, Lon: 168.3}, 100, nil)
	t.Logf("fallback peak per hour: Prague %.1f, Košice %.1f, the Faroes %.1f, Vanuatu %.1f; %d drawn at Košice in 48 h", prague, east, remote, pacific, len(flights))
	if !(prague > 2*remote) || pacific != 0 || east <= 0 {
		t.Errorf("fallback: Prague %.1f, Košice %.1f, the Faroes %.1f, Vanuatu %.1f", prague, east, remote, pacific)
	}
}

// TestOverflightFlowsOff: without Flows the generator is as before.
func TestOverflightFlowsOff(t *testing.T) {
	cfg := DefaultScheduleConfig()
	o := OverflightOptions{Centre: flowsLKPR, RadiusNM: 100, PerHour: 10, Seed: 9, Exclude: []string{"LKPR"}}
	from := time.Date(2026, 10, 1, 6, 0, 0, 0, time.UTC)
	got := Overflights(cfg, o, from, from.Add(3*time.Hour))
	if r := OverflightRate(cfg, o, from); r != 10*cfg.Waves[LocalHour("LKPR", flowsLKPR, from)] {
		t.Errorf("rate %.2f", r)
	}
	if len(got) == 0 {
		t.Fatal("none")
	}
}

// TestOverflightFlowsShipped: the shipped areas' streams have airports
// to draw from in the shipped config and give overflights crossing them.
func TestOverflightFlowsShipped(t *testing.T) {
	cfg := DefaultScheduleConfig()
	for _, c := range []struct {
		icao string
		p    airport.LatLon
	}{{"LKPR", flowsLKPR}, {"EGLL", airport.LatLon{Lat: 51.47, Lon: -0.4543}}} {
		area, ok := OverflightAreaAt(c.p)
		if !ok {
			t.Fatalf("%s: no area", c.icao)
		}
		o := OverflightOptions{Centre: c.p, RadiusNM: 100, Seed: 5, Exclude: []string{c.icao}, Flows: true}
		from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
		flights := drawHours(cfg, o, from, 72)
		var want float64
		for h := from; h.Before(from.Add(72 * time.Hour)); h = h.Add(time.Hour) {
			want += OverflightRate(cfg, o, h)
		}
		long := 0
		for _, f := range flights {
			if f.DistanceNM > 2000 {
				long++
				if l := cfg.limits(f.Type); l.MaxNM < f.DistanceNM {
					t.Errorf("%s: %s %s → %s %.0f NM beyond its range", c.icao, f.Type, f.Origin, f.Destination, f.DistanceNM)
				}
			}
		}
		t.Logf("%s (%s): %d in 72 h, mean %.0f, %d long haul", c.icao, area, len(flights), want, long)
		if float64(len(flights)) < 0.7*want {
			t.Errorf("%s: %d drawn, want about %.0f (streams without pairs?)", c.icao, len(flights), want)
		}
	}
}

// drawHours draws n hours from from, an hour at a time (as the manager
// asks: call signs are unique within a call).
func drawHours(cfg ScheduleConfig, o OverflightOptions, from time.Time, n int) []Flight {
	var out []Flight
	for h := range n {
		at := from.Add(time.Duration(h) * time.Hour)
		out = append(out, Overflights(cfg, o, at, at.Add(time.Hour))...)
	}
	return out
}
