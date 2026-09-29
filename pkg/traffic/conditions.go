//go:build windows
// +build windows

package traffic

import (
	"fmt"
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/nav"
)

// Approach conditions: spacing on final follows the weather as it does in
// life. Low visibility procedures widen it to protect the ILS sensitive
// areas; the reduced 2.5 NM radar separation is only for good visibility
// on a dry runway with short runway occupancy (ICAO Doc 4444 §8.7.3.2);
// a wet or contaminated runway keeps aircraft on it longer and brakes
// worse; and a headwind on final slows the ground speed, so a distance
// takes longer to fly (time-based separation keeps the time instead).

// RunwaySurface is the state of the runway.
type RunwaySurface uint8

const (
	RunwayDry          RunwaySurface = iota
	RunwayWet                        // rain
	RunwayContaminated               // snow, or precipitation at or below 0 °C
)

var runwaySurfaceNames = [...]string{"dry", "wet", "contaminated"}

func (s RunwaySurface) String() string {
	if int(s) < len(runwaySurfaceNames) {
		return runwaySurfaceNames[s]
	}
	return "unknown"
}

// MarshalText makes the surface its name in JSON.
func (s RunwaySurface) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// ApproachConditions are the weather on final that spacing depends on.
type ApproachConditions struct {
	// VisibilityM and CeilingFt; 0 is unknown (taken as good).
	VisibilityM float64       `json:"visibilityM"`
	CeilingFt   float64       `json:"ceilingFt"`
	HeadwindKts float64       `json:"headwindKts"` // on final; negative: tailwind
	Surface     RunwaySurface `json:"surface"`
}

// Thresholds: low visibility procedures below LVPVisibilityM (an RVR of
// 550 m, CAT II/III) or a ceiling below LVPCeilingFt; the reduced
// separation needs ReducedVisibilityM and ReducedCeilingFt or better.
const (
	LVPVisibilityM     = 550
	LVPCeilingFt       = 200
	ReducedVisibilityM = 5000
	ReducedCeilingFt   = 1000
	// LVPSpacingNM is the least spacing on final in low visibility
	// procedures: the aircraft ahead must be clear of the ILS sensitive
	// area before the next one is at 2 NM.
	LVPSpacingNM = 6.0
	// ReducedRadarSeparationNM is the reduced minimum on final (Doc 4444
	// §8.7.3.2), where it is allowed.
	ReducedRadarSeparationNM = 2.5
	// ContaminatedExtraNM is added on a contaminated runway (poor braking,
	// longer on the runway).
	ContaminatedExtraNM = 1.0
)

// ConditionsFrom are the approach conditions of a runway (its heading,
// degrees true) in the weather.
func ConditionsFrom(w nav.Weather, runwayHeadingTrue float64) ApproachConditions {
	head, _ := w.Components(runwayHeadingTrue)
	c := ApproachConditions{VisibilityM: w.VisibilityM, CeilingFt: w.CeilingFt, HeadwindKts: head}
	switch {
	case w.Precip == nav.PrecipSnow, w.Precip == nav.PrecipRain && w.TempC <= 0:
		c.Surface = RunwayContaminated
	case w.Precip == nav.PrecipRain:
		c.Surface = RunwayWet
	}
	return c
}

// LowVisibility reports low visibility procedures.
func (c ApproachConditions) LowVisibility() bool {
	return c.VisibilityM > 0 && c.VisibilityM < LVPVisibilityM || c.CeilingFt > 0 && c.CeilingFt < LVPCeilingFt
}

// ReducedAllowed reports whether the reduced radar separation may be used.
func (c ApproachConditions) ReducedAllowed() bool {
	return c.Surface == RunwayDry && (c.VisibilityM == 0 || c.VisibilityM >= ReducedVisibilityM) &&
		(c.CeilingFt == 0 || c.CeilingFt >= ReducedCeilingFt)
}

// ArrivalSpacing is the spacing on final behind a leader in the
// conditions, and why it differs from the wake minimum ("" when it does
// not): reduced to ReducedRadarSeparationNM when allowReduced and the
// conditions allow it (and no wake minimum applies), ContaminatedExtraNM
// more on a contaminated runway, at least LVPSpacingNM in low visibility.
func ArrivalSpacing(leader, follower Wake, scheme SeparationScheme, c ApproachConditions, allowReduced bool) (float64, string) {
	nm := ArrivalSeparationNM(leader, follower, scheme)
	why := ""
	if nm == MinRadarSeparationNM && allowReduced && c.ReducedAllowed() {
		nm, why = ReducedRadarSeparationNM, "reduced separation"
	}
	if c.Surface == RunwayContaminated {
		nm, why = nm+ContaminatedExtraNM, "contaminated runway"
	}
	if c.LowVisibility() && nm < LVPSpacingNM {
		nm, why = LVPSpacingNM, "low visibility procedures"
	}
	return nm, why
}

// FinalGroundKts is the ground speed on final at a true airspeed in the
// headwind (never below 80 kt).
func (c ApproachConditions) FinalGroundKts(finalKts float64) float64 {
	if finalKts <= 0 {
		finalKts = 140
	}
	return math.Max(80, finalKts-c.HeadwindKts)
}

// RunwayOccupancyIn is RunwayOccupancy on the surface: 15 % longer wet,
// 40 % contaminated.
func RunwayOccupancyIn(w Wake, landing bool, s RunwaySurface) time.Duration {
	d := RunwayOccupancy(w, landing)
	switch s {
	case RunwayWet:
		return d * 115 / 100
	case RunwayContaminated:
		return d * 140 / 100
	}
	return d
}

// String describes the conditions for logs: "vis 400 m, ceiling 100 ft,
// 12 kt headwind, wet".
func (c ApproachConditions) String() string {
	s := ""
	if c.VisibilityM > 0 {
		s += fmt.Sprintf("vis %.0f m, ", c.VisibilityM)
	}
	if c.CeilingFt > 0 {
		s += fmt.Sprintf("ceiling %.0f ft, ", c.CeilingFt)
	}
	if c.HeadwindKts >= 0 {
		s += fmt.Sprintf("%.0f kt headwind, ", c.HeadwindKts)
	} else {
		s += fmt.Sprintf("%.0f kt tailwind, ", -c.HeadwindKts)
	}
	return s + c.Surface.String()
}
