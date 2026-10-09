package traffic

import (
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// LandingETA is how long an arrival at pos flying gsKts takes to land at
// threshold along route (its STAR and approach still to fly; nil: in a
// straight line): the distance to go as the sequencer measures it
// (DistanceToGo), flown at its speed now (at least LandingETAFarKts) until
// 40 NM out, then at 250 kt to 15 NM, 180 kt to 5 NM and 140 kt on the
// final — the speeds arrivals slow to. For a descent PA's "landing in
// about 25 minutes".
func LandingETA(pos airport.LatLon, gsKts float64, route []airport.LatLon, threshold airport.LatLon) time.Duration {
	d := DistanceToGo(pos, route, threshold)
	type band struct{ fromNM, kts float64 }
	bands := []band{{40, math.Max(gsKts, LandingETAFarKts)}, {15, 250}, {5, 180}, {0, 140}}
	hours := 0.0
	for _, b := range bands {
		if d > b.fromNM {
			hours += (d - b.fromNM) / b.kts
			d = b.fromNM
		}
	}
	return time.Duration(hours * float64(time.Hour))
}

// LandingETAFarKts: LandingETA's speed beyond 40 NM at least (a ground
// speed not known yet, or slow in a climb).
const LandingETAFarKts = 250.0
