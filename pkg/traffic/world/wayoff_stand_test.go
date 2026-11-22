package world

import (
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// wayOffArrival has no way off clear to its own stand; to another it has
// one where clear says so.
type wayOffArrival struct {
	arrivalCtl
	clear bool
	asked []int
	took  []int
}

func (a *wayOffArrival) Plan() *traffic.ArrivalPlan { return &traffic.ArrivalPlan{EndTurn: true} }

func (a *wayOffArrival) State() traffic.ArrivalState { return traffic.ArrivalApproaching }

func (a *wayOffArrival) WayOffClear([]airport.Occupied) bool { return false }

func (a *wayOffArrival) PlanWayOff(parking int, _ []airport.Occupied) func() bool {
	a.asked = append(a.asked, parking)
	if parking < 0 || !a.clear {
		return nil
	}
	return func() bool {
		a.took = append(a.took, parking)
		return true
	}
}

// TestWayOffChangesStand (#1640): with no exit clear to its stand an
// arrival not down yet is given the nearest other that is free, fits and is
// of its kind, and lets its own go. With none it keeps its stand and is
// marked for the departure to go first; a player's is never moved (#1096).
func TestWayOffChangesStand(t *testing.T) {
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
	park := g.Layout.Parking
	dist := func(i int) float64 {
		return calc.HaversineMeters(park[was].Position.Lat, park[was].Position.Lon, park[i].Position.Lat, park[i].Position.Lon)
	}
	byDistance := slices.DeleteFunc(slices.Clone(g.Layout.SuitableStands(half)), func(i int) bool {
		return i == was || park[i].Type != park[was].Type
	})
	slices.SortStableFunc(byDistance, func(a, b int) int {
		switch {
		case dist(a) < dist(b):
			return -1
		case dist(a) > dist(b):
			return 1
		}
		return 0
	})

	for _, tc := range []struct {
		name          string
		clear, player bool
		moves         bool
	}{
		{"another stand", true, false, true},
		{"none", false, false, false},
		{"a player's", true, true, false},
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
			a := traffic.NewStandAllocator(nil, g)
			if err := a.Occupy(was, "TST1", half); err != nil {
				t.Fatal(err)
			}
			// The nearest is someone's: never that one.
			if err := a.Occupy(byDistance[0], "TST2", half); err != nil {
				t.Fatal(err)
			}
			want := -1
			for _, i := range byDistance {
				if a.Free(i, half) && a.TakenFrom(i, "TST1", 0) == "" {
					want = i
					break
				}
			}
			if want < 0 {
				t.Fatal("no free stand: no case")
			}
			arr := &wayOffArrival{clear: tc.clear}
			it := &controlled{ID: 1, Tail: "TST1", ICAO: "LKPR", cc: cc, arr: arr, graph: g, stands: a, stand: was, removed: make(chan struct{})}
			it.view.Model, it.view.Stand = model, park[was].Label()
			it.gates.Store(tc.player)
			it.wayOffPlanning.Store(true)
			cc.planWayOff(it, arr, nil)
			if n := len(arr.asked); n > 1+wayOffStands {
				t.Errorf("one search asked %d times, the cap is %d", n, 1+wayOffStands)
			}

			if len(arr.asked) == 0 || arr.asked[0] >= 0 {
				t.Fatalf("asked %v: not its own stand first", arr.asked)
			}
			if !tc.moves {
				if len(arr.took) != 0 || it.stand != was {
					t.Errorf("moved to %d (took %v), want its own %d", it.stand, arr.took, was)
				}
				if o, ok := a.Occupant(was); !ok || o.Owner != "TST1" {
					t.Errorf("its own stand is no longer its own: %+v", o)
				}
				if !tc.clear && !it.wayOffBlocked.Load() {
					t.Error("not marked for the departure to go first")
				}
				if !tc.clear {
					cc.keepWayOffClear([]*controlled{it})
					if it.wayOffPlanning.Load() {
						t.Error("searched again at once")
					}
				}
				return
			}
			if it.stand != want || it.view.Stand != park[want].Label() || !slices.Equal(arr.took, []int{want}) {
				t.Fatalf("stand %s (took %v), want %s", it.view.Stand, arr.took, park[want].Label())
			}
			if o, ok := a.Occupant(want); !ok || o.Owner != "TST1" {
				t.Errorf("the new stand is not reserved for it: %+v", o)
			}
			if o, ok := a.Occupant(was); ok && o.Owner == "TST1" {
				t.Error("still holds the stand it left")
			}
		})
	}
}
