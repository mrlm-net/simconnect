package airport

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// A tug's way from the nearest vehicle depot to a stand at LKPR: mostly on
// the vehicle roads, never on a runway.
func TestVehicleRoute(t *testing.T) {
	l := loadLKPR(t)
	depots := l.VehicleDepots()
	if len(depots) == 0 {
		t.Fatal("LKPR has vehicle parking spots")
	}
	i, err := l.ParkingIndex("B9")
	if err != nil {
		t.Fatal(err)
	}
	stand := l.Parking[i].Position
	best, bestD := -1, 1e18
	for _, d := range depots {
		if m := distM(l.Parking[d].Position, stand); m < bestD {
			best, bestD = d, m
		}
	}
	route, err := l.VehicleRoute(l.Parking[best].Position, stand)
	if err != nil {
		t.Fatal(err)
	}
	length := 0.0
	for k := 1; k < len(route); k++ {
		length += distM(route[k-1], route[k])
	}
	t.Logf("depot %s to B9: %d points, %.0f m (straight %.0f m)", l.Parking[best].Label(), len(route), length, bestD)
	if len(route) < 3 || length < bestD {
		t.Errorf("route of %d points, %.0f m", len(route), length)
	}
	// No point on a runway's centreline strip.
	for _, p := range route {
		for _, r := range l.Runways {
			if onRunwayStrip(r, p) {
				t.Errorf("route crosses onto runway %s at %v", r.Primary.Name, p)
				return
			}
		}
	}
	if _, err := l.VehicleRoute(LatLon{Lat: 10, Lon: 10}, stand); err == nil {
		t.Error("a route from nowhere")
	}
	_ = types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_VEHICLE
}

// onRunwayStrip: within half the runway's width of its centreline, between
// its ends.
func onRunwayStrip(r Runway, p LatLon) bool {
	a, b := r.Primary.Threshold, r.Secondary.Threshold
	ab, ap := distM(a, b), distM(a, p)
	bp := distM(b, p)
	if ap > ab || bp > ab {
		return false
	}
	// Height of the triangle over ab.
	s := (ab + ap + bp) / 2
	area := s * (s - ab) * (s - ap) * (s - bp)
	if area <= 0 {
		return true
	}
	h := 2 * math.Sqrt(area) / ab
	return h < r.Width/2
}

// From a stand with a service road behind it, the way out joins the road
// straight from the stand: its first leg reaches a vehicle road, not the
// taxiway in front.
func TestVehicleRouteJoinsRoad(t *testing.T) {
	l := loadLKPR(t)
	g := l.vehicleGraph()
	if len(g.roads) == 0 {
		t.Fatal("LKPR has vehicle roads")
	}
	joined := 0
	for i, p := range l.Parking {
		if p.Type == types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_VEHICLE || !l.NearVehicleRoad(p.Position) {
			continue
		}
		depots := l.VehicleDepots()
		route, err := l.VehicleRoute(p.Position, l.Parking[depots[0]].Position)
		if err != nil {
			continue
		}
		onRoad := false
		for _, r := range g.roads {
			if distM(route[1], closestOnSegment(route[1], g.pos[r[0]], g.pos[r[1]])) < 1 {
				onRoad = true
			}
		}
		if !onRoad {
			t.Errorf("stand %d (%s): the first leg ends off the roads at %v", i, p.Label(), route[1])
		}
		joined++
	}
	if joined == 0 {
		t.Error("no stand near a vehicle road")
	}
	t.Logf("%d stands join a road", joined)
}
