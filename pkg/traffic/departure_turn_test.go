package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// along is the point nm from p on hdg.
func along(p airport.LatLon, hdg, nm float64) airport.LatLon {
	lat, lon := calc.DisplaceByHeading(p.Lat, p.Lon, hdg, nm*1852)
	return airport.LatLon{Lat: lat, Lon: lon}
}

// TestClosestApproach: two flown along their ways in time. Head-on on one
// line they meet; on parallel lines 5 NM apart they pass 5 NM apart; one
// behind the other the same way at the same speed keeps its distance; a
// way that has ended (an arrival landed) is no longer met.
func TestClosestApproach(t *testing.T) {
	o := airport.LatLon{Lat: 45, Lon: 15}
	east := []RoutePoint{{Position: o, Kts: 200}, {Position: along(o, 90, 60), Kts: 200}}
	west := []RoutePoint{{Position: along(o, 90, 30), Kts: 150}, {Position: o, Kts: 150}}
	if d := ClosestApproachNM(east, 0, 1, west, 1); d > 0.05 {
		t.Errorf("head-on on one line: closest %.2f NM, want 0", d)
	}
	side := along(o, 0, 5)
	parallel := []RoutePoint{{Position: along(side, 90, 30), Kts: 150}, {Position: side, Kts: 150}}
	if d := ClosestApproachNM(east, 0, 1, parallel, 1); math.Abs(d-5) > 0.05 {
		t.Errorf("parallel 5 NM apart: closest %.2f NM, want 5", d)
	}
	ahead := []RoutePoint{{Position: along(o, 90, 10), Kts: 200}, {Position: along(o, 90, 70), Kts: 200}}
	if d := ClosestApproachNM(east, 0, 1, ahead, 1); math.Abs(d-10) > 0.05 {
		t.Errorf("in trail 10 NM: closest %.2f NM, want 10", d)
	}
	// Down before the other is airborne: 30 NM at 150 kt is 12 minutes.
	if d := ClosestApproachNM(east, 13*time.Minute, 1, west, 1); !math.IsInf(d, 1) {
		t.Errorf("one airborne after the other has landed: closest %.2f NM, want none", d)
	}
	// Faster, they meet farther out, still on the line.
	if d := ClosestApproachNM(east, 0, 1.25, west, 0.85); d > 0.05 {
		t.Errorf("head-on, speeds scaled: closest %.2f NM, want 0", d)
	}
}

// TestDepartureTurnPath: a departure off a runway heading 090 turned right
// heading 135 at a corner ahead flies to the corner, 10 NM on 135, then
// on 090 again, off to the side; handed over past the corner it turns a
// mile on from where it is.
func TestDepartureTurnPath(t *testing.T) {
	off := airport.LatLon{Lat: 45, Lon: 15}
	corner := along(off, 90, 3)
	turn := DepartureTurn{Corner: corner, Heading: 135, LegNM: 10}
	p := DepartureTurnPath(off, 90, turn, nil)
	if len(p) != 3 {
		t.Fatalf("path %v: want corner, leg end and on", p)
	}
	if d := calc.HaversineNM(p[0].Lat, p[0].Lon, corner.Lat, corner.Lon); d > 0.01 {
		t.Errorf("corner %.2f NM off", d)
	}
	if d := calc.HaversineNM(p[0].Lat, p[0].Lon, p[1].Lat, p[1].Lon); math.Abs(d-10) > 0.05 {
		t.Errorf("leg %.2f NM, want 10", d)
	}
	if b := calc.BearingDegrees(p[0].Lat, p[0].Lon, p[1].Lat, p[1].Lon); math.Abs(headingDiff(b, 135)) > 0.5 {
		t.Errorf("leg on %.1f, want 135", b)
	}
	if on := along(p[1], 90, ProcedureContinueMeters/1852); calc.HaversineNM(p[2].Lat, p[2].Lon, on.Lat, on.Lon) > 0.05 {
		t.Errorf("on after the leg to %v, want %v on the take-off heading 090", p[2], on)
	}
	past := along(off, 90, 3.2)
	c := turn.CornerFrom(past, 90)
	if d := calc.HaversineNM(past.Lat, past.Lon, c.Lat, c.Lon); math.Abs(d-departureTurnAheadNM) > 0.01 {
		t.Errorf("handed over past the corner: turns %.2f NM on, want %.1f", d, departureTurnAheadNM)
	}
}

// TestDepartureTurnHeadOn: a departure straight out on 090 and an arrival
// on a 30 NM straight-in to the other end meet on the extended
// centreline; turned right heading 135 for 8 NM and back on 090 they pass
// more than OppositeSeparationNM apart (TestDeclaredField's departureFirst).
func TestDepartureTurnHeadOn(t *testing.T) {
	thr09 := airport.LatLon{Lat: 45, Lon: 15}
	thr27 := along(thr09, 90, 1.3)
	off := along(thr09, 90, 1)
	arrIn := []RoutePoint{{Position: along(thr27, 90, 30), Kts: 90}, {Position: thr27, Kts: 90}}
	way := func(pts []airport.LatLon) []RoutePoint {
		out := []RoutePoint{{Position: off, Kts: 180}}
		for _, p := range pts {
			out = append(out, RoutePoint{Position: p, Kts: 250})
		}
		return out
	}
	straight := way([]airport.LatLon{along(off, 90, 200)})
	if d := ClosestApproachNM(straight, time.Minute, 1, arrIn, 1); d >= OppositeSeparationNM {
		t.Fatalf("straight out: closest %.1f NM, want them to meet", d)
	}
	turn := DepartureTurn{Corner: along(thr27, 90, 1), Heading: 135, LegNM: 8}
	if d := ClosestApproachNM(way(DepartureTurnPath(off, 90, turn, nil)), time.Minute, 1, arrIn, 1); d < OppositeSeparationNM {
		t.Errorf("turned right heading 135 for 8 NM: closest %.1f NM, want at least %.0f", d, OppositeSeparationNM)
	}
}
