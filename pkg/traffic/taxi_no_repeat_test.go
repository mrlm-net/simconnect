package traffic

import (
	"errors"
	"testing"
)

// A CONTROLLER DOES NOT SAY A THING TWICE (#1269). The player's screenshot:
// the identical taxi clearance transmitted three times over, read back each
// time. "Taxi to holding point runway 24" to an aircraft already taxiing
// there on that very clearance says nothing — but with a limit in force it
// says a great deal (the limit is lifted, carry on), so the flag alone
// cannot be the answer.
func TestTaxiClearanceAlreadyInForce(t *testing.T) {
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{HoldForClearances: true})
	if !run(TaxiAwaitingPushback, 60*60) {
		t.Fatal(ctl.State())
	}
	if ctl.Cleared("taxi") {
		t.Fatal("on the stand, nothing given: taxi must not read as already cleared")
	}
	ctl.ClearPushback()
	if !run(TaxiAwaitingTaxi, 60*600) {
		t.Fatal(ctl.State())
	}

	ctl.ClearToTaxi()
	if !ctl.Cleared("taxi") {
		t.Error("cleared to taxi with no limit: the clearance IS the one in force")
	}

	// A limit makes it sayable again — "taxi" then means "limit lifted".
	route := ctl.Route()
	first, second := route.Nodes[len(route.Nodes)/3], route.Nodes[2*len(route.Nodes)/3]
	if err := ctl.ClearUpTo(first); err != nil {
		t.Fatal(err)
	}
	if ctl.Cleared("taxi") {
		t.Error("a limit in force: an unlimited taxi clearance lifts it, so it is not already given")
	}

	// THE SAME LIMIT AGAIN IS THE SAME CLEARANCE. Refused, so the host
	// transmits nothing rather than a second identical instruction.
	if err := ctl.ClearUpTo(first); !errors.Is(err, ErrAlreadyCleared) {
		t.Errorf("the limit already in force: want ErrAlreadyCleared, got %v", err)
	}
	// A different node is a new limit and always goes through.
	if err := ctl.ClearUpTo(second); err != nil {
		t.Errorf("a new limit further along: want it accepted, got %v", err)
	}
	// Lifted, and then it is the clearance in force again.
	ctl.ClearToTaxi()
	if !ctl.Cleared("taxi") {
		t.Error("limit removed: the unlimited clearance is now the one in force")
	}
}

// Hold position puts a limit back, so the taxi clearance means something
// again: it is what lets the aircraft go on (ClearToTaxi's own contract).
func TestTaxiClearanceSayableAfterHoldPosition(t *testing.T) {
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{HoldForClearances: true})
	if !run(TaxiAwaitingPushback, 60*60) {
		t.Fatal(ctl.State())
	}
	ctl.ClearPushback()
	ctl.ClearStartUp()
	if !run(TaxiAwaitingTaxi, 60*600) {
		t.Fatal(ctl.State())
	}
	ctl.ClearToTaxi()
	if !run(TaxiTaxiing, 60*600) {
		t.Fatal(ctl.State())
	}
	if !ctl.Cleared("taxi") {
		t.Fatal("taxiing on an unlimited clearance: want it read as already given")
	}
	if err := ctl.HoldPosition(); err != nil {
		t.Fatal(err)
	}
	if ctl.Cleared("taxi") {
		t.Error("held in position: the taxi clearance is what releases it, so it must be sayable")
	}
}
