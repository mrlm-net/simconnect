package airport

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// The facility data gives an AF leg (a DME arc) no arc centre. In
// simconnect-data's 15,256 the recommended navaid (Origin) is the centre
// and Rho the radius: the fixes at the legs' ends lie on that circle within
// 500 m in all but a few (#1646).
const (
	// arcRadiusTolMeters: how far from an AF leg's radius its ends may lie
	// (0.5 NM, and at most a quarter of the radius).
	arcRadiusTolMeters = 926.0
	// arcMaxSweepDeg: an AF leg round more than this isn't drawn as an arc:
	// the turn direction or an end is wrong (a step back along a short arc
	// goes 350° round). The longest consistent one in the data is 277°.
	arcMaxSweepDeg = 300.0
	// arcEarthMeters is the earth's radius, as calc.HaversineMeters has it.
	arcEarthMeters = 6371000.0
)

// arcCentre is the centre of l's arc flown from from to its fix: ArcCenter
// where the data has one (RF legs), else for an AF leg its navaid, where
// that makes a sound arc: a turn direction, both ends at Rho from the
// navaid and arcMaxSweepDeg round at most. ok false: a straight leg.
func (l Leg) arcCentre(from LatLon) (LatLon, bool) {
	if !l.HasFix() {
		return LatLon{}, false
	}
	if l.ArcCenter.Lat != 0 || l.ArcCenter.Lon != 0 {
		return l.ArcCenter, true
	}
	if l.Type != types.SIMCONNECT_FACILITY_LEG_TYPE_AF || l.Origin == (LatLon{}) || l.Rho <= 0 || l.TurnLeft == l.TurnRight {
		return LatLon{}, false
	}
	o, tol := l.Origin, math.Min(arcRadiusTolMeters, l.Rho/4)
	for _, e := range []LatLon{from, l.Position} {
		if math.Abs(calc.HaversineMeters(o.Lat, o.Lon, e.Lat, e.Lon)-l.Rho) > tol {
			return LatLon{}, false
		}
	}
	if calc.HaversineMeters(from.Lat, from.Lon, l.Position.Lat, l.Position.Lon) < samePointMeters {
		return LatLon{}, false
	}
	sweep := math.Mod(calc.BearingDegrees(o.Lat, o.Lon, l.Position.Lat, l.Position.Lon)-calc.BearingDegrees(o.Lat, o.Lon, from.Lat, from.Lon)+720, 360)
	if l.TurnLeft {
		sweep = math.Mod(360-sweep, 360)
	}
	if sweep > arcMaxSweepDeg {
		return LatLon{}, false
	}
	return o, true
}

// arcTo samples l's arc about center from from to its fix. A DME arc is
// tens of kilometres across, where displace's flat earth is some hundred
// metres off the radius (a jog at each end): its points are put on the
// sphere the bearings and distances are measured on.
func (l Leg) arcTo(center, from LatLon) []LatLon {
	if l.ArcCenter.Lat != 0 || l.ArcCenter.Lon != 0 {
		return arcPoints(center, from, l.Position, l.TurnRight)
	}
	return arcPointsBy(center, from, l.Position, l.TurnRight, destination)
}

// destination is the point d meters from p along the great circle that
// leaves it on bearing.
func destination(p LatLon, bearing, d float64) LatLon {
	const rad = math.Pi / 180
	lat, b, a := p.Lat*rad, bearing*rad, d/arcEarthMeters
	to := math.Asin(math.Sin(lat)*math.Cos(a) + math.Cos(lat)*math.Sin(a)*math.Cos(b))
	dLon := math.Atan2(math.Sin(b)*math.Sin(a)*math.Cos(lat), math.Cos(a)-math.Sin(lat)*math.Sin(to))
	return LatLon{Lat: to / rad, Lon: math.Mod(p.Lon+dLon/rad+540, 360) - 180}
}

// arcInterceptMeters is how far along course c (true) from p a CI or VI
// leg meets the DME arc of next, the AF leg after it: where the course
// first crosses that circle, if the arc is sound from there (arcCentre).
// False for other legs, and where p is on the circle already.
func arcInterceptMeters(l Leg, next *Leg, p LatLon, c float64) (float64, bool) {
	switch l.Type {
	case types.SIMCONNECT_FACILITY_LEG_TYPE_CI, types.SIMCONNECT_FACILITY_LEG_TYPE_VI:
	default:
		return 0, false
	}
	if next == nil || next.Type != types.SIMCONNECT_FACILITY_LEG_TYPE_AF || next.Origin == (LatLon{}) || next.Rho <= 0 {
		return 0, false
	}
	if math.Abs(calc.HaversineMeters(next.Origin.Lat, next.Origin.Lon, p.Lat, p.Lon)-next.Rho) < dmeEndMinMeters {
		return 0, false
	}
	d, ok := circleMeters(p, c, next.Origin, next.Rho)
	if !ok {
		return 0, false
	}
	if _, ok := next.arcCentre(displace(p, c, d)); !ok {
		return 0, false
	}
	return d, true
}
