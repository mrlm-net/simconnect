package traffic

import (
	"math"
	"testing"
)

// TestVFRMinima: Table S5-1 (SERA.5001) by band and class.
func TestVFRMinima(t *testing.T) {
	nan := math.NaN()
	for _, c := range []struct {
		name         string
		class        AirspaceClass
		altFt, aglFt float64
		opts         []VFRMinimaOption
		visM         float64
		clear        bool
	}{
		{"D at FL120", ClassD, 12000, 10800, nil, 8000, false},
		{"G at 10,000 ft", ClassG, 10000, 8000, nil, 8000, false},
		{"E at 6,000 ft", ClassE, 6000, 4800, nil, 5000, false},
		{"G at 6,000 ft, 4,800 AGL", ClassG, 6000, 4800, nil, 5000, false},
		{"G at 2,500 ft", ClassG, 2500, 1300, nil, 5000, true},
		// Over high ground the low band runs to 1,000 ft above it.
		{"G at 4,000 ft, 800 AGL", ClassG, 4000, 800, nil, 5000, true},
		{"F at 4,000 ft, height not known", ClassF, 4000, nan, nil, 5000, false},
		{"D at 2,000 ft", ClassD, 2000, 800, nil, 5000, false},
		{"G at 2,000 ft, reduced", ClassG, 2000, 800, []VFRMinimaOption{VFRReducedVisibility(1000)}, 1500, true},
		{"D at 2,000 ft, reduced: not for D", ClassD, 2000, 800, []VFRMinimaOption{VFRReducedVisibility(1500)}, 5000, false},
		// Transition altitude below 10,000 ft: FL100 bounds the band.
		{"E at 9,800 ft, FL101", ClassE, 9800, 8000, []VFRMinimaOption{VFRFlightLevel(10100)}, 8000, false},
		{"E at 10,100 ft, FL98", ClassE, 10100, 8000, []VFRMinimaOption{VFRFlightLevel(9800)}, 5000, false},
	} {
		m := VFRMinima(c.class, c.altFt, c.aglFt, c.opts...)
		if m.VisibilityM != c.visM || m.ClearOfCloud != c.clear || m.SurfaceInSight != c.clear {
			t.Errorf("%s: %+v, want %v m, clear of cloud %v", c.name, m, c.visM, c.clear)
		}
		if !c.clear && (m.CloudHorizontalM != 1500 || m.CloudVerticalFt != 1000) {
			t.Errorf("%s: distance from cloud %v m / %v ft", c.name, m.CloudHorizontalM, m.CloudVerticalFt)
		}
	}
}

// TestVMCCheck: visibility checked, the cloud part always unknown.
func TestVMCCheck(t *testing.T) {
	m := VFRMinima(ClassD, 2000, 800)
	for _, c := range []struct {
		vis       float64
		known, ok bool
	}{{9999, true, true}, {5000, true, true}, {4000, true, false}, {0, false, false}} {
		r := m.Check(c.vis)
		if r.VisibilityKnown != c.known || r.VisibilityOK != c.ok || !r.CloudUnknown {
			t.Errorf("%v m: %+v", c.vis, r)
		}
	}
}

// TestCheckControlZone: SERA.5005(b) and 5010(c); nothing decided by a
// ceiling not given.
func TestCheckControlZone(t *testing.T) {
	for _, c := range []struct {
		vis, ceil       float64
		heli            bool
		vfr, svfr, ceUn bool
	}{
		{9999, 0, false, true, false, true},
		{4000, 0, false, false, true, true},
		{1500, 0, false, false, true, true},
		{1200, 0, false, false, false, true},
		{1200, 0, true, false, true, true},
		{9999, 1200, false, false, true, false},
		{9999, 500, false, false, false, false},
		{9999, 1500, false, true, false, false},
	} {
		r := CheckControlZone(c.vis, c.ceil, c.heli)
		if r.VFR != c.vfr || r.SpecialVFR != c.svfr || r.CeilingUnknown != c.ceUn {
			t.Errorf("vis %v ceiling %v heli %v: %+v", c.vis, c.ceil, c.heli, r)
		}
	}
	if m := SpecialVFRMinima(false); m.VisibilityM != 1500 || !m.ClearOfCloud || !m.SurfaceInSight {
		t.Errorf("special VFR %+v", m)
	}
	if SpecialVFRMinima(true).VisibilityM != 800 {
		t.Error("special VFR helicopter")
	}
}
