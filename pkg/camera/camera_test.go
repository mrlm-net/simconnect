package camera

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// An offset to the right is SimConnect's negative x (its x points left).
func TestPoseData(t *testing.T) {
	d, mask := Pose{Eye: On(42, 60, 10, -5), Target: On(42, 0, 2, 0), FovDeg: 45}.Data()
	if d.Position.X != -60 || d.Position.Y != 10 || d.Position.Z != -5 || d.PositionReferentialObjectID != 42 ||
		d.PositionReferential != types.SIMCONNECT_POSITION_REFERENTIAL_SIMOBJECT || mask != types.SIMCONNECT_CAMERA_DATA_MASK_ALL_TARGETED {
		t.Fatalf("%+v %v", d, mask)
	}
	if math.Abs(d.Fov-45*math.Pi/180) > 1e-9 {
		t.Errorf("fov %v", d.Fov)
	}
	w, _ := Pose{Eye: At(50.1, 14.26, 400), Target: On(0, 0, 0, 0)}.Data()
	if w.Position.X != 50.1 || w.Position.Y != 14.26 || w.Position.Z != 400 || w.PositionReferential != types.SIMCONNECT_POSITION_REFERENTIAL_WORLD {
		t.Fatalf("world %+v", w)
	}
	// Round trip through the packed layout.
	b := d.Bytes()
	back, ok := types.CameraDataFrom(b[:])
	if !ok || back != d {
		t.Errorf("packed: %+v", back)
	}
}

func TestShots(t *testing.T) {
	o := Orbit("orbit", 7, 30, 5, 2, 90, 180, 50, 8*time.Second, Linear)
	if p := o.PoseAt(0); math.Abs(p.Eye.Offset.Right-30) > 1e-9 || math.Abs(p.Eye.Offset.Forward) > 1e-9 {
		t.Errorf("start: %+v", p.Eye.Offset)
	}
	if p := o.PoseAt(8 * time.Second); math.Abs(p.Eye.Offset.Forward+30) > 1e-9 {
		t.Errorf("end behind: %+v", p.Eye.Offset)
	}
	if Hold("long", Pose{}, time.Minute).Length() != MaxShot {
		t.Error("a shot longer than MaxShot")
	}
	if Smooth(0) != 0 || Smooth(1) != 1 || math.Abs(Smooth(0.5)-0.5) > 1e-9 {
		t.Error("Smooth")
	}
}

type fakeAPI struct {
	acquired, released int
	set                []types.SIMCONNECT_DATA_CAMERA
}

func (f *fakeAPI) CameraAcquire(string) error { f.acquired++; return nil }
func (f *fakeAPI) CameraRelease(string) error { f.released++; return nil }
func (f *fakeAPI) CameraSet(d types.SIMCONNECT_DATA_CAMERA, _ types.SIMCONNECT_CAMERA_DATA_MASK) error {
	f.set = append(f.set, d)
	return nil
}

func status(s types.SIMCONNECT_CAMERA_AVAILABILITY) engine.Message {
	r := &types.SIMCONNECT_RECV_CAMERA_STATUS{SIMCONNECT_RECV: types.SIMCONNECT_RECV{DwID: types.DWORD(types.SIMCONNECT_RECV_ID_CAMERA_STATUS)}, AcquiredState: s}
	return engine.Message{SIMCONNECT_RECV: &r.SIMCONNECT_RECV}
}

// The director acquires once, plays the shots in turn and holds the last
// pose; a cut replaces the rest.
func TestDirector(t *testing.T) {
	f := &fakeAPI{}
	d := NewDirector(f, "test")
	var names []string
	d.OnShot(func(s Shot) { names = append(names, s.Name()) })
	a := Hold("a", Pose{Eye: On(1, 10, 0, 0)}, 2*time.Second)
	b := Hold("b", Pose{Eye: On(1, 20, 0, 0)}, 2*time.Second)
	d.Play(a, b)
	now := time.Now()
	if err := d.Tick(now); err != nil || f.acquired != 1 {
		t.Fatalf("acquire: %v %d", err, f.acquired)
	}
	if err := d.Tick(now); err != ErrNotAcquired || f.acquired != 1 {
		t.Fatalf("waiting: %v %d", err, f.acquired)
	}
	d.Handle(status(types.SIMCONNECT_CAMERA_ACQUIRED))
	for i := 0; i <= 50; i++ {
		d.Tick(now.Add(time.Duration(i) * 100 * time.Millisecond))
	}
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Fatalf("shots %v", names)
	}
	if last := f.set[len(f.set)-1]; last.Position.X != -20 {
		t.Errorf("held last pose %+v", last.Position)
	}
	d.Play(Hold("c", Pose{Eye: On(1, 30, 0, 0)}, time.Second))
	d.Tick(now.Add(6 * time.Second))
	if f.set[len(f.set)-1].Position.X != -30 || names[len(names)-1] != "c" {
		t.Errorf("cut: %+v %v", f.set[len(f.set)-1].Position, names)
	}
	d.Release()
	if f.released != 1 || d.Acquired() {
		t.Error("release")
	}
}

// A path passes through its keys and every drone move stays near its
// aircraft, in its frame.
func TestPathAndMoves(t *testing.T) {
	p := Path("p", 4*time.Second, Linear,
		Key{0, Pose{Eye: On(1, 0, 0, 0)}}, Key{0.5, Pose{Eye: On(1, 10, 0, 0)}}, Key{1, Pose{Eye: On(1, 10, 10, 0)}})
	if e := p.PoseAt(2 * time.Second).Eye.Offset; math.Abs(e.Right-10) > 1e-9 || math.Abs(e.Up) > 1e-9 {
		t.Errorf("middle key %+v", e)
	}
	if e := p.PoseAt(4 * time.Second).Eye.Offset; math.Abs(e.Up-10) > 1e-9 {
		t.Errorf("last key %+v", e)
	}
	s := Size{Span: 36, Length: 38}
	for _, m := range []MoveFunc{RevealRise, Flyover, SpiralDescend, LeadChase, ParallaxTrack, HeroLowPushIn, PullBackReveal, TopDown, TopOrbit, MainGearDetail, SideDolly, HeadOnPass, LightsDetail,
		EngineDetail, NoseGearDetail, CockpitDetail, TailDetail, WingtipAlong, ChaseRise} {
		sh := m(9, s, -1, 6*time.Second)
		for i := 0; i <= 12; i++ {
			pose := sh.PoseAt(time.Duration(i) * time.Second / 2)
			o := pose.Eye.Offset
			if pose.Eye.Frame != Object || pose.Eye.Object != 9 || math.Hypot(math.Hypot(o.Right, o.Up), o.Forward) > 150 {
				t.Errorf("%s: %+v", sh.Name(), pose.Eye)
				break
			}
		}
	}
}
