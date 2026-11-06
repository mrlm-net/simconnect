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
		if got := gateHelper(svc, g, at(tc.stand), tc.model, nil); got != jetway.HelperTitle(tc.want, false) {
			t.Errorf("%s at %s: %q, want the %s helper", tc.model, tc.stand, got, tc.want)
		}
	}
	// With the wider helper's door variants installed: the aircraft's own
	// door, else an A320's; a widebody keeps its own helper.
	installed := map[string]bool{jetway.HelperDoorTitle("B788", "B738"): true, jetway.HelperDoorTitle("B763", "A320"): true}
	has := func(t string) bool { return installed[t] }
	for _, tc := range []struct{ stand, model, want string }{
		{"B14", "FSLTL_B738_RYR", jetway.HelperDoorTitle("B788", "B738")},
		{"C24", "FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery", jetway.HelperDoorTitle("B763", "A320")},
		{"B14", "FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery", jetway.HelperTitle("B788", false)},
		{"B14", "FSLTL_B77W_UAE", jetway.HelperTitle("B77W", false)},
	} {
		if got := gateHelper(svc, g, at(tc.stand), tc.model, has); got != tc.want {
			t.Errorf("%s at %s: %q, want %q", tc.model, tc.stand, got, tc.want)
		}
	}
}

// A widebody whose installed span would fit a smaller gate takes the wider
// helper with that helper's own door, not an A320's door variant: on LKPR
// B14 (35 m) the "B748 D-A320" helper, the A320's door figures on the
// 747's datum (nose gear 2.79 m ahead of it), stood QTR337's B789 with its
// reference 29 m into the circle, nose gear 17 m past it (live,
// 2026-11-05). Its nose gear stays inside the circle; an A320 keeps the
// door variant.
func TestGateHelperWidebodyNoDoorVariant(t *testing.T) {
	g, err := airport.BuildGraph(lkprLayout(t))
	if err != nil {
		t.Fatal(err)
	}
	b14, err := g.Layout.ParkingIndex("B14")
	if err != nil {
		t.Fatal(err)
	}
	// The installed figures (feet), as the map reads them.
	setDoors(map[string]addons.ModelDoor{
		"FSLTL_B789_GE_QTR":                {Type: "B789", First: addons.InteractivePoint{Long: 31.43}, NoseGearFt: 75.125, HasNoseGear: true},
		"FSLTL_A320_LYX":                   {Type: "A320", First: addons.InteractivePoint{Long: 37.65}, NoseGearFt: 37.319, HasNoseGear: true},
		"MyCrew Jetway Helper B748":        {Type: "B748", First: addons.InteractivePoint{Long: -40.2}, NoseGearFt: 9.15, HasNoseGear: true},
		"MyCrew Jetway Helper B748 D-A320": {Type: "B748", First: addons.InteractivePoint{Long: 37.65}, NoseGearFt: 9.15, HasNoseGear: true},
	}, map[string]addons.ModelDoor{"B789": {WingSpanFt: 176.8}, "A320": {WingSpanFt: 117.4}, "B748": {WingSpanFt: 203.1}})
	defer setDoors(nil, nil)
	svc := jetway.NewHelpers(nil, jetway.HelperOptions{Types: []string{"A320", "B748", "B789"}})
	has := func(t string) bool { return t == jetway.HelperDoorTitle("B748", "A320") }
	r := g.Layout.Parking[b14].Radius
	for _, tc := range []struct{ model, want string }{
		{"FSLTL_B789_GE_QTR", jetway.HelperTitle("B748", false)},
		{"FSLTL_A320_LYX", jetway.HelperDoorTitle("B748", "A320")},
	} {
		h := gateHelper(svc, g, b14, tc.model, has)
		if h != tc.want {
			t.Errorf("%s at B14: %q, want %q", tc.model, h, tc.want)
		}
		if tc.model != "FSLTL_B789_GE_QTR" {
			continue
		}
		nose, _ := noseGearM(tc.model)
		off := gateNoseOffset(tc.model, h, SimNoseGearInsetM)
		if past := nose - off; past > 0 {
			t.Errorf("%s at B14 by %q: reference %.1f m into the %.0f m circle, nose gear %.1f m past it", tc.model, h, r-off, r, past)
		}
	}
}
