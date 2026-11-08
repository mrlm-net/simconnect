package traffic

import (
	"errors"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

// The stop on the stand moves with SetStandPlace (the taxi-in planned again
// to the same stand), not after touchdown.
func TestArrivalSetStandPlace(t *testing.T) {
	ctl, _ := startArrival(t)
	g := ctl.req.Graph
	stand := ctl.req.Parking
	before := ctl.Plan().Stop
	nose := ctl.req.NoseOffset
	if nose <= 0 {
		nose = DefaultNoseOffsetMeters
	}
	if changed, err := ctl.SetStandPlace(nose-9, 0); err != nil || !changed {
		t.Fatalf("SetStandPlace: %v %v", changed, err)
	}
	p := ctl.Plan()
	if ctl.req.Parking != stand || ctl.req.NoseOffset != nose-9 {
		t.Errorf("parking %d (want %d), nose offset %.2f (want %.2f)", ctl.req.Parking, stand, ctl.req.NoseOffset, nose-9)
	}
	want, err := StandStop(g, p.Route, nose-9)
	if err != nil {
		t.Fatal(err)
	}
	if d := calc.HaversineMeters(p.Stop.Lat, p.Stop.Lon, want.Lat, want.Lon); d > 1 {
		t.Errorf("stop %.1f m from the new place", d)
	}
	if d := calc.HaversineMeters(p.Stop.Lat, p.Stop.Lon, before.Lat, before.Lon); d < 8 || d > 10 {
		t.Errorf("stop moved %.1f m, want 9", d)
	}
	if changed, err := ctl.SetStandPlace(nose-9, 0); err != nil || changed {
		t.Errorf("the same place again: %v %v", changed, err)
	}
	ctl.mu.Lock()
	ctl.state = ArrivalRollout
	ctl.mu.Unlock()
	if _, err := ctl.SetStandPlace(nose, 0); !errors.Is(err, ErrStandFixed) {
		t.Errorf("after touchdown: %v", err)
	}
	if ctl.req.NoseOffset != nose-9 {
		t.Errorf("nose offset %.2f after a refusal", ctl.req.NoseOffset)
	}
}
