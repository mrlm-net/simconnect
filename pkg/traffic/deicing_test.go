//go:build windows
// +build windows

package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestDeicingOnStand: cleared to push, the aircraft is de-iced on the stand
// first (Deicing, no push), then pushes back (#323).
func TestDeicingOnStand(t *testing.T) {
	ctl, _, run, now := injectedDeparture(t, TaxiRequest{HoldForClearances: true, Deice: &Deicing{Dwell: 30 * time.Second}})
	if !run(TaxiAwaitingPushback, 60*60) {
		t.Fatal(ctl.State())
	}
	ctl.ClearPushback()
	start := *now
	run(TaxiPushback, 60*20)
	if ctl.State() != TaxiAwaitingPushback || !ctl.last.Deicing {
		t.Fatalf("20 s after the clearance: state %v, de-icing %v", ctl.State(), ctl.last.Deicing)
	}
	if !run(TaxiPushback, 60*60) {
		t.Fatalf("state %v, want the push after the de-icing", ctl.State())
	}
	if d := now.Sub(start); d < 27*time.Second || d > 40*time.Second {
		t.Errorf("pushed %v after the clearance, want about 30 s of de-icing and the beacon lead", d)
	}
	if ctl.last.Deicing || !ctl.deiced {
		t.Error("de-icing not finished")
	}
}

// TestDeicingAtPad: the route passes the pad, the aircraft stops there
// with its taxi light off for the treatment, then taxis on to the runway.
func TestDeicingAtPad(t *testing.T) {
	g := lkprGraph(t)
	c22, _ := g.Layout.ParkingIndex("C22")
	plain, err := g.RouteToRunway(c22, "24", airport.RouteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	padAt := g.Nodes[plain.Nodes[len(plain.Nodes)*2/3]].Position
	pad := &airport.DeicingPad{Name: "TEST", Position: padAt}
	ctl, _, run, now := injectedDeparture(t, TaxiRequest{HoldForClearances: true, Deice: &Deicing{Pad: pad, Dwell: 20 * time.Second}})
	run(TaxiAwaitingPushback, 60*60)
	ctl.ClearPushback()
	ctl.ClearToTaxi()
	var deicedAt time.Time
	for i := 0; i < 60*900 && ctl.State() != TaxiHoldingShort; i++ {
		run(TaxiHoldingShort, 1)
		if ctl.last.Deicing && deicedAt.IsZero() {
			deicedAt = *now
			p := ctl.mover.Pose()
			if d := localDist(NoseGear(p.Position, p.Heading, ctl.profile()), padAt); d > 3 {
				t.Errorf("stopped %.1f m from the pad", d)
			}
			if ctl.lights.Taxi {
				t.Error("taxi light on while de-icing")
			}
		}
	}
	if deicedAt.IsZero() {
		t.Fatal("never stopped at the pad")
	}
	if ctl.State() != TaxiHoldingShort || ctl.last.HoldingShortOf != ctl.runway.Name() {
		t.Fatalf("state %v, want holding short of the runway after the pad", ctl.State())
	}
	if !ctl.deiced || !ctl.lights.Taxi {
		t.Error("not finished, or the taxi light stayed off")
	}
}
