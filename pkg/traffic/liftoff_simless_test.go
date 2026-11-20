package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// With no simulator (SimlessAI, Vector), a take-off's writes carry the
// on-ground flag from the start of the roll through the rotation, and the
// first one without it is the first with the main wheels above the runway
// (#1605: TNCM 10, an A320 rolled 33 s of 39 with it off). The writes to
// MSFS are as they were: the same height, pitch and position every frame,
// the roll still without the flag (Unpinned).
func TestSimlessTakeoffOnGroundFlag(t *testing.T) {
	for _, c := range []struct {
		model              string
		cg, static, struts float64
	}{
		{"A320", 12.25, 1.00, 0.62},
		{"B744", 15.30, 0.40, 0.90},
	} {
		t.Run(c.model, func(t *testing.T) {
			msfs, simless := &eventClient{}, &eventClient{}
			injM, injS := NewInjector(msfs), NewInjector(NewSimlessAI(simless))
			w := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
			const groundFt = 12.0
			start := airport.LatLon{Lat: 18.04, Lon: -63.12}
			obj := uint32(7)
			for _, inj := range []*Injector{injM, injS} {
				inj.now = func() time.Time { return w }
				if err := inj.Takeover(obj); err != nil {
					t.Fatal(err)
				}
				inj.SetModel(obj, "Model "+c.model)
				inj.mu.Lock()
				inj.struts[strutsKey(c.cg)] = c.struts
				inj.mu.Unlock()
			}
			m := NewTakeoffMover(start, 98, 0, TakeoffProfileFor(c.model))
			mainM := MotionProfileFor(c.model).RefAheadMeters
			pose := m.Pose()
			shown, pitch, simGround := c.cg, c.static, 1.0
			firstAbove, firstOff, offRoll := -1, -1, 0
			var prev types.SIMCONNECT_DATA_INITPOSITION
			for k := 0; k < 120*120; k++ {
				g := injectGround{GroundFt: groundFt, CGFt: c.cg, StaticPitch: c.static, PlaneFt: groundFt + shown, PlanePitch: pitch,
					OnGround: simGround, GS: pose.GroundSpeedKts, Lat: pose.Position.Lat, Lon: pose.Position.Lon}
				for _, inj := range []*Injector{injM, injS} {
					inj.Handle(sampleMsg(DefaultInjectRequestBase+2*uint32(inj.objects[obj].slot)+1, obj, g))
				}
				pose = m.Step(1.0 / 120)
				ap := pose.ApproachPose()
				ap.MainGearBehindM = mainM
				ap.RunwayFt = groundFt
				for _, inj := range []*Injector{injM, injS} {
					if err := inj.PlaceAir(obj, ap); err != nil {
						t.Fatal(err)
					}
				}
				pm, ps := placements(msfs), placements(simless)
				a, b := pm[len(pm)-1], ps[len(ps)-1]
				if a.OnGround == 0 && pose.Phase != TakeoffAirborne {
					offRoll++
				}
				fa, fb := a, b
				fa.OnGround, fb.OnGround = 0, 0
				if fa != fb {
					t.Fatalf("k=%d: the write differs beyond the flag: MSFS %+v, simless %+v", k, a, b)
				}
				if firstAbove < 0 && pose.HeightFt > 0 {
					firstAbove = k
				}
				if firstOff < 0 && b.OnGround == 0 {
					firstOff = k
				}
				if firstAbove < 0 && b.OnGround == 0 {
					t.Fatalf("k=%d: simless write without the flag, wheels on the runway (%.0f kt, pitch %.2f°, phase %d)", k, pose.GroundSpeedKts, pose.PitchDeg, pose.Phase)
				}
				if firstOff >= 0 && b.OnGround != 0 {
					t.Fatalf("k=%d: the flag back after lift-off (%.2f ft)", k, pose.HeightFt)
				}
				if k > 0 {
					if d := math.Abs(b.Altitude - prev.Altitude); pose.HeightFt < 2 && d > strutsStepFt {
						t.Fatalf("k=%d: altitude stepped %.2f ft in a frame", k, d)
					}
					if d := math.Abs(b.Pitch - prev.Pitch); d > 0.2 {
						t.Fatalf("k=%d: pitch stepped %.2f° in a frame", k, d)
					}
				}
				prev = b
				shown, pitch = a.Altitude-groundFt, a.Pitch
				simGround = float64(a.OnGround)
				w = w.Add(time.Second / 120)
				if pose.HeightFt > 50 {
					break
				}
			}
			if firstAbove < 0 {
				t.Fatal("never lifted off")
			}
			if firstOff != firstAbove {
				t.Errorf("first simless write without the flag at frame %d, first with the wheels above the runway at %d", firstOff, firstAbove)
			}
			if offRoll == 0 {
				t.Errorf("MSFS writes: the roll now carries the flag throughout, want it unchanged (Unpinned)")
			}
		})
	}
}
