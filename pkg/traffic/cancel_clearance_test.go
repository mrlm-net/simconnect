package traffic

import (
	"errors"
	"testing"
)

// TestCancelTakeoffClearance (#1685): lined up, the cancel holds it there
// until cleared again; rolling, it is too late and the roll goes on.
func TestCancelTakeoffClearance(t *testing.T) {
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{HoldForClearances: true})
	run(TaxiAwaitingPushback, 60*60)
	ctl.ClearPushback()
	ctl.ClearToTaxi()
	if !run(TaxiHoldingShort, 60*900) {
		t.Fatal(ctl.State())
	}
	ctl.ClearToLineUp()
	if !run(TaxiLinedUp, 60*300) {
		t.Fatal(ctl.State())
	}
	if err := ctl.ClearForTakeoff(); err != nil {
		t.Fatal(err)
	}
	if err := ctl.CancelTakeoffClearance(); err != nil {
		t.Fatalf("lined up: %v", err)
	}
	for i := 0; i < 60*20; i++ {
		run(TaxiComplete, 1)
	}
	if ctl.State() != TaxiLinedUp {
		t.Fatalf("cancelled, %v, want lined up still", ctl.State())
	}
	if err := ctl.ClearForTakeoff(); err != nil {
		t.Fatal(err)
	}
	if !run(TaxiDeparting, 60*30) {
		t.Fatal(ctl.State())
	}
	for i := 0; i < 60*8; i++ {
		run(TaxiComplete, 1)
	}
	kts := ctl.takeoff.Pose().GroundSpeedKts
	if err := ctl.CancelTakeoffClearance(); !errors.Is(err, ErrTooLate) {
		t.Errorf("rolling at %.0f kt: %v, want ErrTooLate", kts, err)
	}
	for i := 0; i < 60*3; i++ {
		run(TaxiComplete, 1)
	}
	if ctl.State() != TaxiDeparting || ctl.takeoff == nil || ctl.takeoff.Pose().GroundSpeedKts <= kts {
		t.Errorf("after the cancel: %v, not rolling on faster than %.0f kt", ctl.State(), kts)
	}
}
