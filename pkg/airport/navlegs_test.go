//go:build windows
// +build windows

package airport

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func idents(pts []NavPoint) string {
	var s []string
	for _, n := range pts {
		if n.Computed() {
			s = append(s, "*")
		} else {
			s = append(s, n.Ident)
		}
	}
	return strings.Join(s, " ")
}

// TestResolveSID: VOZ5M from 24 climbs straight ahead to 518 m (a computed
// point on the runway heading, turned true), then its fixes to VOZ.
func TestResolveSID(t *testing.T) {
	p := loadLKPRProcedures(t)
	l := loadLKPR(t)
	rwy, end, _ := l.RunwayEnd("24")
	der := rwy.Primary.Threshold
	if end.Name == rwy.Primary.Name {
		der = rwy.Secondary.Threshold
	}
	pts, err := p.ResolveSID("VOZ5M", "24", "", der, l.Altitude)
	if err != nil {
		t.Fatal(err)
	}
	if got := idents(pts); got != "* PR411 PR412 VOZ" {
		t.Fatalf("VOZ5M: %s", got)
	}
	ca := pts[0]
	if ca.LegType != types.SIMCONNECT_FACILITY_LEG_TYPE_CA || math.Abs(ca.AltMin-518) > 1 || math.Abs(ca.Course-244) > 0.5 {
		t.Errorf("climb %+v", ca)
	}
	d := calc.HaversineMeters(der.Lat, der.Lon, ca.Position.Lat, ca.Position.Lon)
	if want := (518 - l.Altitude) / climbGradient; math.Abs(d-math.Max(1000, want)) > 50 {
		t.Errorf("climb ends %.0f m out", d)
	}
	if pts[3].Kind != "V" {
		t.Errorf("VOZ kind %q", pts[3].Kind)
	}
	if _, err := p.ResolveSID("VOZ5M", "06", "", der, 0); !errors.Is(err, ErrNoTransition) {
		t.Errorf("VOZ5M from 06: %v", err)
	}
	if _, err := p.ResolveSID("NOPE1A", "24", "", der, 0); !errors.Is(err, ErrNoProcedure) {
		t.Errorf("NOPE1A: %v", err)
	}
	// A SID with one runway transition resolves without a runway.
	if pts, err := p.ResolveSID("VOZ4A", "", "", der, 0); err != nil || idents(pts) != "PR402 PR403 PR404 VOZ" {
		t.Errorf("VOZ4A: %s %v", idents(pts), err)
	}
}

// TestResolveSTAR: GOLO4T ends at KUVIX, then a heading to radar vectors.
func TestResolveSTAR(t *testing.T) {
	p := loadLKPRProcedures(t)
	pts, err := p.ResolveSTAR("GOLO4T", "", "06")
	if err != nil {
		t.Fatal(err)
	}
	if got := idents(pts); got != "GOLOP PR711 PR712 PR513 KUVIX *" {
		t.Fatalf("GOLO4T: %s", got)
	}
	if last := pts[len(pts)-1]; !last.Vectors || math.Abs(last.Course-244) > 0.5 {
		t.Errorf("vectors %+v", last)
	}
}

// TestResolveApproach: ILS 06 via KUVIX; the transition's CI06 merges with
// the final's, the threshold is the MAP at 383 m.
func TestResolveApproach(t *testing.T) {
	p := loadLKPRProcedures(t)
	pts, err := p.ResolveApproach("ils 06", "KUVIX")
	if err != nil {
		t.Fatal(err)
	}
	if got := idents(pts); got != "KUVIX PR741 PR742 CI06 FF06 RW06" {
		t.Fatalf("ILS 06: %s", got)
	}
	if !pts[0].IAF || !pts[4].FAF || !pts[5].MAP || math.Abs(pts[5].AltMin-383) > 1 || pts[5].AltMax != pts[5].AltMin {
		t.Errorf("flags %+v", pts)
	}
	// The OKL transition flies outbound from FF06 (FC): a computed point.
	pts, err = p.ResolveApproach("ILS 06", "OKL")
	if err != nil || idents(pts) != "OKL FF06 * CI06 FF06 RW06" {
		t.Errorf("ILS 06 OKL: %s %v", idents(pts), err)
	}
	if _, err := p.ResolveApproach("ILS 06", "GOLOP"); !errors.Is(err, ErrNoTransition) {
		t.Errorf("GOLOP: %v", err)
	}
	m, err := p.MissedApproach("ILS 06")
	if err != nil || len(m) != 1 || !m[0].Vectors || math.Abs(m[0].AltMin-1219) > 1 {
		t.Errorf("missed %+v %v", m, err)
	}
}

// TestArrival: GOLOP for 06 is GOLO4T to KUVIX, then the ILS 06 via its
// KUVIX transition down to the threshold at 4000 ft until the FAF.
func TestArrival(t *testing.T) {
	p := loadLKPRProcedures(t)
	pts, err := p.Arrival("06", "GOLOP")
	if err != nil {
		t.Fatal(err)
	}
	if got := idents(pts); got != "GOLOP PR711 PR712 PR513 KUVIX PR741 PR742 CI06 FF06 RW06" {
		t.Fatalf("arrival: %s", got)
	}
	for _, n := range pts[5:9] {
		if math.Abs(n.AltMin-1219) > 1 {
			t.Errorf("%s at %.0f m", n.Ident, n.AltMin)
		}
	}
	if !pts[4].IAF || !pts[9].MAP {
		t.Errorf("KUVIX IAF %v, RW06 MAP %v", pts[4].IAF, pts[9].MAP)
	}
	// No STAR from OKL: its approach transition.
	if pts, err := p.Arrival("06", "OKL"); err != nil || !strings.HasPrefix(idents(pts), "OKL FF06") {
		t.Errorf("OKL: %s %v", idents(pts), err)
	}
	// Unknown entry: the final only.
	if pts, err := p.Arrival("06", ""); err != nil || idents(pts) != "CI06 FF06 RW06" {
		t.Errorf("no entry: %s %v", idents(pts), err)
	}
}

// TestProcedureSelection: ATC-style picks at LKPR.
func TestProcedureSelection(t *testing.T) {
	p := loadLKPRProcedures(t)
	if a, ok := p.BestApproach("06"); !ok || a.Name != "ILS 06" {
		t.Errorf("best 06: %s", a.Name)
	}
	if n := len(p.ApproachesFor("06")); n != 2 {
		t.Errorf("%d approaches to 06", n)
	}
	sid, enroute, ok := p.SIDToward("24", "VOZ")
	if !ok || !strings.HasPrefix(sid.Name, "VOZ") || enroute != "" {
		t.Errorf("SID 24 VOZ: %s %q %v", sid.Name, enroute, ok)
	}
	if _, _, ok := p.SIDToward("24", "GOLOP"); ok {
		t.Error("SID toward GOLOP")
	}
	star, _, ok := p.STARFrom("06", "GOLOP")
	if !ok || star.Name != "GOLO4T" {
		t.Errorf("STAR 06 GOLOP: %s", star.Name)
	}
	for _, s := range p.SIDsFor("24") {
		if _, ok := runwayTransition(s.RunwayTransitions, "24"); !ok {
			t.Errorf("%s not from 24", s.Name)
		}
	}
	if n := len(p.STARsFor("06")); n != 4 {
		t.Errorf("%d STARs to 06", n)
	}
}

// TestRunwayMatches: loose matching of procedure runways.
func TestRunwayMatches(t *testing.T) {
	for _, c := range []struct {
		have, want string
		ok         bool
	}{
		{"24", "24", true}, {"RW06", "6", true}, {"24B", "24L", true}, {"24", "24R", true},
		{"ALL", "12", true}, {"24L", "24R", false}, {"24", "06", false},
	} {
		if got := runwayMatches(c.have, c.want); got != c.ok {
			t.Errorf("%s ~ %s = %v", c.have, c.want, got)
		}
	}
}

// TestInterceptMeters: a course north meets a course east into a fix 5 km
// north, 5 km east where the lines cross.
func TestInterceptMeters(t *testing.T) {
	p := LatLon{Lat: 50, Lon: 14}
	fix := displace(displace(p, 0, 5000), 90, 5000)
	d, ok := interceptMeters(p, 0, fix, 90)
	if !ok || math.Abs(d-5000) > 50 {
		t.Errorf("%.0f %v", d, ok)
	}
	if _, ok := interceptMeters(p, 180, fix, 90); ok {
		t.Error("intercept behind")
	}
}
