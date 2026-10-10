package traffic

import (
	"sync"
	"testing"
	"time"
)

// landingRoll flies an injected arrival of model to LKPR runway end onto
// stand C22 and reports where it touched down, the seconds from touchdown
// to clear of the runway, and the exit taken.
func landingRoll(t *testing.T, model, end string) (touchdown, secs, along float64, exit string) {
	t.Helper()
	g := lkprGraph(t)
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	st, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: end, Parking: st, Model: model, Tail: "OKABC",
		InjectApproach: true, RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var td float64 // the event goroutine's, not the named result (read after the unlock)
	go func() {
		for e := range ctl.Events() {
			if e.Touchdown != 0 {
				mu.Lock()
				td = e.Touchdown
				mu.Unlock()
			}
		}
	}()
	p := ctl.Plan()
	mon := DefaultArrivalRequestBase + arrReqMonitor
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	var down, clear time.Time
	for i := 0; i < 60*1200 && ctl.State() < ArrivalAwaitingTaxi; i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 77, p.End.Threshold, 0, 0, 0, false))
		if down.IsZero() && ctl.State() == ArrivalRollout {
			down = now
		}
		if clear.IsZero() && ctl.State() == ArrivalVacating {
			clear = now
		}
	}
	if down.IsZero() || clear.IsZero() {
		t.Fatalf("%s: state %v", model, ctl.State())
	}
	mu.Lock()
	touchdown = td
	mu.Unlock()
	return touchdown, clear.Sub(down).Seconds(), p.Exit.Along, p.Exit.Taxiway
}

// TestLightAircraftLandingRoll: a light single touches down near the
// threshold and turns off at the first exit it can make, clear of the
// runway well before an airliner would be; an airliner keeps its exit.
// LKPR 24's first exit is C, 1305 m in (the C172 rolled 83 s to D before).
func TestLightAircraftLandingRoll(t *testing.T) {
	for _, c := range []struct {
		model, end, exit string
		maxSecs          float64
	}{
		{"C172", "24", "C", 60}, {"C172", "06", "L", 70}, {"P28A", "30", "R", 35}, {"A320", "24", "D", 60},
	} {
		td, secs, along, exit := landingRoll(t, c.model, c.end)
		t.Logf("%s on %s: touchdown %.0f m, clear %.0f s later at exit %s (%.0f m along)", c.model, c.end, td, secs, exit, along)
		if exit != c.exit {
			t.Errorf("%s on %s: exit %s, want %s", c.model, c.end, exit, c.exit)
		}
		if secs > c.maxSecs {
			t.Errorf("%s on %s: clear %.0f s after touchdown, want ≤ %.0f", c.model, c.end, secs, c.maxSecs)
		}
		if c.model != "A320" && td > exitLightTouchdownMeters {
			t.Errorf("%s: touchdown %.0f m past the threshold, planned by %.0f", c.model, td, exitLightTouchdownMeters)
		}
	}
}
