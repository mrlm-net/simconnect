//go:build windows
// +build windows

package gsx

import (
	"slices"
	"testing"
)

// GSX's state from its variables, as its manual documents them.
func TestRead(t *testing.T) {
	s := Read(map[string]float64{
		"L:FSDT_GSX_BOARDING_STATE": 5, "L:FSDT_GSX_DEPARTURE_STATE": 1, "L:FSDT_GSX_REFUELING_STATE": 6,
		"L:FSDT_GSX_NUMPASSENGERS": 111, "L:FSDT_GSX_NUMPASSENGERS_BOARDING": 40,
		"L:FSDT_GSX_BOARDING_CARGO": 1, "L:FSDT_GSX_BOARDING_CARGO_PERCENT": 37.5,
		"L:FSDT_GSX_AIRCRAFT_EXIT_1_TOGGLE": 1, "L:FSDT_GSX_AIRCRAFT_CARGO_2_TOGGLE": 1,
		"L:FSDT_GSX_SetGate_Name": 14, "L:FSDT_GSX_SetGate_Number": 3, "L:FSDT_GSX_SetGate_Suffix": -1,
	})
	if !s.Running || s.Boarding != Performing || s.Refueling != Completed || s.Departure != Callable || s.Boarding.String() != "performing" {
		t.Errorf("services %+v", s)
	}
	if s.Passengers != 111 || s.PassengersBoarding != 40 || !s.LoadingCargo || s.CargoLoadedPct != 37.5 {
		t.Errorf("passengers and cargo %+v", s)
	}
	if !slices.Equal(s.WaitingFor, []string{"exit 1", "cargo 2"}) {
		t.Errorf("waiting for %v", s.WaitingFor)
	}
	if s.Gate != "C3" {
		t.Errorf("gate %q, want C3 (GATE_C 3)", s.Gate)
	}
	if s := Read(map[string]float64{}); s.Running || s.Gate != "" {
		t.Errorf("no GSX: %+v", s)
	}
}
