package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/addons"
	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/jetway"
)

// TestGateHelperBySize: at LKPR a narrowbody docks with its own helper on
// a 22 m gate, with one too wide for 22 m gates on C24 (28 m), and with one
// too wide for C24 on B14 and C22 (35 m); a widebody keeps its own.
func TestGateHelperBySize(t *testing.T) {
	g, err := airport.BuildGraph(lkprLayout(t))
	if err != nil {
		t.Fatal(err)
	}
	// The installed models' spans (ft), as the map reads them.
	setDoors(nil, map[string]addons.ModelDoor{"B763": {WingSpanFt: 156.1}, "A306": {WingSpanFt: 147.1}})
	defer setDoors(nil, nil)
	svc := jetway.NewHelpers(nil, jetway.HelperOptions{Types: []string{"A320", "B738", "A306", "B763", "B788", "A332", "B77W"}})
	at := func(label string) int {
		s, err := g.Layout.ParkingIndex(label)
		if err != nil {
			t.Fatalf("no stand %s", label)
		}
		return s
	}
	for _, tc := range []struct{ stand, model, want string }{
		{"B15", "FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery", "A320"},
		{"C24", "FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery", "B763"},
		{"B14", "FSLTL_B738_RYR", "B788"},
		{"C22", "FSLTL_B738_RYR", "B788"},
		{"B14", "FSLTL_B77W_UAE", "B77W"},
	} {
		if got := gateHelper(svc, g, at(tc.stand), tc.model); got != jetway.HelperTitle(tc.want, false) {
			t.Errorf("%s at %s: %q, want the %s helper", tc.model, tc.stand, got, tc.want)
		}
	}
}
