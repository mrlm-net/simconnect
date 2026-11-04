package traffic

import (
	"testing"
	"time"
)

// A runway change re-plans the push of a departure on its stand off the
// connection's goroutine, as its spawn does (TaxiWithPushInBackground):
// planned in the call, every aircraft stood still that long (live, LKPR
// 24 to 06: TVS124 on its stand, 3.3 s of frames held back).
func TestChangeRunwayPlansPushInBackground(t *testing.T) {
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{}, TaxiWithPushInBackground())
	deadline := time.Now().Add(10 * time.Second)
	for {
		ctl.mu.Lock()
		pending := ctl.pushPending
		ctl.mu.Unlock()
		if !pending {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the spawn's push never planned")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := ctl.ChangeRunway("06", "", nil); err != nil {
		t.Fatal(err)
	}
	ctl.mu.Lock()
	pending := ctl.pushPending
	ctl.mu.Unlock()
	if !pending {
		t.Fatal("the push was planned in the runway change call, on the connection's goroutine")
	}
	for i := 0; i < 1000; i++ {
		ctl.mu.Lock()
		pending = ctl.pushPending
		ctl.mu.Unlock()
		if !pending {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !run(TaxiHoldingShort, 60*900) {
		t.Fatalf("never held short: %v", ctl.State())
	}
	if r := ctl.Route(); r == nil || r.RunwayEnd != "06" {
		t.Fatalf("route to %v, want 06", r)
	}
}
