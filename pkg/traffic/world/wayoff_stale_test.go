package world

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// staleArrival finds a way off to another stand, or none; during changes
// things once it has and before that is put in place.
type staleArrival struct {
	arrivalCtl
	state  traffic.ArrivalState
	to     int // the stand it has a way off to; negative none
	during func()
	took   bool
}

func (a *staleArrival) State() traffic.ArrivalState { return a.state }

func (a *staleArrival) Plan() *traffic.ArrivalPlan { return &traffic.ArrivalPlan{EndTurn: true} }

func (a *staleArrival) WayOffClear([]airport.Occupied) bool { return false }

func (a *staleArrival) PlanWayOff(parking int, _ []airport.Occupied) func() bool {
	if parking < 0 && a.to >= 0 {
		return nil
	}
	if a.during != nil {
		a.during()
		a.during = nil
	}
	if a.to < 0 {
		return nil
	}
	a.to = parking
	return func() bool {
		a.took = true
		return true
	}
}

type staleDeparture struct {
	departureCtl
	state traffic.TaxiState
	route *airport.Route
}

func (d *staleDeparture) Route() *airport.Route { return d.route }

func (d *staleDeparture) State() traffic.TaxiState { return d.state }

// TestWayOffDropsStaleSearch (#1640): the search ends later than it began,
// so what it found, a stand or none, is put in place only if it still
// holds: the stand free, the arrival not down, not a player's, on the same
// stand, and the departures where they were. One taxiing out counts by
// the point it will hold at, so a search outlasts its rolling there.
func TestWayOffDropsStaleSearch(t *testing.T) {
	g, err := airport.BuildGraph(lkprLayout(t))
	if err != nil {
		t.Fatal(err)
	}
	was, err := g.Layout.ParkingIndex("C22")
	if err != nil {
		t.Fatal(err)
	}
	const model = "FSLTL_A320_LYX"
	half := traffic.MotionProfileFor(model).SpanMeters / 2

	type run struct {
		it     *controlled
		arr    *staleArrival
		dep    *staleDeparture
		depIt  *controlled
		stands *traffic.StandAllocator
	}
	for _, tc := range []struct {
		name   string
		none   bool
		rolls  bool
		change func(r *run)
		holds  bool
	}{
		{name: "nothing changed", holds: true},
		{name: "nothing changed, none found", none: true, holds: true},
		{name: "touched down", change: func(r *run) { r.arr.state = traffic.ArrivalRollout }},
		{name: "cleared to taxi in", change: func(r *run) { r.arr.state = traffic.ArrivalTaxiing }},
		{name: "cancelled", change: func(r *run) { r.arr.state = traffic.ArrivalCancelled }},
		{name: "a player's", change: func(r *run) { r.it.gates.Store(true) }},
		{name: "given another stand", change: func(r *run) {
			r.it.mu.Lock()
			r.it.stand++
			r.it.mu.Unlock()
		}},
		{name: "departure gone", change: func(r *run) { r.dep.state = traffic.TaxiLiningUp }},
		{name: "departure gone, none found", none: true, change: func(r *run) { r.dep.state = traffic.TaxiLiningUp }},
		{name: "departure moved on", change: func(r *run) {
			r.depIt.mu.Lock()
			r.depIt.view.Position.Lat += 0.0005
			r.depIt.mu.Unlock()
		}},
		{name: "departure rolling at 10 kt", rolls: true, holds: true, change: func(r *run) {
			r.depIt.mu.Lock()
			r.depIt.view.Position.Lat += rolledIn(time.Second)
			r.depIt.mu.Unlock()
		}},
		{name: "departure rolling at 10 kt, none found", none: true, rolls: true, holds: true, change: func(r *run) {
			r.depIt.mu.Lock()
			r.depIt.view.Position.Lat += rolledIn(5 * time.Second)
			r.depIt.mu.Unlock()
		}},
		{name: "rolling, sent to another holding point", rolls: true, change: func(r *run) {
			r.dep.route = &airport.Route{Points: []airport.LatLon{g.Layout.Parking[0].Position, g.Layout.Parking[2].Position}}
		}},
		{name: "stands all taken", change: func(r *run) {
			for i := range g.Layout.Parking {
				if i != was {
					_ = r.stands.Occupy(i, "TST3", 1)
				}
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := New(Options{DataDir: t.TempDir()})
			cc := newControlCenter(nil, w.st.core)
			stop := make(chan struct{})
			defer close(stop)
			go func() {
				for {
					select {
					case f := <-cc.cmds:
						f()
					case <-stop:
						return
					}
				}
			}()
			r := &run{arr: &staleArrival{state: traffic.ArrivalApproaching}, dep: &staleDeparture{state: traffic.TaxiHoldingShort}, stands: traffic.NewStandAllocator(nil, g)}
			if tc.none {
				r.arr.to = -1
			}
			if err := r.stands.Occupy(was, "TST1", half); err != nil {
				t.Fatal(err)
			}
			r.it = &controlled{ID: 1, Tail: "TST1", ICAO: "LKPR", cc: cc, arr: r.arr, graph: g, stands: r.stands, stand: was, removed: make(chan struct{})}
			r.it.view.Model, r.it.view.Stand = model, g.Layout.Parking[was].Label()
			r.depIt = &controlled{ID: 2, Tail: "TST2", ICAO: "LKPR", cc: cc, dep: r.dep, graph: g, removed: make(chan struct{})}
			r.depIt.view.Model, r.depIt.view.Position = model, g.Layout.Parking[0].Position
			cc.mu.Lock()
			cc.items[1], cc.items[2] = r.it, r.depIt
			cc.mu.Unlock()
			if tc.rolls {
				r.dep.state = traffic.TaxiTaxiing
				r.dep.route = &airport.Route{Points: []airport.LatLon{g.Layout.Parking[0].Position, g.Layout.Parking[1].Position}}
			}
			occ := wayOffTaken(r.it, cc.snapshotItems())
			if len(occ) != 1 {
				t.Fatalf("%d places taken, want the departure's", len(occ))
			}
			if tc.change != nil {
				r.arr.during = func() { tc.change(r) }
			}
			r.it.wayOffPlanning.Store(true)
			cc.planWayOff(r.it, r.arr, occ)

			switch {
			case tc.none:
				if got := r.it.wayOffBlocked.Load(); got != tc.holds {
					t.Errorf("departure first %v, want %v", got, tc.holds)
				}
			case tc.holds:
				if !r.arr.took {
					t.Error("not put in place though it holds")
				}
			case r.arr.took:
				t.Error("put in place though it no longer holds")
			default:
				if o, ok := r.stands.Occupant(r.arr.to); ok && o.Owner == "TST1" && r.arr.to != was {
					t.Error("kept the stand it was not given")
				}
			}
		})
	}
}

// rolledIn is how many degrees of latitude 10 kt covers in d.
func rolledIn(d time.Duration) float64 { return 10 * 1852.0 / 3600 * d.Seconds() / 111195 }
