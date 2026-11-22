package traffic

import (
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// TestArrivalAttitudeOwnDefinition (#1636): the attitude the hand-over
// blends from is no part of the monitor, which is read every frame: that
// keeps exactly its SimVars, and the attitude is a definition by itself,
// read ONCE.
func TestArrivalAttitudeOwnDefinition(t *testing.T) {
	g := lkprGraph(t)
	ec := &eventClient{}
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
	c22, err := g.Layout.ParkingIndex("C22")
	if err != nil {
		t.Fatal(err)
	}
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "A320", InjectApproach: true}); err != nil {
		t.Fatal(err)
	}
	ctl.now = time.Now
	monitor := []string{
		"PLANE LATITUDE", "PLANE LONGITUDE", "PLANE ALT ABOVE GROUND", "PLANE HEADING DEGREES TRUE", "GROUND VELOCITY", "SIM ON GROUND",
		"VERTICAL SPEED", "LIGHT LANDING", "LIGHT TAXI", "LIGHT STROBE", "LIGHT BEACON", "LIGHT NAV",
		"LIGHT LOGO", "LIGHT WING", "PLANE ALTITUDE", "SIMULATION TIME",
	}
	if got := ec.defs[DefaultArrivalDefinitionBase+arrDefMonitor]; !slices.Equal(got, monitor) {
		t.Errorf("the monitor, read every frame, is now %v, want %v", got, monitor)
	}
	if got, want := ec.defs[DefaultArrivalDefinitionBase+arrDefAttitude], []string{"PLANE PITCH DEGREES", "PLANE BANK DEGREES"}; !slices.Equal(got, want) {
		t.Errorf("the attitude definition is %v, want %v", got, want)
	}
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	n := len(ec.periods)
	ctl.askAttitude()
	if got := ec.periods[n:]; !slices.Equal(got, []types.SIMCONNECT_PERIOD{types.SIMCONNECT_PERIOD_ONCE}) {
		t.Errorf("the attitude asked at %v, want ONCE", got)
	}
}
