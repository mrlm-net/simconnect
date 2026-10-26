package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

// An arrival keeps its exit speed until its tail is clear of the runway as
// the tower counts it (40 m past the edge), not slowing to taxi speed with
// its nose just off the surface (offline LKPR: an A320 at 12 kt on D held
// the runway, the arrival behind cleared to land at 1.0 NM).
func TestArrivalKeepsExitSpeedUntilClear(t *testing.T) {
	g := lkprGraph(t)
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	st, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: st, Model: "A320", Tail: "OKABC",
		InjectApproach: true, RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	p := ctl.Plan()
	rwy, _, _ := g.Layout.RunwayEnd("24")
	a, b := rwy.Primary.Threshold, rwy.Secondary.Threshold
	mon := DefaultArrivalRequestBase + arrReqMonitor
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	var down, clear time.Time
	var kts float64
	for i := 0; i < 60*1200 && ctl.State() < ArrivalAwaitingTaxi && clear.IsZero(); i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 77, p.End.Threshold, 0, 0, 0, false))
		if down.IsZero() && ctl.State() == ArrivalRollout {
			down = now
		}
		if !down.IsZero() && ctl.mover != nil {
			q := ctl.mover.Pose()
			if math.Abs(calc.CrossTrackMeters(a.Lat, a.Lon, b.Lat, b.Lon, q.Position.Lat, q.Position.Lon)) > rwy.Width/2+40 {
				clear, kts = now, q.GroundSpeedKts
			}
		}
	}
	if clear.IsZero() {
		t.Fatalf("never clear of the runway (state %v)", ctl.State())
	}
	secs := clear.Sub(down).Seconds()
	t.Logf("exit %s: clear %.0f s after touchdown at %.0f kt", p.Exit.Taxiway, secs, kts)
	if kts < 15 {
		t.Errorf("down to %.0f kt before its tail was clear of the runway", kts)
	}
	if secs > 55 {
		t.Errorf("clear %.0f s after touchdown, want ≤ 55", secs)
	}
}
