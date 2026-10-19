package world

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// A runway change (live, TVS356 on VLM 6 for 24, then 06): an arrival past
// the new STAR's entry fix joins it at the approach's initial fix, never
// turned back to the entry against the arrivals coming in on it; one
// before the entry flies the new STAR from it.
func TestJoinNewSTAR(t *testing.T) {
	b, err := os.ReadFile("../../airport/testdata/LKPR-procedures.json")
	if err != nil {
		t.Fatal(err)
	}
	var p airport.Procedures
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	to24, err := p.Arrival("24", "VLM")
	if err != nil {
		t.Fatal(err)
	}
	to06, err := p.Arrival("06", "VLM")
	if err != nil {
		t.Fatal(err)
	}
	off := func(a, b airport.LatLon, hdg float64) float64 {
		return math.Abs(math.Mod(calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon)-hdg+540, 360) - 180)
	}
	for at := 1; at < len(to24)-2; at++ {
		if to24[at].Kind == "R" || to24[at].Position == (airport.LatLon{}) {
			continue
		}
		pos := to24[at].Position
		hdg := calc.BearingDegrees(to24[at-1].Position.Lat, to24[at-1].Position.Lon, pos.Lat, pos.Lon)
		got := joinNewSTAR(pos, hdg, to06)
		if len(got) < 2 {
			t.Fatalf("at %s: %d points left", to24[at].Ident, len(got))
		}
		if got[0].Ident == to06[0].Ident && off(pos, got[0].Position, hdg) > 90 {
			t.Errorf("at %s, heading %.0f: turned back to %s", to24[at].Ident, hdg, got[0].Ident)
		}
		if got[0].Ident != to06[0].Ident && !got[0].IAF {
			t.Errorf("at %s: joined at %s, want the entry or the IAF", to24[at].Ident, got[0].Ident)
		}
	}
	// Before VLM, inbound: the whole new STAR.
	in := calc.BearingDegrees(to24[0].Position.Lat, to24[0].Position.Lon, to24[1].Position.Lat, to24[1].Position.Lon)
	lat, lon := calc.DisplaceByHeading(to24[0].Position.Lat, to24[0].Position.Lon, in+180, 10*1852)
	if got := joinNewSTAR(airport.LatLon{Lat: lat, Lon: lon}, in, to06); got[0].Ident != to06[0].Ident {
		t.Errorf("10 NM before VLM: joined at %s, want VLM", got[0].Ident)
	}
}
