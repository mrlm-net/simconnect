package flight

import (
	"slices"
	"strings"
	"testing"
)

// lightsOn sets the landing lights and the beacon on every sample.
func lightsOn(tr *Track) *Track {
	for i := range tr.Samples {
		tr.Samples[i].Lights |= LightLanding | LightBeacon
	}
	return tr
}

func codes(a Assessment) []string {
	var out []string
	for _, f := range a.Findings {
		out = append(out, f.Code)
	}
	return out
}

// TestAssessClean: the synthetic flight flown well scores 100, with its
// profile read: lift-off, rotation rate, approach gates, touchdown.
func TestAssessClean(t *testing.T) {
	a := Assess(lightsOn(syntheticFlight()), AssessOptions{})
	if a.Score != 100 || len(a.Findings) != 0 {
		t.Fatalf("score %d, findings %v", a.Score, codes(a))
	}
	p := a.Profile
	if p.LiftoffKts < 147 || p.RotateKts < 139 || p.RotateKts > 143 || p.MaxPitchRate > 3 {
		t.Errorf("take-off: rotate %.0f, lift-off %.0f, %.1f°/s", p.RotateKts, p.LiftoffKts, p.MaxPitchRate)
	}
	if p.TouchdownFpm != 150 || p.Gate500Kts == 0 || p.ApproachKts < 139 || p.ApproachKts > 145 {
		t.Errorf("landing: %.0f fpm, gate 500 %.0f kt, Vapp %.0f", p.TouchdownFpm, p.Gate500Kts, p.ApproachKts)
	}
}

// TestAssessFaults: a hard landing, the gear up at 500 ft, no landing
// lights, and a touchdown long and off the centreline, each found and
// scored (major 15, minor 5).
func TestAssessFaults(t *testing.T) {
	tr := syntheticFlight() // no lights: landing-lights-off (minor)
	ss := tr.Samples
	touch := len(ss) - 1
	ss[touch-1].VS = -750 // hard (major)
	for i := range ss {
		if !ss[i].OnGround && agl(ss[i]) < 600 && agl(ss[i]) > 400 && i > touch-2000 {
			ss[i].GearHandle = false // the gear up through 500 ft: unstable (major)
		}
	}
	thr := AssessRunway{Lat: ss[touch].Lat, Lon: ss[touch].Lon - 0.02, Heading: 90} // 1.4 km before, 0 off
	thr.Lat += 0.0001                                                               // 11 m to the side
	a := Assess(tr, AssessOptions{Runway: &thr})
	got := codes(a)
	for _, want := range []string{"landing-lights-off", "hard-landing", "unstable-500", "long-landing", "off-centreline"} {
		if !slices.Contains(got, want) {
			t.Errorf("no %s in %v", want, got)
		}
	}
	if want := 100 - 2*MajorPoints - 3*MinorPoints; a.Score != want {
		t.Errorf("score %d, want %d (%v)", a.Score, want, got)
	}
}

// TestAssessTaxiFast: the fastest taxi speed is said, not the first over
// the limit (live: 38 kt read "30 kt").
func TestAssessTaxiFast(t *testing.T) {
	tr := lightsOn(syntheticFlight())
	var pre []Sample
	for _, gs := range []float64{10, 30.4, 38, 20} {
		pre = append(pre, Sample{OnGround: true, GS: gs, IAS: 0, GearHandle: true, FlapsIndex: 2, Lights: LightLanding | LightBeacon})
	}
	tr.Samples = append(pre, tr.Samples...)
	a := Assess(tr, AssessOptions{})
	for _, f := range a.Findings {
		if f.Code == "taxi-fast" {
			if f.Value != 38 || !strings.Contains(f.Text, "38 kt") {
				t.Errorf("%v: %s", f.Value, f.Text)
			}
			return
		}
	}
	t.Errorf("no taxi-fast in %v", codes(a))
}
