package traffic

import "testing"

// With no simulator (SimlessAI, Vector) no model lands flat: there is no
// simulator to show it on its extended struts and drop it at its static
// pitch, so every model touches down on its main gear at the flare
// attitude and derotates after (#1609, TNCM 10, KLM685 B744: its struts as
// learnt beyond strutsFlatFt, the nose let down over the last 10 ft and all
// its gear on the runway together at 0°). The struts here are ones that
// make it land flat with MSFS (TestLandingDerotatesSmoothly).
func TestSimlessLandingDerotates(t *testing.T) {
	kinds := []struct {
		model      string
		cg, struts float64
	}{
		{"B744", 18.50, 1.55},
		{"B744", 18.50, 0.80},
		{"B77W", 17.20, 1.40},
	}
	for _, ap := range []struct{ icao, end string }{{"TNCM", "10"}, {"LKPR", "24"}} {
		g := landingGraph(t, ap.icao)
		if g == nil {
			continue
		}
		for _, c := range kinds {
			name := ap.icao + "-" + ap.end + "-" + c.model
			if c.struts > strutsFlatFt {
				name += "-flatWithMSFS"
			}
			t.Run(name, func(t *testing.T) {
				ps := landingPosesOn(t, g, ap.end, c.model, c.cg, c.struts, true)
				if len(ps) < 60 {
					t.Fatalf("only %d writes", len(ps))
				}
				checkLandingPoses(t, ps, 1.5, false)
				down := -1
				for k, w := range ps {
					if w.p.OnGround != 0 {
						down = k
						break
					}
				}
				if down < 0 {
					t.Fatal("never on the ground")
				}
				if p := ps[down].p.Pitch; p > -2 {
					t.Errorf("touched down at pitch %.2f, want at least 2° nose up", -p)
				}
				for k := down + 1; k < len(ps); k++ {
					if ps[k].p.Pitch < ps[k-1].p.Pitch-0.01 {
						t.Errorf("write %d: the nose came up again on the runway, %.3f → %.3f", k, ps[k-1].p.Pitch, ps[k].p.Pitch)
						break
					}
				}
			})
		}
	}
}
