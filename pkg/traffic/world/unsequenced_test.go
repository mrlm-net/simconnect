package world

import (
	"errors"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// stubArrival takes the instructions an arrival not in the sequence can be given.
type stubArrival struct {
	arrivalCtl
	speed float64
}

func (s *stubArrival) AssignSpeed(kts float64) (float64, error) {
	s.speed = kts
	return kts, nil
}
func (s *stubArrival) ProcedureRoute() []airport.LatLon { return nil }

// TestUnsequencedArrivalInstructions: radar instructions reach an arrival with no
// place in the landing order; what is about that order, or needs a runway to join,
// does not (#1414).
func TestUnsequencedArrivalInstructions(t *testing.T) {
	w := New(Options{DataDir: t.TempDir()})
	cc := newControlCenter(nil, w.st.core)
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case f := <-cc.cmds:
				f()
			case <-stop:
				return
			}
		}
	}()
	arr := &stubArrival{}
	cc.items[1] = &controlled{ID: 1, Tail: "CSA1", ICAO: "LKPR", cc: cc, arr: arr, removed: make(chan struct{})}
	q := &sequences{cc: cc, seq: map[string]*traffic.ApproachSequencer{}}

	if err := q.approachDo("LKPR", "CSA1", "speed", nil, 240, 0, spacing{}); err != nil {
		t.Errorf("speed refused: %v", err)
	}
	if arr.speed != 240 {
		t.Errorf("flying %.0f kt, told 240", arr.speed)
	}
	for _, action := range []string{"up", "down", "hold", "release", "slow", "joinfinal", "holdat", "direct"} {
		if err := q.approachDo("LKPR", "CSA1", action, nil, 0, 0, spacing{}); !errors.Is(err, traffic.ErrNotSequenced) {
			t.Errorf("%s: %v, want not sequenced", action, err)
		}
	}
	if err := q.approachDo("LKPR", "NOBODY", "speed", nil, 240, 0, spacing{}); !errors.Is(err, traffic.ErrNotSequenced) {
		t.Errorf("a stranger: %v", err)
	}
}
