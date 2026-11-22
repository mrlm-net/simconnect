package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// saidArrival is cleared to taxi, as far as its controller goes, once it
// taxis.
type saidArrival struct{ *staleArrival }

func (a saidArrival) TaxiInForce(airport.NodeID) bool { return a.state == traffic.ArrivalTaxiing }

// TestWayOffStandSaidOnce (#1640): a stand changed for the way off is named
// once on the radio, in the taxi clearance. The change itself says nothing,
// and none is made once that clearance is out: ground would give it again,
// to another stand (taxiInForce: changed words go out).
func TestWayOffStandSaidOnce(t *testing.T) {
	g, err := airport.BuildGraph(lkprLayout(t))
	if err != nil {
		t.Fatal(err)
	}
	was, err := g.Layout.ParkingIndex("C22")
	if err != nil {
		t.Fatal(err)
	}
	const model = "FSLTL_A320_LYX"
	for _, tc := range []struct {
		name  string
		late  bool // the search ends after the taxi clearance
		moves bool
	}{
		{"found before the taxi clearance", false, true},
		{"found after the taxi clearance", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := New(Options{DataDir: t.TempDir()})
			cc := newControlCenter(nil, w.st.core)
			var stands []string
			cc.radio = traffic.NewRadio(traffic.RadioOptions{Now: cc.clock.Now, OnTransmission: func(x traffic.Transmission) {
				if s := x.Params[traffic.ParamStand]; s != "" && !x.Pilot {
					stands = append(stands, s)
				}
			}})
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
			if err := a.Occupy(was, "TST1", traffic.MotionProfileFor(model).SpanMeters/2); err != nil {
				t.Fatal(err)
			}
			arr := saidArrival{&staleArrival{state: traffic.ArrivalApproaching}}
			it := &controlled{ID: 1, Tail: "TST1", ICAO: "LKPR", cc: cc, arr: arr, graph: g, stands: a, stand: was, removed: make(chan struct{})}
			it.view.Model, it.view.Stand = model, g.Layout.Parking[was].Label()
			// Ground's taxi clearance, as it is given: not again in the same
			// words.
			clearTaxi := func() {
				if !it.taxiInForce("taxi", -1) {
					it.say(it.phrase("taxi", -1))
				}
				arr.state = traffic.ArrivalTaxiing
			}
			if tc.late {
				arr.during = clearTaxi
			}
			it.wayOffPlanning.Store(true)
			cc.planWayOff(it, arr, nil)
			if !tc.late && len(stands) != 0 {
				t.Errorf("the change said the stand itself: %q", stands)
			}
			clearTaxi()
			clearTaxi()

			if moved := it.stand != was; moved != tc.moves {
				t.Errorf("stand changed %v, want %v", moved, tc.moves)
			}
			if len(stands) != 1 {
				t.Fatalf("the stand said %d times (%q), want once", len(stands), stands)
			}
			if stands[0] != it.view.Stand {
				t.Errorf("said stand %q, it taxis to %q", stands[0], it.view.Stand)
			}
		})
	}
}
