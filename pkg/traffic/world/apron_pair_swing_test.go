package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// swingArr is an arrival taxiing in whose controller gives the way it
// drives, apart from its route.
type swingArr struct {
	passArr
	way []airport.LatLon
}

func (a *swingArr) TaxiInWay() []airport.LatLon { return a.way }

// TestPairedArrivalSwing (#1694): a departure waits for a backtracking
// arrival taxiing in whose route keeps clear of its way out, but whose turn
// onto its stand swings across it (TFFJ: onto 7W it came 27 m past its
// route's last turn and passed a departure taxiing out at 18.8 m); one
// whose driven way keeps clear too it does not wait for.
func TestPairedArrivalSwing(t *testing.T) {
	cc, g := pairBed(t)
	stand := g.Layout.Parking[0].Position
	lane := offBy(stand, 0, 120)
	far := offBy(stand, 180, 600) // the route: well clear of the way out
	for _, tc := range []struct {
		name  string
		swing bool
		held  bool
	}{
		{"swinging across its way out", true, true},
		{"driving its route", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := inAlong(t, g, far, true)
			arr, dep := pairOf(cc, g, traffic.ArrivalTaxiing, plan, 0, stand, lane)
			arr.view.Position = plan.Route.Points[1]
			way := plan.Route.Points[1:]
			if tc.swing {
				// From where it is round across the lane east of the way out's
				// turn, and back to its stand by its route's end.
				way = []airport.LatLon{plan.Route.Points[1], offBy(lane, 90, 150), offBy(lane, 90, 160), plan.Route.Points[len(plan.Route.Points)-1]}
			}
			arr.arr = &swingArr{passArr: passArr{state: traffic.ArrivalTaxiing, plan: plan}, way: way}
			if o, _ := dep.pairedArrival(); (o == arr) != tc.held {
				t.Errorf("held %v, want %v", o == arr, tc.held)
			}
		})
	}
}
