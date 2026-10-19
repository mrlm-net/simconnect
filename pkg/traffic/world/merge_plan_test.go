package world

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestMergeNeedTwoSTARs: two arrivals on LKPR's VLM 5S and LOMK 8S to 24,
// reaching the point where their ways meet together, are planned apart
// before it (the later loses the time); one a full spacing ahead there is
// no merge to plan (live, TVS1113 and TVS122 met at 7000 ft, caught only
// by the conflict watch).
func TestMergeNeedTwoSTARs(t *testing.T) {
	b, err := os.ReadFile("../../airport/testdata/LKPR-procedures.json")
	if err != nil {
		t.Fatal(err)
	}
	var p airport.Procedures
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	way := func(fix string) []traffic.RoutePoint {
		r, err := p.Arrival("24", fix)
		if err != nil {
			t.Fatal(err)
		}
		return releaseProfile(r)
	}
	a, c := way("VLM"), way("LOMKI")
	// Where they meet: the nearest points of the two.
	ia, ic, best := 0, 0, 1e9
	for i := range a {
		for j := range c {
			if d := calc.HaversineNM(a[i].Position.Lat, a[i].Position.Lon, c[j].Position.Lat, c[j].Position.Lon); d < best {
				ia, ic, best = i, j, d
			}
		}
	}
	if best > 2 {
		t.Fatalf("the two STARs come no closer than %.1f NM", best)
	}
	// Each started back along its way nm from the meeting point, flying on.
	back := func(w []traffic.RoutePoint, at int, nm float64, tail string) (traffic.TrackedAircraft, []traffic.RoutePoint) {
		i := at
		for i > 0 && nm > 0 {
			d := calc.HaversineNM(w[i-1].Position.Lat, w[i-1].Position.Lon, w[i].Position.Lat, w[i].Position.Lon)
			if d >= nm {
				brg := calc.BearingDegrees(w[i].Position.Lat, w[i].Position.Lon, w[i-1].Position.Lat, w[i-1].Position.Lon)
				lat, lon := calc.DisplaceByHeading(w[i].Position.Lat, w[i].Position.Lon, brg, nm*1852)
				pos := airport.LatLon{Lat: lat, Lon: lon}
				ac := traffic.TrackedAircraft{Observation: traffic.Observation{Tail: tail, Position: pos, AltFt: w[i-1].AltFt, GroundKts: 250,
					Heading: calc.BearingDegrees(lat, lon, w[i].Position.Lat, w[i].Position.Lon)}, Phase: traffic.PhaseArriving, Ours: true}
				return ac, w[i:]
			}
			nm -= d
			i--
		}
		t.Fatalf("%s: way too short", tail)
		return traffic.TrackedAircraft{}, nil
	}
	lead, leadPlan := back(c, ic, 20, "TVS1113")
	me, plan := back(a, ia, 20, "TVS122")
	need, within := mergeNeed(me, plan, lead, leadPlan)
	if need < mergeMinNeed {
		t.Fatalf("level at the merge: need %s, want time lost before it", need)
	}
	if within <= 0 || within > 25 {
		t.Errorf("lost within %.1f NM, want before the merge 20 NM on", within)
	}
	me, plan = back(a, ia, 32, "TVS122")
	if need, _ := mergeNeed(me, plan, lead, leadPlan); need != 0 {
		t.Errorf("12 NM behind at the merge: need %s, want none", need)
	}
}
