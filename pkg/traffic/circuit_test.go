//go:build windows
// +build windows

package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/convert"
)

// side is which side of the line through a on heading hdg p lies: negative
// left, positive right (meters).
func side(a airport.LatLon, hdg float64, p airport.LatLon) float64 {
	return math.Sin((localBearing(a, p)-hdg)*math.Pi/180) * localDist(a, p)
}

// A C172's circuit on LKPR 24: left-hand by default, 1000 ft above the
// field on the downwind, the downwind about a mile out, the final on the
// centreline; right-hand and other figures as configured.
func TestCircuit(t *testing.T) {
	l := lkprGraph(t).Layout
	p := ProfileFor("Asobo PassiveAircraft C172")
	_, end, _ := l.RunwayEnd("24")
	field := convert.MetersToFeet(l.Altitude)
	c, err := NewCircuit(l, "24", CircuitConfig{}, p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Side != CircuitLeft || len(c.Points) != 6 || c.DownwindNM < CircuitMinDownwindNM || c.DownwindNM > 1.5 {
		t.Fatalf("circuit %+v", c)
	}
	dw, _ := c.Point(LegDownwind)
	if math.Abs(dw.AltFt-(field+1000)) > 1 {
		t.Errorf("downwind at %.0f ft, field %.0f", dw.AltFt, field)
	}
	if s := side(end.Threshold, end.Heading, dw.Position); s > -0.7*1852 {
		t.Errorf("left-hand downwind %.0f m from the centreline", s)
	}
	fin, _ := c.Point(LegFinal)
	if s := side(end.Threshold, end.Heading, fin.Position); math.Abs(s) > 5 {
		t.Errorf("final %.0f m off the centreline", s)
	}
	if fin.Kts >= dw.Kts || fin.AltFt >= dw.AltFt {
		t.Errorf("final %+v after downwind %+v", fin, dw)
	}
	// The 45° join: outside the circuit, a mile from midfield, met at 45°.
	entry, mid := c.JoinDownwind()
	if d := localDist(entry.Position, mid.Position); math.Abs(d-1852) > 20 {
		t.Errorf("join %.0f m from midfield", d)
	}
	if side(end.Threshold, end.Heading, entry.Position) > side(end.Threshold, end.Heading, mid.Position) {
		t.Error("join from inside the circuit")
	}
	downwindDir := end.Heading + 180
	if a := math.Abs(headingDiff(localBearing(entry.Position, mid.Position), downwindDir)); math.Abs(a-45) > 2 {
		t.Errorf("join meets the downwind at %.0f°", a)
	}
	// Configured: right-hand, 800 ft, a 1.2 NM downwind.
	r, _ := NewCircuit(l, "24", CircuitConfig{Side: CircuitRight, HeightFt: 800, DownwindNM: 1.2}, p)
	rdw, _ := r.Point(LegDownwind)
	if s := side(end.Threshold, end.Heading, rdw.Position); math.Abs(s-1.2*1852) > 20 {
		t.Errorf("right-hand downwind %.0f m from the centreline", s)
	}
	if math.Abs(rdw.AltFt-(field+800)) > 1 {
		t.Errorf("configured height: %.0f", rdw.AltFt)
	}
	if wps := c.Waypoints(LegDownwind); len(wps) < 4 {
		t.Errorf("%d waypoints from downwind", len(wps))
	}
	if _, err := NewCircuit(l, "99", CircuitConfig{}, p); err != ErrNoRunway {
		t.Errorf("no runway: %v", err)
	}
}

// A circuit arrival appears at the 45° entry heading for midfield, flies
// midfield, downwind, base and the final point, and joins there, a mile
// out, with a short-final takeover allowed.
func TestPlanCircuitArrival(t *testing.T) {
	l := lkprGraph(t).Layout
	c, err := NewCircuit(l, "24", CircuitConfig{}, ProfileFor("C172"))
	if err != nil {
		t.Fatal(err)
	}
	p := PlanCircuitArrival(c)
	entry, mid := c.JoinDownwind()
	at := airport.LatLon{Lat: p.Spawn.Latitude, Lon: p.Spawn.Longitude}
	if localDist(at, entry.Position) > 1 || math.Abs(headingDiff(p.Spawn.Heading, localBearing(entry.Position, mid.Position))) > 1 {
		t.Errorf("spawn %+v, entry %+v", p.Spawn, entry)
	}
	if len(p.Waypoints) != 4 || math.Abs(p.JoinMeters-CircuitBaseNM*1852) > 30 || p.MinJoinMeters >= p.JoinMeters {
		t.Errorf("%d waypoints, join %.0f m, min %.0f m", len(p.Waypoints), p.JoinMeters, p.MinJoinMeters)
	}
	if p.minJoin() != p.MinJoinMeters || (&ArrivalProcedure{}).minJoin() != 2*1852 {
		t.Error("minJoin")
	}
}

// TestCircuitDeparture: from LKPR 24 (left-hand circuit, to the south of
// the centreline) a VFR departure goes straight out to an exit ahead, by
// the crosswind to one on the circuit's side, turns away to one on the
// other side, and leaves behind by the downwind on the circuit's side or
// the mirrored legs on the other side, never crossing the centreline
// before the exit; the exit is VFRExitNM out, VFRExitAboveFt above circuit
// height. MSFS AI gets it at the circuit speed, from the first point ahead.
func TestCircuitDeparture(t *testing.T) {
	l := lkprGraph(t).Layout
	p := ProfileFor("Asobo PassiveAircraft C172")
	c, err := NewCircuit(l, "24", CircuitConfig{}, p)
	if err != nil {
		t.Fatal(err)
	}
	hdg := c.heading
	thr, _ := c.Point(LegRunway)
	up, _ := c.Point(LegUpwind)
	// Signed distance right of the centreline (m).
	right := func(q airport.LatLon) float64 {
		return calc.CrossTrackMeters(thr.Position.Lat, thr.Position.Lon, up.Position.Lat, up.Position.Lon, q.Lat, q.Lon)
	}
	for _, tc := range []struct {
		name    string
		rel     float64
		n       int
		circuit bool // the legs on the circuit's side (left of 24)
	}{
		{"ahead", 10, 2, false}, {"circuit side", -90, 3, true}, {"other side", 90, 2, false},
		{"behind, circuit side", -170, 4, true}, {"behind, other side", 170, 4, false},
	} {
		route := c.Departure(hdg + tc.rel)
		if len(route) != tc.n {
			t.Errorf("%s: %d points, want %d", tc.name, len(route), tc.n)
			continue
		}
		for _, q := range route[1 : len(route)-1] {
			if r := right(q.Position); tc.circuit && r > -100 || !tc.circuit && r < 100 {
				t.Errorf("%s: a leg %.0f m right of the centreline", tc.name, r)
			}
		}
		exit := route[len(route)-1]
		if d := localDist(exit.Position, airport.LatLon{Lat: l.Latitude, Lon: l.Longitude}) / 1852; d < VFRExitNM-1.5 || d > VFRExitNM+1.5 {
			t.Errorf("%s: exit %.1f NM out", tc.name, d)
		}
		if want := (c.HeightFt + VFRExitAboveFt) * 0.3048; math.Abs(exit.AltMax-want) > 1 {
			t.Errorf("%s: exit at %.0f m, want %.0f", tc.name, exit.AltMax, want)
		}
		wps := VFRDepartureWaypoints(offsetHeading(up.Position, hdg+180, 600), hdg, route, MaxBankDeg(p))
		if len(wps) < 2 || math.Abs(wps[0].KtsSpeed-CircuitKts(p)) > 1 {
			t.Errorf("%s: %d waypoints, first at %.0f kt", tc.name, len(wps), wps[0].KtsSpeed)
		}
	}
}

// TestReportingPoints: a VFR departure via a reporting point ends over it,
// named; an arrival over one appears there and flies to the 45° entry
// first (#566).
func TestReportingPoints(t *testing.T) {
	l := lkprGraph(t).Layout
	c, err := NewCircuit(l, "24", CircuitConfig{}, ProfileFor("C172"))
	if err != nil {
		t.Fatal(err)
	}
	field := airport.LatLon{Lat: l.Latitude, Lon: l.Longitude}
	nov := ReportingPoint{Name: "NOVEMBER", Position: offsetHeading(field, 0, 6*1852)}
	route := c.DepartureVia(nov)
	last := route[len(route)-1]
	if last.Ident != "NOVEMBER" || localDist(last.Position, nov.Position) > 1 {
		t.Errorf("departure ends at %q %.0f m from NOVEMBER", last.Ident, localDist(last.Position, nov.Position))
	}
	proc := PlanCircuitArrivalFrom(c, &nov)
	at := airport.LatLon{Lat: proc.Spawn.Latitude, Lon: proc.Spawn.Longitude}
	entry, _ := c.JoinDownwind()
	first := airport.LatLon{Lat: proc.Waypoints[0].Latitude, Lon: proc.Waypoints[0].Longitude}
	if localDist(at, nov.Position) > 1 || localDist(first, entry.Position) > 1 {
		t.Errorf("appears %.0f m from NOVEMBER, first waypoint %.0f m from the 45° entry", localDist(at, nov.Position), localDist(first, entry.Position))
	}
	if n := len(proc.Waypoints); n != len(PlanCircuitArrival(c).Waypoints)+1 {
		t.Errorf("%d waypoints", n)
	}
}
