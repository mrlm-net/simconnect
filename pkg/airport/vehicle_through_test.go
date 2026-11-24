package airport

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// throughLayout: taxi points A (0, 0) and B (100, 0) m, a stand S between
// them at (50, 0) with a lead-in from each (PARKING), and with around a
// taxiway round by (0, 180) and (100, 180): 460 m at vehicleOffRoad, dearer
// than the 100 m of lead-ins it would cost before (#1630).
func throughLayout(around bool) (*Layout, LatLon, LatLon, LatLon) {
	at := func(x, y float64) LatLon {
		return LatLon{Lat: 50.1 + y/111320, Lon: 14.26 + x/(111320*math.Cos(50.1*math.Pi/180))}
	}
	a, b, s := at(0, 0), at(100, 0), at(50, 0)
	l := &Layout{
		TaxiPoints: []TaxiPoint{{Index: 0, Position: a}, {Index: 1, Position: b}},
		Parking:    []Parking{{Index: 0, Type: types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_MEDIUM, Radius: 20, Position: s}},
		TaxiPaths: []TaxiPath{
			{Index: 0, Type: types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_PARKING, Start: 0, End: 0},
			{Index: 1, Type: types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_PARKING, Start: 1, End: 0},
		},
	}
	if around {
		l.TaxiPoints = append(l.TaxiPoints, TaxiPoint{Index: 2, Position: at(0, 180)}, TaxiPoint{Index: 3, Position: at(100, 180)})
		l.TaxiPaths = append(l.TaxiPaths,
			TaxiPath{Index: 2, Type: types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_TAXI, Start: 0, End: 2},
			TaxiPath{Index: 3, Type: types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_TAXI, Start: 2, End: 3},
			TaxiPath{Index: 4, Type: types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_TAXI, Start: 3, End: 1},
		)
	}
	return l, a, b, s
}

// TestVehicleRouteNotThroughStand (#1630): a way never runs through
// another stand (in by one lead-in, out by another) while there is any
// other, however much longer; with none, it still does.
func TestVehicleRouteNotThroughStand(t *testing.T) {
	near := func(route []LatLon, s LatLon) bool {
		for _, p := range route {
			if distM(p, s) < 5 {
				return true
			}
		}
		return false
	}
	routes := map[string]func(*Layout, LatLon, LatLon) ([]LatLon, error){
		"VehicleRoute": func(l *Layout, a, b LatLon) ([]LatLon, error) { return l.VehicleRoute(a, b) },
		"VehicleRoutePose": func(l *Layout, a, b LatLon) ([]LatLon, error) {
			return l.VehicleRoutePose(a, math.NaN(), b, math.NaN())
		},
	}
	for name, route := range routes {
		t.Run(name, func(t *testing.T) {
			l, a, b, s := throughLayout(false)
			r, err := route(l, a, b)
			if err != nil || !near(r, s) {
				t.Fatalf("no other way: %v, %d points, through the stand %v: want the way through it: nothing checked", err, len(r), near(r, s))
			}
			l, a, b, s = throughLayout(true)
			r, err = route(l, a, b)
			if err != nil {
				t.Fatalf("the way round: %v", err)
			}
			if near(r, s) {
				t.Errorf("through the stand with a way round: %d points", len(r))
			}
		})
	}
}
