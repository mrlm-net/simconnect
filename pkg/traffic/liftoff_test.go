package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Rotating with the on-ground flag (struts let out, strutsAhead), the
// reference point rising as the nose comes up on the main gear: the flag
// goes before the height sent passes its static CG height on full struts,
// where the simulator flipped it airborne and back (live FKUJS PC-24, LKPR
// 24, 2026-11-09), and the height sent never steps.
func TestLiftOffDropsFlagAtStruts(t *testing.T) {
	for _, c := range []struct {
		name               string
		cg, static, struts float64
		mainM              float64
	}{
		{"PC24", 7.15, 2.68, 0.32, 1.0},
		{"A320-like", 12.25, 1.00, 0.70, 1.0},
		{"CJ3-like", 5.40, 1.50, 0.41, 1.0},
		{"long mains", 9.00, 0.80, 0.55, 2.5},
	} {
		t.Run(c.name, func(t *testing.T) {
			ec := &eventClient{}
			inj := NewInjector(ec)
			w := time.Date(2026, 11, 9, 15, 28, 0, 0, time.UTC)
			inj.now = func() time.Time { return w }
			const groundFt = 1159.47
			start := airport.LatLon{Lat: 50.1, Lon: 14.25}
			obj := uint32(42)
			if err := inj.Takeover(obj); err != nil {
				t.Fatal(err)
			}
			inj.SetModel(obj, "Model Z")
			inj.mu.Lock()
			inj.struts[strutsKey(c.cg)] = c.struts
			inj.mu.Unlock()
			m := NewTakeoffMover(start, 245, 100, DefaultTakeoffProfile())
			pose := m.Pose()
			sim := &simStruts{cg: c.cg, static: c.static, struts: c.struts, shown: c.cg, rolling: true}
			pitch, kts, shown, simGround := c.static, pose.GroundSpeedKts, c.cg, 1.0
			prev, air, flagged, dropped := math.NaN(), 0, false, false
			for k := 0; k < 60*60 && air < 120; k++ {
				inj.Handle(sampleMsg(DefaultInjectRequestBase+2*uint32(inj.objects[obj].slot)+1, obj, injectGround{GroundFt: groundFt, CGFt: c.cg, StaticPitch: c.static,
					PlaneFt: groundFt + shown, PlanePitch: pitch, OnGround: simGround, GS: kts, Lat: pose.Position.Lat, Lon: pose.Position.Lon}))
				pose = m.Step(1.0 / 60)
				if pose.Phase == TakeoffAirborne {
					air++
				}
				ap := pose.ApproachPose()
				ap.MainGearBehindM = c.mainM // as departure_inject sends it
				if err := inj.PlaceAir(obj, ap); err != nil {
					t.Fatal(err)
				}
				ps := placements(ec)
				s := ps[len(ps)-1]
				h := s.Altitude - groundFt
				if s.OnGround != 0 {
					if ap.PitchDeg > 0 {
						flagged = true
					}
					if dropped {
						t.Fatalf("k=%d: the flag back after lift-off (%.2f ft)", k, h)
					}
					// The simulator's contact: beyond its full struts it flips it airborne.
					if h > c.cg+c.struts+0.02 {
						t.Fatalf("k=%d: %.2f ft sent with the flag, over its full struts at %.2f (pitch %.2f)", k, h, c.cg+c.struts, ap.PitchDeg)
					}
				} else if flagged {
					dropped = true
				}
				// Near the ground (the climb itself is 0.2 ft a frame).
				if !math.IsNaN(prev) && h < c.cg+c.struts+1 && math.Abs(h-prev) > strutsStepFt {
					t.Fatalf("k=%d: height sent stepped %.2f → %.2f ft", k, prev, h)
				}
				shown, simGround = sim.step(s, groundFt, s.OnGround != 0), 1
				if s.OnGround == 0 {
					simGround = 0
				}
				prev, pitch, kts = h, s.Pitch, pose.GroundSpeedKts
				w = w.Add(time.Second / 60)
			}
			if !flagged || !dropped {
				t.Fatalf("rotation with the flag %v, dropped %v", flagged, dropped)
			}
		})
	}
}
