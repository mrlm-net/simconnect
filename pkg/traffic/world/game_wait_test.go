package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestGameBoardingIsNotWaiting: a departure on its stand before its crew
// calls is boarding, not waiting for ATC; it waits from the call on, and the
// other clearance stops count as before (#1675).
func TestGameBoardingIsNotWaiting(t *testing.T) {
	push := traffic.TaxiAwaitingPushback.String()
	for _, c := range []struct {
		v    ControlView
		want bool
	}{
		{ControlView{Kind: "departure", State: push}, false},
		{ControlView{Kind: "departure", State: push, Request: "pushback"}, true},
		{ControlView{Kind: "departure", State: push, Request: "start_up"}, true},
		{ControlView{Kind: "departure", State: traffic.TaxiAwaitingTaxi.String()}, true},
		{ControlView{Kind: "departure", State: traffic.TaxiHoldingShort.String()}, true},
		{ControlView{Kind: "arrival", State: traffic.ArrivalHoldingShort.String()}, true},
		{ControlView{Kind: "departure", State: traffic.TaxiTaxiing.String()}, false},
		{ControlView{Kind: "departure", State: traffic.TaxiTaxiing.String(), AtLimit: true}, true},
	} {
		if got := waitingForATC(c.v); got != c.want {
			t.Errorf("%s %s request %q: waiting %v, want %v", c.v.Kind, c.v.State, c.v.Request, got, c.want)
		}
	}
}
