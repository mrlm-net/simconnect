package world

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/addons"
	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/jetway"
)

// A widebody sent to a wider helper takes that helper's variant with its
// own type's door when installed, and the door variants (jetway.copyDoor:
// as far behind the nose gear as on their type) stop each type where the
// simulator parks its own: live, LKPR B14, an Emirates 777 by the B748's
// own door had the jetway 15.5 m aft of L1; an A320 by "B748 D-A320" stood
// 8.5 m further in than an A320 parks. The FSLTL models' figures (feet).
func TestGateHelperWidebodyOwnDoorVariant(t *testing.T) {
	g, err := airport.BuildGraph(lkprLayout(t))
	if err != nil {
		t.Fatal(err)
	}
	b14, err := g.Layout.ParkingIndex("B14")
	if err != nil {
		t.Fatal(err)
	}
	setDoors(map[string]addons.ModelDoor{
		"FSLTL_B77W_UAE":                   {Type: "B77W", First: addons.InteractivePoint{Long: 98.6624, Lat: -9.512}, NoseGearFt: 101.725, HasNoseGear: true},
		"FSLTL_A320_LYX":                   {Type: "A320", First: addons.InteractivePoint{Long: 37.654, Lat: -6.137}, NoseGearFt: 37.319, HasNoseGear: true},
		"MyCrew Jetway Helper B748":        {Type: "B748", First: addons.InteractivePoint{Long: -40.17, Lat: -10.65}, NoseGearFt: 9.163, HasNoseGear: true, DatumFt: 83.5},
		"MyCrew Jetway Helper B748 D-B77W": {Type: "B748", First: addons.InteractivePoint{Long: 6.100, Lat: -9.512}, NoseGearFt: 9.163, HasNoseGear: true, DatumFt: 83.5},
		"MyCrew Jetway Helper B748 D-A320": {Type: "B748", First: addons.InteractivePoint{Long: 9.498, Lat: -6.137}, NoseGearFt: 9.163, HasNoseGear: true, DatumFt: 83.5},
	}, map[string]addons.ModelDoor{"B77W": {WingSpanFt: 176.8}, "A320": {WingSpanFt: 117.4}, "B748": {WingSpanFt: 224.4}})
	defer setDoors(nil, nil)
	defer standPastWarned.Clear()
	svc := jetway.NewHelpers(nil, jetway.HelperOptions{Types: []string{"A320", "B748", "B77W"}})
	installed := map[string]bool{jetway.HelperDoorTitle("B748", "B77W"): true, jetway.HelperDoorTitle("B748", "A320"): true}
	has := func(t string) bool { return installed[t] }
	r := g.Layout.Parking[b14].Radius
	for _, tc := range []struct {
		model, want string
		past        bool
	}{
		// 101.725 ft to the nose gear: 1.36 m past a 35 m circle's point,
		// as the simulator parks its own 777 there (its helper 1.36 m
		// ahead of the point, live).
		{"FSLTL_B77W_UAE", jetway.HelperDoorTitle("B748", "B77W"), r < 36.36},
		{"FSLTL_A320_LYX", jetway.HelperDoorTitle("B748", "A320"), false},
	} {
		h := gateHelper(svc, g, b14, tc.model, has)
		if h != tc.want {
			t.Errorf("%s at B14: %q, want %q", tc.model, h, tc.want)
			continue
		}
		nose, _ := noseGearM(tc.model)
		off := gateNoseOffset(tc.model, h, SimNoseGearInsetM)
		if want := nose + SimNoseGearInsetM; math.Abs(off-want) > 0.01 {
			t.Errorf("%s by %q: offset %.2f m, its own type's %.2f", tc.model, h, off, want)
		}
		if _, right, _ := jetwayDoorOffset(tc.model, h); math.Abs(right) > 0.01 {
			t.Errorf("%s by %q: door %.2f m to the side", tc.model, h, right)
		}
		if got := (*controlCenter)(nil).standPastCentre(g, b14, tc.model, off); got != tc.past {
			t.Errorf("%s by %q: offset %.2f on %.0f m, past the point %v", tc.model, h, off, r, got)
		}
	}
	// By the B748's own door the 777 would stand 15.5 m behind the point.
	off := gateNoseOffset("FSLTL_B77W_UAE", jetway.HelperTitle("B748", false), SimNoseGearInsetM)
	if !(*controlCenter)(nil).standPastCentre(g, b14, "FSLTL_B77W_UAE", off) || off-r < 14 {
		t.Errorf("by the B748's own door: offset %.2f on %.0f m", off, r)
	}
}

// With the package written before the door variants moved (the A320's door
// copied as written, 37.654 ft, and no "B748 D-B77W"), every stop is as
// before: the offsets come from the installed helper's door, so nothing
// moves until the helpers are written again (LKPR B14: the A320 26.86 m
// ahead of the point, the 777 by the B748's own door).
func TestGateHelperOldPackageUnchanged(t *testing.T) {
	g, err := airport.BuildGraph(lkprLayout(t))
	if err != nil {
		t.Fatal(err)
	}
	b14, err := g.Layout.ParkingIndex("B14")
	if err != nil {
		t.Fatal(err)
	}
	setDoors(map[string]addons.ModelDoor{
		"FSLTL_B77W_UAE":                   {Type: "B77W", First: addons.InteractivePoint{Long: 98.6624, Lat: -9.512}, NoseGearFt: 101.725, HasNoseGear: true},
		"FSLTL_A320_LYX":                   {Type: "A320", First: addons.InteractivePoint{Long: 37.654, Lat: -6.137}, NoseGearFt: 37.319, HasNoseGear: true},
		"MyCrew Jetway Helper B748":        {Type: "B748", First: addons.InteractivePoint{Long: -40.16792, Lat: -10.651472}, NoseGearFt: 9.163, HasNoseGear: true, DatumFt: 83.5},
		"MyCrew Jetway Helper B748 D-A320": {Type: "B748", First: addons.InteractivePoint{Long: 37.654, Lat: -6.137}, NoseGearFt: 9.163, HasNoseGear: true, DatumFt: 83.5},
	}, map[string]addons.ModelDoor{"B77W": {WingSpanFt: 176.8}, "A320": {WingSpanFt: 117.4}, "B748": {WingSpanFt: 224.4}})
	defer setDoors(nil, nil)
	svc := jetway.NewHelpers(nil, jetway.HelperOptions{Types: []string{"A320", "B748", "B77W"}})
	has := func(t string) bool { return t == jetway.HelperDoorTitle("B748", "A320") }
	r := g.Layout.Parking[b14].Radius
	for _, tc := range []struct {
		model, want string
		ahead       float64 // of the parking point
	}{
		{"FSLTL_A320_LYX", jetway.HelperDoorTitle("B748", "A320"), 26.86},
		{"FSLTL_B77W_UAE", jetway.HelperTitle("B748", false), -15.46},
	} {
		h := gateHelper(svc, g, b14, tc.model, has)
		off := gateNoseOffset(tc.model, h, SimNoseGearInsetM)
		if h != tc.want || math.Abs(r-off-tc.ahead) > 0.05 {
			t.Errorf("%s at B14: %q, %.2f m ahead of the point; want %q, %.2f", tc.model, h, r-off, tc.want, tc.ahead)
		}
	}
}

// A widebody with no helper of its own type (its helper the fallback's)
// takes no door variant of that type's: an A320's door on a 787 stood its
// nose gear 20 m past the circle (live, LKPR B14, QTR337).
func TestGateHelperWidebodyFallbackNoVariant(t *testing.T) {
	g, err := airport.BuildGraph(lkprLayout(t))
	if err != nil {
		t.Fatal(err)
	}
	b14, err := g.Layout.ParkingIndex("B14")
	if err != nil {
		t.Fatal(err)
	}
	setDoors(nil, map[string]addons.ModelDoor{"A320": {WingSpanFt: 117.4}, "B748": {WingSpanFt: 224.4}})
	defer setDoors(nil, nil)
	svc := jetway.NewHelpers(nil, jetway.HelperOptions{Types: []string{"A320", "B748"}, FallbackType: "A320"})
	has := func(t string) bool { return t == jetway.HelperDoorTitle("B748", "A320") }
	if h := gateHelper(svc, g, b14, "FSLTL_B789_GE_QTR", has); h == jetway.HelperDoorTitle("B748", "A320") {
		t.Errorf("B789 by %q", h)
	}
}
