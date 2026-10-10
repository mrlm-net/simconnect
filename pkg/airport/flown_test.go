package airport

import (
	"encoding/json"
	"math"
	"os"
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

func loadLayout(t *testing.T, icao string) *Layout {
	t.Helper()
	b, err := os.ReadFile("testdata/" + icao + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var raw RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// fixPosition is the position of fix ident in p's legs.
func fixPosition(p Procedures, ident string) (LatLon, bool) {
	var all []Leg
	for _, s := range append(p.Arrivals, p.Departures...) {
		all = append(all, s.Legs...)
		for _, tr := range append(s.RunwayTransitions, s.EnrouteTransitions...) {
			all = append(all, tr.Legs...)
		}
	}
	for _, l := range all {
		if l.Fix == ident && l.HasFix() {
			return l.Position, true
		}
	}
	return LatLon{}, false
}

// TestFlownRouteLOWWToLKPR: out of LOWW 16 on the runway heading (no
// procedures there) and on course, the filed route to APRAQ, the STAR
// APRA2S entered there, ILS 24 through the transition starting at the
// STAR's last fix (RATEV), to the threshold; the missed approach apart; no
// point twice in a row and no hairpin.
func TestFlownRouteLOWWToLKPR(t *testing.T) {
	loww, lkpr := loadLayout(t, "LOWW"), loadLKPR(t)
	p := loadLKPRProcedures(t)
	apraq, ok := fixPosition(p, "APRAQ")
	if !ok {
		t.Fatal("no APRAQ")
	}
	mid := LatLon{Lat: (loww.Latitude + apraq.Lat) / 2, Lon: (loww.Longitude + apraq.Lon) / 2}
	r, err := FlownRouteFor(FlownRequest{Departure: loww, Arrival: lkpr, DepartureRunway: "16", ArrivalRunway: "24",
		ArrivalProcedures: p, Route: []RouteFix{{Ident: "MIDPT", Position: mid}, {Ident: "APRAQ", Position: apraq}}})
	if err != nil {
		t.Fatal(err)
	}
	if r.STAR != "APRA2S" || r.Approach != "ILS 24" || r.ApproachTransition != "RATEV" || r.SID != "" {
		t.Errorf("took SID %q STAR %q approach %q via %q", r.SID, r.STAR, r.Approach, r.ApproachTransition)
	}
	var idents []string
	for _, pt := range r.Points {
		if pt.Ident != "" {
			idents = append(idents, pt.Ident)
		}
	}
	if idents[0] != "RW16" || idents[len(idents)-1] != "RW24" {
		t.Errorf("from %s to %s, want RW16 to RW24", idents[0], idents[len(idents)-1])
	}
	for _, want := range []string{"MIDPT", "APRAQ", "RATEV", "CI24", "FF24"} {
		if !slices.Contains(idents, want) {
			t.Errorf("no %s in %v", want, idents)
		}
	}
	if i, j := slices.Index(idents, "RATEV"), slices.Index(idents, "CI24"); i < 0 || j < i {
		t.Errorf("RATEV and CI24 out of order: %v", idents)
	}
	if len(r.Missed) == 0 {
		t.Error("no missed approach")
	}
	pts := r.Points
	for i := 1; i < len(pts); i++ {
		a, b := pts[i-1].Position, pts[i].Position
		if calc.HaversineMeters(a.Lat, a.Lon, b.Lat, b.Lon) <= flownSameMeters {
			t.Errorf("point %d repeats %d", i, i-1)
		}
		if i+1 < len(pts) {
			c := pts[i+1].Position
			in, out := calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon), calc.BearingDegrees(b.Lat, b.Lon, c.Lat, c.Lon)
			if turn := math.Abs(math.Mod(out-in+540, 360) - 180); turn > 150 {
				t.Errorf("hairpin of %.0f° at point %d (%s)", turn, i, pts[i].Ident)
			}
		}
	}
}

// TestFlownRouteSynthetic: with no procedures at the arrival, a final
// FlownFinalNM out on the extended centreline, then the threshold.
func TestFlownRouteSynthetic(t *testing.T) {
	loww, lkpr := loadLayout(t, "LOWW"), loadLKPR(t)
	r, err := FlownRouteFor(FlownRequest{Departure: loww, Arrival: lkpr, DepartureRunway: "16", ArrivalRunway: "24"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Approach != "synthetic" {
		t.Fatalf("approach %q", r.Approach)
	}
	n := len(r.Points)
	final, thr := r.Points[n-2], r.Points[n-1]
	if final.Ident != "FINAL" || thr.Ident != "RW24" {
		t.Fatalf("ends %s, %s", final.Ident, thr.Ident)
	}
	if d := calc.HaversineNM(final.Position.Lat, final.Position.Lon, thr.Position.Lat, thr.Position.Lon); math.Abs(d-FlownFinalNM) > 0.1 {
		t.Errorf("final joined %.1f NM out", d)
	}
	a := r.Points[n-3].Position
	in := calc.BearingDegrees(a.Lat, a.Lon, final.Position.Lat, final.Position.Lon)
	rwy, _ := lkpr.runwayEnd("24")
	if turn := math.Abs(math.Mod(rwy.Heading-in+540, 360) - 180); turn > 90 {
		t.Errorf("intercept of %.0f°", turn)
	}
}

// TestFlownRouteSID: out of LKPR on a SID that ends at the first route fix,
// joining the route there (the fix not twice); Smoothed keeps fly-over fixes.
func TestFlownRouteSID(t *testing.T) {
	lkpr, loww := loadLKPR(t), loadLayout(t, "LOWW")
	p := loadLKPRProcedures(t)
	sids := p.SIDsFor("24")
	if len(sids) == 0 {
		t.Fatal("no SID for 24")
	}
	rt, _ := runwayTransition(sids[0].RunwayTransitions, "24")
	exit := lastFixIdent(append(slices.Clone(rt.Legs), sids[0].Legs...))
	at, ok := fixPosition(p, exit)
	if !ok {
		t.Fatalf("no %s", exit)
	}
	r, err := FlownRouteFor(FlownRequest{Departure: lkpr, Arrival: loww, DepartureRunway: "24", ArrivalRunway: "16",
		DepartureProcedures: p, Route: []RouteFix{{Ident: exit, Position: at}, {Ident: "NEXT", Position: LatLon{Lat: at.Lat - 0.5, Lon: at.Lon + 0.5}}}})
	if err != nil {
		t.Fatal(err)
	}
	if r.SID != sids[0].Name {
		t.Errorf("SID %q, want %s", r.SID, sids[0].Name)
	}
	n := 0
	for _, pt := range r.Points {
		if pt.Ident == exit {
			n++
			if pt.Phase != "sid" {
				t.Errorf("%s in phase %s", exit, pt.Phase)
			}
		}
	}
	if n != 1 {
		t.Errorf("%s %d times", exit, n)
	}
	if s := r.Smoothed(TurnRadiusEnroute); len(s) < len(r.Points) {
		t.Errorf("smoothed %d points of %d", len(s), len(r.Points))
	}
}

// TestFlownRouteWrongRunway: a SID filed for another runway (the runway
// changed) is taken as not given and one of its family for the runway
// flown; an approach for another runway gives way to the runway's best.
func TestFlownRouteWrongRunway(t *testing.T) {
	lkpr := loadLKPR(t)
	p := loadLKPRProcedures(t)
	var other, want string
	for _, d := range p.SIDsFor("12") {
		for _, e := range p.SIDsFor("30") {
			if family(d.Name) == family(e.Name) && d.Name != e.Name {
				other, want = d.Name, family(e.Name)
			}
		}
	}
	if other == "" {
		t.Skip("no SID family serving both 12 and 30")
	}
	r, err := FlownRouteFor(FlownRequest{Departure: lkpr, Arrival: lkpr, DepartureRunway: "30", ArrivalRunway: "24",
		DepartureProcedures: p, ArrivalProcedures: p, SID: other, Approach: "ILS 06"})
	if err != nil {
		t.Fatal(err)
	}
	if r.SID == other || family(r.SID) != want {
		t.Errorf("SID %q for 30, want one of %s* (not %s)", r.SID, want, other)
	}
	if r.Approach != "ILS 24" {
		t.Errorf("approach %q for 24", r.Approach)
	}
}
