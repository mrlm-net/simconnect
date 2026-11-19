package traffic

import (
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
