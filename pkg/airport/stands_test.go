package airport

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/types"
)

func loadEDDM(t testing.TB) *Layout {
	t.Helper()
	b, err := os.ReadFile("testdata/EDDM-layout.json")
	if err != nil {
		t.Fatal(err)
	}
	var l Layout
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	return &l
}

// TestParkingConflictsLKPR: 20 pairs of LKPR stands overlap (two more only
// touch, by under half a meter); the split S22/S22A pair is one of them, and
// conflicts are symmetric.
func TestParkingConflictsLKPR(t *testing.T) {
	l := loadLKPR(t)
	pairs := 0
	for _, p := range l.Parking {
		for _, q := range l.ParkingConflicts(p.Index) {
			if !slices.Contains(l.ParkingConflicts(q), p.Index) {
				t.Errorf("%s conflicts with %s but not the other way", p.Label(), l.Parking[q].Label())
			}
			if q > p.Index {
				pairs++
			}
		}
	}
	if pairs != 20 {
		t.Errorf("%d conflict pairs, want 20", pairs)
	}
	if got := l.ParkingConflicts(66); !slices.Equal(got, []int{65}) {
		t.Errorf("S22 conflicts %v, want [65] (S22A)", got)
	}
	if l.ParkingConflicts(-1) != nil || l.ParkingConflicts(len(l.Parking)) != nil {
		t.Error("conflicts of an unknown stand")
	}
}

func TestStandSize(t *testing.T) {
	for _, c := range []struct {
		typ    types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE
		radius float64
		want   StandSize
	}{
		{types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_MEDIUM, 40, StandMedium}, // the TYPE wins
		{types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_HEAVY, 22, StandHeavy},
		{types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_SMALL, 30, StandSmall},
		{types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_CARGO, 12, StandSmall}, // by RADIUS
		{types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_CARGO, 22, StandMedium},
		{types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_CARGO, 35, StandHeavy},
		{types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_FUEL, 30, StandNone},
		{types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_VEHICLE, 30, StandNone},
	} {
		if got := (Parking{Type: c.typ, Radius: c.radius}).Size(); got != c.want {
			t.Errorf("type %d radius %.0f: %v, want %v", c.typ, c.radius, got, c.want)
		}
	}
}

// TestSuitableStandsLKPR: an A320 (half span 18 m) fits the gates and ramps
// with RADIUS 18 m and more; fuel and vehicle spots never qualify, and a
// TYPE filter narrows the list.
func TestSuitableStandsLKPR(t *testing.T) {
	l := loadLKPR(t)
	all := l.SuitableStands(18)
	if len(all) == 0 {
		t.Fatal("no stands for an A320")
	}
	for _, i := range all {
		if p := l.Parking[i]; p.Radius < 18 || p.Size() == StandNone {
			t.Errorf("%s (radius %.0f, %v) is not suitable", p.Label(), p.Radius, p.Size())
		}
	}
	heavy := l.SuitableStands(18, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_HEAVY)
	if len(heavy) == 0 || len(heavy) >= len(all) {
		t.Fatalf("%d heavy gates of %d stands", len(heavy), len(all))
	}
	c22, _ := l.ParkingIndex("C22")
	if !slices.Contains(heavy, c22) {
		t.Error("C22 is a heavy gate")
	}
	if n := len(l.SuitableStands(1000)); n != 0 {
		t.Errorf("%d stands with a 1 km radius", n)
	}
}

// TestParkingAirlinesEDDM: EDDM assigns airlines to most of its stands; a
// stand serves its airlines (case-insensitive) and a stand without any
// serves every airline.
func TestParkingAirlinesEDDM(t *testing.T) {
	l := loadEDDM(t)
	with := 0
	var assigned, open *Parking
	for i := range l.Parking {
		p := &l.Parking[i]
		if len(p.Airlines) > 0 {
			with++
			if assigned == nil {
				assigned = p
			}
		} else if open == nil && p.Size() != StandNone {
			open = p
		}
	}
	if with != 119 {
		t.Errorf("%d stands with airlines, want 119", with)
	}
	if assigned == nil || open == nil {
		t.Fatal("fixture lacks stands with and without airlines")
	}
	code := assigned.Airlines[0]
	if !assigned.ServesAirline(code) || !assigned.ServesAirline(string([]rune(code)[0]+32)+code[1:]) {
		t.Errorf("%s does not serve %s", assigned.Label(), code)
	}
	if assigned.ServesAirline("ZZZ") {
		t.Errorf("%s serves ZZZ", assigned.Label())
	}
	if !open.ServesAirline("ZZZ") || !assigned.ServesAirline("") {
		t.Error("open stand or empty code")
	}
}
