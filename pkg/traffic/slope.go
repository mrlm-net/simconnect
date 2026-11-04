package traffic

import (
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Ground slope. An aircraft on the ground is placed at its static pitch,
// which is its pitch on level ground: on a sloping runway or taxiway its
// nose wheel floated above the ground facing downhill and sank into it
// facing uphill (live LKPR 06/24, about 0.6 % down towards the 06 end: an
// A220's nose wheel 10 to 15 cm off, read as a wheelie on the runway). So
// its pitch takes the slope between the ground under its nose gear and
// under its main gear, as the ground map has seen them, eased in.

// slopeState is a placed aircraft's ground slope: degrees, nose up
// positive, the one shown and the one it eases to, and when it was last
// shown.
type slopeState struct {
	deg, target float64
	at          time.Time
}

const (
	// slopeMaxDeg: a slope read steeper than this is not the ground (a
	// stale cell, a bridge): not taken.
	slopeMaxDeg = 3.0
	// slopeDegPerSec: the slope shown follows the ground at most this fast
	// (the cells are read to about 0.1 ft: over a wheelbase, a tenth of a
	// degree from one cell to the next).
	slopeDegPerSec = 0.5
	// slopeFreshAfter: not placed for this long, the slope is taken at once.
	slopeFreshAfter = 2 * time.Second
	// slopeBaseM: the slope is read over this at least (groundSlope).
	slopeBaseM = 15.0
)

// groundSlope is the slope (degrees, nose up positive) of the ground under
// o placed at pos facing hdg: from the ground under its main gear to the
// ground under its nose gear. False when the ground map has not seen
// both, or o's gear is not known. i.mu held.
func (i *Injector) groundSlope(o *injected, pos airport.LatLon, hdg float64) (float64, bool) {
	if o.wheelbase <= 0 {
		return 0, false
	}
	// Over slopeBaseM at least, about the middle of its gear: the cells are
	// about a metre, and a light aircraft's gear barely two apart.
	mid := offsetHeading(pos, hdg, o.wheelbase/2-o.refAhead)
	half := math.Max(o.wheelbase, slopeBaseM) / 2
	mains, nose := cellOf(offsetHeading(mid, hdg, -half)), cellOf(offsetHeading(mid, hdg, half))
	gm, okM := i.ground.cells[mains]
	gn, okN := i.ground.cells[nose]
	if !okM || !okN || math.Abs(float64(gm)-o.groundFt) >= groundMapTrustFt || math.Abs(float64(gn)-o.groundFt) >= groundMapTrustFt {
		return 0, false
	}
	// Between the cells, not the points read: a cell is a metre wide.
	d := alongHeading(mains.at(), hdg, nose.at())
	if d < half {
		return 0, false
	}
	deg := math.Atan2(float64(gn-gm)*0.3048, d) * 180 / math.Pi
	if math.Abs(deg) > slopeMaxDeg {
		return 0, false
	}
	return deg, true
}

// slopeShown is the ground slope to pitch o by, placed at pos facing hdg
// (degrees, nose up positive): the ground's (groundSlope), eased in at
// slopeDegPerSec; where the ground is not known, the last. i.mu held.
func (i *Injector) slopeShown(o *injected, pos airport.LatLon, hdg float64) float64 {
	s := &o.slope
	if deg, ok := i.groundSlope(o, pos, hdg); ok {
		s.target = deg
	}
	now := i.now()
	if s.at.IsZero() || now.Sub(s.at) > slopeFreshAfter {
		s.deg = s.target
	} else {
		step := slopeDegPerSec * now.Sub(s.at).Seconds()
		s.deg += math.Max(-step, math.Min(step, s.target-s.deg))
	}
	s.at = now
	return s.deg
}

// at is the cell's centre.
func (c groundCell) at() airport.LatLon {
	return airport.LatLon{Lat: float64(c.lat) * groundCellDeg, Lon: float64(c.lon) * groundCellDeg}
}
