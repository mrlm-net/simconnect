package world

import (
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// The backstop and the arrival's plan (#1654): it never changes a player's
// (#1096), and it follows one #1640 plans again.

// gatedOutcome is what is the player's of an arrival, as it ends waiting
// for their taxi clearance.
type gatedOutcome struct {
	exit    airport.NodeID
	route   []airport.NodeID
	stop    airport.LatLon
	stand   int
	label   string
	waiting bool
	taxied  bool
	stopped bool
	nearest float64
}

// flyGated flies the arrival under the player's clearances to its wait for
// taxi: with no crew, one that can move off its way, or one that can't.
func flyGated(t *testing.T, crew string) gatedOutcome {
	t.Helper()
	b := newBackstopBed(t)
	b.holdForTaxi = true
	it := b.cc.items[1]
	it.gates.Store(true)
	it.view.Manual = true
	it.stand = len(b.g.Layout.Parking) - 1
	it.view.Stand = b.g.Layout.Parking[it.stand].Label()
	switch crew {
	case "none":
		b.cc.rescue.by = map[string]*rescueCrew{}
		b.prio = map[string]bool{}
	case "stays":
		for _, tr := range b.crew.trucks {
			tr.Layout = nil
		}
	}
	b.plan()
	if crew == "moves" {
		// Both off it before it lands.
		b.wait(10*time.Minute, nil)
		for i, tr := range b.crew.trucks {
			if b.crew.reported[tr] != 0 {
				t.Fatalf("vehicle %d has not moved off: nothing checked", i)
			}
		}
	}
	var out gatedOutcome
	reported := false
	b.until = func(s backstopSeen) bool {
		st := b.ctl.State()
		out.taxied = out.taxied || st == traffic.ArrivalTaxiing
		out.waiting = st == traffic.ArrivalAwaitingTaxi && s.still >= 90*time.Second
		return out.waiting || out.taxied
	}
	res := b.fly(func(backstopSeen) []traffic.GroundPose {
		// The crew sees the plan it has now. planArrival has only that and
		// its state: the backstop calling anything else of it panics.
		b.arr.plan = b.ctl.Plan()
		if crew == "none" {
			return nil
		}
		v := b.standing()
		reported = reported || len(v) > 0
		return v
	})
	if crew == "stays" && !reported {
		t.Fatal("none reported standing: nothing checked")
	}
	p := b.ctl.Plan()
	out.exit, out.route, out.stop = p.Exit.Node, slices.Clone(p.Route.Nodes), p.Stop
	out.stand, out.label = it.stand, it.view.Stand
	out.stopped, out.nearest = res.stopped, res.nearest
	return out
}

// TestRescueBackstopGatedArrival: an arrival under a player's clearances
// brakes for a vehicle as for traffic ahead (#1640), and has the exit,
// route, stand and clearance it has with no crew out.
func TestRescueBackstopGatedArrival(t *testing.T) {
	want := flyGated(t, "none")
	if !want.waiting || want.taxied {
		t.Fatalf("no crew: waiting %v, taxied %v: nothing to compare", want.waiting, want.taxied)
	}
	for _, crew := range []string{"moves", "stays"} {
		t.Run(crew, func(t *testing.T) {
			got := flyGated(t, crew)
			if got.exit != want.exit {
				t.Errorf("exit node %d, want the player's %d", got.exit, want.exit)
			}
			if !slices.Equal(got.route, want.route) {
				t.Errorf("route %v, want the player's %v", got.route, want.route)
			}
			if got.stand != want.stand || got.label != want.label || got.stop != want.stop {
				t.Errorf("stand %d %q stop %v, want the player's %d %q %v", got.stand, got.label, got.stop, want.stand, want.label, want.stop)
			}
			if got.taxied || !got.waiting {
				t.Errorf("taxied %v, waiting %v: want it waiting for the player's taxi clearance", got.taxied, got.waiting)
			}
			if crew != "stays" {
				return
			}
			if !got.stopped {
				t.Error("did not brake to a stop for a reported vehicle")
			}
			if got.nearest < backstopClearance-0.1 {
				t.Errorf("came within %.1f m of a reported vehicle, want %.1f m", got.nearest, backstopClearance)
			}
		})
	}
}

// Where rescueBacktrackField has its exits (metres from 09's threshold)
// and their holding positions.
const (
	backtrackExitNear = 250.0
	backtrackExitFar  = 520.0
	backtrackHoldOff  = 90.0
	backtrackHalf     = 950.0
)

// rescueBacktrackField is a runway 09/27 of 1900 m with two exits, both
// too near 09's threshold to vacate by landing on it: it rolls to the end
// and backtracks, to the far one first. A parallel taxiway A 200 m south, a
// gate at its west end (point west), a fire station off the far exit and
// standby given at both exits' holding positions, a vehicle at each.
func rescueBacktrackField(t *testing.T) (l *airport.Layout, west int) {
	t.Helper()
	var pts []airport.RawTaxiPoint
	var paths []airport.RawTaxiPath
	pt := func(x, z float32, typ int32) int32 {
		pts = append(pts, airport.RawTaxiPoint{Type: typ, BiasX: x, BiasZ: z})
		return int32(len(pts) - 1)
	}
	link := func(a, b, typ int32, name uint32, width float32, rwy int32) {
		paths = append(paths, airport.RawTaxiPath{Type: typ, Width: width, RunwayNumber: rwy, Start: a, End: b, NameIndex: name})
	}
	const half = float32(backtrackHalf)
	near, far := float32(backtrackExitNear)-half, float32(backtrackExitFar)-half
	rw, rn, rf, re := pt(-half, 0, 1), pt(near, 0, 1), pt(far, 0, 1), pt(half, 0, 1)
	link(rw, rn, 2, 0, 45, 9)
	link(rn, rf, 2, 0, 45, 9)
	link(rf, re, 2, 0, 45, 9)
	hn, hf := pt(near, -float32(backtrackHoldOff), 2), pt(far, -float32(backtrackHoldOff), 2)
	link(rn, hn, 1, 2, 23, 0)
	link(rf, hf, 1, 3, 23, 0)
	an, af := pt(near, -200, 1), pt(far, -200, 1)
	link(hn, an, 1, 2, 23, 0)
	link(hf, af, 1, 3, 23, 0)
	aw, ae := pt(-half, -200, 1), pt(half, -200, 1)
	link(aw, an, 1, 1, 23, 0)
	link(an, af, 1, 1, 23, 0)
	link(af, ae, 1, 1, 23, 0)
	s, lead := pt(far, -400, 1), pt(far, -520, 1)
	link(af, s, 1, 4, 15, 0)
	link(s, lead, 1, 4, 15, 0)
	link(lead, 0, 3, 0, 15, 0)
	raw := airport.RawAirport{ICAO: "XONE", Name: "Two exits back", Latitude: 50.12, Longitude: 13.81, Altitude: 300,
		Runways:    []airport.RawRunway{{Latitude: 50.12, Longitude: 13.81, Altitude: 300, Heading: 90, Length: 2 * half, Width: 45, PrimaryNumber: 9, SecondaryNumber: 27}},
		Parking:    []airport.RawParking{{Name: 14, Number: 1, Type: int32(types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_VEHICLE), Radius: 10, BiasX: far, BiasZ: -600, RescueVehicles: 2}},
		TaxiPoints: pts, TaxiPaths: paths, TaxiNames: []string{"", "A", "B", "C", "S"}}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	_, end, ok := l.RunwayEnd("09")
	if !ok {
		t.Fatal("no runway 09")
	}
	for _, along := range []float64{backtrackExitFar, backtrackExitNear} {
		at := backstopAhead(backstopAhead(end.Threshold, end.Heading, along), end.Heading+90, backtrackHoldOff)
		l.RescueStandby = append(l.RescueStandby, airport.RescueStandbyPoint{Runway: "09", Position: at, Heading: end.Heading - 90})
	}
	return l, int(aw)
}

// replanArrival is the bed's arrival as the World re-plans its way off
// (#1640): the real controller's plan and search, before touchdown.
type replanArrival struct {
	*planArrival
	ctl *traffic.ArrivalController
}

func (a replanArrival) Plan() *traffic.ArrivalPlan { return a.ctl.Plan() }

func (a replanArrival) WayOffClear(occ []airport.Occupied) bool { return a.ctl.WayOffClear(occ) }

func (a replanArrival) PlanWayOff(parking int, occ []airport.Occupied) func() bool {
	return a.ctl.PlanWayOff(parking, occ)
}

// TestRescueBackstopReplannedWayOff: a departure holds short on the
// arrival's exit back, so #1640 plans it off by the other, where a vehicle
// stands at standby that was clear of the way before. That one moves off
// the new way, or, with no way to go, is reported in the frame after the
// change and the arrival, flown here, stops short of it.
func TestRescueBackstopReplannedWayOff(t *testing.T) {
	for _, canMove := range []bool{true, false} {
		name := "moves off"
		if !canMove {
			name = "stops short"
		}
		t.Run(name, func(t *testing.T) {
			l, west := rescueBacktrackField(t)
			b := newBackstopBedOn(t, l, west)
			if !canMove {
				for _, tr := range b.crew.trucks {
					tr.Layout = nil
				}
			}
			var nearHold, farHold airport.LatLon
			_, end, _ := l.RunwayEnd("09")
			at := func(along float64) airport.LatLon {
				return backstopAhead(backstopAhead(end.Threshold, end.Heading, along), end.Heading+90, backtrackHoldOff)
			}
			nearHold, farHold = at(backtrackExitNear), at(backtrackExitFar)
			var stands *traffic.SimObjectRescue
			for _, tr := range b.crew.trucks {
				if p, _, ok := tr.Track(); ok && backstopNear(p.Position, []airport.LatLon{nearHold, nearHold}) < 40 {
					stands = tr
				}
			}
			if stands == nil {
				t.Fatal("no vehicle at standby by the near exit: nothing checked")
			}

			b.plan()
			before := b.ctl.Plan()
			if !before.EndTurn {
				t.Fatal("it doesn't roll to the end: nothing checked")
			}
			b.wait(5*time.Minute, nil) // the backstop's answer to that plan
			if b.crew.reported[stands] != 0 {
				t.Fatal("the vehicle by the near exit is on the first way already: nothing checked")
			}

			// #1640's own search, put in place as the World does.
			stop := make(chan struct{})
			t.Cleanup(func() { close(stop) })
			b.cc.cmds = make(chan func())
			go func() {
				for {
					select {
					case f := <-b.cc.cmds:
						f()
					case <-stop:
						return
					}
				}
			}()
			it := b.cc.items[1]
			arr := replanArrival{b.arr, b.ctl}
			it.arr, it.cc, it.graph, it.removed = arr, b.cc, b.g, make(chan struct{})
			it.stand = len(l.Parking) - 1
			it.view.Stand = l.Parking[it.stand].Label()
			dep := &controlled{ID: 2, Kind: "departure", Tail: "TST2", ICAO: "XONE", cc: b.cc, graph: b.g,
				dep: &staleDeparture{state: traffic.TaxiHoldingShort}, removed: make(chan struct{})}
			dep.view.Position = farHold
			b.cc.items[2] = dep
			occ := wayOffTaken(it, b.cc.snapshotItems())
			if len(occ) != 1 || arr.WayOffClear(occ) {
				t.Fatal("the departure is not on the way off: nothing checked")
			}
			it.wayOffPlanning.Store(true)
			b.cc.planWayOff(it, arr, occ)
			after := b.ctl.Plan()
			if after == before || after.Exit.Node == before.Exit.Node {
				t.Fatal("not planned off by the other exit: nothing checked")
			}
			var way []airport.LatLon
			for _, n := range after.Exit.Path {
				way = append(way, b.g.Nodes[n].Position)
			}
			way = append(way, after.Route.Points[min(len(after.Exit.Path), len(after.Route.Points)):]...)
			keep := b.prof.SpanMeters/2 + traffic.VehicleClearMeters
			if p, _, ok := stands.Track(); !ok || backstopNear(p.Position, way) > keep {
				t.Fatal("no vehicle stands on the new way: nothing checked")
			}

			b.now = b.now.Add(100 * time.Millisecond)
			b.step(false)
			if b.crew.reported[stands] == 0 {
				t.Error("not reported in the frame after the plan changed")
			}

			if canMove {
				b.wait(10*time.Minute, nil)
				for i, tr := range b.crew.trucks {
					p, _, ok := tr.Track()
					if !ok {
						t.Fatalf("vehicle %d gone", i)
					}
					if near := backstopNear(p.Position, way); near <= keep || b.crew.reported[tr] != 0 {
						t.Errorf("vehicle %d: %.1f m from the new way (keep %.1f m), reported %v", i, near, keep, b.crew.reported[tr] != 0)
					}
				}
			} else {
				res := b.fly(func(backstopSeen) []traffic.GroundPose { return b.standing() })
				if !res.stopped {
					t.Error("did not stop short of the vehicle on its new way")
				}
				if res.nearest < backstopClearance-0.1 {
					t.Errorf("came within %.1f m of a reported vehicle, want %.1f m", res.nearest, backstopClearance)
				}
				if !res.on {
					t.Error("did not go on once it was gone")
				}
			}
			if got := b.ctl.Plan(); got.Exit.Node != after.Exit.Node || !slices.Equal(got.Route.Nodes, after.Route.Nodes) {
				t.Error("the way off #1640 planned was changed")
			}
		})
	}
}
