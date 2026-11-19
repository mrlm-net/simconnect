package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// TestTransits: the neighbours' movements crossing the area (#1485): over
// the core, never from or to a focus airport, ends at least 100 NM apart,
// their floor out of view; a plausible volume at LKPR, fewer at a quiet
// airport; deterministic for a seed.
func TestTransits(t *testing.T) {
	cfg := DefaultScheduleConfig()
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	count := map[string]int{}
	for _, c := range []struct {
		icao string
		p    airport.LatLon
	}{{"LKPR", flowsLKPR}, {"LZKZ", airport.LatLon{Lat: 48.6631, Lon: 21.2411}}} {
		o := TransitOptions{Centre: c.p, RadiusNM: 100, Seed: 11, Exclude: []string{c.icao}}
		var all []Flight
		for h := 0; h < 48; h++ {
			at := from.Add(time.Duration(h) * time.Hour)
			all = append(all, Transits(cfg, o, at, at.Add(time.Hour))...)
		}
		count[c.icao] = len(all)
		var deps, near int
		for _, f := range all {
			a, b := endPos(cfg, f.Origin), endPos(cfg, f.Destination)
			dist := calc.HaversineNM(a.Lat, a.Lon, b.Lat, b.Lon)
			if f.Origin == c.icao || f.Destination == c.icao || dist < transitMinNM {
				t.Errorf("%s: %s %s → %s %.0f NM", c.icao, f.Callsign, f.Origin, f.Destination, dist)
			}
			if xt := math.Abs(calc.CrossTrackMeters(a.Lat, a.Lon, b.Lat, b.Lon, c.p.Lat, c.p.Lon)) / 1852; xt > 60 {
				t.Errorf("%s: %s %s → %s passes %.0f NM from the centre", c.icao, f.Callsign, f.Origin, f.Destination, xt)
			}
			departure := calc.HaversineNM(c.p.Lat, c.p.Lon, a.Lat, a.Lon) <= 200
			if departure {
				deps++
			}
			if !f.Exit.After(f.Enter) {
				t.Errorf("%s: %s exits %v before it enters %v", c.icao, f.Callsign, f.Exit, f.Enter)
			}
			// Its floor out of view: where it is at 10,000 ft from its
			// neighbour.
			n, x := a, b
			if !departure {
				n, x = b, a
			}
			fl := FloorAlongNM(f.Type, TransitFloorFt, departure)
			if d := calc.HaversineNM(n.Lat, n.Lon, x.Lat, x.Lon); fl < d {
				lat, lon := calc.IntermediatePoint(n.Lat, n.Lon, x.Lat, x.Lon, fl/d)
				if calc.HaversineNM(c.p.Lat, c.p.Lon, lat, lon) < TransitViewNM-1 {
					near++
				}
			}
		}
		t.Logf("%s: %d transits in 48 h (%d from a neighbour)", c.icao, len(all), deps)
		if near > 0 {
			t.Errorf("%s: %d transits at their floor in view", c.icao, near)
		}
	}
	if count["LKPR"] < 48*3 || count["LKPR"] > 48*30 {
		t.Errorf("LKPR: %d transits in 48 h, want 3 to 30 an hour on average", count["LKPR"])
	}
	if count["LZKZ"] >= count["LKPR"] {
		t.Errorf("LZKZ %d transits, LKPR %d: want fewer at the quiet one", count["LZKZ"], count["LKPR"])
	}
	o := TransitOptions{Centre: flowsLKPR, RadiusNM: 100, Seed: 3, Exclude: []string{"LKPR"}}
	x, y := Transits(cfg, o, from.Add(7*time.Hour), from.Add(8*time.Hour)), Transits(cfg, o, from.Add(7*time.Hour), from.Add(8*time.Hour))
	if len(x) != len(y) {
		t.Fatalf("seeded twice: %d and %d", len(x), len(y))
	}
	for i := range x {
		if x[i].Callsign != y[i].Callsign || !x[i].Enter.Equal(y[i].Enter) {
			t.Fatalf("seeded twice: %s and %s", x[i].Callsign, y[i].Callsign)
		}
	}
}
