package traffic

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestRoundAircraft: a leg from behind the tail to the nose goes round a
// wingtip, every part of it clear of the aircraft; a clear leg is left.
func TestRoundAircraft(t *testing.T) {
	prof := MotionProfileFor("A320")
	pose := GroundPose{Position: airport.LatLon{Lat: 50.1, Lon: 14.26}, Heading: 0}
	f := aircraftFrame{nose: NoseGear(pose.Position, pose.Heading, prof), hdg: pose.Heading, prof: prof}
	behind := f.from(0, -60)
	front := f.from(0, TugApproachMeters)
	if f.clear(behind, front) {
		t.Fatal("a leg through the fuselage reads clear")
	}
	via := roundAircraft(behind, front, pose, prof)
	if len(via) == 0 {
		t.Fatal("no way round")
	}
	pts := append(append([]airport.LatLon{behind}, via...), front)
	for i := 1; i < len(pts); i++ {
		if !f.clear(pts[i-1], pts[i]) {
			t.Errorf("leg %d crosses the aircraft", i)
		}
	}
	side := f.from(60, 30)
	if roundAircraft(side, front, pose, prof) != nil {
		t.Error("a clear leg was detoured")
	}
}

// TestTugsKeepOffAircraftLROP: at every LROP stand the tug's way in, from
// the nearest depot along the roads to the nose, keeps off the aircraft.
func TestTugsKeepOffAircraftLROP(t *testing.T) {
	b, err := os.ReadFile("../airport/testdata/LROP-layout.json")
	if err != nil {
		t.Skip(err)
	}
	var l airport.Layout
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	prof := MotionProfileFor("A320")
	crossed, checked := 0, 0
	for _, p := range l.Parking {
		if p.Size() == airport.StandNone || p.Radius*2 < prof.SpanMeters {
			continue
		}
		pose := GroundPose{Position: p.Position, Heading: p.Heading}
		nose := NoseGear(pose.Position, pose.Heading, prof)
		front := offsetHeading(nose, pose.Heading, TugApproachMeters)
		depot, ok := nearestDepot(&l, front)
		if !ok {
			t.Skip("no depots")
		}
		route, err := l.VehicleRoute(depot, front)
		if err != nil || len(route) < 2 {
			continue
		}
		checked++
		f := aircraftFrame{nose: nose, hdg: pose.Heading, prof: prof}
		n := len(route)
		if !f.clear(route[n-2], route[n-1]) {
			crossed++
			via := roundAircraft(route[n-2], route[n-1], pose, prof)
			pts := append(append([]airport.LatLon{route[n-2]}, via...), route[n-1])
			for i := 1; i < len(pts); i++ {
				if !f.clear(pts[i-1], pts[i]) {
					t.Errorf("stand %s: still through the aircraft", p.Label())
				}
			}
		}
	}
	t.Logf("%d stands, %d ways in crossed the aircraft before going round", checked, crossed)
}
