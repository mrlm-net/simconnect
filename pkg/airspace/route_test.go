package airspace

import (
	"math"
	"testing"
	"time"
)

// TestRoute (#1259): north to LKPR at 2,000 ft, below the Prague CTA's
// floor of 1,000 ft AGL, the one airspace entered is Ruzyne CTR, a
// clearance, about 8.3 NM along; west to east through Usti nad Labem, the
// path starts in its ATZ and the CTA's E and enters prohibited area 10.
func TestRoute(t *testing.T) {
	s := store(t, StorePreferNewerSet(), StoreSkipBroken())
	es, err := s.Route([]PathPoint{{Lat: 49.8508, Lon: 14.26, AltM: 610, GroundM: 365}, {Lat: lkpr.Lat, Lon: lkpr.Lon, AltM: 610, GroundM: 365}})
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 1 || es[0].Name != "RUZYNE CTR" || es[0].Need != NeedClearance || es[0].Start ||
		math.Abs(es[0].AlongM-15389) > 300 || es[0].FloorM != 0 || math.Abs(es[0].CeilingM-1066.8) > 0.1 {
		t.Fatalf("north to LKPR: %+v", es)
	}
	es, err = s.Route([]PathPoint{{Lat: 50.6564, Lon: 13.95, AltM: 600, GroundM: 200}, {Lat: 50.6564, Lon: 14.10, AltM: 600, GroundM: 200}})
	if err != nil {
		t.Fatal(err)
	}
	need := map[string]Need{}
	for _, e := range es {
		need[e.Name] = e.Need
		if e.Name == "10 (USTI NAD LABEM)" && (e.Start || e.AlongM < 3000 || e.AlongM > 5000) {
			t.Errorf("area 10 entered at %.0f m (start %v)", e.AlongM, e.Start)
		}
	}
	if need["USTI NAD LABEM ATZ"] != NeedClearance || need["PRAGUE CTA 2"] != NeedNone || need["10 (USTI NAD LABEM)"] != NeedAvoid {
		t.Errorf("Usti: %v", need)
	}
	// A country's own meaning replaces the default.
	es, _ = s.Route([]PathPoint{{Lat: 49.8508, Lon: 14.26, AltM: 610, GroundM: 365}, {Lat: lkpr.Lat, Lon: lkpr.Lon, AltM: 610, GroundM: 365}},
		RouteNeeds(map[BoundaryType]Need{ClassD: NeedInform}))
	if len(es) != 1 || es[0].Need != NeedInform {
		t.Errorf("RouteNeeds: %+v", es)
	}
}

// TestAhead: 120 kt for 5 minutes due north is 10 NM.
func TestAhead(t *testing.T) {
	p := Ahead(PathPoint{Lat: 50, Lon: 14, AltM: 600}, 0, 120, 5*time.Minute)
	if len(p) != 2 || math.Abs((p[1].Lat-50)*60-10) > 0.05 || math.Abs(p[1].Lon-14) > 1e-9 || p[1].AltM != 600 {
		t.Errorf("ahead %+v", p)
	}
}

// TestFrequency: a record's frequency as text, "" for null.
func TestFrequency(t *testing.T) {
	for raw, want := range map[string]string{"null": "", `"118.100"`: "118.100", "118.1": "118.1", "": ""} {
		if got := frequencyOf([]byte(raw)); got != want {
			t.Errorf("%q: %q, want %q", raw, got, want)
		}
	}
}
