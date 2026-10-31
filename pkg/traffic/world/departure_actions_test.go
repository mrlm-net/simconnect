package world

import (
	"errors"
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// standDep is a departureCtl answering only what departureActions asks of it.
// (wire_test.go already has a fakeDep, for the wire's calls.)
type standDep struct {
	departureCtl
	facesOut bool
	started  bool
	pushed   bool
	taxied   bool
}

func (f *standDep) FacesOut() bool        { return f.facesOut }
func (f *standDep) Route() *airport.Route { return nil }
func (f *standDep) RunwayNext() bool      { return false }

// Cleared mirrors TaxiController.Cleared for the flags these tests set.
func (f *standDep) Cleared(action string) bool {
	switch action {
	case "pushback", "pushstart":
		return f.pushed
	case "startup":
		return f.started
	case "taxi":
		return f.taxied
	}
	return false
}

// A clearance already given is not offered again (#1231): "push and start
// approved" must not be followed by the offer to say "start up approved" to the
// same aircraft. The taxi state is identical either way, which is why answering
// from the state alone got this wrong.
func TestDepartureActionsStartUpNotOfferedTwice(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state traffic.TaxiState
	}{
		{"pushing back", traffic.TaxiPushback},
		{"pushed, awaiting taxi", traffic.TaxiAwaitingTaxi},
	} {
		t.Run(tc.name, func(t *testing.T) {
			notYet := departureActions(tc.state, "", &standDep{})
			if !slices.Contains(notYet, "startup") {
				t.Fatalf("start-up not yet cleared: want it offered, got %v", notYet)
			}
			given := departureActions(tc.state, "", &standDep{started: true})
			if slices.Contains(given, "startup") {
				t.Errorf("start-up already cleared: must not be offered again, got %v", given)
			}
		})
	}
}

// A stand the aircraft taxis straight out of asks for start-up instead of a
// pushback — and once that is given, it is not asked for again either.
func TestDepartureActionsFacesOut(t *testing.T) {
	notYet := departureActions(traffic.TaxiAwaitingPushback, "", &standDep{facesOut: true})
	if !slices.Contains(notYet, "startup") || slices.Contains(notYet, "pushback") {
		t.Fatalf("faces out: want start-up and no pushback, got %v", notYet)
	}
	// Nothing to push away from: "start up and taxi" is one call and one
	// answer, so the taxi clearance IS due here.
	if !slices.Contains(notYet, "taxi") {
		t.Errorf("faces out: want taxi offered (it taxis straight out), got %v", notYet)
	}
	given := departureActions(traffic.TaxiAwaitingPushback, "", &standDep{facesOut: true, started: true})
	if slices.Contains(given, "startup") {
		t.Errorf("faces out, already started: must not offer start-up again, got %v", given)
	}
}

// Push-and-start is a clearance a controller says at a stand more often than the
// two separately, and the engine accepts it; it is now suggested rather than left
// for a host to add by hand (#1231).
func TestDepartureActionsOffersPushAndStart(t *testing.T) {
	got := departureActions(traffic.TaxiAwaitingPushback, "", &standDep{})
	if !slices.Contains(got, "pushback") || !slices.Contains(got, "pushstart") {
		t.Errorf("at a stand: want both pushback and pushstart, got %v", got)
	}
}

// A nil controller is "not known": the clearance stays on offer rather than being
// hidden on a guess.
func TestDepartureActionsNilController(t *testing.T) {
	if got := departureActions(traffic.TaxiPushback, "", nil); !slices.Contains(got, "startup") {
		t.Errorf("nil controller: want start-up still offered, got %v", got)
	}
}

// The case a player hit at Esenboga: push-and-start given, the aircraft still
// "awaiting pushback" because the push has not begun — and Pushback was still
// the offered next clearance, for an aeroplane already cleared to push (#1231).
func TestDepartureActionsPushNotOfferedAfterPushAndStart(t *testing.T) {
	given := departureActions(traffic.TaxiAwaitingPushback, "", &standDep{pushed: true, started: true})
	for _, gone := range []string{"pushback", "pushstart", "startup"} {
		if slices.Contains(given, gone) {
			t.Errorf("already cleared to push and start: %q must not be offered, got %v", gone, given)
		}
	}
	// And nothing else either. An aeroplane cleared to push and start, with
	// the tug on its way, has had every word ATC has for it; the next one on
	// the frequency is the crew's, asking to taxi.
	if len(given) != 0 {
		t.Errorf("cleared to push and start: want nothing left to say, got %v", given)
	}
}

// NO TAXI CLEARANCE BEFORE THE AIRCRAFT HAS ASKED (#1268). The player's
// complaint: "the sequence of commands … cannot give taxi when it hasn't
// called ground". On the stand the crew has asked for a push; it has not
// asked to taxi, and a tug is attached, so it could not go if it were told
// to. The clearance is due at TaxiAwaitingTaxi, where the crew calls ready.
func TestDepartureActionsNoTaxiOnTheStand(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state traffic.TaxiState
		ctl   *standDep
	}{
		{"on the stand, nothing given", traffic.TaxiAwaitingPushback, &standDep{}},
		{"on the stand, push approved", traffic.TaxiAwaitingPushback, &standDep{pushed: true}},
		{"being pushed", traffic.TaxiPushback, &standDep{pushed: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := departureActions(tc.state, "", tc.ctl)
			for _, gone := range []string{"taxi", "upto"} {
				if slices.Contains(got, gone) {
					t.Errorf("%s: %q must not be offered, got %v", tc.name, gone, got)
				}
			}
		})
	}
	// Pushed, tug clearing, the crew asking: now it is due.
	due := departureActions(traffic.TaxiAwaitingTaxi, "", &standDep{pushed: true, started: true})
	for _, want := range []string{"taxi", "upto"} {
		if !slices.Contains(due, want) {
			t.Errorf("awaiting taxi: want %q offered, got %v", want, due)
		}
	}
}

// A TAKE-OFF CLEARANCE IS OFFERED AT THE RUNWAY, NOT ON THE STAND (#1231).
// It was on offer from the push onwards, which is not a thing anyone says.
func TestDepartureActionsTakeoffOnlyAtTheRunway(t *testing.T) {
	ctl := &standDep{pushed: true, started: true}
	for _, s := range []traffic.TaxiState{traffic.TaxiAwaitingPushback, traffic.TaxiPushback, traffic.TaxiAwaitingTaxi, traffic.TaxiTaxiing} {
		if got := departureActions(s, "", ctl); slices.Contains(got, "takeoff") {
			t.Errorf("%v: take-off must not be offered away from the runway, got %v", s, got)
		}
	}
	// At its own holding point it is exactly what the tower says next.
	if got := departureActions(traffic.TaxiHoldingShort, "", ctl); !slices.Contains(got, "takeoff") {
		t.Errorf("holding short of its runway: want take-off offered, got %v", got)
	}
	if got := departureActions(traffic.TaxiLinedUp, "", ctl); !slices.Contains(got, "takeoff") {
		t.Errorf("lined up: want take-off offered, got %v", got)
	}
}

// A clearance that means something when it is said again is never withheld: a
// taxi clearance with a limit in force lifts the limit, and hold position is
// always sayable. The IDENTICAL one is not (#1269) — see TaxiController.Cleared.
func TestDepartureActionsRepeatableClearancesSurvive(t *testing.T) {
	ctl := &standDep{pushed: true, started: true}
	for _, tc := range []struct {
		state traffic.TaxiState
		want  []string
	}{
		{traffic.TaxiTaxiing, []string{"hold", "taxi", "upto"}},
		{traffic.TaxiAwaitingTaxi, []string{"taxi", "upto"}},
	} {
		got := departureActions(tc.state, "", ctl)
		for _, w := range tc.want {
			if !slices.Contains(got, w) {
				t.Errorf("%v: want %q offered, got %v", tc.state, w, got)
			}
		}
	}
	// With it in force and no limit left, it is the clearance already given:
	// "taxi to holding point runway 24" to an aircraft taxiing there says
	// nothing, and it was being said three times over.
	taxiing := departureActions(traffic.TaxiTaxiing, "", &standDep{pushed: true, started: true, taxied: true})
	if slices.Contains(taxiing, "taxi") {
		t.Errorf("already taxiing on an unlimited clearance: taxi must not be offered again, got %v", taxiing)
	}
	// `upto` stays: a limit is a new clearance whatever came before it.
	if !slices.Contains(taxiing, "upto") {
		t.Errorf("want upto still offered (a new limit is a new clearance), got %v", taxiing)
	}
}

// A taxi clearance in other words, a changed route or runway, still goes out;
// the once-only ones are refused (#1270 over #1275, which stops the same words
// before act).
func TestActTaxiInForceNotRefused(t *testing.T) {
	it := &controlled{dep: &standDep{pushed: true, started: true, taxied: true}}
	if err := it.act("taxi", -1); err != nil {
		t.Errorf("taxi in other words: %v", err)
	}
	if err := it.act("pushback", -1); !errors.Is(err, traffic.ErrAlreadyCleared) {
		t.Errorf("pushback again: want ErrAlreadyCleared, got %v", err)
	}
}

func (f *standDep) ClearToTaxi() {}

// On the stand the offer follows the crew, not the taxi state (#1308).
func TestAskedFor(t *testing.T) {
	await, pushing, ready := traffic.TaxiAwaitingPushback.String(), traffic.TaxiPushback.String(), traffic.TaxiAwaitingTaxi.String()
	for _, tc := range []struct {
		name      string
		state     string
		delivered bool
		called    string
		want      []string
	}{
		{"with delivery", await, false, "", nil},
		{"with delivery, its call waiting", await, false, "pushback", nil},
		{"delivered, not called", await, true, "", nil},
		{"asks to push", await, true, "pushback", []string{"pushback", "pushstart"}},
		{"faces out, asks to start", await, true, "start_up", []string{"startup", "taxi", "upto"}},
		{"being pushed", pushing, true, "", nil},
		{"pushed, tug still there", ready, true, "", nil},
		{"pushed, asks to start", ready, true, "start_up", []string{"startup"}},
		{"pushed, asks to taxi", ready, true, "taxi", []string{"taxi", "upto"}},
		{"taxiing, not delivered", traffic.TaxiTaxiing.String(), false, "", []string{"pushback", "pushstart", "startup", "taxi", "upto"}},
	} {
		all := []string{"pushback", "pushstart", "startup", "taxi", "upto"}
		if got := askedFor(all, tc.state, tc.delivered, tc.called); !slices.Equal(got, tc.want) && (len(got) > 0 || len(tc.want) > 0) {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The offer says which actions cannot be given on their own (#1308).
func TestActionParams(t *testing.T) {
	got := actionParams([]string{"taxi", "upto", "climb"})
	if len(got) != 2 || got["upto"] != "node" || got["climb"] != "level" {
		t.Errorf("params: %v", got)
	}
	if got := actionParams([]string{"taxi"}); got != nil {
		t.Errorf("none needed: %v", got)
	}
}
