package world

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestRunwaysCross: LKPR's 06/24 and 12/30 cross; a runway parallel to
// 06/24 does not; 06/24 is not crossed by itself shifted along its line.
func TestRunwaysCross(t *testing.T) {
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
	r24, _, ok1 := l.RunwayEnd("24")
	r30, _, ok2 := l.RunwayEnd("30")
	if !ok1 || !ok2 {
		t.Fatal("no 24 or 30")
	}
	if !runwaysCross(r24, r30) {
		t.Error("06/24 and 12/30 do not cross")
	}
	// A parallel 1500 m north.
	par := r24
	shift := func(p airport.LatLon) airport.LatLon { return airport.LatLon{Lat: p.Lat + 1500/111320.0, Lon: p.Lon} }
	par.Primary.Threshold, par.Secondary.Threshold = shift(r24.Primary.Threshold), shift(r24.Secondary.Threshold)
	if runwaysCross(r24, par) {
		t.Error("parallel runways cross")
	}
}
