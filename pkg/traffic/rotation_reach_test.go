package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Take-offs of a model whose struts reach less than they extend, the reach
// not known (live THY1718 B739: 0.57 ft of struts, 9.07 → 8.50 at
// rotation, its reach never learnt): the take-off that meets it learns it,
// the next ones do not step.
func TestRotationLearnsReach(t *testing.T) {
	for _, c := range []struct {
		name                      string
		cg, static, struts, reach float64
	}{
		{"0.57/0.42", 9.95, 1.00, 0.57, 0.42},
		{"0.49/0.30", 9.97, 0.71, 0.49, 0.30},
		{"E195-like 1.00/0.60", 9.99, 1.00, 1.00, 0.60},
		{"A20N-EZY-like 1.14/0.55", 10.01, 1.00, 1.14, 0.55},
	} {
		t.Run(c.name, func(t *testing.T) {
			ec := &eventClient{}
			inj := NewInjector(ec)
			w := time.Date(2026, 11, 8, 20, 11, 0, 0, time.UTC)
			inj.now = func() time.Time { return w }
			const groundFt = 1165.0
			start := airport.LatLon{Lat: 50.1, Lon: 14.25}
			for run := 0; run < 4; run++ {
				obj := uint32(42 + run)
				if err := inj.Takeover(obj); err != nil {
					t.Fatal(err)
				}
				inj.SetModel(obj, "Model Y")
				sim := &simStruts{cg: c.cg, static: c.static, struts: c.struts, reach: c.reach, shown: c.cg, rolling: true}
				m := NewTakeoffMover(start, 245, 100, DefaultTakeoffProfile())
				pose := m.Pose()
				pitch, kts := c.static, pose.GroundSpeedKts
				prev, steps, worst := math.NaN(), 0, 0.0
				for k := 0; k < 60*30; k++ {
					inj.Handle(sampleMsg(DefaultInjectRequestBase+2*uint32(inj.objects[obj].slot)+1, obj, injectGround{GroundFt: groundFt, CGFt: c.cg, StaticPitch: c.static,
						PlaneFt: groundFt + sim.shown, PlanePitch: pitch, OnGround: 1, GS: kts, Lat: pose.Position.Lat, Lon: pose.Position.Lon}))
					pose = m.Step(1.0 / 60)
					if pose.Phase == TakeoffAirborne {
						break
					}
					if err := inj.PlaceAir(obj, pose.ApproachPose()); err != nil {
						t.Fatal(err)
					}
					ps := placements(ec)
					s := ps[len(ps)-1]
					h := sim.step(s, groundFt, true)
					pitch, kts = s.Pitch, pose.GroundSpeedKts
					if !math.IsNaN(prev) && math.Abs(h-prev) > strutsStepFt {
						steps++
						worst = math.Max(worst, math.Abs(h-prev))
					}
					prev = h
					w = w.Add(time.Second / 60)
				}
				r, ok := inj.reachOf(inj.objects[obj])
				t.Logf("take-off %d: %d steps, worst %.2f; struts %.2f, reach %v %.2f", run+1, steps, worst, inj.strutsOf(inj.objects[obj]), ok, r)
				if floor := c.struts - r + 0.02; run >= 2 && worst > math.Max(floor, strutsStepFt) {
					t.Errorf("take-off %d stepped %d times (worst %.2f ft, the floor %.2f)", run+1, steps, worst, floor)
				}
				if err := inj.Release(obj); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
