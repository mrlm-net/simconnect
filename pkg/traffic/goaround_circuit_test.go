package traffic

import (
	"math"
	"testing"
	"time"
)

// TestGoAroundClearOfTheCircuit: an IFR go-around at LKPR 24 (3715 m)
// climbs straight ahead past the VFR circuit's crosswind turn, with
// GoAroundCircuitClearNM to spare, before it turns (the user: the
// go-around's procedure clear of the VFR pattern).
func TestGoAroundClearOfTheCircuit(t *testing.T) {
	g := lkprGraph(t)
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "A320", Tail: "CSA9",
		InjectApproach: true, RollThroughChance: -1}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	mon := DefaultArrivalRequestBase + arrReqMonitor
	p := ctl.Plan()
	for i := 0; i < 60*600 && ctl.approach != nil && ctl.approach.Pose().HeightFt > 300; i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 77, p.End.Threshold, 0, 0, 0, false))
	}
	if err := ctl.GoAround(); err != nil {
		t.Fatal(err)
	}
	r, _, _ := g.Layout.RunwayEnd("24")
	want := r.Length/1852 + CircuitUpwindNM + GoAroundCircuitClearNM
	corners := ctl.ProcedureCorners()
	if len(corners) == 0 {
		t.Fatal("no go-around corners")
	}
	if along := alongHeading(p.End.Threshold, p.End.Heading, corners[0]) / 1852; math.Abs(along-want) > 0.1 {
		t.Errorf("turns %.1f NM past the threshold, want %.1f (past the circuit's crosswind)", along, want)
	}
}
