package traffic

import (
	"fmt"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Limits of TestBacktrackTrafficAhead: the most the speed may change over
// half a second (m/s²), and the nose tip's least distance to the body
// ahead, the traffic gap less what may be run on into it.
const (
	backtrackTrafficAccel = 2.0
	backtrackTrafficFps   = 60
)

// TestBacktrackTrafficAhead (#1640): off a backtrack at 10 kt with an
// aircraft standing nose on in its way, the arrival brakes at a rate,
// never to nearer than the traffic's clearance, stands still while
// awaiting taxi and as it is cleared, and taxis off at a rate once the
// other has gone. Before, it saw the traffic only once off the runway and
// stood from 10 kt in one frame, crept on into the gap while awaiting taxi
// and stood again in one frame as it taxied.
func TestBacktrackTrafficAhead(t *testing.T) {
	for _, beyond := range []float64{22, 26, 30, 34, 38, 45, 60} {
		t.Run(fmt.Sprintf("%.0fm", beyond), func(t *testing.T) { backtrackTrafficRun(t, beyond) })
	}
}

func backtrackTrafficRun(t *testing.T, beyond float64) {
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
	var at, speeds []float64
	var stoppedAt, stillS time.Time
	waitS, waitMoved, closest := math.NaN(), 0.0, math.Inf(1)
	cleared, gone, saw, waited := false, false, false, false
	last := math.NaN()
	for i := 0; i < backtrackTrafficFps*1500; i++ {
		now = now.Add(time.Second / backtrackTrafficFps)
		var pos airport.LatLon
		facing := 0.0
		m := ctl.mover
		if m != nil && !gone {
			p := m.Path()
			pos = p.PointAt(ctl.clearDist + beyond)
			facing = math.Mod(localBearing(pos, p.PointAt(ctl.clearDist+beyond-2))+360, 360)
			pic.Report(other, pos, facing, DefaultMotionProfile(), now)
		}
		ctl.Handle(arrivalPositionMsg(mon, 77, g.Layout.Parking[0].Position, 0, 0, 0, false))
		if m = ctl.mover; m == nil {
			continue
		}
		pose, state := m.Pose(), ctl.State()
		if state.Terminal() {
			break
		}
		if pose.Distance < ctl.clearDist-150 {
			continue
		}
		if !math.IsNaN(last) {
			at = append(at, pose.Distance)
			speeds = append(speeds, (pose.Distance-last)*backtrackTrafficFps)
		}
		last = pose.Distance
		if !gone {
			if body, who := pic.blocking(77, m.Path(), pose.Distance, TrafficLookMeters, ctl.profile().SpanMeters/2, now); who.id == other {
				saw = saw || !math.IsInf(body, 1)
			}
			if saw {
				closest = math.Min(closest, noseGap(pose, ctl.profile(), pos, facing, DefaultMotionProfile()))
			}
		}
		if state == ArrivalAwaitingTaxi {
			if math.IsNaN(waitS) {
				waitS, waited = pose.Distance, true
			}
			waitMoved = math.Max(waitMoved, pose.Distance-waitS)
		}
		if !pose.Stopped {
			stoppedAt = time.Time{}
		} else if stoppedAt.IsZero() {
			stoppedAt = now
		}
		still := !stoppedAt.IsZero() && now.Sub(stoppedAt) > 20*time.Second
		switch {
		case !cleared && still && state == ArrivalAwaitingTaxi:
			ctl.ClearToTaxi()
			cleared, stillS = true, now
		case !gone && cleared && now.Sub(stillS) > 15*time.Second,
			!gone && !cleared && still && state != ArrivalAwaitingTaxi:
			gone, stillS = true, now
		}
		if gone && now.Sub(stillS) > 70*time.Second {
			break
		}
	}
	if !saw {
		t.Fatal("the aircraft ahead was never in its way")
	}
	if !gone {
		t.Fatalf("never stopped for it (%s)", ctl.State())
	}
	worst, where := 0.0, 0.0
	for i := backtrackTrafficFps / 2; i < len(speeds); i++ {
		if a := math.Abs(speeds[i]-speeds[i-backtrackTrafficFps/2]) * 2; a > worst {
			worst, where = a, at[i]-ctl.clearDist
		}
	}
	frame := 0.0
	for i := 1; i < len(speeds); i++ {
		frame = math.Max(frame, math.Abs(speeds[i]-speeds[i-1]))
	}
	t.Logf("up to %.2f m/s² (%.1f m past the edge), %.2f m/s a frame, closest %.2f m, moved %.2f m waiting, top %.1f m/s", worst, where, frame, closest, waitMoved, slices.Max(speeds))
	if worst > backtrackTrafficAccel {
		t.Errorf("speed changed at %.2f m/s², want at most %.1f", worst, backtrackTrafficAccel)
	}
	if frame > 0.5 {
		t.Errorf("speed changed %.2f m/s in one frame: a dead stop", frame)
	}
	if closest < least-0.05 {
		t.Errorf("nose %.2f m from the aircraft ahead, want its %.0f m clearance", closest, least)
	}
	if waited && waitMoved > 0.05 {
		t.Errorf("moved %.2f m while awaiting taxi", waitMoved)
	}
	if n := len(at); n == 0 || at[n-1] < ctl.clearDist+5 {
		t.Errorf("never taxied on once the way was free")
	}
}

// TestTrafficFoundNearKeepsClearance (#1640): traffic found too near to
// stop the gap behind it is braked for at a rate, running on into the gap
// as far as its clearance and never nearer. Where the room is there (the
// run-on's stop within the clearance) it never takes more than 2 m/s²;
// with less it stops harder, still no nearer. Before, with its stop behind
// it, it stood from 10 kt in one frame.
func TestTrafficFoundNearKeepsClearance(t *testing.T) {
	prof := DefaultMotionProfile()
	path, err := NewGroundPath([]airport.LatLon{lkpr, offset(lkpr, 0, 700)}, prof)
	if err != nil {
		t.Fatal(err)
	}
	least := TrafficGapMeters - TrafficOverrunMeters
	noseTip := (pushNoseFactor - 1) * prof.WheelbaseMeters
	for _, kts := range []float64{4, 7, 10} {
		v := kts * ktsToMS
		// The room a stop at 2 m/s² takes, with the frames its look ahead
		// and the brakes take to come.
		need := v*v/(2*backtrackTrafficAccel) + v*0.6
		for _, room := range []float64{need / 2, need, need + 3, need + 8} {
			m := NewGroundMoverFrom(path, prof, 0, kts)
			pic := NewGroundPicture()
			d := groundDrive{mover: m, picture: pic, object: 1, prof: prof, followTraffic: true}
			now := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
			// A body appears: its nearest point this far ahead of the nose
			// tip, the clearance in it.
			const otherNose = 12.6*1.35 - 1.0
			at := path.PointAt(m.Pose().Distance + noseTip + least + room + otherNose)
			var speeds []float64
			closest, last := math.Inf(1), m.Pose().Distance
			for range backtrackTrafficFps * 30 {
				now = now.Add(time.Second / backtrackTrafficFps)
				pic.Report(9, at, 180, prof, now)
				d.followAhead(now)
				p := m.Step(1.0 / backtrackTrafficFps)
				speeds = append(speeds, (p.Distance-last)*backtrackTrafficFps)
				last = p.Distance
				closest = math.Min(closest, noseGap(p, prof, at, 180, prof))
			}
			if math.IsInf(closest, 1) || !m.Pose().Stopped {
				t.Errorf("%.0f kt, %.1f m of room: did not stop for it", kts, room)
				continue
			}
			if closest < least-0.05 {
				t.Errorf("%.0f kt, %.1f m of room: closed to %.2f m, want its %.0f m clearance", kts, room, closest, least)
			}
			if room < need {
				continue
			}
			worst, prev := 0.0, v
			for i, s := range speeds {
				if i >= backtrackTrafficFps/2 {
					prev = speeds[i-backtrackTrafficFps/2]
				}
				worst = math.Max(worst, (prev-s)*2)
			}
			t.Logf("%.0f kt, %.1f m of room: up to %.2f m/s², closest %.2f m", kts, room, worst, closest)
			if worst > backtrackTrafficAccel {
				t.Errorf("%.0f kt, %.1f m of room: braked at %.2f m/s², want at most %.1f", kts, room, worst, backtrackTrafficAccel)
			}
		}
	}
}

// noseGap is how far the nose tip of an aircraft at pose is from the
// true nose of one standing at pos facing hdg, by the two airframes
// alone (blocking, which the stop comes from, is not asked); negative
// once the tip is past that nose, into the body.
func noseGap(pose GroundPose, mine MotionProfile, pos airport.LatLon, hdg float64, theirs MotionProfile) float64 {
	tip := offsetHeading(pose.Position, pose.Heading, mine.WheelbaseMeters*pushNoseFactor-mine.RefAheadMeters)
	nose := offsetHeading(pos, hdg, theirs.WheelbaseMeters*pushNoseFactor-theirs.RefAheadMeters)
	gap := localDist(tip, nose)
	if gap < 30 && math.Abs(headingDiff(localBearing(tip, nose), hdg)) < 90 {
		return -gap
	}
	return gap
}

// TestBlockingSeesTheNose (#1640): the body an aircraft stops for begins
// at the true nose of one facing it, whatever its length. Before, the
// axis was sampled every 5 m from the tail and up to 5 m of nose were
// not there.
func TestBlockingSeesTheNose(t *testing.T) {
	mine := DefaultMotionProfile()
	path, err := NewGroundPath([]airport.LatLon{lkpr, offset(lkpr, 0, 300)}, mine)
	if err != nil {
		t.Fatal(err)
	}
	for tail := 4.0; tail <= 40; tail += 0.7 {
		theirs := mine
		theirs.TailMeters, theirs.SpanMeters = tail, 20
		pic := NewGroundPicture()
		now := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
		pos := path.PointAt(120)
		pic.Report(9, pos, 180, theirs, now)
		body, who := pic.blocking(1, path, 0, 200, mine.SpanMeters/2, now)
		nose := offsetHeading(pos, 180, theirs.WheelbaseMeters*pushNoseFactor-theirs.RefAheadMeters)
		if want := localDist(path.PointAt(0), nose); who.id != 9 || body > want+0.05 {
			t.Errorf("tail %.1f m: body from %.2f m along, its nose is at %.2f m", tail, body, want)
		}
	}
}
