package world

import (
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
}

func (f *standDep) FacesOut() bool        { return f.facesOut }
func (f *standDep) StartUpCleared() bool  { return f.started }
func (f *standDep) Route() *airport.Route { return nil }

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
			// The rest of the strip is untouched.
			for _, want := range []string{"taxi", "upto", "takeoff"} {
				if !slices.Contains(given, want) {
					t.Errorf("want %q still offered, got %v", want, given)
				}
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
