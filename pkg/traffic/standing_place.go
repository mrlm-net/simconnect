package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// StandingPlace is the place an aircraft of prof standing with its
// reference point at pos, heading hdg, takes for a route to keep clear of
// (airport.Occupied): its body from tail end to nose tip, as wide that a
// route clear of it passes neither wing to wing within the wingtip margin
// nor so near that the one taxiing it gives way to that body (#1641).
func StandingPlace(pos airport.LatLon, hdg float64, prof MotionProfile) airport.Occupied {
	if prof == (MotionProfile{}) {
		prof = DefaultMotionProfile()
	}
	half := prof.SpanMeters / 2
	if half <= 0 {
		half = DefaultHalfSpanMeters
	}
	tail := prof.TailMeters
	if tail <= 0 {
		tail = 20.5
	}
	nose := prof.WheelbaseMeters*pushNoseFactor - prof.RefAheadMeters
	var pts []airport.LatLon
	for d := -tail; d < nose; d += occupiedStepMeters {
		pts = append(pts, offsetHeading(pos, hdg, d))
	}
	pts = append(pts, offsetHeading(pos, hdg, nose))
	wide := GiveWayMarginMeters + TailplaneShare*half - airport.DefaultWingtipMargin
	return airport.Occupied{Points: pts, HalfSpan: math.Max(half, wide)}
}
