package traffic

import (
	"fmt"
	"math"
	"testing"
	"time"
)

// TestRolloutBrakesForRunwayTraffic (#1680): an arrival on its landing roll
// brakes for an aircraft on the runway ahead of it, standing there or
// backtracking towards it, keeps the traffic clearance from its body and
// never passes through it. On a made-up field whose exits lean back, so
// the arrival itself rolls to the end to backtrack. Before, the rollout
// saw no traffic: at TNCM AAL432 rolled at 43 kt through DAL1979
// backtracking on the runway, 0.9 m apart.
func TestRolloutBrakesForRunwayTraffic(t *testing.T) {
	for _, c := range []struct {
		ahead float64 // metres past the touchdown
		kts   float64 // its speed towards the arrival
	}{{700, 0}, {900, 0}, {1200, 0}, {900, 8}} {
		t.Run(fmt.Sprintf("%.0fm_%.0fkt", c.ahead, c.kts), func(t *testing.T) { rolloutTrafficRun(t, c.ahead, c.kts) })
	}
}

// rolloutOtherKeepsM: the one coming towards the arrival stops this far
// ahead of its reference point, reference to reference, whatever the
// arrival does: what is left of the gap is the arrival's.
const rolloutOtherKeepsM = 80.0

func rolloutTrafficRun(t *testing.T, ahead, kts float64) {
	g := midExitGraph(t, 10)
	ec := &eventClient{}
	inj, pic := NewInjector(ec), NewGroundPicture()
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj), ArrivalWithGroundPicture(pic))
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "27", Parking: 0, Model: "A321", InjectApproach: true,
		HoldForClearance: true, RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1000, 12))
	mon := DefaultArrivalRequestBase + arrReqMonitor
	const other = 900
	least := TrafficGapMeters - TrafficOverrunMeters
	var path *GroundPath
	at := ahead // the other's place along the arrival's path
	closest, stopped, frame := math.Inf(1), false, 0.0
	last, lastV := math.NaN(), math.NaN()
	for i := 0; i < backtrackTrafficFps*900; i++ {
		now = now.Add(time.Second / backtrackTrafficFps)
		ctl.Handle(arrivalPositionMsg(mon, 77, g.Layout.Parking[0].Position, 0, 0, 0, false))
		m := ctl.mover
		state := ctl.State()
		if m == nil || state != ArrivalRollout {
			if path != nil {
				break // off its landing roll
			}
			continue
		}
		if path == nil {
			path = m.Path()
			if ctl.backtracking && at >= ctl.backFrom-100 || at >= ctl.clearDist {
				t.Fatalf("the other at %.0f m is not on the runway ahead (back from %.0f m, clear at %.0f m)", at, ctl.backFrom, ctl.clearDist)
			}
		}
		if m.Path() != path {
			t.Fatal("the arrival was planned again")
		}
		pose := m.Pose()
		pos := path.PointAt(at)
		facing := math.Mod(localBearing(pos, path.PointAt(at-2))+360, 360) // towards the arrival
		pic.Report(other, pos, facing, DefaultMotionProfile(), now)
		closest = math.Min(closest, noseGap(pose, ctl.profile(), pos, facing, DefaultMotionProfile()))
		if !math.IsNaN(last) {
			v := (pose.Distance - last) * backtrackTrafficFps
			if !math.IsNaN(lastV) {
				frame = math.Max(frame, math.Abs(v-lastV))
			}
			lastV = v
		}
		last = pose.Distance
		if pose.Distance >= at {
			t.Fatalf("rolled through the aircraft ahead: at %.1f m, it at %.1f m", pose.Distance, at)
		}
		if pose.Stopped {
			stopped = true
			break
		}
		// It comes on until rolloutOtherKeepsM ahead of the arrival, then
		// stands: never backing away in front of it, or one still creeping
		// up to its stop would never stand (a flake, not a stop).
		if next := at - kts*ktsToMS/backtrackTrafficFps; kts > 0 && next >= pose.Distance+rolloutOtherKeepsM {
			at = next
		}
	}
	if path == nil {
		t.Fatal("never on its landing roll")
	}
	t.Logf("closest %.2f m to the aircraft ahead, %.2f m/s in one frame", closest, frame)
	if !stopped {
		t.Errorf("never stopped for the aircraft ahead on its runway (%s)", ctl.State())
	}
	if closest < least-0.05 {
		t.Errorf("nose %.2f m from the aircraft ahead, want its %.0f m clearance", closest, least)
	}
	if frame > 0.5 {
		t.Errorf("speed changed %.2f m/s in one frame: a dead stop", frame)
	}
}
