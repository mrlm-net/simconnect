package traffic

import (
	"errors"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

const (
	landA320 = "FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery"
	landDHC6 = "FSLTL_DHC6_WIA-Winair"
)

// TestRequiredLandingRun (#1718): the landing run an injected landing
// needs. An A320's is more than 1300 m and less than 1500 m (its
// short-field landing, TestLandingFitsRequiredRun); a DHC-6's fits 1300 m.
// An end fits by its landing distance available, a declared one first.
func TestRequiredLandingRun(t *testing.T) {
	if n := RequiredLandingRun(landA320); n <= 1300 || n >= 1500 {
		t.Errorf("A320 landing run %.0f m, want more than 1300 and less than 1500", n)
	}
	if n := RequiredLandingRun(landDHC6); n > 1300 {
		t.Errorf("DHC-6 landing run %.0f m, want 1300 at most", n)
	}
	r := airport.Runway{Length: 2400}
	declared := airport.RunwayEnd{Name: "27", LDAM: 1300}
	full := airport.RunwayEnd{Name: "27"}
	switch {
	case RunwayEndLandingLongEnough(landA320, r, declared):
		t.Error("A320 fits a 1300 m landing distance")
	case !RunwayEndLandingLongEnough(landA320, r, full):
		t.Error("A320 does not fit 2400 m")
	case !RunwayEndLandingLongEnough(landDHC6, r, declared):
		t.Error("DHC-6 does not fit 1300 m")
	case !RunwayEndLandingLongEnough("", r, declared):
		t.Error("an empty model does not fit")
	}
}

// TestArrivalTooShortToLand (#1718): LKPR's 24 with a landing distance of
// 1300 m declared. An A320 arrival is not started onto it
// (ErrRunwayTooShortToLand), a DHC-6 is; nor is an A320 flying in to 06
// moved onto it by a runway change.
func TestArrivalTooShortToLand(t *testing.T) {
	g := declaredGraph(t, map[string]func(*airport.RunwayEnd){"24": func(e *airport.RunwayEnd) { e.LDAM = 1300 }})
	procs := lkprProcedures(t)
	route24, err := procs.Arrival("24", "LOMKI")
	if err != nil {
		t.Fatal(err)
	}
	c22, _ := g.Layout.ParkingIndex("C22")
	start := func(model, rwy string, route []airport.NavPoint) (*ArrivalController, error) {
		ec := &eventClient{}
		ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
		err := ctl.Start(ArrivalRequest{Graph: g, Runway: rwy, Parking: c22, Model: model, Tail: "CSA1", InjectApproach: true, Procedure: route})
		if err == nil {
			go func() {
				for range ctl.Events() {
				}
			}()
		}
		return ctl, err
	}
	if _, err := start(landA320, "24", route24); !errors.Is(err, ErrRunwayTooShortToLand) {
		t.Errorf("A320 onto 24 (1300 m): %v, want ErrRunwayTooShortToLand", err)
	}
	if _, err := start(landDHC6, "24", route24); err != nil {
		t.Errorf("DHC-6 onto 24 (1300 m): %v", err)
	}
	route06, err := procs.Arrival("06", "LOMKI")
	if err != nil {
		t.Fatal(err)
	}
	ctl, err := start(landA320, "06", route06)
	if err != nil {
		t.Fatalf("A320 onto 06: %v", err)
	}
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, route06[0].Position, 9000, 0, 250, false))
	if err := ctl.ChangeRunway("24", route24, nil); !errors.Is(err, ErrRunwayTooShortToLand) {
		t.Errorf("A320 moved from 06 to 24 (1300 m): %v, want ErrRunwayTooShortToLand", err)
	}
}
