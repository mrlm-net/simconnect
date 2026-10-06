package traffic

import (
	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// PointSnapNM: a point picked on the map within this of a named fix ahead
// on the route is that fix (ATC says "direct" to it).
const PointSnapNM = 1.0

// DirectTo sends an arrival flying its procedure to p, picked on the map
// (#443), then on along its route from the first point after the one
// nearest p; nothing ahead climbs. p within PointSnapNM of a named fix
// ahead is that fix: fix is its name ("cleared direct to …"). A point
// elsewhere is flown on a radar vector: v is the heading from where it is
// ("fly heading …"); once there it is told to resume own navigation direct
// to its next named fix (VectorDue). ErrNotOnProcedure on the final or in
// a circuit, ErrHolding in the hold.
func (c *ArrivalController) DirectTo(p airport.LatLon) (fix string, v Vector, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.flyingProc || c.proc == nil || c.req.Circuit != nil || len(c.corners) < 3 {
		return "", Vector{}, ErrNotOnProcedure
	}
	if c.holding != nil {
		return "", Vector{}, ErrHolding
	}
	pos := c.last.Position
	final := len(c.corners) - 2 // align and join: the final, kept
	k := c.cornerAhead()
	if pos == (airport.LatLon{}) || k >= final {
		return "", Vector{}, ErrNotOnProcedure
	}
	ll := func(i int) airport.LatLon { return airport.LatLon{Lat: c.corners[i].Latitude, Lon: c.corners[i].Longitude} }
	// A named fix ahead near p: there.
	for j := k; j < final; j++ {
		if n := c.cornerName(j); n != "" && calc.HaversineNM(p.Lat, p.Lon, ll(j).Lat, ll(j).Lon) <= PointSnapNM {
			c.reroute(pos, append([]types.SIMCONNECT_DATA_WAYPOINT(nil), c.corners[j:]...), append([]string(nil), c.cornerNames[j:]...))
			c.note("direct to "+n, nil)
			return n, Vector{}, nil
		}
	}
	// A point: to it, then on from the point after the nearest one.
	near, best := k, -1.0
	for j := k; j < final; j++ {
		if d := calc.HaversineNM(p.Lat, p.Lon, ll(j).Lat, ll(j).Lon); best < 0 || d < best {
			near, best = j, d
		}
	}
	next := min(near+1, final)
	w := c.corners[near]
	plain := append([]types.SIMCONNECT_DATA_WAYPOINT{procedureWaypoint(p, w.Altitude, w.KtsSpeed)}, c.corners[next:]...)
	names := append([]string{""}, c.cornerNames[next:]...)
	c.reroute(pos, plain, names)
	resume := ""
	for j := 1; j < len(names) && j < len(plain)-2; j++ {
		if names[j] != "" {
			resume = names[j]
			break
		}
	}
	v = Vector{HeadingDeg: calc.BearingDegrees(pos.Lat, pos.Lon, p.Lat, p.Lon)}
	v.Turn = TurnTo(c.last.Heading, v.HeadingDeg)
	if resume != "" {
		c.vectors = []Vector{{At: p, Fix: resume}} // there: back on its own navigation
	}
	c.vectored = true
	c.note("vectored to a point", nil)
	return "", v, nil
}

// reroute flies plain (named by names) from pos, with nothing ahead
// climbing; earlier vectors are dropped. c.mu held.
func (c *ArrivalController) reroute(pos airport.LatLon, plain []types.SIMCONNECT_DATA_WAYPOINT, names []string) {
	noClimb(plain, c.altitudeNow())
	out := roundedChain(pos, plain, MaxBankDeg(*c.aircraft()))
	c.note("direct", c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, out))
	c.proc.Waypoints, c.procNext = out, 0
	c.corners, c.cornerNames, c.cornerNext = plain, names, 0
	c.vectors = nil
	c.joinMinM, c.lastRunwayM = 0, 0
}
