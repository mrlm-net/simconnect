package nav

import (
	"slices"
	"strings"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// The overflights crossing LKPR on the busiest flow, south-east to north-west
// and back (the live schedule's pairs).
var (
	southEast = []AirportInfo{
		{ICAO: "LTAI", Position: airport.LatLon{Lat: 36.8987, Lon: 30.8005}},
		{ICAO: "LTFM", Position: airport.LatLon{Lat: 41.2753, Lon: 28.7519}},
		{ICAO: "OMDB", Position: airport.LatLon{Lat: 25.2528, Lon: 55.3644}},
		{ICAO: "OTHH", Position: airport.LatLon{Lat: 25.2731, Lon: 51.6081}},
		{ICAO: "LROP", Position: airport.LatLon{Lat: 44.5711, Lon: 26.085}},
		{ICAO: "LGAV", Position: airport.LatLon{Lat: 37.9364, Lon: 23.9445}},
		{ICAO: "LZKZ", Position: airport.LatLon{Lat: 48.6631, Lon: 21.2411}},
	}
	northWest = []AirportInfo{
		{ICAO: "EHAM", Position: airport.LatLon{Lat: 52.3086, Lon: 4.7639}},
		{ICAO: "EBBR", Position: airport.LatLon{Lat: 50.9014, Lon: 4.4844}},
		{ICAO: "EDDH", Position: airport.LatLon{Lat: 53.6304, Lon: 9.9882}},
		farAirports[2], // EGLL
		farAirports[0], // LFPG
	}
)

// crossing is the airways of a route, in order.
func crossing(fp *FlightPlan) string {
	var legs []string
	for _, w := range fp.Waypoints {
		if w.Phase == PhaseEnroute && w.Airway != "" && w.Airway != Direct {
			legs = append(legs, w.Airway)
		}
	}
	return strings.Join(slices.Compact(legs), " ")
}

// TestOverflightRouteVariety: the overflights of one flow do not all fly
// one chain of airways: a flight's Variant picks among the routes within a
// few percent of the shortest (live, LKPR: 36 of 80 overflights on
// L617 T710 T707 N869 north-west of Prague).
func TestOverflightRouteVariety(t *testing.T) {
	g := loadLKPRAirways(t)
	count := map[string]int{}
	n := 0
	for _, a := range southEast {
		for _, b := range northWest {
			for v := range uint64(6) {
				for _, p := range [][2]AirportInfo{{a, b}, {b, a}} {
					fp, err := Plan(FlightPlanRequest{Departure: p[0], Arrival: p[1], Type: "A320", Variant: v * 0x9e3779b97f4a7c15}, g)
					if err != nil {
						t.Fatal(err)
					}
					if airwayLegs(fp) < 3 {
						t.Errorf("%s→%s variant %d: %d airway legs: %s", p[0].ICAO, p[1].ICAO, v, airwayLegs(fp), fp.Route)
					}
					base, err := Plan(FlightPlanRequest{Departure: p[0], Arrival: p[1], Type: "A320"}, g)
					if err != nil {
						t.Fatal(err)
					}
					if fp.DistanceNM > 1.05*base.DistanceNM {
						t.Errorf("%s→%s variant %d: %.0f NM, the shortest choice %.0f: %s", p[0].ICAO, p[1].ICAO, v, fp.DistanceNM, base.DistanceNM, fp.Route)
					}
					pairs := map[string]bool{}
					ws := strings.Fields(crossing(fp))
					for i := 1; i < len(ws); i++ {
						p := []string{ws[i-1], ws[i]}
						slices.Sort(p)
						pairs[p[0]+" "+p[1]] = true
					}
					for p := range pairs {
						count[p]++
					}
					n++
				}
			}
		}
	}
	type kv struct {
		k string
		n int
	}
	var top []kv
	for k, c := range count {
		top = append(top, kv{k, c})
	}
	slices.SortFunc(top, func(a, b kv) int { return b.n - a.n })
	for _, x := range top[:min(5, len(top))] {
		t.Logf("%3d  %s", x.n, x.k)
	}
	if top[0].n*3 > n {
		t.Errorf("%d of %d plans fly %s: one chain for the whole flow", top[0].n, n, top[0].k)
	}
}
