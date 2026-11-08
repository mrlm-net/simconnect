package traffic

import (
	"math"
	"testing"
)

// The tug sits on the nose wheel of every type: on the nose gear where the
// model has it (FSLTL flight_model.cfg contact points, from the reference
// point), not where the motion profile's wheelbase and 1 m reference point
// put it (KLM B738 at LKPR 22A: 1.6 m ahead, the wheel out of the cradle;
// B772 6.1 m behind, MD11 7.9 m). The aircraft's own profile is unchanged.
func TestTugOnTheNoseWheel(t *testing.T) {
	cases := []struct {
		title string
		nose  float64 // metres ahead of the reference point
	}{
		// Narrowbodies.
		{"FSLTL_A320_VCL", 11.37},
		{"FSLTL_A21N_WZZ", 15.49},
		{"FSLTL_B738_KLM", 13.02},
		{"AIB_B738_BAW-British Airways", 13.02},
		{"FSLTL_B737_KLM", 9.94},
		{"FSLTL_B739_UAL", 14.41},
		{"FSLTL_B733_ZZZZ", 9.99},
		// Regionals.
		{"FSLTL_E190_KLM", 11.80},
		{"FSLTL_E170_LOT", 9.01},
		{"FSLTL_CRJ7_ZZZZ", 12.82},
		{"FSLTL_AT76_ZZZZ", 10.72},
		{"FSLTL_DH8D_ZZZZ", 14.63},
		{"FSLTL_BCS3_SWR", 13.62},
		// Widebodies.
		{"FSLTL_A332_ZZZZ", 21.97},
		{"FSLTL_A332F_MAS", 21.97}, // no cfg of its own: the A332's
		{"FSLTL_A333_DLH", 25.25},
		{"FSLTL_A359_AFR", 26.91},
		{"FSLTL_A388_UAE", 31.32},
		{"FSLTL_B744_BAW", 24.09},
		{"FSLTL_B748_DLH", 28.24}, // datum 83.5 ft ahead of the reference point
		{"FSLTL_B763_UPS", 17.90},
		{"FSLTL_B772_BAW", 31.01},
		{"FSLTL_B77W_UAE", 31.01},
		{"Asobo PassiveAircraft B777-300ER :: B777_300ER_KLM", 31.01},
		{"FSLTL_B788_BAW", 19.68},
		{"FSLTL_B789_QFA", 22.90},
		{"FSLTL_MD11F_FDX", 19.54},
		// GA and turboprops.
		{"FSLTL_Asobo_208B_FDX", 3.26}, // datum 13 ft
		{"FSLTL_Asobo_C172SP", 1.34},
		{"FSLTL_Asobo_TBM930", 2.65},
	}
	for _, c := range cases {
		prof := MotionProfileFor(c.title)
		tug := NewSimObjectTug(nil, nil, DefaultTugTitle, 9001, prof)
		fitTug(tug, c.title)
		got := localDist(lkpr, tug.at(GroundPose{Position: lkpr, Heading: 245}).Position)
		if want := c.nose + TugAheadMeters; math.Abs(got-want) > 0.05 {
			t.Errorf("%s: tug %.2f m ahead of the reference point, want %.2f (nose gear %.2f + %.1f)", c.title, got, want, c.nose, TugAheadMeters)
		}
		if p := MotionProfileFor(c.title); p != prof {
			t.Errorf("%s: the aircraft's own profile changed", c.title)
		}
	}
}

// Through a departure too: TaxiController.Start fits its tug, and the
// aircraft keeps its own profile.
func TestTaxiControllerFitsTheTug(t *testing.T) {
	g := airportGraph(t, "EHAM")
	i, err := g.Layout.ParkingIndex("U26")
	if err != nil {
		t.Fatal(err)
	}
	ec := &eventClient{}
	title := "FSLTL_B738_KLM"
	tug := NewSimObjectTug(ec, NewInjector(ec), DefaultTugTitle, 9001, MotionProfileFor(title))
	ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
	if err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: "09", Model: title, Tail: "T1", RollingTakeoffChance: -1, Tug: tug}); err != nil {
		t.Fatal(err)
	}
	if got, want := localDist(lkpr, tug.at(GroundPose{Position: lkpr, Heading: 245}).Position), 13.02+TugAheadMeters; math.Abs(got-want) > 0.05 {
		t.Errorf("tug %.2f m ahead of the reference point, want %.2f", got, want)
	}
	if p := ctl.profile(); p.RefAheadMeters != DefaultMotionProfile().RefAheadMeters {
		t.Errorf("the aircraft's own reference point moved to %.1f m", p.RefAheadMeters)
	}
}
