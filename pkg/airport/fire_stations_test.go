package airport

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// TestFireStations: rescueVehicles in the layout JSON makes a vehicle
// depot a fire station; a stand with it set is not one, and a layout
// without it has none (#1587).
func TestFireStations(t *testing.T) {
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
	if fs := l.FireStations(); len(fs) != 0 {
		t.Fatalf("no count given, fire stations %v", fs)
	}
	depot, stand := -1, -1
	for i, p := range raw.Parking {
		switch {
		case depot < 0 && types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE(p.Type) == types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_VEHICLE:
			depot = i
		case stand < 0 && types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE(p.Type) != types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_VEHICLE:
			stand = i
		}
	}
	if depot < 0 || stand < 0 {
		t.Skip("no depot or stand at LKPR")
	}
	var set RawParking
	if err := json.Unmarshal([]byte(`{"type":0,"rescueVehicles":3}`), &set); err != nil || set.RescueVehicles != 3 {
		t.Fatalf("rescueVehicles from JSON: %d, %v", set.RescueVehicles, err)
	}
	raw.Parking[depot].RescueVehicles, raw.Parking[stand].RescueVehicles = 3, 2
	if l, err = BuildLayout(raw); err != nil {
		t.Fatal(err)
	}
	if fs := l.FireStations(); len(fs) != 1 || fs[0] != depot || l.Parking[depot].RescueVehicles != 3 {
		t.Errorf("fire stations %v, want [%d] with 3", fs, depot)
	}
	out, err := json.Marshal(l.Parking[depot])
	if err != nil {
		t.Fatal(err)
	}
	var back Parking
	if err := json.Unmarshal(out, &back); err != nil || back.RescueVehicles != 3 {
		t.Errorf("round trip %s: %d, %v", out, back.RescueVehicles, err)
	}
}
