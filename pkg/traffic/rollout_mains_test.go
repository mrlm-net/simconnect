package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Rollouts of models whose height on the main gear was learnt below their
// extended struts (live LKPR 24 2026-11-09: CSA1383 A320 12.87 → 12.44 ft
// as the nose came down, THY948 A321), the nose let down as ApproachMover
// does (smoothstep): no step, the first rollout or the second.
func TestRolloutMainsBelowStruts(t *testing.T) {
	for _, c := range []struct {
		name                      string
		cg, static, struts, reach float64
		title                     string
	}{
		{"Praetor500-410", 4.10, 0.00, 2.38, 0.30, "ZZ_GA"},
		{"B739-850", 8.50, 1.00, 0.57, 0.42, "ZZ_B739"},
		{"A320", 12.25, 0.95, 0.62, 0.57, "ZZ_A320"},
		{"A321", 12.46, 1.00, 0.77, 0.62, "ZZ_A321"},
		{"E195", 13.30, 1.00, 1.04, 0.65, "ZZ_E195"},
		{"B789", 16.85, 1.00, 0.78, 0.70, "ZZ_B789"},
		{"A20N-EZY", 12.00, 1.00, 1.13, 0.59, "ZZ_A20N"},
	} {
		t.Run(c.name, func(t *testing.T) {
			ec := &eventClient{}
			inj := NewInjector(ec)
			for run := 0; run < 2; run++ {
				steps, worst := rolloutMains(t, inj, ec, uint32(42+run), c.cg, c.static, c.struts, c.reach, c.title)
				r, ok := inj.reachOf(inj.objects[uint32(42+run)])
				t.Logf("run %d: %d steps, worst %.2f ft; reach learnt %v %.2f", run, steps, worst, ok, r)
				if run == 1 && steps > 0 {
					t.Errorf("second rollout stepped %d times (worst %.2f ft)", steps, worst)
				}
			}
		})
	}
}

func rolloutMains(t *testing.T, inj *Injector, ec *eventClient, obj uint32, cg, static, struts, reach float64, title string) (int, float64) {
	w := time.Date(2026, 11, 8, 21, 6, 0, 0, time.UTC).Add(time.Duration(obj) * time.Minute)
	inj.now = func() time.Time { return w }
	const groundFt = 1163.0
	if err := inj.Takeover(obj); err != nil {
		t.Fatal(err)
	}
	inj.SetModel(obj, title)
	inj.mu.Lock()
	inj.mains[title] = cg + 0.19
	inj.rests[title] = [2]float64{cg, static}
	inj.mu.Unlock()
	sim := &simStruts{cg: cg, static: static, struts: struts, reach: reach, shown: cg + struts, rolling: true}
	start := airport.LatLon{Lat: 50.1, Lon: 14.25}
	x, kts, nose := 0.0, 110.0, 3.0
	pitch := static - nose
	prev, steps, worst := math.NaN(), 0, 0.0
	for k := 0; k < 60*12; k++ {
		pos := offsetHeading(start, 245, x)
		inj.Handle(sampleMsg(DefaultInjectRequestBase+2*uint32(inj.objects[obj].slot)+1, obj, injectGround{GroundFt: groundFt, CGFt: cg, StaticPitch: static,
			PlaneFt: groundFt + sim.shown, PlanePitch: pitch, OnGround: 1, GS: kts, Lat: pos.Lat, Lon: pos.Lon}))
		if k > 60 {
			f := math.Min(1, float64(k-60)/(4*60))
			nose = 3 * (1 - f*f*(3-2*f))
		}
		n := len(placements(ec))
		if err := inj.PlaceAir(obj, ApproachPose{Position: pos, Heading: 245, PitchDeg: nose, GroundSpeedKts: kts,
			OnGround: true, Landing: true, MainGearBehindM: 2.5}); err != nil {
			t.Fatal(err)
		}
		ps := placements(ec)
		if len(ps) == n {
			t.Fatal("not placed")
		}
		s := ps[len(ps)-1]
		h := sim.step(s, groundFt, true)
		pitch = s.Pitch
		if !math.IsNaN(prev) && math.Abs(h-prev) > strutsStepFt {
			steps++
			worst = math.Max(worst, math.Abs(h-prev))
		}
		prev = h
		x += kts * ktsToMS / 60
		kts = math.Max(50, kts-3.0/60*1.94)
		w = w.Add(time.Second / 60)
	}
	return steps, worst
}
