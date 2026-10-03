//go:build windows
// +build windows

package nav

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

// A plan written as .pln reads back: airports, rules, cruise, the STAR and
// approach with their runway, the waypoints with their positions.
func TestReadPLN(t *testing.T) {
	w := StaticWeather(60, 10, 9999, 15, 5, 1013)
	fp, err := Plan(FlightPlanRequest{Departure: eddf, Arrival: lkprInfo(t), Type: "B738", ArrWeather: &w}, loadLKPRAirways(t))
	if err != nil {
		t.Fatal(err)
	}
	b, err := fp.PLN()
	if err != nil {
		t.Fatal(err)
	}
	p, err := ReadPLN(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if p.Departure != "EDDF" || p.Destination != "LKPR" || p.Rules != "IFR" || p.CruiseFt != float64(fp.CruiseFL*100) {
		t.Errorf("header %+v", p)
	}
	if p.STAR != "LOMK8T" || p.Approach != "ILS" || p.ArrivalRunway != "06" {
		t.Errorf("arrival %q %q %q, want LOMK8T ILS 06", p.STAR, p.Approach, p.ArrivalRunway)
	}
	if len(p.Waypoints) < 3 || p.Waypoints[0].Ident != "EDDF" || p.Waypoints[len(p.Waypoints)-1].Ident != "LKPR" {
		t.Fatalf("%d waypoints", len(p.Waypoints))
	}
	lk := p.Waypoints[len(p.Waypoints)-1].Position
	if math.Abs(lk.Lat-50.1008) > 0.01 || math.Abs(lk.Lon-14.26) > 0.01 {
		t.Errorf("LKPR at %+v", lk)
	}
}

// A user-made VFR plan (written by the airport map's VFR tool) reads too.
func TestReadPLNUserPoints(t *testing.T) {
	const src = `<?xml version="1.0" encoding="UTF-8"?>
<SimBase.Document Type="AceXML" version="1,0">
    <FlightPlan.FlightPlan>
        <FPType>VFR</FPType>
        <CruisingAlt>3500</CruisingAlt>
        <DepartureID>LKHK</DepartureID>
        <DestinationID>LKPR</DestinationID>
        <ATCWaypoint id="LKHK"><ATCWaypointType>Airport</ATCWaypointType><WorldPosition>N50° 15' 11.88",E15° 50' 43.08",+000791.00</WorldPosition><ICAO><ICAOIdent>LKHK</ICAOIdent></ICAO></ATCWaypoint>
        <ATCWaypoint id="WP01"><ATCWaypointType>User</ATCWaypointType><WorldPosition>S12° 30' 0.00",W045° 15' 0.00",+003500.00</WorldPosition></ATCWaypoint>
    </FlightPlan.FlightPlan>
</SimBase.Document>`
	p, err := ReadPLN(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if p.Rules != "VFR" || p.CruiseFt != 3500 || len(p.Waypoints) != 2 {
		t.Fatalf("%+v", p)
	}
	wp := p.Waypoints[1]
	if wp.Type != "User" || wp.AltFt != 3500 || math.Abs(wp.Position.Lat+12.5) > 1e-6 || math.Abs(wp.Position.Lon+45.25) > 1e-6 {
		t.Errorf("user point %+v", wp)
	}
	if _, err := ReadPLN(strings.NewReader("not xml")); err == nil {
		t.Error("garbage read")
	}
}

// The MSFS 2024 layout (SimBrief's "M24" export, AppVersionMajor 12): the
// runways and procedures in DepartureDetails, ArrivalDetails and
// ApproachDetails, the waypoints without positions.
func TestReadPLNMSFS2024(t *testing.T) {
	p, err := ReadPLNFile("testdata/LKPRLKPD_M24.pln")
	if err != nil {
		t.Fatal(err)
	}
	if p.Departure != "LKPR" || p.Destination != "LKPD" || p.Rules != "IFR" || p.CruiseFt != 15000 {
		t.Errorf("header %+v", p)
	}
	if p.DepartureRunway != "06" || p.SID != "" {
		t.Errorf("departure %q %q, want 06 and no SID", p.DepartureRunway, p.SID)
	}
	if p.STAR != "BEKV1Q" || p.Approach != "RNAV" || p.ArrivalRunway != "09" {
		t.Errorf("arrival %q %q %q, want BEKV1Q RNAV 09", p.STAR, p.Approach, p.ArrivalRunway)
	}
	if len(p.Waypoints) != 1 || p.Waypoints[0].Ident != "BEKVI" || p.Waypoints[0].Region != "LK" {
		t.Errorf("waypoints %+v", p.Waypoints)
	}
}
