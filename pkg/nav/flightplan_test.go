package nav

import (
	"encoding/json"
	"encoding/xml"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

func loadLKPRProcedures(t *testing.T) *airport.Procedures {
	t.Helper()
	b, err := os.ReadFile("../airport/testdata/LKPR-procedures.json")
	if err != nil {
		t.Fatal(err)
	}
	var p airport.Procedures
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return &p
}

func lkprInfo(t *testing.T) AirportInfo {
	return AirportInfo{ICAO: "LKPR", Layout: loadLKPR(t), Procedures: loadLKPRProcedures(t)}
}

// Airports known only by position (no layout, no procedures).
var (
	eddm = AirportInfo{ICAO: "EDDM", Name: "Munich", Position: airport.LatLon{Lat: 48.3538, Lon: 11.7861}, ElevationM: 453}
	eddf = AirportInfo{ICAO: "EDDF", Name: "Frankfurt", Position: airport.LatLon{Lat: 50.0333, Lon: 8.5706}, ElevationM: 111}
)

func planIdents(wps []Waypoint) string {
	var s []string
	for _, w := range wps {
		if w.Ident == "" {
			s = append(s, "*")
		} else {
			s = append(s, w.Ident)
		}
	}
	return strings.Join(s, " ")
}

// TestPlanDeparture: LKPR 24 (wind 240/10) to Munich leaves on a SID to
// the south-west, joins the airways near Munich and ends at the airport.
func TestPlanDeparture(t *testing.T) {
	w := StaticWeather(240, 10, 9999, 15, 5, 1013)
	fp, err := Plan(FlightPlanRequest{Departure: lkprInfo(t), Arrival: eddm, Type: "A20N", DepWeather: &w,
		DepLimits: lkprLimits}, loadLKPRAirways(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Log("\n" + fp.String())
	if fp.DepartureRunway != "24" || !strings.HasPrefix(fp.SID, "DOBE") {
		t.Errorf("runway %s SID %s", fp.DepartureRunway, fp.SID)
	}
	wps := fp.Waypoints
	if wps[0].Ident != "RW24" || wps[len(wps)-1].Ident != "EDDM" || wps[len(wps)-1].Kind != PointAirport {
		t.Errorf("ends: %s", planIdents(wps))
	}
	if !strings.HasPrefix(fp.Route, fp.SID+" DOBEN ") {
		t.Errorf("route %q", fp.Route)
	}
	for i, w := range wps {
		if i > 0 && w.DistanceNM < wps[i-1].DistanceNM {
			t.Errorf("%s: distance goes back", w.Ident)
		}
	}
	if fp.STAR != "" || fp.Approach != "" {
		t.Errorf("procedures at EDDM: %s %s", fp.STAR, fp.Approach)
	}
}

// TestPlanArrival: from Frankfurt to LKPR 06 (wind 060/10): the western
// STAR via LOMKI to BAROX, then the ILS 06 via BAROX at 4000 ft to the
// threshold.
func TestPlanArrival(t *testing.T) {
	w := StaticWeather(60, 10, 9999, 15, 5, 1013)
	fp, err := Plan(FlightPlanRequest{Departure: eddf, Arrival: lkprInfo(t), Type: "B738", ArrWeather: &w}, loadLKPRAirways(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Log("\n" + fp.String())
	if fp.ArrivalRunway != "06" || fp.STAR != "LOMK8T" || fp.Approach != "ILS 06" || fp.ApproachTransition != "BAROX" {
		t.Fatalf("runway %s STAR %s approach %s via %s", fp.ArrivalRunway, fp.STAR, fp.Approach, fp.ApproachTransition)
	}
	wps := fp.Waypoints
	if wps[0].Ident != "EDDF" {
		t.Errorf("first %s", wps[0].Ident)
	}
	if got := planIdents(wps); !strings.HasSuffix(got, "LOMKI PR707 BAROX PR742 CI06 FF06 RW06") {
		t.Errorf("arrival: %s", got)
	}
	last := wps[len(wps)-1]
	if last.Ident != "RW06" || !last.MAP || last.Phase != PhaseApproach {
		t.Errorf("last %+v", last)
	}
	for _, w := range wps[len(wps)-4 : len(wps)-1] {
		if w.Ident != "TOD" && (math.Abs(w.AltMinFt-4000) > 1 || w.AltFt < 4000) {
			t.Errorf("%s: constraint %s planned %.0f", w.Ident, w.Constraint(), w.AltFt)
		}
	}
	if !strings.HasSuffix(fp.Route, " LOMKI LOMK8T") || !strings.HasPrefix(fp.Route, "DCT ") {
		t.Errorf("route %q", fp.Route)
	}
}

// TestCruiseLevel: the semicircular rule and the short-hop cap.
func TestCruiseLevel(t *testing.T) {
	p := PerformanceFor("A20N")
	for _, c := range []struct {
		track, dist float64
		want        int
	}{
		{90, 800, 390}, {270, 800, 380}, {179, 800, 390}, {180, 800, 380},
	} {
		if got := CruiseLevel(p, c.track, c.dist, 1000, 1000); got != c.want {
			t.Errorf("track %.0f %0.f NM: FL%d, want FL%d", c.track, c.dist, got, c.want)
		}
	}
	// Short hops fly lower, still by the rule; the climb and descent fit.
	prev := 999
	for _, d := range []float64{300, 150, 80, 40} {
		fl := CruiseLevel(p, 90, d, 1000, 1000)
		if fl%20 != 10 || fl > prev {
			t.Errorf("%.0f NM: FL%d after FL%d", d, fl, prev)
		}
		if c, e := profileNM(p, float64(fl)*100, 1000, 1000); fl > 50 && c+e > d {
			t.Errorf("%.0f NM: FL%d needs %.0f NM", d, fl, c+e)
		}
		prev = fl
	}
	if fl := CruiseLevel(PerformanceFor("AT76"), 300, 800, 0, 0); fl != 240 {
		t.Errorf("AT76 westbound FL%d", fl)
	}
	// The floor: 2000 ft above the higher airport, by the rule.
	if fl := CruiseLevel(p, 90, 10, 1000, 1000); fl != 30 {
		t.Errorf("10 NM: FL%d", fl)
	}
}

// TestPlanFuel: fuel adds up and scales sensibly with the type.
func TestPlanFuel(t *testing.T) {
	g := loadLKPRAirways(t)
	plan := func(typ string) *FlightPlan {
		fp, err := Plan(FlightPlanRequest{Departure: lkprInfo(t), Arrival: eddm, Type: typ, AlternateFuelKg: 1200}, g)
		if err != nil {
			t.Fatal(err)
		}
		return fp
	}
	a, b := plan("A20N"), plan("B77W")
	f := a.Fuel
	if f.Total != f.Taxi+f.Trip+f.Contingency+f.Alternate+f.Reserve || f.Alternate != 1200 {
		t.Errorf("fuel %+v", f)
	}
	if math.Abs(f.Contingency-0.05*f.Trip) > 1 || f.Reserve != 1100 {
		t.Errorf("contingency %.0f reserve %.0f", f.Contingency, f.Reserve)
	}
	// About an hour's burn or less for a ~200 NM flight.
	if f.Trip < 800 || f.Trip > 2500 || a.ETE.Minutes() < 25 || a.ETE.Minutes() > 60 {
		t.Errorf("trip %.0f kg in %v over %.0f NM", f.Trip, a.ETE, a.DistanceNM)
	}
	if b.Fuel.Trip < 2.5*f.Trip {
		t.Errorf("B77W %.0f vs A20N %.0f", b.Fuel.Trip, f.Trip)
	}
	if a.TOCNM <= 0 || a.TODNM <= a.TOCNM || a.TODNM >= a.DistanceNM {
		t.Errorf("TOC %.0f TOD %.0f of %.0f", a.TOCNM, a.TODNM, a.DistanceNM)
	}
}

// TestPlanNoGraph: without airways and procedures, everything is direct.
func TestPlanNoGraph(t *testing.T) {
	fp, err := Plan(FlightPlanRequest{Departure: eddf, Arrival: eddm, Type: "XXXX", CruiseFL: 240}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := planIdents(fp.Waypoints); got != "EDDF TOC TOD EDDM" {
		t.Errorf("waypoints %s", got)
	}
	if fp.CruiseFL != 240 || fp.Route != "" || fp.Performance.Type != "" {
		t.Errorf("FL%d route %q type %q", fp.CruiseFL, fp.Route, fp.Performance.Type)
	}
	if _, err := Plan(FlightPlanRequest{Departure: AirportInfo{ICAO: "XXXX"}, Arrival: eddm}, nil); err == nil {
		t.Error("no position: no error")
	}
}

// TestPLN: the .pln is AceXML that reads back, from the departure airport
// to the destination, with procedure tags and the coordinate format.
func TestPLN(t *testing.T) {
	w := StaticWeather(60, 10, 9999, 15, 5, 1013)
	fp, err := Plan(FlightPlanRequest{Departure: eddf, Arrival: lkprInfo(t), Type: "B738", ArrWeather: &w}, loadLKPRAirways(t))
	if err != nil {
		t.Fatal(err)
	}
	b, err := fp.PLN()
	if err != nil {
		t.Fatal(err)
	}
	var doc plnDocument
	if err := xml.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	f := doc.FlightPlan
	if doc.Type != "AceXML" || f.FPType != "IFR" || f.DepartureID != "EDDF" || f.DestinationID != "LKPR" ||
		f.CruisingAlt != itoaInt(fp.CruiseFL*100) {
		t.Errorf("header %+v", f)
	}
	wps := f.Waypoints
	if first, last := wps[0], wps[len(wps)-1]; first.ID != "EDDF" || first.Type != "Airport" || last.ID != "LKPR" || last.Type != "Airport" {
		t.Errorf("ends %s %s", first.ID, last.ID)
	}
	var star, app int
	for _, w := range wps {
		if w.ArrivalFP == "LOMK8T" {
			star++
		}
		if w.ApproachTypeFP == "ILS" && w.RunwayNumberFP == "6" {
			app++
		}
		if w.ID == "RW06" || w.ID == "TOC" || w.ID == "TOD" || w.ID == "" {
			t.Errorf("waypoint %q in the .pln", w.ID)
		}
		if w.Type != "Airport" && (w.ICAO == nil || w.ICAO.Region == "") {
			t.Errorf("%s without a region", w.ID)
		}
	}
	if star < 2 || app < 3 {
		t.Errorf("%d STAR, %d approach waypoints", star, app)
	}
	if !strings.Contains(string(b), "<SimBase.Document Type=\"AceXML\" version=\"1,1\">") {
		t.Errorf("root:\n%s", b[:200])
	}
	// Coordinates as MSFS writes them, quotes unescaped.
	if want := "<DepartureLLA>" + FormatLLA(eddf.Position, 111*feetPerMeter) + "</DepartureLLA>"; !strings.Contains(string(b), want) {
		t.Errorf("no %s", want)
	}
}

func itoaInt(n int) string { b, _ := json.Marshal(n); return string(b) }

// TestFormatLLA: the .pln coordinate format.
func TestFormatLLA(t *testing.T) {
	got := FormatLLA(airport.LatLon{Lat: 50.100869, Lon: 14.260064}, 1247)
	if want := "N50° 6' 3.13\",E14° 15' 36.23\",+001247.00"; got != want {
		t.Errorf("%s, want %s", got, want)
	}
	if got := FormatLLA(airport.LatLon{Lat: -33.9461, Lon: -151.1772}, -5); got != "S33° 56' 45.96\",W151° 10' 37.92\",-000005.00" {
		t.Errorf("%s", got)
	}
}

// TestPositionAt: along the plan, at the planned altitude and track.
func TestPositionAt(t *testing.T) {
	w := StaticWeather(60, 10, 9999, 15, 5, 1013)
	fp, err := Plan(FlightPlanRequest{Departure: eddf, Arrival: lkprInfo(t), Type: "B738", ArrWeather: &w}, loadLKPRAirways(t))
	if err != nil {
		t.Fatal(err)
	}
	p, alt, trk := fp.PositionAt(fp.DistanceNM / 2)
	if alt < float64(fp.CruiseFL)*100-1000 {
		t.Errorf("halfway at %.0f ft, cruise FL%d", alt, fp.CruiseFL)
	}
	if trk < 45 || trk > 135 { // EDDF → LKPR: about east
		t.Errorf("track %.0f", trk)
	}
	start, _, _ := fp.PositionAt(0)
	if d := calc.HaversineNM(start.Lat, start.Lon, fp.Waypoints[0].Position.Lat, fp.Waypoints[0].Position.Lon); d > 0.1 {
		t.Errorf("at 0 NM %.1f NM from the first point", d)
	}
	if p == start {
		t.Error("halfway is the start")
	}
}
