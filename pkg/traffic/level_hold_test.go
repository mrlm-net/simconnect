package traffic

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// Two holding their route levels a standard level apart are separated
// though the autopilot reads a few hundred feet off them (live: KLM396 at
// FL380 read 38004 ft, THY1687 at FL370 read 37132 ft, "lose separation in
// 0s at 872 ft"); one whose route climbs into the other is not.
func TestPredictConflictsHeldLevels(t *testing.T) {
	ahead := func(a TrackedAircraft, alt float64) []RoutePoint {
		var out []RoutePoint
		for nm := 10.0; nm <= 80; nm += 10 {
			lat, lon := calc.DisplaceByHeading(a.Position.Lat, a.Position.Lon, a.Heading, nm*1852)
			out = append(out, RoutePoint{Position: airport.LatLon{Lat: lat, Lon: lon}, AltFt: alt})
		}
		return out
	}
	klm := air(1, "KLM396", 0, 0, 38004, 290, 470, 0, true)
	thy := air(2, "THY1687", -2, 0, 37132, 110, 470, 0, true)
	levels := map[string]float64{"KLM396": 38000, "THY1687": 37000}
	o := ConflictOptions{Profile: func(a TrackedAircraft) []RoutePoint { return ahead(a, levels[a.Tail]) }}
	if cs := PredictConflicts([]TrackedAircraft{klm, thy}, o); len(cs) != 0 {
		t.Errorf("FL380 and FL370 held, 872 ft read: %+v", cs)
	}
	levels["THY1687"] = 38000 // its route up to KLM396's level
	if cs := PredictConflicts([]TrackedAircraft{klm, thy}, o); len(cs) == 0 {
		t.Error("one's route climbing to the other's level: no conflict")
	}
}
