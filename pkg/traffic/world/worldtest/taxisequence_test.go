package worldtest

import (
	"fmt"
	"net/http"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestNoTaxiClearanceOnTheStand walks the departure sequence as a player sees
// it and asserts what is OFFERED at each step (#1268).
//
// The player's complaint: "the sequence of commands … cannot give taxi when it
// hasn't called ground". On its stand a departure has asked for a push, with a
// tug about to connect; a taxi clearance there is a clearance for a
// conversation that has not happened, and it could not go if it were told to.
//
// And the counterpart, which is why Request is on the view at all: once push
// and start are given there is nothing left for a controller to say, so the
// offered list is EMPTY — correct, and indistinguishable from a stuck game
// unless the crew's own request is readable beside it.
func TestNoTaxiClearanceOnTheStand(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 6}); err != nil {
		t.Fatal(err)
	}

	// A departure ON GROUND'S FREQUENCY, asking to push.
	//
	// NOT MERELY ONE OFFERED "pushback". At spawn the aircraft is Delivery's,
	// still waiting on its en-route clearance, and the taxi state is already
	// "awaiting pushback" — so a wait on the offered list alone matched
	// instantly, read an empty Request (the push gate had not opened) and got
	// 403 from `as=ground`, which is the engine correctly refusing a clearance
	// from a position the aircraft is not working. The crew's own request IS
	// the readiness signal, so wait on that.
	//
	// `FacesOut` stands are a different sequence on purpose (no push to ask
	// for: start-up and taxi are one call), and they ask for "start_up", so
	// this condition picks a pushback stand by construction.
	var dep world.ControlView
	if !h.Until(10*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "departure" && !v.Done && v.ATC == "ground" && v.Request == "pushback" {
				dep = v
				return true
			}
		}
		return false
	}) {
		t.Fatal("no departure asking ground to push in 10 simulated minutes")
	}

	// ON THE STAND: push, yes. Taxi, no.
	for _, want := range []string{"pushback", "pushstart"} {
		if !slices.Contains(dep.Actions, want) {
			t.Errorf("%s on its stand: want %q offered, got %v", dep.Tail, want, dep.Actions)
		}
	}
	for _, gone := range []string{"taxi", "upto", "takeoff"} {
		if slices.Contains(dep.Actions, gone) {
			t.Errorf("%s on its stand: %q must not be offered, got %v", dep.Tail, gone, dep.Actions)
		}
	}

	// Push and start approved in one.
	if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/pushstart?as=ground", dep.ID), nil); err != nil {
		t.Fatalf("push and start as ground: %v", err)
	}

	// NOTHING LEFT TO SAY. ATC has had its say; the next word on the frequency
	// is the crew's. An empty list here is the point, not a gap.
	h.Advance(5 * time.Second)
	now, ok := h.Aircraft(dep.Tail)
	if !ok {
		t.Fatalf("%s gone from the board", dep.Tail)
	}
	if len(now.Actions) != 0 {
		t.Errorf("%s cleared to push and start: want nothing offered, got %v", dep.Tail, now.Actions)
	}

	// SAYING IT AGAIN IS REFUSED, not transmitted a second time (#1269).
	if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/pushstart?as=ground", dep.ID), nil); err == nil {
		t.Errorf("%s: a second push-and-start was accepted; want it refused", dep.Tail)
	}

	// Pushed, tug clear, the crew calling ready: NOW the taxi clearance is due.
	if !h.Until(10*time.Minute, func() bool {
		v, ok := h.Aircraft(dep.Tail)
		return ok && slices.Contains(v.Actions, "taxi")
	}) {
		v, _ := h.Aircraft(dep.Tail)
		t.Fatalf("%s: taxi never came on offer (state %q, request %q, actions %v)", dep.Tail, v.State, v.Request, v.Actions)
	}
	after, _ := h.Aircraft(dep.Tail)
	if after.State == "awaiting pushback" {
		t.Errorf("%s: taxi offered while still on the stand (%s)", dep.Tail, after.State)
	}
	if after.ActionParams["upto"] != "node" {
		t.Errorf("%s: upto offered without saying it needs a node: %v", dep.Tail, after.ActionParams)
	}
	if !slices.Contains(after.Actions, "upto") {
		t.Errorf("%s ready to taxi: want upto offered beside taxi, got %v", dep.Tail, after.Actions)
	}
}
