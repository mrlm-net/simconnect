package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestCircuitExtendDownwind: a VFR arrival in the LKPR 24 circuit asked to
// lose a minute (behind an IFR arrival, #569) flies a longer downwind: its
// base turn further out, no speed change (a light aircraft flies its
// circuit speed), the route longer by about what it was asked.
func TestCircuitExtendDownwind(t *testing.T) {
	g := lkprGraph(t)
	p := ProfileFor("C172")
	c, err := NewCircuit(g.Layout, "24", CircuitConfig{}, p)
	if err != nil {
		t.Fatal(err)
	}
	ec := &eventClient{}
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
	st, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: st, Model: "Asobo PassiveAircraft C172", Tail: "OKABC",
		InjectApproach: true, Circuit: &c}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 78))
	entry, _ := c.JoinDownwind()
	ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 78, entry.Position, entry.AltFt, 0, CircuitKts(p), false))
	base, _ := c.Point(LegBase)
	thr, _ := c.Point(LegRunway)
	before := pathNM(ctl.ProcedureRoute())
	a, err := ctl.AbsorbDelay(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	after := pathNM(ctl.ProcedureRoute())
	if a.ExtraNM <= 0 || a.SpeedKts != 0 {
		t.Fatalf("absorption %+v: want a longer downwind, no speed", a)
	}
	// The new base turn: further from the threshold than the old one.
	var far float64
	for _, q := range ctl.ProcedureRoute() {
		if d := localDist(q, thr.Position); d > far && localDist(q, base.Position) > 100 {
			far = d
		}
	}
	if far <= localDist(base.Position, thr.Position) {
		t.Errorf("base turn not moved out: farthest point %.0f m, base %.0f m", far, localDist(base.Position, thr.Position))
	}
	t.Logf("route %.1f → %.1f NM, stretch %.1f NM", before, after, a.ExtraNM)
	_ = airport.LatLon{}
}

// TestCircuitOrbit: asked to lose 4 minutes, a C172 in the circuit has its
// downwind extended at most CircuitMaxExtendNM each way and the rest left
// over (no dog-leg); an orbit then takes about two minutes, flown from
// where it is before the rest of its circuit.
func TestCircuitOrbit(t *testing.T) {
	g := lkprGraph(t)
	p := ProfileFor("C172")
	c, _ := NewCircuit(g.Layout, "24", CircuitConfig{}, p)
	ec := &eventClient{}
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
	st, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: st, Model: "Asobo PassiveAircraft C172", Tail: "OKORB",
		InjectApproach: true, Circuit: &c}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 80))
	entry, _ := c.JoinDownwind()
	ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 80, entry.Position, entry.AltFt, 0, CircuitKts(p), false))
	a, err := ctl.AbsorbDelay(4 * time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if a.ExtraNM > 2*CircuitMaxExtendNM+0.1 || a.Left < 30*time.Second {
		t.Errorf("absorption %+v: want at most %.0f NM more and the rest left", a, 2*CircuitMaxExtendNM)
	}
	n := len(ctl.proc.Waypoints)
	d, err := ctl.Orbit()
	if err != nil {
		t.Fatal(err)
	}
	if d < 90*time.Second || d > 3*time.Minute || len(ctl.proc.Waypoints) <= n {
		t.Errorf("orbit %v, waypoints %d → %d", d, n, len(ctl.proc.Waypoints))
	}
	t.Logf("absorbed %+v, orbit %v", a, d.Round(time.Second))
}
