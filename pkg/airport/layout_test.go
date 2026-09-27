//go:build windows
// +build windows

package airport

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// loadLKPR builds the Layout from facility data captured in MSFS 2024.
func loadLKPR(t testing.TB) *Layout {
	t.Helper()
	b, err := os.ReadFile("testdata/LKPR.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestBuildLayoutLKPRCounts(t *testing.T) {
	l := loadLKPR(t)
	if l.ICAO != "LKPR" {
		t.Errorf("ICAO = %q, want LKPR", l.ICAO)
	}
	for _, c := range []struct {
		name      string
		got, want int
	}{
		{"runways", len(l.Runways), 2},
		{"parking", len(l.Parking), 89},
		{"taxi points", len(l.TaxiPoints), 1967},
		{"taxi paths", len(l.TaxiPaths), 2350},
		{"taxi names", len(l.TaxiNames), 29},
		{"hold-short points", len(l.HoldShortPoints()), 21},
	} {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

func TestBuildLayoutIndexesMatchPositions(t *testing.T) {
	l := loadLKPR(t)
	for i, p := range l.TaxiPoints {
		if p.Index != i {
			t.Fatalf("TaxiPoints[%d].Index = %d", i, p.Index)
		}
	}
	for i, p := range l.Parking {
		if p.Index != i {
			t.Fatalf("Parking[%d].Index = %d", i, p.Index)
		}
	}
	for i, p := range l.TaxiPaths {
		if p.Index != i {
			t.Fatalf("TaxiPaths[%d].Index = %d", i, p.Index)
		}
	}
}

func TestRunwayEnds(t *testing.T) {
	l := loadLKPR(t)
	cases := []struct {
		query, name, runway string
		heading             float64
	}{
		{"24", "24", "06/24", 245},
		{"6", "06", "06/24", 65},
		{"RW06", "06", "06/24", 65},
		{"12", "12", "12/30", 127},
		{"rwy30", "30", "12/30", 307},
	}
	for _, c := range cases {
		r, end, ok := l.RunwayEnd(c.query)
		if !ok {
			t.Errorf("RunwayEnd(%q) not found", c.query)
			continue
		}
		if end.Name != c.name || r.Name() != c.runway {
			t.Errorf("RunwayEnd(%q) = %s on %s, want %s on %s", c.query, end.Name, r.Name(), c.name, c.runway)
		}
		if math.Abs(end.Heading-c.heading) > 1 {
			t.Errorf("RunwayEnd(%q).Heading = %.1f, want ~%.0f", c.query, end.Heading, c.heading)
		}
	}
	if _, _, ok := l.RunwayEnd("18"); ok {
		t.Error("RunwayEnd(18) found on LKPR")
	}
}

func TestRunwayThresholds(t *testing.T) {
	l := loadLKPR(t)
	for _, r := range l.Runways {
		// Thresholds are placed on the WGS84 ellipsoid; HaversineMeters uses a
		// mean-radius sphere, which reads about 0.3% short at 50°N.
		d := calc.HaversineMeters(r.Primary.Threshold.Lat, r.Primary.Threshold.Lon, r.Secondary.Threshold.Lat, r.Secondary.Threshold.Lon)
		if math.Abs(d-r.Length) > r.Length*0.005 {
			t.Errorf("%s: threshold distance %.1f m, want runway length %.1f m", r.Name(), d, r.Length)
		}
		// The primary end faces its heading, so the secondary threshold lies ahead of it.
		b := calc.BearingDegrees(r.Primary.Threshold.Lat, r.Primary.Threshold.Lon, r.Secondary.Threshold.Lat, r.Secondary.Threshold.Lon)
		if diff := math.Abs(math.Mod(b-r.Primary.Heading+540, 360) - 180); diff > 1 {
			t.Errorf("%s: primary→secondary bearing %.1f, want primary heading %.1f", r.Name(), b, r.Primary.Heading)
		}
	}
}

func TestParkingLabels(t *testing.T) {
	l := loadLKPR(t)
	c22 := l.ParkingByLabel("c22")
	if len(c22) != 1 {
		t.Fatalf("ParkingByLabel(c22) = %d spots, want 1", len(c22))
	}
	p := c22[0]
	if p.Index != 18 || p.Name != types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_GATE_C ||
		p.Type != types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_HEAVY || !p.IsGate() {
		t.Errorf("C22 = %+v, want index 18, GATE_C, GATE_HEAVY", p)
	}
	// Two separate stands share NAME=S_PARKING, NUMBER=22 at LKPR.
	if n := len(l.ParkingByLabel("S22")); n != 2 {
		t.Errorf("ParkingByLabel(S22) = %d spots, want 2", n)
	}
}

func TestParkingLabelFormats(t *testing.T) {
	cases := []struct {
		name, suffix types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME
		number       uint32
		want         string
	}{
		{types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_GATE_A, 0, 1, "A1"},
		{types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_GATE_Z, 0, 9, "Z9"},
		{types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_NE_PARKING, 0, 3, "NE3"},
		{types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_PARKING, 0, 7, "7"},
		{types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_GATE, 0, 12, "12"},
		{types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_GATE_B, types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_GATE_A, 4, "B4A"},
	}
	for _, c := range cases {
		if got := (Parking{Name: c.name, Suffix: c.suffix, Number: c.number}).Label(); got != c.want {
			t.Errorf("Label(%d, %d, %d) = %q, want %q", c.name, c.suffix, c.number, got, c.want)
		}
	}
}

func TestPathEndpointsResolveEveryPath(t *testing.T) {
	l := loadLKPR(t)
	for _, p := range l.TaxiPaths {
		a, b, ok := l.PathEndpoints(p)
		if !ok {
			t.Fatalf("path %d (type %d, %d→%d) does not resolve", p.Index, p.Type, p.Start, p.End)
		}
		// PARKING paths join a stand to the nearby taxi line. With END read as a
		// taxi point they would be 0.5–3.3 km long at LKPR.
		if p.EndsAtParking() {
			if d := calc.HaversineMeters(a.Lat, a.Lon, b.Lat, b.Lon); d > 150 {
				t.Errorf("PARKING path %d is %.0f m long", p.Index, d)
			}
		}
	}
}

func TestHoldShortPointsAreOnTaxiways(t *testing.T) {
	l := loadLKPR(t)
	deg := map[int32]int{}
	for _, p := range l.TaxiPaths {
		if p.Type == types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_PATH || p.Type == types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_TAXI {
			deg[p.Start]++
			deg[p.End]++
		}
	}
	for _, h := range l.HoldShortPoints() {
		if deg[int32(h.Index)] == 0 {
			t.Errorf("hold-short point %d is not on a taxiway", h.Index)
		}
	}
}

func TestBuildLayoutNoData(t *testing.T) {
	if _, err := BuildLayout(RawAirport{ICAO: "ZZZZ"}); !errors.Is(err, ErrNoData) {
		t.Errorf("BuildLayout(empty) error = %v, want ErrNoData", err)
	}
}

func TestLayoutJSONRoundTrip(t *testing.T) {
	l := loadLKPR(t)
	b, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	var back Layout
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.TaxiPaths) != len(l.TaxiPaths) || back.Parking[18].Label() != "C22" || back.Runways[0].Name() != "06/24" {
		t.Error("Layout does not survive a JSON round trip")
	}
}

func TestNormalizeRunwayEnd(t *testing.T) {
	for in, want := range map[string]string{"6": "06", "06": "06", " rw6l ": "06L", "RWY27R": "27R", "36": "36", "N": "N"} {
		if got := normalizeRunwayEnd(in); got != want {
			t.Errorf("normalizeRunwayEnd(%q) = %q, want %q", in, got, want)
		}
	}
}
