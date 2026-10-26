package traffic

import "testing"

// TestDepartureOnGroundAsItIs: what a departure's status says of the ground
// is so from its first frame on the stand (#1230: false until it first
// moved) and turns false the frame it lifts off, not with the next progress
// event a second on (#1243: a view had it on the runway, climbing).
func TestDepartureOnGroundAsItIs(t *testing.T) {
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{RollingTakeoffChance: 1})
	var last TaxiEvent
	got := false
	drain := func() {
		for {
			select {
			case ev, ok := <-ctl.Events():
				if !ok {
					return // done
				}
				last, got = ev, true
			default:
				return
			}
		}
	}
	if !run(TaxiAwaitingPushback, 600) {
		t.Fatalf("state %v, want awaiting pushback", ctl.State())
	}
	run(TaxiComplete, 1) // a frame on the stand
	drain()
	if !got || !last.OnGround {
		t.Fatalf("on its stand the status says on the ground %v (an event %v)", last.OnGround, got)
	}
	lifted, behind := false, 0
	for i := 0; i < 60*1500 && ctl.State() != TaxiComplete && !ctl.State().Terminal(); i++ {
		run(TaxiComplete, 1)
		drain()
		if ctl.State() != TaxiDeparting {
			if last.State < TaxiDeparting && !last.OnGround {
				t.Fatalf("%v: the status says airborne", last.State)
			}
			continue
		}
		if ctl.takeoffPhase == TakeoffAirborne {
			lifted = true
			if last.OnGround {
				behind++
			}
		}
	}
	if !lifted {
		t.Fatalf("never lifted off: %v", ctl.State())
	}
	if behind > 0 {
		t.Errorf("airborne %d frames while the status still said on the ground", behind)
	}
}
