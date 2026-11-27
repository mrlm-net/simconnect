package world

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// stretchedArrival loses what it is asked on a longer route, with no speed
// to give.
type stretchedArrival struct {
	arrivalCtl
}

func (s *stretchedArrival) AbsorbWithin(time.Duration, float64) (traffic.Absorption, error) {
	return traffic.Absorption{ExtraNM: 2}, nil
}
func (s *stretchedArrival) AbsorbDelay(time.Duration) (traffic.Absorption, error) {
	return traffic.Absorption{ExtraNM: 2}, nil
}
func (s *stretchedArrival) ProcedureRoute() []airport.LatLon { return nil }

// TestSlowNumberOnly: a "slow" for spacing that only stretches the
// arrival's route says no number alone; the number is kept for its next
// instruction (#1428; nightly v0.199.0, TestSequenceCallsPhrased:
// "Ryanair 5E, number 4").
func TestSlowNumberOnly(t *testing.T) {
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
	cs := []string{"DLH1A", "CSA2B", "EZY3C", "RYR5E"}
	for i, c := range cs {
		cc.items[i+1] = &controlled{ID: i + 1, Tail: c, ICAO: "LKPR", cc: cc, arr: &stretchedArrival{}, removed: make(chan struct{})}
	}
	s := traffic.NewApproachSequencer("24", traffic.SequencerOptions{})
	var in []traffic.ApproachAircraft
	for i, c := range cs {
		in = append(in, traffic.ApproachAircraft{Callsign: c, Wake: traffic.Wake{ICAO: traffic.WakeMedium, Recat: traffic.RecatD}, DistanceToGoNM: 10 + 5*float64(i), GroundKts: 220, FinalKts: 140, Runway: "24"})
	}
	s.Update(cc.clock.Now(), in)
	q := &sequences{cc: cc, seq: map[string]*traffic.ApproachSequencer{"LKPR 24": s}, seqSaid: map[string]seqSaid{}}
	_, e, ok := q.entryOf("LKPR", "RYR5E")
	if !ok || e.Number < 2 {
		t.Fatalf("RYR5E in the sequence %v, number %d", ok, e.Number)
	}
	if err := q.approachSpace("LKPR", "RYR5E", time.Minute, 20, nil); err != nil {
		t.Fatalf("slow: %v", err)
	}
	for _, c := range cc.agenda.waiting() {
		if c.tail == "RYR5E" {
			t.Errorf("a call for RYR5E with only its route stretched (number %d): a number alone", e.Number)
		}
	}
	if n := q.numberFor("LKPR", "RYR5E"); n != e.Number {
		t.Errorf("its number %d not kept for its next instruction (%d)", e.Number, n)
	}
}
