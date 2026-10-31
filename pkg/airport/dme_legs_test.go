package airport

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

const (
	legCD = types.SIMCONNECT_FACILITY_LEG_TYPE_CD
	legVD = types.SIMCONNECT_FACILITY_LEG_TYPE_VD
	legFD = types.SIMCONNECT_FACILITY_LEG_TYPE_FD
	legFC = types.SIMCONNECT_FACILITY_LEG_TYPE_FC
)

// TestDMELegEnds: a CD, VD or FD leg ends at its distance from the navaid, not
// that far from where it starts, and past the old 20 km when charted so.
func TestDMELegEnds(t *testing.T) {
	p := LatLon{Lat: 50, Lon: 14}
	east := func(m float64) LatLon { return displace(p, 90, m) }
	cases := []struct {
		name string
		leg  Leg
		want float64 // meters along the course; 0 no answer
	}{
		{"navaid ahead, first crossing", Leg{Type: legCD, Distance: 10000, OriginFix: "X", Origin: east(30000)}, 20000},
		{"navaid behind, flying away", Leg{Type: legVD, Distance: 11000, OriginFix: "X", Origin: east(-5000)}, 6000},
		{"inside the circle, navaid ahead", Leg{Type: legCD, Distance: 11000, OriginFix: "X", Origin: east(5000)}, 16000},
		{"from the navaid, 42 km charted", Leg{Type: legFD, Distance: 42000, OriginFix: "X", Origin: p}, 42000},
		{"never that near", Leg{Type: legCD, Distance: 1000, OriginFix: "X", Origin: displace(p, 0, 20000)}, 0},
		{"behind, only farther away", Leg{Type: legCD, Distance: 1000, OriginFix: "X", Origin: east(-20000)}, 0},
		{"older data, no navaid", Leg{Type: legCD, Distance: 10000}, 0},
		{"not a DME leg", Leg{Type: legFC, Distance: 10000, OriginFix: "X", Origin: east(30000)}, 0},
	}
	for _, c := range cases {
		got, ok := dmeEndMeters(c.leg, p, 90)
		if c.want == 0 {
			if ok {
				t.Errorf("%s: %.0f m, want none", c.name, got)
			}
			continue
		}
		if !ok || math.Abs(got-c.want) > 100 {
			t.Errorf("%s: %.0f m (%v), want %.0f", c.name, got, ok, c.want)
		}
	}
}

// TestDMELegInPathAndPoints: the map's path and the flown points end the leg
// alike; without the navaid both stay as they were.
func TestDMELegInPathAndPoints(t *testing.T) {
	p := LatLon{Lat: 50, Lon: 14}
	nav := displace(p, 90, 30000)
	leg := Leg{Type: legCD, Course: 90, Distance: 10000, OriginFix: "X", Origin: nav}
	dist := func(a, b LatLon) float64 { return calc.HaversineMeters(a.Lat, a.Lon, b.Lat, b.Lon) }

	alt := 0.0
	if n := openPoint(leg, p, &alt, 90, nil); math.Abs(dist(n.Position, nav)-10000) > 100 {
		t.Errorf("point %.0f m from the navaid, want 10000", dist(n.Position, nav))
	}
	path := ProcedurePath([]Leg{leg}, p, 0, 0, 0)
	if end := path[len(path)-1]; math.Abs(dist(end, nav)-10000) > 100 {
		t.Errorf("path ends %.0f m from the navaid, want 10000", dist(end, nav))
	}

	leg.OriginFix, leg.Origin = "", LatLon{}
	if n := openPoint(leg, p, &alt, 90, nil); math.Abs(dist(n.Position, p)-10000) > 100 {
		t.Errorf("older data: point %.0f m along, want 10000", dist(n.Position, p))
	}
	leg.Distance = 42000
	path = ProcedurePath([]Leg{leg}, p, 0, 0, 0)
	if end := path[len(path)-1]; math.Abs(dist(end, p)-20000) > 100 {
		t.Errorf("older data: path %.0f m along, want the 20000 cap", dist(end, p))
	}
}

// TestDecodeLegReadsTheNavaid: the fields follow the rest of the record, as
// requested; a shorter record leaves them empty.
func TestDecodeLegReadsTheNavaid(t *testing.T) {
	if legFields[len(legFields)-3] != "ORIGIN_ICAO" {
		t.Fatalf("fields end %v", legFields[len(legFields)-3:])
	}
	b := make([]byte, 136)
	copy(b[112:], "LON")
	put := func(off int, v float64) {
		bits := math.Float64bits(v)
		for i := 0; i < 8; i++ {
			b[off+i] = byte(bits >> (8 * i))
		}
	}
	put(120, 51.4872)
	put(128, -0.4665)
	l := decodeLeg(b)
	if l.OriginFix != "LON" || l.Origin.Lat != 51.4872 || l.Origin.Lon != -0.4665 {
		t.Errorf("got %q %+v", l.OriginFix, l.Origin)
	}
	if l := decodeLeg(b[:112]); l.OriginFix != "" || l.Origin != (LatLon{}) {
		t.Errorf("short record: %q %+v", l.OriginFix, l.Origin)
	}
}
