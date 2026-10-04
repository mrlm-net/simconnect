package traffic

import (
	"fmt"
	"math"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// AirspaceClass is an ICAO airspace class as far as separation goes (#570):
// C, D, E or G (A and B separate everyone, as C does IFR).
type AirspaceClass string

const (
	ClassC AirspaceClass = "C"
	ClassD AirspaceClass = "D"
	ClassE AirspaceClass = "E"
	ClassG AirspaceClass = "G"
)

// SeparationRequired reports whether ATC separates two aircraft flying
// under rules a and b ("IFR" or "VFR") in class cl:
//
//   - C (and A, B): IFR from IFR and from VFR; VFR from VFR is not
//     separated, only told of each other;
//   - D, E: only IFR from IFR; the rest get traffic information (in E as far
//     as practical);
//   - G: nobody.
func SeparationRequired(cl AirspaceClass, a, b string) bool {
	ifrA, ifrB := a != "VFR", b != "VFR"
	switch cl {
	case ClassG:
		return false
	case ClassD, ClassE:
		return ifrA && ifrB
	default: // C, and anything stricter
		return ifrA || ifrB
	}
}

// ClockPosition is where the other aircraft is as a pilot looks for it:
// the o'clock (1–12) of bearing brg relative to track trk.
func ClockPosition(trk, brg float64) int {
	rel := math.Mod(brg-trk+360, 360)
	h := int(math.Round(rel / 30))
	if h == 0 {
		h = 12
	}
	return h
}

// TrafficRelative is how other traffic is told to an aircraft at pos on
// track trk: its o'clock, distance (NM, at least 1) and direction relative
// to the aircraft ("opposite direction", "same direction", "crossing left
// to right", "crossing right to left").
func TrafficRelative(pos airport.LatLon, trk float64, other airport.LatLon, otherTrk float64) (clock int, nm float64, dir string) {
	clock = ClockPosition(trk, localBearing(pos, other))
	nm = math.Max(1, math.Round(localDist(pos, other)/1852))
	d := headingDiff(trk, otherTrk)
	switch {
	case math.Abs(d) >= 135:
		dir = "opposite direction"
	case math.Abs(d) <= 45:
		dir = "same direction"
	case d > 0:
		dir = "crossing left to right"
	default:
		dir = "crossing right to left"
	}
	return clock, nm, dir
}

// TrafficInformation is traffic information to cs from pos (as said): the
// other aircraft's o'clock, distance, direction, type and level, "OKABC,
// traffic, 2 o'clock, 3 miles, opposite direction, Airbus A320, 2500 feet".
func TrafficInformation(pos Position, cs string, clock int, nm float64, dir, typ, level string) Transmission {
	unit := "miles"
	if nm == 1 {
		unit = "mile"
	}
	parts := []string{fmt.Sprintf("%d o'clock", clock), fmt.Sprintf("%.0f %s", nm, unit)}
	for _, p := range []string{dir, typ, level} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return Transmission{Position: pos, Callsign: cs, Intent: IntentTrafficInfo,
		Text: cs + ", traffic, " + strings.Join(parts, ", ")}
}
