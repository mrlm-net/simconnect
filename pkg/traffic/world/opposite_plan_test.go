package world

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// oppositeRunway is a runway 09/27 of 2400 m at 45N 15E.
func oppositeRunway() (airport.Runway, airport.RunwayEnd, airport.RunwayEnd) {
	w := airport.LatLon{Lat: 45, Lon: 15}
	lat, lon := calc.DisplaceByHeading(w.Lat, w.Lon, 90, 2400)
	e := airport.LatLon{Lat: lat, Lon: lon}
	r := airport.Runway{Heading: 90, Length: 2400, Width: 30,
		Primary:   airport.RunwayEnd{Name: "09", Heading: 90, Threshold: w},
		Secondary: airport.RunwayEnd{Name: "27", Heading: 270, Threshold: e}}
	return r, r.Primary, r.Secondary
}

// TestOppositePlanned: off 09 with an arrival on a 30 NM straight-in to
// 27, straight out meets it; the turns are tried the least off the way
// first, and the first that clears keeps it clear at every speed and
// lift-off planned. With no arrival there, nothing is met.
func TestOppositePlanned(t *testing.T) {
	rwy, e09, e27 := oppositeRunway()
	lat, lon := calc.DisplaceByHeading(e27.Threshold.Lat, e27.Threshold.Lon, 90, 30*1852)
	in := []oppositeWay{{tail: "ARR1", path: []traffic.RoutePoint{{Position: airport.LatLon{Lat: lat, Lon: lon}, Kts: 90}, {Position: e27.Threshold, Kts: 90}}}}
	way := func(turn *traffic.DepartureTurn) func(float64) []traffic.RoutePoint {
		return func(late float64) []traffic.RoutePoint { return plannedWayOut(rwy, e09, nil, turn, late) }
	}
	if who := oppositeMeets(way(nil), nil); who != "" {
		t.Errorf("no arrival: meets %q", who)
	}
	if who := oppositeMeets(way(nil), in); who != "ARR1" {
		t.Fatalf("straight out against a straight-in to the other end: meets %q, want ARR1", who)
	}
	turns := oppositeTurns(rwy, e09, nil, in, 0)
	if len(turns) != len(oppositeTurnDeg)*2*len(oppositeLegNM) {
		t.Fatalf("%d turns tried", len(turns))
	}
	off := func(tn traffic.DepartureTurn) float64 {
		return tn.LegNM * math.Abs(math.Sin(headingDiff(e09.Heading, tn.Heading)*math.Pi/180))
	}
	for i := 1; i < len(turns); i++ {
		if off(turns[i]) < off(turns[i-1])-1e-9 {
			t.Fatalf("turn %d (%+v) is less off the way than the one before (%+v)", i, turns[i], turns[i-1])
		}
	}
	var found *traffic.DepartureTurn
	for _, tn := range turns {
		if oppositeMeets(way(&tn), in) == "" {
			found = &tn
			break
		}
	}
	if found == nil {
		t.Fatal("no turn clears an arrival 30 NM out")
	}
	need := traffic.OppositeSeparationNM + oppositeBufferNM
	for _, late := range []float64{0, oppositeCornerLateNM} {
		for _, d := range oppositeLiftoffIn {
			for _, ds := range oppositeDepScales {
				for _, as := range oppositeArrScales {
					if c := traffic.ClosestApproachNM(plannedWayOut(rwy, e09, nil, found, late), d, ds, in[0].path, as); c < need {
						t.Errorf("turn %+v, corner %.0f NM late, lift-off in %s, speeds x%.2f x%.2f: %.1f NM, want %.0f", *found, late, d, ds, as, c, need)
					}
				}
			}
		}
	}
	t.Logf("turned %s heading %.0f for %.0f NM", traffic.TurnTo(e09.Heading, found.Heading), found.Heading, found.LegNM)
}

// TestOppositeTurnSide: a turn goes toward the SID's first point, and with
// no SID away from where the first arrival is.
func TestOppositeTurnSide(t *testing.T) {
	rwy, e09, e27 := oppositeRunway()
	at := func(brg, nm float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(e27.Threshold.Lat, e27.Threshold.Lon, brg, nm*1852)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	north := []oppositeWay{{tail: "ARR1", path: []traffic.RoutePoint{{Position: at(45, 20)}, {Position: e27.Threshold}}}}
	if tn := oppositeTurns(rwy, e09, nil, north, 0)[0]; traffic.TurnTo(e09.Heading, tn.Heading) != "right" {
		t.Errorf("an arrival from the north-east: first turn %+v, want right (away)", tn)
	}
	south := []oppositeWay{{tail: "ARR1", path: []traffic.RoutePoint{{Position: at(135, 20)}, {Position: e27.Threshold}}}}
	if tn := oppositeTurns(rwy, e09, nil, south, 0)[0]; traffic.TurnTo(e09.Heading, tn.Heading) != "left" {
		t.Errorf("an arrival from the south-east: first turn %+v, want left (away)", tn)
	}
	sid := []airport.NavPoint{{Ident: "NORTH", Position: at(30, 15)}}
	if tn := oppositeTurns(rwy, e09, sid, south, 0)[0]; traffic.TurnTo(e09.Heading, tn.Heading) != "left" {
		t.Errorf("a SID to the north-east: first turn %+v, want left (toward it)", tn)
	}
}
