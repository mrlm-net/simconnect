package world

import (
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// A TAXI CLEARANCE IS NOT SAID TO SOMETHING IN THE AIR.
//
// arrivalActions answered "land, go around, taxi, up to" for ArrivalApproaching
// and ArrivalLanding — an aeroplane on final, a thousand feet up, offered a
// clearance to taxi. The engine takes it (ClearToTaxi only sets a flag), so
// nothing refused it and nothing said it was wrong; it simply taught an order
// of the job that does not exist. The departure side of this is #1308.
func TestArrivalActionsNoGroundClearanceInTheAir(t *testing.T) {
	for _, s := range []traffic.ArrivalState{traffic.ArrivalApproaching, traffic.ArrivalLanding} {
		got := arrivalActions(s)
		for _, gone := range []string{"taxi", "upto"} {
			if slices.Contains(got, gone) {
				t.Errorf("%v: %q must not be offered to an aircraft in the air, got %v", s, gone, got)
			}
		}
		// What IS the tower's word at that point, and both of them.
		for _, want := range []string{"land", "goaround"} {
			if !slices.Contains(got, want) {
				t.Errorf("%v: want %q offered, got %v", s, want, got)
			}
		}
	}
}

// THE ROLL IS THE CREW'S. Between touchdown and being clear of the runway the
// aeroplane is stopping; there is nothing for a controller to say, and the next
// word on the frequency is its own call to ground. Offering a taxi clearance
// here was offering one to an aircraft still ON the runway it has to vacate.
func TestArrivalActionsNothingToSayOnTheRoll(t *testing.T) {
	if got := arrivalActions(traffic.ArrivalRollout); len(got) != 0 {
		t.Errorf("on the landing roll: want nothing offered, got %v", got)
	}
}

// And the moment it is off the runway it is ground's, so the taxi clearance is
// due — without waiting for it to come to a full stop.
func TestArrivalActionsTaxiDueOnceVacated(t *testing.T) {
	for _, s := range []traffic.ArrivalState{traffic.ArrivalVacating, traffic.ArrivalAwaitingTaxi} {
		got := arrivalActions(s)
		for _, want := range []string{"taxi", "upto"} {
			if !slices.Contains(got, want) {
				t.Errorf("%v: want %q offered, got %v", s, want, got)
			}
		}
	}
}

// Vacating is ground's, so a staffed tower can hand it on there rather than
// holding it to a standstill. ArrivalPosition is what handoffDue follows.
func TestArrivalIsGroundsOnceOffTheRunway(t *testing.T) {
	for _, tc := range []struct {
		s    traffic.ArrivalState
		want traffic.Position
	}{
		{traffic.ArrivalLanding, traffic.PosTower},
		{traffic.ArrivalRollout, traffic.PosTower},
		{traffic.ArrivalVacating, traffic.PosGround},
		{traffic.ArrivalAwaitingTaxi, traffic.PosGround},
	} {
		if got := traffic.ArrivalPosition(tc.s, true); got != tc.want {
			t.Errorf("%v: %s, want %s", tc.s, got, tc.want)
		}
	}
}

// The clearances further along the taxi are untouched: a crossing is still a
// crossing, and holding position still works.
func TestArrivalActionsOnTheGroundUnchanged(t *testing.T) {
	if got := arrivalActions(traffic.ArrivalTaxiing); !slices.Contains(got, "hold") {
		t.Errorf("taxiing: want hold offered, got %v", got)
	}
	if got := arrivalActions(traffic.ArrivalHoldingShort); !slices.Contains(got, "cross") {
		t.Errorf("holding short: want cross offered, got %v", got)
	}
}

// A click names an exit by ANY node on it. The turn-off where it leaves the
// centreline, the first taxiway node off the surface and the path between them
// are one turn-off to whoever clicked; matching only one of them would refuse a
// click on the exit a chart actually draws.
func TestExitAtNode(t *testing.T) {
	exits := []airport.RunwayExit{
		{RunwayNode: 10, Node: 11, Path: []airport.NodeID{10, 99, 11}, Taxiway: "A4"},
		{RunwayNode: 20, Node: 21, Path: []airport.NodeID{20, 21}, Taxiway: "B2"},
	}
	for _, tc := range []struct {
		name string
		node airport.NodeID
		want string
	}{
		{"the runway node it leaves by", 10, "A4"},
		{"the first taxiway node off the surface", 11, "A4"},
		{"a node on the path between them", 99, "A4"},
		{"another exit entirely", 21, "B2"},
	} {
		got, ok := exitAtNode(exits, tc.node)
		if !ok || got.Taxiway != tc.want {
			t.Errorf("%s: %q (found %v), want %q", tc.name, got.Taxiway, ok, tc.want)
		}
	}
	// NOT APPROXIMATED. A click nowhere near an exit is refused, because
	// bestExit weighs what the aircraft can make at its speed and "the nearest
	// exit to wherever you clicked" throws that away.
	if _, ok := exitAtNode(exits, 1234); ok {
		t.Error("a node on no exit was matched to one")
	}
	if _, ok := exitAtNode(nil, 10); ok {
		t.Error("an exit was found on a runway with none")
	}
}
