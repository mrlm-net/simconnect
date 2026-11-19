package airport

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// runwayFixLayout is one 3000 m runway 06/24, 24's landing threshold
// displaced displacedM in.
func runwayFixLayout(displacedM float64) *Layout {
	center := LatLon{Lat: 50.1, Lon: 14.25}
	at := func(hdg float64) LatLon {
		lat, lon := calc.DisplaceByHeading(center.Lat, center.Lon, hdg, 1500)
		return LatLon{Lat: lat, Lon: lon}
	}
	return &Layout{ICAO: "TEST", Runways: []Runway{{
		Center: center, Heading: 60, Length: 3000, Width: 45,
		Primary:   RunwayEnd{Number: 6, Name: "06", Heading: 60, Threshold: at(240)},
		Secondary: RunwayEnd{Number: 24, Name: "24", Heading: 240, Threshold: at(60), DisplacedM: displacedM},
	}}}
}

func fixAlong(p LatLon, hdg, m float64) LatLon {
	lat, lon := calc.DisplaceByHeading(p.Lat, p.Lon, hdg, m)
	return LatLon{Lat: lat, Lon: lon}
}

func fixDistM(a, b LatLon) float64 { return calc.HaversineMeters(a.Lat, a.Lon, b.Lat, b.Lon) }

// A runway fix 0.9 NM down the runway (as the facility data has them, at
// the centre: #1573) goes to the runway end: the landing threshold in a
// STAR and an approach's final and missed approach, the pavement end in a
// SID; the other fixes stay, and p is not changed.
func TestAtRunwayEndsMovesRunwayFix(t *testing.T) {
	l := runwayFixLayout(300)
	e24 := l.Runways[0].Secondary
	down := fixAlong(e24.Threshold, 240, 0.9*1852) // 0.9 NM down runway 24
	faf := fixAlong(e24.Threshold, 60, 5*1852)
	rw := func(tp types.SIMCONNECT_FACILITY_LEG_TYPE) Leg {
		return Leg{Type: tp, Fix: "RW24", FixKind: "R", Position: down}
	}
	p := Procedures{ICAO: "TEST",
		Departures: []Procedure{{Name: "SID1", RunwayTransitions: []Transition{{Runway: "24", Legs: []Leg{
			rw(types.SIMCONNECT_FACILITY_LEG_TYPE_IF), {Type: types.SIMCONNECT_FACILITY_LEG_TYPE_CA, Course: 240, Alt1: 500}}}}}},
		Arrivals: []Procedure{{Name: "STAR1", RunwayTransitions: []Transition{{Runway: "24", Legs: []Leg{
			{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_IF, Fix: "FAF24", FixKind: "W", Position: faf}, rw(types.SIMCONNECT_FACILITY_LEG_TYPE_TF)}}}}},
		Approaches: []Approach{{Name: "ILS 24", Runway: "24",
			Final: []Leg{{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_IF, Fix: "FAF24", FixKind: "W", Position: faf, FAF: true},
				func() Leg { m := rw(types.SIMCONNECT_FACILITY_LEG_TYPE_CF); m.MAP = true; return m }()},
			Missed: []Leg{rw(types.SIMCONNECT_FACILITY_LEG_TYPE_TF)}}},
	}
	got, moved := p.AtRunwayEnds(l)
	if !moved {
		t.Fatal("moved false")
	}
	if d := fixDistM(down, e24.Threshold); math.Abs(d-0.9*1852) > 10 {
		t.Fatalf("setup: fix %.0f m from the pavement end", d)
	}
	checks := []struct {
		what string
		got  LatLon
		want LatLon
	}{
		{"SID", got.Departures[0].RunwayTransitions[0].Legs[0].Position, e24.Threshold},
		{"STAR", got.Arrivals[0].RunwayTransitions[0].Legs[1].Position, e24.LandingThreshold()},
		{"final MAP", got.Approaches[0].Final[1].Position, e24.LandingThreshold()},
		{"missed", got.Approaches[0].Missed[0].Position, e24.LandingThreshold()},
		{"FAF kept", got.Approaches[0].Final[0].Position, faf},
		{"STAR FAF kept", got.Arrivals[0].RunwayTransitions[0].Legs[0].Position, faf},
	}
	for _, c := range checks {
		if d := fixDistM(c.got, c.want); d > 0.01 {
			t.Errorf("%s: %.1f m from where it belongs", c.what, d)
		}
	}
	if d := fixDistM(e24.Threshold, e24.LandingThreshold()); math.Abs(d-300) > 1 {
		t.Errorf("landing threshold %.0f m in, want 300", d)
	}
	// p itself is untouched.
	if p.Approaches[0].Final[1].Position != down || p.Departures[0].RunwayTransitions[0].Legs[0].Position != down {
		t.Error("p changed")
	}
	// Placed again: nothing moves.
	if _, again := got.AtRunwayEnds(l); again {
		t.Error("placed procedures moved again")
	}
}

// A runway fix already on its end (within a few metres) is kept and p
// comes back as it was, sharing its slices; so do fixes too far away to
// be that runway's, a runway the layout lacks, another airport's layout
// and none.
func TestAtRunwayEndsKeeps(t *testing.T) {
	l := runwayFixLayout(0)
	e24 := l.Runways[0].Secondary
	near := fixAlong(e24.Threshold, 240, 3)
	mk := func(fix string, pos LatLon) Procedures {
		return Procedures{ICAO: "TEST", Approaches: []Approach{{Name: "ILS 24", Runway: "24",
			Final: []Leg{{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_CF, Fix: fix, FixKind: "R", Position: pos, MAP: true}}}}}
	}
	for name, c := range map[string]struct {
		p Procedures
		l *Layout
	}{
		"on the threshold": {mk("RW24", near), l},
		"too far":          {mk("RW24", fixAlong(e24.Threshold, 60, 3000+2000)), l},
		"unknown runway":   {mk("RW33", fixAlong(e24.Threshold, 240, 1500)), l},
		"other airport":    {func() Procedures { p := mk("RW24", fixAlong(e24.Threshold, 240, 1500)); p.ICAO = "OTHR"; return p }(), l},
		"no layout":        {mk("RW24", fixAlong(e24.Threshold, 240, 1500)), nil},
	} {
		got, moved := c.p.AtRunwayEnds(c.l)
		if moved {
			t.Errorf("%s: moved", name)
		}
		if &got.Approaches[0] != &c.p.Approaches[0] || got.Approaches[0].Final[0].Position != c.p.Approaches[0].Final[0].Position {
			t.Errorf("%s: not p itself", name)
		}
	}
}

// offlineProcedures are icao's raw layout and procedures from the
// simconnect-data clone ($SIMCONNECT_DATA, else beside this checkout);
// the test skips without them.
func offlineProcedures(t *testing.T, icao string) (RawAirport, Procedures) {
	t.Helper()
	dir := os.Getenv("SIMCONNECT_DATA")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "simconnect-data")
	}
	dir = filepath.Join(dir, icao)
	if _, err := os.Stat(filepath.Join(dir, "procedures.json")); err != nil {
		t.Skipf("no offline %s at %s (SIMCONNECT_DATA)", icao, dir)
	}
	var raw RawAirport
	var p Procedures
	for f, v := range map[string]any{"layout.json": &raw, "procedures.json": &p} {
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, v); err != nil {
			t.Fatal(err)
		}
	}
	return raw, p
}

// mapOf is the position of approach name's MAP, resolved from p.
func mapOf(t *testing.T, p Procedures, name string) LatLon {
	t.Helper()
	pts, err := p.ResolveApproach(name, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range pts {
		if n.MAP {
			return n.Position
		}
	}
	t.Fatalf("%s: no MAP", name)
	return LatLon{}
}

// KJFK ILS 04L (#1573): the facility data's RW04L is the runway's centre,
// 0.92 NM from the FAA CIFP threshold (40.62310, -73.78473); placed, the
// MAP is on the landing threshold. The simulator's 04L starts 140 m before
// the CIFP threshold, which it does not displace: with that displacement
// in the layout (as a data repo fills it, #1560) the MAP is the CIFP point.
func TestAtRunwayEndsKJFK04L(t *testing.T) {
	raw, p := offlineProcedures(t, "KJFK")
	cifp := LatLon{Lat: 40.62310, Lon: -73.78473}
	if d := fixDistM(mapOf(t, p, "ILS 04L"), cifp); d < 0.8*1852 {
		t.Fatalf("data: RW04L %.0f m from the CIFP threshold, the test expects the centre", d)
	}
	l, err := BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	e, _ := l.runwayEnd("04L")
	placed, moved := p.AtRunwayEnds(l)
	if !moved {
		t.Fatal("nothing moved")
	}
	if d := fixDistM(mapOf(t, placed, "ILS 04L"), e.LandingThreshold()); d > 0.01 {
		t.Errorf("MAP %.1f m from the landing threshold", d)
	}
	// The CIFP threshold: on the centreline, 140 m in from the pavement end.
	in := calc.HaversineMeters(e.Threshold.Lat, e.Threshold.Lon, cifp.Lat, cifp.Lon)
	off := math.Abs(in * math.Sin((calc.BearingDegrees(e.Threshold.Lat, e.Threshold.Lon, cifp.Lat, cifp.Lon)-e.Heading)*math.Pi/180))
	if math.Abs(in-140) > 15 || off > 15 {
		t.Fatalf("CIFP threshold %.0f m in, %.0f m off the centreline", in, off)
	}
	for i, r := range raw.Runways {
		if r.PrimaryNumber == 4 && r.PrimaryDesignator == 1 { // 04L
			raw.Runways[i].PrimaryDisplacedM = 140
		}
	}
	l, err = BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	placed, _ = p.AtRunwayEnds(l)
	if d := fixDistM(mapOf(t, placed, "ILS 04L"), cifp); d > 30 {
		t.Errorf("displaced 140 m: MAP %.0f m from the CIFP threshold", d)
	}
}

// Every approach's runway fix ends on its landing threshold once placed,
// and every SID's on its pavement end, at the airports of #1573 and the
// test beds (all at the runway's centre in the data).
func TestAtRunwayEndsOffline(t *testing.T) {
	for _, icao := range []string{"KJFK", "KLAX", "LKPR", "EGLL"} {
		raw, p := offlineProcedures(t, icao)
		l, err := BuildLayout(raw)
		if err != nil {
			t.Fatal(err)
		}
		placed, _ := p.AtRunwayEnds(l)
		n := 0
		check := func(legs []Leg, landing bool, what string) {
			for _, lg := range legs {
				if lg.FixKind != "R" || !lg.HasFix() {
					continue
				}
				e, ok := l.runwayEnd(lg.Fix)
				if !ok {
					continue
				}
				want := e.Threshold
				if landing {
					want = e.LandingThreshold()
				}
				if d := fixDistM(lg.Position, want); d > runwayFixSameMeters {
					t.Errorf("%s %s %s: %.0f m from its runway end", icao, what, lg.Fix, d)
				}
				n++
			}
		}
		for _, a := range placed.Approaches {
			check(a.Final, true, a.Name)
			check(a.Missed, true, a.Name+" missed")
			for _, tr := range a.Transitions {
				check(tr.Legs, true, a.Name+" via "+tr.Name)
			}
		}
		for _, s := range placed.Departures {
			check(s.Legs, false, s.Name)
			for _, tr := range s.RunwayTransitions {
				check(tr.Legs, false, s.Name+" "+tr.Runway)
			}
		}
		if n == 0 {
			t.Errorf("%s: no runway fixes", icao)
		}
	}
}
