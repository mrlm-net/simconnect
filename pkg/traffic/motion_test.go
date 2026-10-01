//go:build windows
// +build windows

package traffic

import (
	"errors"
	"math"
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// offset returns the point east/north metres from o.
func offset(o airport.LatLon, east, north float64) airport.LatLon {
	return airport.LatLon{Lat: o.Lat + north/metersPerDegree, Lon: o.Lon + east/(metersPerDegree*math.Cos(o.Lat*math.Pi/180))}
}

var lkpr = airport.LatLon{Lat: 50.1008, Lon: 14.26}

// drive steps m at 60 Hz until it arrives or maxSeconds pass, returning the
// poses.
func drive(m *GroundMover, maxSeconds float64) []GroundPose {
	var out []GroundPose
	for t := 0.0; t < maxSeconds; t += 1.0 / 60 {
		p := m.Step(1.0 / 60)
		out = append(out, p)
		if p.Arrived {
			break
		}
	}
	return out
}

func TestGroundMoverStraight(t *testing.T) {
	prof := DefaultMotionProfile()
	path, err := NewGroundPath([]airport.LatLon{lkpr, offset(lkpr, 0, 600)}, prof)
	if err != nil {
		t.Fatal(err)
	}
	poses := drive(NewGroundMover(path, prof), 300)
	last := poses[len(poses)-1]
	if !last.Arrived || math.Abs(last.Distance-path.Length()) > 0.05 {
		t.Fatalf("did not arrive at the end: %+v (length %.1f)", last, path.Length())
	}
	top, dt := 0.0, 1.0/60
	var acc []float64
	for i, p := range poses {
		top = math.Max(top, p.GroundSpeedKts)
		if i > 0 {
			acc = append(acc, (p.GroundSpeedKts-poses[i-1].GroundSpeedKts)*ktsToMS/dt)
		}
		if math.Abs(headingDiff(p.Heading, 0)) > 0.01 {
			t.Fatalf("heading %.3f on a straight north path", p.Heading)
		}
	}
	if math.Abs(top-prof.CruiseKts) > 0.1 {
		t.Errorf("top speed %.2f kt, want cruise %.0f", top, prof.CruiseKts)
	}
	for i := 1; i < len(acc)-1; i++ { // the final settle may snap
		if acc[i] > prof.Accel+1e-6 || acc[i] < -1.5*prof.Decel-1e-6 {
			t.Fatalf("acceleration %.3f m/s² at step %d outside limits", acc[i], i)
		}
		if j := math.Abs(acc[i]-acc[i-1]) / dt; j > prof.Jerk+1e-3 {
			t.Fatalf("jerk %.3f m/s³ at step %d above %.2f", j, i, prof.Jerk)
		}
	}
}

func TestGroundMoverTurnEasesInAndOut(t *testing.T) {
	prof := DefaultMotionProfile()
	path, err := NewGroundPath([]airport.LatLon{lkpr, offset(lkpr, 0, 300), offset(lkpr, 300, 300)}, prof)
	if err != nil {
		t.Fatal(err)
	}
	poses := drive(NewGroundMover(path, prof), 300)
	last := poses[len(poses)-1]
	if !last.Arrived {
		t.Fatalf("did not arrive: %+v", last)
	}
	if math.Abs(headingDiff(last.Heading, 90)) > 1 {
		t.Errorf("final heading %.1f, want 90", last.Heading)
	}
	slowest, maxRate, maxRateChange := math.Inf(1), 0.0, 0.0
	prevRate := 0.0
	for i := 1; i < len(poses); i++ {
		p := poses[i]
		if p.Distance > 150 && p.Distance < 450 {
			slowest = math.Min(slowest, p.GroundSpeedKts)
		}
		rate := headingDiff(poses[i-1].Heading, p.Heading) * 60 // °/s
		maxRate = math.Max(maxRate, math.Abs(rate))
		maxRateChange = math.Max(maxRateChange, math.Abs(rate-prevRate))
		prevRate = rate
	}
	if slowest > prof.CruiseKts-2 || slowest < prof.MinTurnKts {
		t.Errorf("slowest speed in the turn %.1f kt, want below cruise and above %.0f", slowest, prof.MinTurnKts)
	}
	if maxRate > 15 { // airliner taxi turns peak around 10–15°/s
		t.Errorf("turn rate up to %.1f°/s, want a gentle turn", maxRate)
	}
	// Eased: the turn rate builds up and dies away without steps.
	if maxRateChange > 0.5 {
		t.Errorf("turn rate jumps by %.2f°/s between frames", maxRateChange)
	}
}

func TestGroundMoverGeometryAndNoDrift(t *testing.T) {
	prof := DefaultMotionProfile()
	path, _ := NewGroundPath([]airport.LatLon{lkpr, offset(lkpr, 0, 100), offset(lkpr, 80, 180)}, prof)
	m := NewGroundMover(path, prof)
	poses := drive(m, 200)
	for _, p := range poses {
		nose := path.PointAt(p.Distance)
		if d := localDist(nose, p.Position); math.Abs(d-(prof.WheelbaseMeters-prof.RefAheadMeters)) > 0.01 {
			t.Fatalf("reference point %.3f m from the nose gear, want %.1f", d, prof.WheelbaseMeters-prof.RefAheadMeters)
		}
	}
	// Standing still for a long time must not move or turn the aircraft
	// (seen live as a slow pivot after stopping).
	stop := m.Pose()
	for range 6000 {
		m.Step(1.0 / 60)
	}
	if p := m.Pose(); localDist(p.Position, stop.Position) > 1e-6 || p.Heading != stop.Heading {
		t.Errorf("stopped aircraft drifted: %+v → %+v", stop, p)
	}
}

func TestGroundMoverHold(t *testing.T) {
	prof := DefaultMotionProfile()
	path, _ := NewGroundPath([]airport.LatLon{lkpr, offset(lkpr, 0, 800)}, prof)
	m := NewGroundMover(path, prof)
	m.Step(40) // at cruise
	m.HoldAt(400)
	var p GroundPose
	for range 60 * 120 {
		if p = m.Step(1.0 / 60); p.Stopped {
			break
		}
	}
	if !p.Stopped || math.Abs(p.Distance-400) > 0.3 || p.Arrived {
		t.Fatalf("hold: %+v, want stopped at 400 m", p)
	}
	m.Step(10)
	if m.Pose().Distance != p.Distance {
		t.Fatalf("moved while holding")
	}
	m.ClearHold()
	if last := drive(m, 300); !last[len(last)-1].Arrived {
		t.Fatalf("did not continue after ClearHold")
	}
}

func TestNewGroundPathTooShort(t *testing.T) {
	if _, err := NewGroundPath([]airport.LatLon{lkpr, lkpr}, DefaultMotionProfile()); !errors.Is(err, ErrPathTooShort) {
		t.Fatalf("err = %v, want ErrPathTooShort", err)
	}
}

// eventClient records client events on top of fakeClient.
type eventClient struct {
	fakeClient
	mapped map[uint32]string
	events []string
}

func (c *eventClient) MapClientEventToSimEvent(id uint32, name string) error {
	if c.mapped == nil {
		c.mapped = map[uint32]string{}
	}
	c.mapped[id] = name
	return nil
}

func (c *eventClient) TransmitClientEvent(obj, id, data, _ uint32, flags types.SIMCONNECT_EVENT_FLAG) error {
	if flags != types.SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY {
		return errors.New("group ID is not a priority")
	}
	c.events = append(c.events, c.mapped[id]+map[uint32]string{0: "=0", 1: "=1"}[data])
	return nil
}

func groundMsg(req, obj uint32, groundFt, cgFt float64) engine.Message {
	var hdr types.SIMCONNECT_RECV_SIMOBJECT_DATA
	off := int(unsafe.Offsetof(hdr.DwData))
	buf := make([]byte, off+int(unsafe.Sizeof(injectGround{})))
	h := (*types.SIMCONNECT_RECV_SIMOBJECT_DATA)(unsafe.Pointer(&buf[0]))
	h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA)
	h.DwRequestID, h.DwObjectID = types.DWORD(req), types.DWORD(obj)
	*(*injectGround)(unsafe.Pointer(&buf[off])) = injectGround{groundFt, cgFt}
	return engine.Message{SIMCONNECT_RECV: (*types.SIMCONNECT_RECV)(unsafe.Pointer(&buf[0]))}
}

func TestInjector(t *testing.T) {
	c := &eventClient{}
	inj := NewInjector(c)
	const obj = 42
	if err := inj.Takeover(obj); err != nil {
		t.Fatal(err)
	}
	if len(c.released) != 1 || c.released[0] != obj {
		t.Errorf("released %v, want [42]", c.released)
	}
	if want := []string{"FREEZE_LATITUDE_LONGITUDE_SET=1", "FREEZE_ALTITUDE_SET=1", "FREEZE_ATTITUDE_SET=1"}; !equalStrings(c.events, want) {
		t.Errorf("takeover events %v, want %v", c.events, want)
	}
	if len(c.periods) != 1 || c.periods[0] != types.SIMCONNECT_PERIOD_SIM_FRAME {
		t.Errorf("ground request periods %v", c.periods)
	}

	pose := GroundPose{Position: lkpr, Heading: 214}
	if err := inj.Place(obj, pose); !errors.Is(err, ErrGroundUnknown) {
		t.Fatalf("Place before ground height: %v", err)
	}
	if ok, _ := inj.Handle(groundMsg(DefaultInjectRequestBase+1, obj, 1200, 12.25)); !ok {
		t.Fatal("ground height not consumed")
	}
	if err := inj.Place(obj, pose); err != nil {
		t.Fatal(err)
	}
	var got types.SIMCONNECT_DATA_INITPOSITION
	copy(unsafe.Slice((*byte)(unsafe.Pointer(&got)), unsafe.Sizeof(got)), c.waypoints[len(c.waypoints)-1])
	if got.Altitude != 1212.25 || got.OnGround != 1 || got.Heading != 214 || got.Latitude != lkpr.Lat {
		t.Errorf("placed %+v", got)
	}

	c.events = nil
	if err := inj.SetLights(obj, LightsTaxi); err != nil {
		t.Fatal(err)
	}
	if len(c.events) != 7 {
		t.Errorf("first SetLights sent %v, want all 7 lights", c.events)
	}
	c.events = nil
	inj.SetLights(obj, LightsRunway)
	if want := []string{"STROBES_SET=1", "LANDING_LIGHTS_SET=1"}; !equalStrings(c.events, want) {
		t.Errorf("SetLights runway sent %v, want %v", c.events, want)
	}

	c.events = nil
	if err := inj.Release(obj); err != nil {
		t.Fatal(err)
	}
	if want := []string{"FREEZE_LATITUDE_LONGITUDE_SET=0", "FREEZE_ALTITUDE_SET=0", "FREEZE_ATTITUDE_SET=0"}; !equalStrings(c.events, want) {
		t.Errorf("release events %v, want %v", c.events, want)
	}
	if c.periods[len(c.periods)-1] != types.SIMCONNECT_PERIOD_NEVER {
		t.Error("ground request not stopped")
	}
	if err := inj.Place(obj, pose); !errors.Is(err, ErrNotInjected) {
		t.Errorf("Place after Release: %v", err)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestGroundPathLimitEnd(t *testing.T) {
	prof := DefaultMotionProfile()
	path, _ := NewGroundPath([]airport.LatLon{lkpr, offset(lkpr, 0, 400)}, prof)
	path.LimitEnd(30, 5)
	poses := drive(NewGroundMover(path, prof), 300)
	for _, p := range poses {
		if path.Length()-p.Distance < 30 && p.GroundSpeedKts > 5.3 { // the jerk limit lets speed trail the plan slightly
			t.Fatalf("%.1f kt %.1f m before the end, want at most 5", p.GroundSpeedKts, path.Length()-p.Distance)
		}
	}
	if !poses[len(poses)-1].Arrived {
		t.Fatal("did not arrive")
	}
}

// TestPushbackMover: pushed straight back then onto a taxiway to the east,
// the aircraft keeps facing the stand direction while straight and ends
// facing away from the turn (nose west of the tail), without jumps.
func TestPushbackMover(t *testing.T) {
	prof := DefaultMotionProfile()
	prof.CruiseKts, prof.MinTurnKts = PushbackSpeedKts, 1
	// The aircraft faces north; the tail is pushed 60 m south, then east.
	gear := offset(lkpr, 0, -prof.RefAheadMeters)
	path, err := NewArcPath([]airport.LatLon{gear, offset(gear, 0, -60), offset(gear, 50, -60)}, prof, PushbackArcMeters)
	if err != nil {
		t.Fatal(err)
	}
	m := NewPushbackMover(path, prof, 0)
	start := m.Pose()
	if localDist(start.Position, lkpr) > 0.01 || math.Abs(headingDiff(start.Heading, 0)) > 0.01 {
		t.Fatalf("start %+v, want at the stand facing north", start)
	}
	poses := drive(m, 600)
	last := poses[len(poses)-1]
	if !last.Arrived {
		t.Fatalf("pushback did not finish: %+v", last)
	}
	// Tail went east, so the nose points west (heading ~270).
	if hd := headingDiff(last.Heading, 270); math.Abs(hd) > 25 {
		t.Errorf("heading after the push %.1f, want about 270", last.Heading)
	}
	top := 0.0
	for i, p := range poses {
		top = math.Max(top, p.GroundSpeedKts)
		if i > 0 && localDist(poses[i-1].Position, p.Position) > 0.1 {
			t.Fatalf("pushback jumped %.2f m at frame %d/%d, %.2f m along, %.2f kt, arrived %v", localDist(poses[i-1].Position, p.Position), i, len(poses), p.Distance, p.GroundSpeedKts, p.Arrived)
		}
		if p.Distance < 30 && math.Abs(headingDiff(p.Heading, 0)) > 0.5 { // the arc starts CornerMeters before the corner
			t.Fatalf("turned during the straight push: %.1f at %.0f m", p.Heading, p.Distance)
		}
	}
	if top > PushbackSpeedKts+0.1 {
		t.Errorf("pushback at %.1f kt", top)
	}
	// An arc, not a pivot: the heading turns gradually (GSX-style pushback).
	maxRate := 0.0
	for i := 1; i < len(poses); i++ {
		maxRate = math.Max(maxRate, math.Abs(headingDiff(poses[i-1].Heading, poses[i].Heading))*60)
	}
	if maxRate > 9 {
		t.Errorf("heading turns up to %.1f°/s during the push: a pivot, not an arc", maxRate)
	}
	t.Logf("final heading %.1f after %.0f m", last.Heading, last.Distance)
}

// Traffic turning up inside the braking distance: the aircraft brakes
// firmly but continuously, never stopping dead on the spot (live it looked
// like a glitch), and comes to rest within TrafficOverrunMeters past the
// traffic stop, inside the gap kept behind the traffic.
func TestGroundMoverBrakesSmoothlyForTraffic(t *testing.T) {
	prof := DefaultMotionProfile()
	path, err := NewGroundPath([]airport.LatLon{lkpr, offset(lkpr, 0, 600)}, prof)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		ahead, maxDecel float64
	}{
		{30, TrafficBrakeFactor * prof.Decel}, // found ahead as usual: firm braking at most
		{12, 4},                               // found right ahead: hard, but no dead stop
	} {
		m := NewGroundMover(path, prof)
		for m.Pose().GroundSpeedKts < 14 {
			m.Step(1.0 / 60)
		}
		at := m.Pose().Distance + c.ahead
		m.SetTrafficStop(at)
		prev := m.Pose().GroundSpeedKts * ktsToMS
		maxDecel := 0.0
		for i := 0; i < 60*60 && m.Pose().GroundSpeedKts > 0; i++ {
			v := m.Step(1.0/60).GroundSpeedKts * ktsToMS
			maxDecel = math.Max(maxDecel, (prev-v)*60)
			prev = v
		}
		if maxDecel > c.maxDecel+0.05 {
			t.Errorf("%.0f m ahead: braked at %.2f m/s², want at most %.2f (no dead stop)", c.ahead, maxDecel, c.maxDecel)
		}
		if d := m.Pose().Distance; d > at+TrafficOverrunMeters+0.01 {
			t.Errorf("%.0f m ahead: stopped %.1f m past the traffic stop, want at most %.0f", c.ahead, d-at, TrafficOverrunMeters)
		}
		t.Logf("%.0f m ahead: stopped %.1f m past the traffic stop, braking up to %.2f m/s²", c.ahead, m.Pose().Distance-at, maxDecel)
	}
}
