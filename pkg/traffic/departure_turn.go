package traffic

import (
	"math"
	"slices"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// OppositeSeparationNM: a departure and an arrival to the other end of its
// runway keep this far apart horizontally, whatever their levels (the
// user, 2026-11-27: 1000 ft does not count; TestDeclaredField had them
// 0.1 NM apart, 5000 ft above each other, on the extended centreline).
const OppositeSeparationNM = 3.0

// DepartureTurn is a turn tower gives with a take-off clearance, "after
// departure turn right heading 135": from Corner on the extended
// centreline past the runway (or departureTurnAheadNM on from where the
// aircraft is handed over to MSFS AI, when that is past it), the true
// Heading for LegNM, then on along its SID from there, or with none on
// the take-off heading again, off to the side (a departure off one end
// kept clear of an arrival to the other, OppositeSeparationNM).
type DepartureTurn struct {
	Corner  airport.LatLon `json:"corner"`
	Heading float64        `json:"heading"`
	LegNM   float64        `json:"legNm"`
}

// departureTurnAheadNM: handed over at or past its corner, a departure
// turns this far on.
const departureTurnAheadNM = 1.0

// CornerFrom is where a departure at pos, climbing out on hdg, begins the
// turn: t.Corner, or departureTurnAheadNM on when that is less than half
// of it ahead.
func (t DepartureTurn) CornerFrom(pos airport.LatLon, hdg float64) airport.LatLon {
	ahead := math.Abs(headingDiff(calc.BearingDegrees(pos.Lat, pos.Lon, t.Corner.Lat, t.Corner.Lon), hdg)) < 90
	if ahead && calc.HaversineNM(pos.Lat, pos.Lon, t.Corner.Lat, t.Corner.Lon) >= departureTurnAheadNM/2 {
		return t.Corner
	}
	lat, lon := calc.DisplaceByHeading(pos.Lat, pos.Lon, hdg, departureTurnAheadNM*1852)
	return airport.LatLon{Lat: lat, Lon: lon}
}

// LegEnd is where the turn's leg from corner ends.
func (t DepartureTurn) LegEnd(corner airport.LatLon) airport.LatLon {
	lat, lon := calc.DisplaceByHeading(corner.Lat, corner.Lon, t.Heading, t.LegNM*1852)
	return airport.LatLon{Lat: lat, Lon: lon}
}

// DepartureTurnPath is the way a departure at pos climbing out on hdg
// flies turned by t: the corner, the end of the leg, then sid on from
// there (DepartureWaypoints), or with none on along hdg.
func DepartureTurnPath(pos airport.LatLon, hdg float64, t DepartureTurn, sid []airport.NavPoint) []airport.LatLon {
	corner := t.CornerFrom(pos, hdg)
	end := t.LegEnd(corner)
	out := []airport.LatLon{corner, end}
	if len(sid) > 0 {
		for _, w := range DepartureWaypoints(end, t.Heading, 0, sid) {
			out = append(out, airport.LatLon{Lat: w.Latitude, Lon: w.Longitude})
		}
		return out
	}
	lat, lon := calc.DisplaceByHeading(end.Lat, end.Lon, hdg, ProcedureContinueMeters)
	return append(out, airport.LatLon{Lat: lat, Lon: lon})
}

// AfterDepartureTurn is tower's amendment before a take-off turning the
// departure off the extended centreline: "CSA1, after departure turn right
// heading 135" (as AfterDepartureClimb), read back "After departure turn
// right heading 135, CSA1". trueDeg is the heading, magVar the variation
// (HeadingSaid); turn the way from the take-off heading.
func AfterDepartureTurn(cs, turn string, trueDeg, magVar float64) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentAfterDepartureTurn,
		Params: map[string]string{ParamTurn: turn, ParamHeading: HeadingSaid(trueDeg, magVar)}})
}

// TurnAfterDeparture has a departure not yet handed to MSFS AI turn as t
// after departure (AfterDepartureTurn): flown from the hand-over. False
// once handed over.
func (c *TaxiController) TurnAfterDeparture(t DepartureTurn) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == TaxiComplete {
		return false
	}
	c.turnAfter = &t
	return true
}

// turnedClimb is wps, the climb a departure at pose is handed over with,
// turned as c.turnAfter: the corner and the end of the leg at the height
// and speed wps has that far on, then its SID from there, or with none the
// rest of wps laid along the take-off heading from the leg's end. Its
// corners are the aircraft's turns (roundCorners). c.mu held.
func (c *TaxiController) turnedClimb(pose TakeoffPose, wps []types.SIMCONNECT_DATA_WAYPOINT) []types.SIMCONNECT_DATA_WAYPOINT {
	t := *c.turnAfter
	field := convert.MetersToFeet(c.req.Graph.Layout.Altitude)
	here := pose.Position
	// The climb as planned by the distance flown: feet MSL, knots.
	type at struct{ nm, ft, kts float64 }
	prof := []at{{0, field + pose.HeightFt, math.Max(pose.GroundSpeedKts, 1)}}
	prev := here
	for _, w := range wps {
		ft := w.Altitude
		if w.Flags&uint32(types.SIMCONNECT_WAYPOINT_ALTITUDE_IS_AGL) != 0 {
			ft += field
		}
		nm := prof[len(prof)-1].nm + calc.HaversineNM(prev.Lat, prev.Lon, w.Latitude, w.Longitude)
		prof = append(prof, at{nm, ft, w.KtsSpeed})
		prev = airport.LatLon{Lat: w.Latitude, Lon: w.Longitude}
	}
	along := func(d float64) (float64, float64) {
		for i := 1; i < len(prof); i++ {
			if a, b := prof[i-1], prof[i]; d <= b.nm {
				f := 0.0
				if b.nm > a.nm {
					f = (d - a.nm) / (b.nm - a.nm)
				}
				return a.ft + f*(b.ft-a.ft), b.kts
			}
		}
		last := prof[len(prof)-1]
		return last.ft, last.kts
	}
	corner := t.CornerFrom(here, pose.Heading)
	end := t.LegEnd(corner)
	dc := calc.HaversineNM(here.Lat, here.Lon, corner.Lat, corner.Lon)
	dl := dc + t.LegNM
	cFt, cKts := along(dc)
	lFt, lKts := along(dl)
	out := []types.SIMCONNECT_DATA_WAYPOINT{procedureWaypoint(corner, cFt, cKts), procedureWaypoint(end, lFt, lKts)}
	if len(c.req.Departure) > 0 {
		out = append(out, DepartureWaypoints(end, t.Heading, lFt, c.req.Departure)...)
	} else {
		last := prof[len(prof)-1]
		for _, a := range prof[1:] {
			if a.nm > dl {
				lat, lon := calc.DisplaceByHeading(end.Lat, end.Lon, pose.Heading, (a.nm-dl)*1852)
				out = append(out, procedureWaypoint(airport.LatLon{Lat: lat, Lon: lon}, a.ft, a.kts))
			}
		}
		lat, lon := calc.DisplaceByHeading(end.Lat, end.Lon, pose.Heading, ProcedureContinueMeters)
		out = append(out, procedureWaypoint(airport.LatLon{Lat: lat, Lon: lon}, last.ft, last.kts))
	}
	c.vectorTo = &end
	first := types.SIMCONNECT_DATA_WAYPOINT{Latitude: here.Lat, Longitude: here.Lon, KtsSpeed: cKts}
	return roundCorners(append([]types.SIMCONNECT_DATA_WAYPOINT{first}, out...), MaxBankDeg(*c.aircraft()))[1:]
}

// ClosestApproachNM is the least horizontal distance between a and b, each
// flown from its first point along its points at their speeds (a point's
// Kts on the way to it; at least closestMinKts) times aScale and bScale,
// a starting aDelay after b: while both fly, a from its start, b until its
// last point (an arrival's threshold). +Inf when they never fly at once.
func ClosestApproachNM(a []RoutePoint, aDelay time.Duration, aScale float64, b []RoutePoint, bScale float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return math.Inf(1)
	}
	o := b[0].Position
	k := math.Cos(o.Lat * math.Pi / 180)
	ta := timedPath(a, aScale, aDelay.Hours(), o, k)
	tb := timedPath(b, bScale, 0, o, k)
	from := math.Max(ta[0].h, tb[0].h)
	to := math.Min(ta[len(ta)-1].h, tb[len(tb)-1].h)
	if len(a) == 1 {
		to = tb[len(tb)-1].h
	}
	if to < from {
		return math.Inf(1)
	}
	// Each stretch between the two paths' corners: both straight, the
	// closest point of the relative motion within it.
	cuts := []float64{from, to}
	for _, p := range ta {
		if p.h > from && p.h < to {
			cuts = append(cuts, p.h)
		}
	}
	for _, p := range tb {
		if p.h > from && p.h < to {
			cuts = append(cuts, p.h)
		}
	}
	slices.Sort(cuts)
	best := math.Inf(1)
	for i := 0; i+1 < len(cuts); i++ {
		h0, h1 := cuts[i], cuts[i+1]
		ax0, ay0 := posAtH(ta, h0)
		ax1, ay1 := posAtH(ta, h1)
		bx0, by0 := posAtH(tb, h0)
		bx1, by1 := posAtH(tb, h1)
		dx, dy := ax0-bx0, ay0-by0
		vx, vy := (ax1-bx1)-dx, (ay1-by1)-dy
		f := 0.0
		if v2 := vx*vx + vy*vy; v2 > 0 {
			f = math.Max(0, math.Min(1, -(dx*vx+dy*vy)/v2))
		}
		best = math.Min(best, math.Hypot(dx+f*vx, dy+f*vy))
	}
	return best
}

// closestMinKts: a point with no speed (or a slower one) is flown at this.
const closestMinKts = 60.0

// timedPoint is a point of a path flown in time: east and north of an
// origin (NM), h hours on.
type timedPoint struct{ x, y, h float64 }

// timedPath is p flown at its speeds times scale from h0 hours, on a flat
// plane about o (k the cosine of its latitude).
func timedPath(p []RoutePoint, scale, h0 float64, o airport.LatLon, k float64) []timedPoint {
	xy := func(q airport.LatLon) (float64, float64) {
		return (q.Lon - o.Lon) * 60 * k, (q.Lat - o.Lat) * 60
	}
	x, y := xy(p[0].Position)
	out := []timedPoint{{x, y, h0}}
	for i := 1; i < len(p); i++ {
		nx, ny := xy(p[i].Position)
		kts := p[i].Kts
		if kts <= 0 {
			kts = p[i-1].Kts
		}
		kts = math.Max(kts, closestMinKts) * math.Max(scale, 0.1)
		last := out[len(out)-1]
		out = append(out, timedPoint{nx, ny, last.h + math.Hypot(nx-last.x, ny-last.y)/kts})
	}
	return out
}

// posAtH is where a timed path is h hours on: at its first point before
// it starts, its last after it ends.
func posAtH(p []timedPoint, h float64) (float64, float64) {
	if h <= p[0].h {
		return p[0].x, p[0].y
	}
	for i := 1; i < len(p); i++ {
		if h <= p[i].h {
			a, b := p[i-1], p[i]
			f := 0.0
			if b.h > a.h {
				f = (h - a.h) / (b.h - a.h)
			}
			return a.x + f*(b.x-a.x), a.y + f*(b.y-a.y)
		}
	}
	l := p[len(p)-1]
	return l.x, l.y
}
