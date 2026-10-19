package world

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// A runway change (live, TVS356 on VLM 6 for 24, then 06; TVS555 sent
// back to LOMKI against OKJAF): an arrival past the new STAR's entry fix
// joins it no farther out than it is, at the approach's initial fix at the
// latest, never turned back to the entry against the arrivals coming in
// on it; one before the entry flies the new STAR from it.
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
		end := to06[len(to06)-1].Position
		out := func(q airport.LatLon) float64 { return calc.HaversineNM(q.Lat, q.Lon, end.Lat, end.Lon) }
		iaf := len(to06)
		for i, n := range to06 {
			if n.IAF {
				iaf = i
				break
			}
		}
		if k := len(to06) - len(got); got[0].Ident != to06[0].Ident && (k > iaf || out(got[0].Position) > out(pos)+0.1 && !got[0].IAF) {
			t.Errorf("at %s (%.0f NM out): joined at %s (%.0f NM out), want no farther out, the IAF at the latest", to24[at].Ident, out(pos), got[0].Ident, out(got[0].Position))
		}
	}
	// Before VLM, inbound: the whole new STAR.
	in := calc.BearingDegrees(to24[0].Position.Lat, to24[0].Position.Lon, to24[1].Position.Lat, to24[1].Position.Lon)
	lat, lon := calc.DisplaceByHeading(to24[0].Position.Lat, to24[0].Position.Lon, in+180, 10*1852)
	if got := joinNewSTAR(airport.LatLon{Lat: lat, Lon: lon}, in, to06); got[0].Ident != to06[0].Ident {
		t.Errorf("10 NM before VLM: joined at %s, want VLM", got[0].Ident)
	}
}
