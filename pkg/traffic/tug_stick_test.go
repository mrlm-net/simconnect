package traffic

import (
	"fmt"
	"math"
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// stickTugAxle: measured from the GSX glTF models (scratchpad tugfit2,
// attach.js, 2026-11-08), with the bar straight: how far the tug's rear
// axle is behind the model origin (the bar's hook end), in metres.
var stickTugAxle = map[string]float64{
	"FSDT_Pushback_Trepel_700": 11.04,
	"FSDT_Pushback_Trepel_280": 9.93,
	"FSDT_Pushback_TMX_150":    9.43,
	"FSDT_Pushback_M1A":        5.53,
}

// stickTugBar: the same, the bar's pivot (FSDT_TowbarAngle) behind the
// origin.
var stickTugBar = map[string]float64{
	"FSDT_Pushback_Trepel_700": 4.35,
	"FSDT_Pushback_Trepel_280": 5.37,
	"FSDT_Pushback_TMX_150":    5.37,
	"FSDT_Pushback_M1A":        3.43,
}

// stickAxle is where the stick tug title's rear axle is and which way the
// tug faces, as the sim shows it from what was written: the model at p, the
// bar's anim frame v (0 to 180, 90 straight; frame 0 hangs the tug on the
// model's left, +x, facing right: 90° clockwise of the bar).
func stickAxle(title string, p types.SIMCONNECT_DATA_INITPOSITION, v float64) (airport.LatLon, float64) {
	o := airport.LatLon{Lat: p.Latitude, Lon: p.Longitude}
	pivot := offsetHeading(o, p.Heading+180, stickTugBar[title])
	h := normDeg(p.Heading + (90 - v))
	return offsetHeading(pivot, h+180, stickTugAxle[title]-stickTugBar[title]), h
}

// slipMeter measures how far off its axis a point travels, over each
// slipStepM of travel (a written bar angle steps by up to TugBarResendDeg:
// a centimetre at the axle, nothing to see, but a frame's whole travel).
type slipMeter struct {
	from    airport.LatLon
	fromH   float64
	t       float64
	started bool
	worst   float64
	moving  int
}

const slipStepM = 0.5

// feed adds the point a, heading h, dt seconds on; minSpeed m/s: slower
// travel is not counted.
func (m *slipMeter) feed(a airport.LatLon, h, dt, minSpeed float64) (float64, bool) {
	m.t += dt
	if !m.started {
		m.from, m.fromH, m.t, m.started = a, h, 0, true
		return 0, false
	}
	d := localDist(m.from, a)
	if d < slipStepM {
		return 0, false
	}
	defer func() { m.from, m.fromH, m.t = a, h, 0 }()
	if d/m.t <= minSpeed {
		return 0, false
	}
	m.moving++
	mean := normDeg(m.fromH + headingDiff(m.fromH, h)/2)
	off := math.Abs(headingDiff(mean, localBearing(m.from, a)))
	off = math.Min(off, 180-off)
	m.worst = math.Max(m.worst, off)
	return off, true
}

// lastBar is the bar angle last written to object 55 (0, across, if none).
func (c *stickClient) lastBar() float64 {
	if b := c.back[55]; len(b) > 0 {
		return b[len(b)-1]
	}
	return 0
}

// stickClient records each object's last placement and its WAGON BACK LINK
// ORIENTATION writes.
type stickClient struct {
	placeClient
	back map[uint32][]float64
}

func (c *stickClient) SetDataOnSimObject(def, obj uint32, f types.SIMCONNECT_DATA_SET_FLAG, n, size uint32, p unsafe.Pointer) error {
	if def == DefaultInjectDefinitionBase+injDefTowbar {
		if c.back == nil {
			c.back = map[uint32][]float64{}
		}
		c.back[obj] = append(c.back[obj], (*[1]float64)(p)[0])
	}
	return c.placeClient.SetDataOnSimObject(def, obj, f, n, size, p)
}

// A GSX stick tug is a towbar model with the tug hung on its far end,
// turned by WAGON BACK LINK ORIENTATION (0 to 180, 90 straight). Never
// written, the tug stood across the bar: over an E195's nose at the stand
// (live LKPR, OE-LWG) and crabbing sideways on the roads. Every stick tug
// gets the bar straight once it is ours; the classic tug (one model) gets no
// write.
func TestStickTugBarStraight(t *testing.T) {
	for _, title := range []string{"FSDT_Pushback_Trepel_700", "FSDT_Pushback_Trepel_280", "FSDT_Pushback_TMX_150", "FSDT_Pushback_M1A", DefaultTugTitle} {
		c := &stickClient{}
		inj := NewInjector(c)
		tug := NewSimObjectTug(c, inj, title, 9001, MotionProfileFor("FSLTL_E190_AUA"))
		pose := GroundPose{Position: lkpr, Heading: 90}
		if err := tug.Attach(pose); err != nil {
			t.Fatal(err)
		}
		tug.Handle(assignedMsg(9001, 55))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, 55, 1200, 3))
		for i := 0; i < 60; i++ {
			if err := tug.Update(pose, true, 1.0/60); err != nil {
				t.Fatal(err)
			}
		}
		got := c.back[55]
		if title == DefaultTugTitle {
			if len(got) != 0 {
				t.Errorf("%s: bar written %v, a one-piece model", title, got)
			}
			continue
		}
		if len(got) == 0 || got[len(got)-1] != TugBarStraight {
			t.Errorf("%s: bar orientation written %v, want %v (straight)", title, got, TugBarStraight)
		}
		if len(got) > 3 {
			t.Errorf("%s: bar written %d times in a second, once is enough", title, len(got))
		}
	}
}

// The E195 (OE-LWG at LKPR) and the other jets that get a stick tug: with
// the bar straight its hook end goes on the nose wheel, the tug a bar's
// length ahead. At -1.8 m (tuned live against the tug standing across the
// bar) the hook sat under the fuselage and the tug under the nose.
func TestStickTugHookOnTheNoseWheel(t *testing.T) {
	for _, model := range []string{"FSLTL_E190_AUA", "E195", "FSLTL_FAIB_A320_SmartWings", "FSLTL_A321_DLH", "FSLTL_FAIB_B738_TVS-Smartwings_NC", "FSLTL_B789_BAW"} {
		prof := MotionProfileFor(model)
		title := TugTitleForModel("", model, ProfileFor(model).WingspanM)
		if _, ok := stickTugAxle[title]; !ok {
			t.Fatalf("%s: tug %s, not a stick tug", model, title)
		}
		c := &stickClient{}
		inj := NewInjector(c)
		tug := NewSimObjectTug(c, inj, title, 9001, prof)
		if a, ok := TugAheadFor(title); ok {
			tug.AheadMeters = a
		}
		pose := GroundPose{Position: lkpr, Heading: 90}
		if err := tug.Attach(pose); err != nil {
			t.Fatal(err)
		}
		nose := NoseGear(pose.Position, pose.Heading, prof)
		tug.Handle(assignedMsg(9001, 55))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, 55, 1200, 3))
		if err := tug.Update(pose, true, 1.0/60); err != nil {
			t.Fatal(err)
		}
		p, ok := c.last[55]
		if !ok {
			t.Fatalf("%s: tug not placed", model)
		}
		hook := airport.LatLon{Lat: p.Latitude, Lon: p.Longitude}
		if d := alongHeading(nose, pose.Heading, hook); math.Abs(d) > 0.3 {
			t.Errorf("%s (%s): hook %.2f m from the nose wheel along the axis, want on it", model, title, d)
		}
		if math.Abs(headingDiff(p.Heading, pose.Heading+180)) > 1 {
			t.Errorf("%s: tug heading %.0f, want facing the aircraft", model, p.Heading)
		}
	}
}

// A stick tug drives in from its depot, pushes and drives home without
// sliding: its rear axle (stickTugAxle behind the shown origin) travels
// along its axis, within 10°, whenever it moves faster than a walk. Its
// origin is the bar's hook, 11 m ahead of the Trepel 700's rear axle; moved
// as if it were the rear axle, the tug swung its tail sideways round every
// turn (live LKPR: "it slides").
func TestStickTugDrivesWithoutSliding(t *testing.T) {
	g := lkprGraph(t)
	l := g.Layout
	for _, stand := range []string{"B9", "C22"} {
		i, err := l.ParkingIndex(stand)
		if err != nil {
			t.Fatal(err)
		}
		model := "FSLTL_E190_AUA"
		prof := MotionProfileFor(model)
		title := TugTitleForModel("", model, ProfileFor(model).WingspanM)
		c := &stickClient{}
		inj := NewInjector(c)
		tug := NewSimObjectTug(c, inj, title, 9001, prof)
		tug.Layout = l
		if a, ok := TugAheadFor(title); ok {
			tug.AheadMeters = a
		}
		st := l.Parking[i]
		pose := GroundPose{Position: StandPoint(st, prof.RefAheadMeters), Heading: st.Heading}
		if err := tug.Attach(pose); err != nil {
			t.Fatal(err)
		}
		tug.Handle(assignedMsg(9001, 55))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, 55, 1200, 3))
		const dt = 1.0 / 60
		var slip slipMeter
		worst, frames := 0.0, 0
		var worstAt string
		check := func(phase string) {
			p, ok := c.last[55]
			if !ok {
				return
			}
			a, h := stickAxle(title, p, c.lastBar())
			// Forward or backing off: off its axis either way.
			if off, ok := slip.feed(a, h, dt, 1.5); ok && off > worst {
				worst, worstAt = off, phase
			}
			frames++
		}
		for n := 0; n < 60*600 && !tug.Connected(); n++ {
			if err := tug.Update(pose, true, dt); err != nil {
				t.Fatal(err)
			}
			check("driving in")
		}
		if !tug.Connected() {
			t.Fatalf("%s: never reached the nose", stand)
		}
		for n := 0; n < 60*900 && !tug.Done(); n++ {
			if err := tug.Update(pose, false, dt); err != nil {
				t.Fatal(err)
			}
			if !tug.Done() {
				check("leaving")
			}
		}
		if slip.moving < 60 {
			t.Fatalf("%s: moving in only %d steps over %d frames", stand, slip.moving, frames)
		}
		if worst > 10 {
			t.Errorf("%s: rear axle travels %.0f° off the tug's heading (%s): it slides", stand, worst, worstAt)
		}
		t.Logf("%s: worst %.1f° over %d steps", stand, worst, slip.moving)
	}
}

// Pushes turning left and right, for an E195, an A320, a B738, an A332 and
// a light aircraft: every tug's model stays on the nose wheel (its origin
// AheadMeters along the bar, as written to the sim); a stick tug turns on
// its bar (its angle written) so its rear axle travels along its axis
// (within 10°) while it moves. The A332's classic tug and the robot are
// placed as before.
func TestTugPushBothWays(t *testing.T) {
	for _, model := range []string{"FSLTL_E190_AUA", "FSLTL_FAIB_A320_SmartWings", "FSLTL_FAIB_B738_TVS-Smartwings_NC", "FSLTL_A332F_MAS", "Asobo PassiveAircraft Cessna 172"} {
		for _, turn := range []float64{-50, 50} {
			prof := MotionProfileFor(model)
			prof.CruiseKts, prof.MinTurnKts = PushbackSpeedKts, 1
			gear := offset(lkpr, 0, -prof.RefAheadMeters)
			path, err := NewArcPath([]airport.LatLon{gear, offset(gear, 0, -40), offset(gear, turn, -40)}, prof, 25)
			if err != nil {
				t.Fatal(err)
			}
			m := NewPushbackMover(path, prof, 0)
			span := ProfileFor(model).WingspanM
			if span == 0 {
				span = 11
			}
			title := TugTitleForModel("", model, span)
			c := &stickClient{}
			inj := NewInjector(c)
			tug := NewSimObjectTug(c, inj, title, 9001, prof)
			if a, ok := TugAheadFor(title); ok {
				tug.AheadMeters = a
			}
			if err := tug.Attach(m.Pose()); err != nil {
				t.Fatal(err)
			}
			tug.Handle(assignedMsg(9001, 55))
			inj.Handle(groundMsg(DefaultInjectRequestBase+1, 55, 1200, 3))
			_, stick := stickTugAxle[title]
			const dt = 1.0 / 60
			var slip slipMeter
			maxOff := 0.0
			for i := 0; i < 60*300; i++ {
				pose := m.Step(dt)
				if err := tug.Update(pose, true, dt); err != nil {
					t.Fatal(err)
				}
				p := c.last[55]
				o := airport.LatLon{Lat: p.Latitude, Lon: p.Longitude}
				nose := NoseGear(pose.Position, pose.Heading, prof)
				want := offsetHeading(nose, p.Heading-tug.YawDeg, tug.AheadMeters)
				maxOff = math.Max(maxOff, localDist(o, want))
				if stick {
					a, h := stickAxle(title, p, c.lastBar())
					slip.feed(a, h, dt, 0.3)
				}
				if pose.Arrived {
					break
				}
			}
			name := fmt.Sprintf("%s %+.0f (%s)", model, turn, title)
			if maxOff > 0.05 {
				t.Errorf("%s: model %.2f m off its spot on the bar", name, maxOff)
			}
			if stick {
				if len(c.back[55]) == 0 {
					t.Errorf("%s: bar never written: the tug stands across it", name)
				}
				if slip.worst > 10 {
					t.Errorf("%s: rear axle %.0f° off its axis while pushing: it slides", name, slip.worst)
				}
			}
			t.Logf("%s: worst axle slip %.1f° over %d steps", name, slip.worst, slip.moving)
		}
	}
}
