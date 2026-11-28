package traffic

import (
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// RunwaySuits reports whether an aircraft of model can use runway r by its
// surface (#1561): never a water runway (the traffic is landplanes); a soft
// one (grass, gravel, dirt and the like: its surface known and not paved,
// airport.Runway.Hard) only a light aircraft (IntersectionClass
// ClassLight). A runway whose surface is not known is taken as paved; an
// empty model suits every runway. Its length is RunwayLongEnough's.
func RunwaySuits(model string, r airport.Runway) bool {
	if model == "" {
		return true
	}
	if r.Surface == "water" {
		return false
	}
	return r.Surface == "" || r.Hard || IntersectionClass(model) == ClassLight
}

// RunwayLongEnough reports whether runway r at an airport elevationFt high
// is as long as model's take-off run there (RequiredTakeoffRun, ISA: the
// figure that has a crew refuse a runway, #1232). An empty model, or a
// runway of no known length: true.
func RunwayLongEnough(model string, r airport.Runway, elevationFt float64) bool {
	return model == "" || r.Length <= 0 || r.Length >= takeoffRunFor(model, elevationFt)
}

// RunwayEndLongEnough is RunwayLongEnough for a take-off from end e of r:
// its take-off run available (airport.RunwayEnd.TORA, #1660) against the
// run. An empty model, or a runway of no known length: true.
func RunwayEndLongEnough(model string, r airport.Runway, e airport.RunwayEnd, elevationFt float64) bool {
	return model == "" || r.Length <= 0 || e.TORA(r.Length) >= takeoffRunFor(model, elevationFt)
}

// RequiredLandingRun is the landing distance an injected landing of model
// needs from the landing threshold (#1718): its short-field landing
// (aimShift), the one it flies where its usual one does not fit, touching
// down shortFieldShare of the distance in (at least
// shortFieldMinTouchdownM, then a tenth of the spread), the nose coming
// down (DerotateSeconds at DerotateDecel), then its brakes building up at
// RolloutJerk to its BrakeDecel, down to its ExitKts. Not a certified
// landing distance: the profiles have none.
func RequiredLandingRun(model string) float64 {
	p := ProfileFor(model)
	return landingRunFor(withDefaults(p.Approach, DefaultApproachProfile()), withDefaults(p.Rollout, DefaultRolloutProfile()))
}

// landingRunFor is RequiredLandingRun for an approach and rollout: the
// least landing distance its short-field touchdown and roll fit in.
func landingRunFor(ap ApproachProfile, ro RolloutProfile) float64 {
	after := landingRollFor(ap, ro) + TouchdownSpreadMeters/10
	if l := shortFieldMinTouchdownM + after; shortFieldShare*l <= shortFieldMinTouchdownM {
		return l
	}
	return after / (1 - shortFieldShare)
}

// landingRollFor is how far an injected landing rolls from its touchdown
// to its exit speed: the nose coming down at DerotateDecel, then braking
// from nothing, building up at RolloutJerk to its BrakeDecel.
func landingRollFor(ap ApproachProfile, ro RolloutProfile) float64 {
	const dt = 0.01
	v, ve, d := ap.TouchdownKts*ktsToMS, ro.ExitKts*ktsToMS, 0.0
	for t := 0.0; t < ap.DerotateSeconds && v > 0; t += dt {
		v = math.Max(0, v-ap.DerotateDecel*dt)
		d += v * dt
	}
	if ro.BrakeDecel <= 0 {
		return d
	}
	for a := 0.0; v > ve; {
		a = math.Min(ro.BrakeDecel, a+RolloutJerk*dt)
		v -= math.Max(a, ap.DerotateDecel) * dt
		d += v * dt
	}
	return d
}

// RunwayEndLandingLongEnough reports whether end e of runway r is long
// enough for model to land on: its landing distance available
// (airport.RunwayEnd.LandingDistance, #1660) against RequiredLandingRun.
// An empty model, or a runway of no known length: true.
func RunwayEndLandingLongEnough(model string, r airport.Runway, e airport.RunwayEnd) bool {
	return model == "" || r.Length <= 0 || e.LandingDistance(r.Length) >= RequiredLandingRun(model)
}

// landingFits is ErrRunwayTooShortToLand when req's runway end is shorter to land
// on than its aircraft's landing run (its approach and rollout as
// resolved); nil when it fits or the end or its length is not known.
func landingFits(req ArrivalRequest) error {
	r, e, ok := req.Graph.Layout.RunwayEnd(req.Runway)
	if !ok || r.Length <= 0 {
		return nil
	}
	need := landingRunFor(approachProfileOf(req), withDefaults(req.Rollout, DefaultRolloutProfile()))
	if lda := e.LandingDistance(r.Length); lda < need {
		return fmt.Errorf("%w: runway %s lands %.0f m, %s needs %.0f m", ErrRunwayTooShortToLand, e.Name, lda, req.Model, need)
	}
	return nil
}

// ErrRunwayTooShortToLand is returned by ArrivalController.Start for a landing on a runway end whose landing
// distance available is shorter than the aircraft's landing run
// (RequiredLandingRun, #1718).
var ErrRunwayTooShortToLand = errors.New("traffic: runway too short for this aircraft to land")

// takeoffRuns: RequiredTakeoffRun by take-off profile and elevation (100
// ft), worked out once.
var takeoffRuns sync.Map

func takeoffRunFor(model string, elevationFt float64) float64 {
	p := TakeoffProfileFor(model)
	if p == (TakeoffProfile{}) {
		p = DefaultTakeoffProfile()
	}
	elevationFt = math.Round(elevationFt/100) * 100
	k := fmt.Sprintf("%v|%.0f", p, elevationFt)
	if v, ok := takeoffRuns.Load(k); ok {
		return v.(float64)
	}
	need := RequiredTakeoffRun(p, TakeoffConditions{ElevationFt: elevationFt})
	takeoffRuns.Store(k, need)
	return need
}
