package world

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// A departure rolling on 30 holds a take-off on 24 until it is past where
// the runways cross; an arrival on a short final to 30 holds take-offs on
// 24, not landings (live, LKPR: TVS833 and CSA522).
func TestCrossBlocked(t *testing.T) {
	b, err := os.ReadFile("../../airport/testdata/LKPR.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	r24, _, _ := l.RunwayEnd("24")
	_, e30, _ := l.RunwayEnd("30")
	x, ok := crossAlong(l, "30", r24)
	if !ok || x <= 0 {
		t.Fatalf("30 and 24 do not cross: %v %v", x, ok)
	}
	at := func(m float64) airport.LatLon { // m along 30 from its threshold
		la, lo := calc.DisplaceByHeading(e30.Threshold.Lat, e30.Threshold.Lon, e30.Heading, m)
		return airport.LatLon{Lat: la, Lon: lo}
	}
	rolling := []crossUse{{icao: l.ICAO, end: "30", tail: "TVS833", pos: at(200)}}
	if crossBlocked(l, "24", rolling, true) != "TVS833" {
		t.Error("take-off on 24 with TVS833 rolling on 30 short of the crossing")
	}
	if crossBlocked(l, "24", rolling, false) != "TVS833" {
		t.Error("landing on 24 with TVS833 rolling on 30 short of the crossing")
	}
	rolling[0].pos = at(x + crossPastM + 200)
	if by := crossBlocked(l, "24", rolling, true); by != "" {
		t.Errorf("take-off on 24 held by %s past the crossing", by)
	}
	if by := crossBlocked(l, "30", rolling, true); by != "" {
		t.Errorf("its own runway held by %s", by)
	}
	final := []crossUse{{icao: l.ICAO, end: "30", tail: "OKABC", pos: at(-3000), final: true}}
	if crossBlocked(l, "24", final, true) != "OKABC" {
		t.Error("take-off on 24 with an arrival on a short final to 30")
	}
	if by := crossBlocked(l, "24", final, false); by != "" {
		t.Errorf("landing on 24 held by %s on its final", by)
	}
}
