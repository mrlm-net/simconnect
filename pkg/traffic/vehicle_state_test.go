package traffic

import (
	"testing"
	"time"
)

// TestVehicleStateWaiting: a vehicle asked for, not in the simulator yet.
func TestVehicleStateWaiting(t *testing.T) {
	tug := NewSimObjectTug(nil, nil, "FSDT_Pushback", 1, MotionProfile{})
	if s := tug.State(); s != VehicleWaiting || tug.Title() != "FSDT_Pushback" {
		t.Errorf("tug %s %q", s, tug.Title())
	}
	if s := NewSimObjectFuelTruck(nil, nil, "Fuel", 2, MotionProfile{}).State(); s != VehicleWaiting {
		t.Errorf("fuel truck %s", s)
	}
}

// TestRadioOccupy: what is said outside the radio holds what it says next.
func TestRadioOccupy(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	r := NewRadio(RadioOptions{Now: func() time.Time { return now }})
	r.Occupy("LKPR", "118.100", now.Add(5*time.Second))
	got := r.Transmit("LKPR", Transmission{Position: PosTower, Callsign: "CSA1", Frequency: "118.100", Text: "CSA1, hello"})
	if got.At.Before(now.Add(5 * time.Second)) {
		t.Errorf("said at %v, before the frequency was clear", got.At)
	}
	if n := len(r.Recent("LKPR", 10)); n != 1 {
		t.Errorf("%d kept, want only the radio's own", n)
	}
}
