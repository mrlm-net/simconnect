package airport

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

// rawLKPR is testdata/LKPR.json as read, without lkprRaw's surface fill.
func rawLKPR(t *testing.T) RawAirport {
	t.Helper()
	b, err := os.ReadFile("testdata/LKPR.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

// Absent displacements write nothing: raw records and the built layout
// marshal exactly as before (#1560).
func TestDisplacedAbsentJSON(t *testing.T) {
	raw := rawLKPR(t)
	a, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(a, []byte("DisplacedM")) {
		t.Fatalf("raw JSON has a displacement field without one set")
	}
	var back RawAirport
	if err := json.Unmarshal(a, &back); err != nil {
		t.Fatal(err)
	}
	if b, _ := json.Marshal(back); !bytes.Equal(a, b) {
		t.Fatalf("raw round trip changed the JSON")
	}
	l, err := BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	lj, _ := json.Marshal(l)
	if bytes.Contains(lj, []byte("displacedM")) {
		t.Fatalf("layout JSON has displacedM without a displacement")
	}
	for _, r := range l.Runways {
		for _, e := range []RunwayEnd{r.Primary, r.Secondary} {
			if e.LandingThreshold() != e.Threshold {
				t.Errorf("%s: landing threshold %v, threshold %v", e.Name, e.LandingThreshold(), e.Threshold)
			}
		}
	}
}

func TestDisplacedJSONRoundTrip(t *testing.T) {
	raw := rawLKPR(t)
	raw.Runways[0].PrimaryDisplacedM = 100
	b, err := json.Marshal(raw.Runways[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(`"primaryDisplacedM":100`)) || bytes.Contains(b, []byte("secondaryDisplacedM")) {
		t.Fatalf("raw runway JSON: %s", b)
	}
	var back RawRunway
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.PrimaryDisplacedM != 100 || back.SecondaryDisplacedM != 0 {
		t.Fatalf("read back %v / %v", back.PrimaryDisplacedM, back.SecondaryDisplacedM)
	}
	l, err := BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	lj, _ := json.Marshal(l.Runways[0])
	var rwy Runway
	if err := json.Unmarshal(lj, &rwy); err != nil {
		t.Fatal(err)
	}
	if rwy.Primary.DisplacedM != 100 || rwy.Secondary.DisplacedM != 0 || bytes.Count(lj, []byte("displacedM")) != 1 {
		t.Fatalf("layout runway JSON: %s", lj)
	}
}

func TestDisplacedThresholdGeometry(t *testing.T) {
	plain, err := BuildLayout(rawLKPR(t))
	if err != nil {
		t.Fatal(err)
	}
	raw := rawLKPR(t)
	raw.Runways[0].PrimaryDisplacedM = 300
	l, err := BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	p, s := l.Runways[0].Primary, l.Runways[0].Secondary
	if p.Threshold != plain.Runways[0].Primary.Threshold || s.Threshold != plain.Runways[0].Secondary.Threshold {
		t.Fatalf("pavement thresholds moved")
	}
	if s.LandingThreshold() != s.Threshold {
		t.Errorf("secondary landing threshold moved")
	}
	// 300 m along the heading as the layout places points (the local flat
	// projection; great-circle distances differ by a few tenths of a percent).
	lt := p.LandingThreshold()
	wLat, wLon := calc.DisplaceByHeading(p.Threshold.Lat, p.Threshold.Lon, p.Heading, 300)
	if lt != (LatLon{Lat: wLat, Lon: wLon}) {
		t.Errorf("landing threshold %v, want %v", lt, LatLon{Lat: wLat, Lon: wLon})
	}
	if b := calc.BearingDegrees(p.Threshold.Lat, p.Threshold.Lon, lt.Lat, lt.Lon); math.Abs(math.Mod(b-p.Heading+540, 360)-180) > 0.5 {
		t.Errorf("landing threshold at bearing %.1f, runway heading %.1f", b, p.Heading)
	}
	// On the runway, between both pavement ends.
	full := calc.HaversineMeters(p.Threshold.Lat, p.Threshold.Lon, s.Threshold.Lat, s.Threshold.Lon)
	in := calc.HaversineMeters(p.Threshold.Lat, p.Threshold.Lon, lt.Lat, lt.Lon)
	left := calc.HaversineMeters(lt.Lat, lt.Lon, s.Threshold.Lat, s.Threshold.Lon)
	if math.Abs(in-300) > 3 || math.Abs(in+left-full) > 0.5 {
		t.Errorf("landing threshold %.1f m in, %.1f m to the far end of %.1f", in, left, full)
	}

	// Not on the runway: ignored.
	for _, bad := range []float32{-50, raw.Runways[0].Length, raw.Runways[0].Length + 10} {
		raw.Runways[0].PrimaryDisplacedM = bad
		l, err := BuildLayout(raw)
		if err != nil {
			t.Fatal(err)
		}
		if e := l.Runways[0].Primary; e.DisplacedM != 0 || e.LandingThreshold() != e.Threshold {
			t.Errorf("displacement %v kept as %v", bad, e.DisplacedM)
		}
	}
}

// ExitFor measures the rollout from the displaced landing threshold:
// the exit for a rollout R is the undisplaced one for R plus the
// displacement.
func TestExitForDisplaced(t *testing.T) {
	plainL, err := BuildLayout(rawLKPR(t))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := BuildGraph(plainL)
	if err != nil {
		t.Fatal(err)
	}
	raw := rawLKPR(t)
	const disp = 400
	for i := range raw.Runways {
		raw.Runways[i].PrimaryDisplacedM, raw.Runways[i].SecondaryDisplacedM = disp, disp
	}
	l, err := BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	g, err := BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	for _, end := range []string{"24", "06"} {
		for _, roll := range []float64{800, 1200, 1600} {
			got, err := g.ExitFor(end, roll)
			if err != nil {
				t.Fatal(err)
			}
			want, err := plain.ExitFor(end, roll+disp)
			if err != nil {
				t.Fatal(err)
			}
			if got.Node != want.Node || got.Along != want.Along {
				t.Errorf("RWY %s rollout %.0f: exit %s at %.0f, want %s at %.0f", end, roll, got.Taxiway, got.Along, want.Taxiway, want.Along)
			}
		}
	}
}
