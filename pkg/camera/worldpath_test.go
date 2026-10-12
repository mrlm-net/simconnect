package camera

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestWorldPathValidateAndLong(t *testing.T) {
	k := func(s float64) WorldKey { return WorldKey{T: time.Duration(s * float64(time.Second)), Lat: 50, Lon: 14, AltM: 400} }
	p := WorldPath{Keys: []WorldKey{k(100), k(104)}, Look: LookAt{Mode: LookFree}}
	if err := p.Validate(); err != nil || p.Long() != nil || p.Length() != 4*time.Second {
		t.Fatalf("valid 4 s path: %v %v %v", p.Validate(), p.Long(), p.Length())
	}
	if (WorldPath{Keys: []WorldKey{k(1)}}).Validate() != ErrFewKeys {
		t.Error("one key")
	}
	if (WorldPath{Keys: []WorldKey{k(2), k(1)}, Look: LookAt{Mode: LookFree}}).Validate() != ErrKeyOrder {
		t.Error("keys out of order")
	}
	if (WorldPath{Keys: []WorldKey{k(1), k(2)}}).Validate() != ErrLookTarget {
		t.Error("LookAircraft without object or track")
	}
	// Over MaxShot: valid, only Long warns.
	long := WorldPath{Keys: []WorldKey{k(0), k(14)}, Look: LookAt{Mode: LookFree}}
	if long.Validate() != nil || !errors.Is(long.Long(), ErrShotTooLong) || long.Length() != 14*time.Second {
		t.Error("a 14 s shot: valid, Long warns, not cut")
	}
}

// Cued at its first key's replay time, the path passes through its keys
// and looks at the aircraft where its track has it then.
func TestWorldPathOnTimeline(t *testing.T) {
	keys := []WorldKey{
		{T: 545 * time.Second, Lat: 50.0, Lon: 14.0, AltM: 400, FovDeg: 40},
		{T: 549 * time.Second, Lat: 50.001, Lon: 14.0, AltM: 420, FovDeg: 50},
		{T: 553 * time.Second, Lat: 50.002, Lon: 14.001, AltM: 440, FovDeg: 60},
	}
	var asked time.Duration
	track := func(rt time.Duration) (float64, float64, float64, bool) { asked = rt; return 50.01, 14.01, 500, true }
	p := WorldPath{Label: "designed", Keys: keys, Look: LookAt{Mode: LookAircraft, Track: track}, Ease: Linear}
	tl := NewTimeline(p.Cue())
	if _, _, ok := tl.PoseAt(544 * time.Second); ok {
		t.Error("before the cue")
	}
	pose, shot, ok := tl.PoseAt(549 * time.Second)
	if !ok || shot.Name() != "designed" || math.Abs(pose.Eye.Lat-50.001) > 1e-9 || math.Abs(pose.FovDeg-50) > 1e-9 {
		t.Fatalf("at the middle key: %+v %v", pose, ok)
	}
	if asked != 549*time.Second || pose.Target.Lat != 50.01 || pose.Target.Frame != World {
		t.Errorf("look at the track at replay time: asked %v, %+v", asked, pose.Target)
	}
	if _, _, ok := tl.PoseAt(554 * time.Second); ok || tl.End() != 553*time.Second {
		t.Error("after the shot")
	}
	// The live object instead: an Object point.
	p.Look = LookAt{Mode: LookAircraft, Object: 7}
	if got := p.PoseAt(0).Target; got.Frame != Object || got.Object != 7 {
		t.Errorf("object look: %+v", got)
	}
}

// LookFree aims by heading and pitch, a heading across north turning the
// short way.
func TestWorldPathFree(t *testing.T) {
	p := WorldPath{Keys: []WorldKey{
		{T: 0, Lat: 50, Lon: 14, AltM: 300, HeadingDeg: 350},
		{T: 2 * time.Second, Lat: 50, Lon: 14, AltM: 300, HeadingDeg: 10},
	}, Look: LookAt{Mode: LookFree}, Ease: Linear}
	tg := p.PoseAt(time.Second).Target
	if tg.Lat <= 50 || math.Abs(tg.Lon-14) > 1e-6 {
		t.Errorf("halfway from 350° to 10° looks north: %+v", tg)
	}
}

// A preset exported as keys plays back where it was.
func TestKeysOf(t *testing.T) {
	ac := ObjectPose{Lat: 50, Lon: 14, AltM: 400, Heading: 90}
	poseAt := func(time.Duration, uint32) (ObjectPose, bool) { return ac, true }
	shot := Hold("side", Pose{Eye: Point{Frame: Object, Object: 3, Offset: Offset{Right: 50}}, Target: Point{Frame: Object, Object: 3}}, 4*time.Second)
	keys := KeysOf(shot, 100*time.Second, time.Second, poseAt)
	if len(keys) != 5 || keys[0].T != 100*time.Second || keys[4].T != 104*time.Second {
		t.Fatalf("keys: %d %+v", len(keys), keys)
	}
	// 50 m right of an aircraft heading east is south of it, looking north.
	if k := keys[0]; k.Lat >= 50 || math.Abs(k.HeadingDeg) > 0.5 && math.Abs(k.HeadingDeg-360) > 0.5 {
		t.Errorf("first key: %+v", k)
	}
	p := WorldPath{Keys: keys, Look: LookAt{Mode: LookFree}}
	if p.Validate() != nil {
		t.Fatal(p.Validate())
	}
}

func TestFocalMM(t *testing.T) {
	if f := FovFromFocalMM(50); math.Abs(f-39.6) > 0.05 {
		t.Errorf("50 mm: %.2f°", f)
	}
	if mm := FocalMMFromFov(FovFromFocalMM(24)); math.Abs(mm-24) > 1e-9 {
		t.Errorf("round trip: %v", mm)
	}
}
