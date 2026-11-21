package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// fieldNo747 is a field like TNCM offline: three heavy gates of 28 m
// radius, the largest it has, and a GA ramp; a 747 (half span 32 m) parks
// on none of them.
func fieldNo747() *airport.Layout {
	l := &airport.Layout{ICAO: "TNCM", Latitude: 18.04, Longitude: -63.11,
		Runways: []airport.Runway{{Length: 2300}}}
	for i, p := range []struct {
		r float64
		k types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE
	}{{28, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_HEAVY}, {28, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_HEAVY},
		{28, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_HEAVY}, {12, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_SMALL}} {
		l.Parking = append(l.Parking, airport.Parking{Index: i, Radius: p.r, Type: p.k})
	}
	return l
}

// TestStandFitRule: the fit check is the stand allocator's rule, a stand
// whose radius takes the type's half span (DefaultHalfSpanMeters when not
// known), of the stand types asked for.
func TestStandFitRule(t *testing.T) {
	l := fieldNo747()
	for typ, want := range map[string]bool{"B744": false, "B77W": false, "B789": false, "B738": true, "A320": true, "E190": true} {
		if got := FitsStands(l, typ); got != want {
			t.Errorf("FitsStands(%s) = %v, want %v (half span %.1f m, largest stand 28 m)", typ, got, want, StandHalfSpan(typ))
		}
	}
	if StandHalfSpan("ZZZZ") != DefaultHalfSpanMeters {
		t.Errorf("an unknown type's half span %.1f, want the allocator's %.1f", StandHalfSpan("ZZZZ"), DefaultHalfSpanMeters)
	}
	if ParksOn(l, 17.9, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_CARGO) {
		t.Error("a cargo stand found where there is none")
	}
	if ParksOn(l, 17.9, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_SMALL) {
		t.Error("a B738 fits a 12 m GA ramp")
	}
	if !ParksOn(l, 17.9, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_HEAVY) {
		t.Error("a B738 does not fit a 28 m gate")
	}
	// The allocator agrees: a B744 is refused, a B738 given a gate.
	a := NewStandAllocator(nil, &airport.Graph{Layout: l})
	if _, err := a.Assign(StandRequirements{Owner: "KLM1", HalfSpan: StandHalfSpan("B744")}); err == nil {
		t.Error("the allocator gave a B744 a stand FitsStands says it has not")
	}
	if _, err := a.Assign(StandRequirements{Owner: "KLM2", HalfSpan: StandHalfSpan("B738")}); err != nil {
		t.Errorf("the allocator refused a B738 FitsStands says fits: %v", err)
	}
}

// TestScheduleDrawsTypesThatPark: an airline whose fleet is mostly 747s
// flies to a field with no stand for one in the types of its fleet that
// park there; none of its flights there is a 747 (follow-up to #1616: a
// B744 EHAM → TNCM flew en route and was cancelled at its handover, "no
// free suitable stand").
func TestScheduleDrawsTypesThatPark(t *testing.T) {
	cfg := DefaultScheduleConfig()
	cfg.Airlines = []Airline{{ICAO: "KLM", Name: "KLM", Telephony: "KLM", Fleet: map[string]float64{"B744": 20, "B738": 1}, Regions: []string{"*"}, Weight: 1}}
	l := fieldNo747()
	from := time.Date(2026, 6, 20, 8, 0, 0, 0, time.UTC)
	flights := Schedule(cfg, ScheduleOptions{Focus: []string{"TNCM"}, Density: 3, Seed: 1616, Layouts: map[string]*airport.Layout{"TNCM": l}}, from, from.Add(6*time.Hour))
	n := 0
	for _, f := range flights {
		if f.Origin != "TNCM" && f.Destination != "TNCM" {
			continue
		}
		n++
		if !FitsStands(l, f.Type) {
			t.Errorf("%s %s → %s in a %s: no stand at TNCM fits it", f.Callsign, f.Origin, f.Destination, f.Type)
		}
	}
	if n == 0 {
		t.Fatal("no flight at TNCM: the airline's other type not drawn")
	}
}
