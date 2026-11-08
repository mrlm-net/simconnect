package traffic

import (
	"testing"
	"time"
)

// A departure started on a gate without stand services, given them while
// it waits (its jetway never came): the stairs come as on a remote stand,
// the buses after them; once pushing, none are taken.
func TestTaxiControllerAddStandServices(t *testing.T) {
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{PushbackAt: time.Now().Add(20 * time.Minute), HoldForClearances: true})
	go func() {
		for range ctl.Events() {
		}
	}()
	if !run(TaxiAwaitingPushback, 600) {
		t.Fatalf("state %v", ctl.State())
	}
	run(TaxiPushback, 60*2)
	stairs := &fakeFuel{arriveAfter: 60, leaveAfter: 60}
	bus := &fakeFuel{arriveAfter: 60, leaveAfter: 60}
	if ctl.AddStandServices(nil, nil, nil) {
		t.Fatal("taken without stairs")
	}
	if !ctl.AddStandServices(stairs, nil, []FuelService{bus}) {
		t.Fatal("not taken while waiting")
	}
	if ctl.AddStandServices(&fakeFuel{}, nil, nil) {
		t.Fatal("taken twice")
	}
	run(TaxiPushback, 60*20)
	if stairs.attached != 1 {
		t.Fatalf("stairs attached %d times", stairs.attached)
	}
	for i := 0; i < 60*60*20 && bus.attached == 0; i++ {
		run(TaxiPushback, 1)
	}
	if bus.attached != 1 {
		t.Fatal("no bus after the stairs")
	}
}
