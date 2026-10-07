package traffic

import (
	"fmt"
	"math"

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
	ll := func(i int) airport.LatLon {
		return airport.LatLon{Lat: c.corners[i].Latitude, Lon: c.corners[i].Longitude}
	}
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

// FinalInterceptDeg is the angle an arrival joining the final at a
// distance (JoinFinal) intercepts the centreline at.
const FinalInterceptDeg = 30.0

// finalInterceptLeadNM: the intercept starts this far outside the
// joining point.
const finalInterceptLeadNM = 3.0

// JoinFinal has an arrival on its procedure join the final where p,
// picked on the map, lies along it (#443): nm out from the threshold, on
// a FinalInterceptDeg intercept from its side, then the final as before.
// Nearer than the align point it goes straight to that. v is the heading
// it is given from where it is. ErrNotOnProcedure on the final or in a
// circuit, ErrHolding in the hold.
func (c *ArrivalController) JoinFinal(p airport.LatLon) (nm float64, v Vector, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.flyingProc || c.proc == nil || c.req.Circuit != nil || len(c.corners) < 2 {
		return 0, Vector{}, ErrNotOnProcedure
	}
	if c.holding != nil {
		return 0, Vector{}, ErrHolding
	}
	pos := c.last.Position
	if pos == (airport.LatLon{}) {
		return 0, Vector{}, ErrNotOnProcedure
	}
	n := len(c.corners)
	align, join := c.corners[n-2], c.corners[n-1]
	t := c.plan.End.Threshold
	out := math.Mod(c.plan.End.Heading+180, 360)
	fLat, fLon := calc.DisplaceByHeading(t.Lat, t.Lon, out, 30*1852)
	alignNM := calc.HaversineNM(t.Lat, t.Lon, align.Latitude, align.Longitude)
	nm = math.Max(alignNM, calc.AlongTrackMeters(t.Lat, t.Lon, fLat, fLon, p.Lat, p.Lon)/1852)
	plain := []types.SIMCONNECT_DATA_WAYPOINT{align, join}
	if nm > alignNM+0.5 {
		alt := align.Altitude + (nm-alignNM)*ProcedureDescentFtPerNm
		jLat, jLon := calc.DisplaceByHeading(t.Lat, t.Lon, out, nm*1852)
		at := airport.LatLon{Lat: jLat, Lon: jLon}
		plain = append([]types.SIMCONNECT_DATA_WAYPOINT{procedureWaypoint(at, alt, align.KtsSpeed)}, plain...)
		// From its side, a lead before: unless it is already inside it.
		if calc.AlongTrackMeters(t.Lat, t.Lon, fLat, fLon, pos.Lat, pos.Lon)/1852 > nm+finalInterceptLeadNM {
			side := 90.0
			if calc.CrossTrackMeters(t.Lat, t.Lon, fLat, fLon, pos.Lat, pos.Lon) < 0 {
				side = -90
			}
			qLat, qLon := calc.DisplaceByHeading(jLat, jLon, out, finalInterceptLeadNM*1852)
			qLat, qLon = calc.DisplaceByHeading(qLat, qLon, out+side, finalInterceptLeadNM*math.Tan(FinalInterceptDeg*math.Pi/180)*1852)
			plain = append([]types.SIMCONNECT_DATA_WAYPOINT{procedureWaypoint(airport.LatLon{Lat: qLat, Lon: qLon}, alt, align.KtsSpeed)}, plain...)
		}
	}
	c.reroute(pos, plain, make([]string, len(plain)))
	v = Vector{HeadingDeg: calc.BearingDegrees(pos.Lat, pos.Lon, plain[0].Latitude, plain[0].Longitude)}
	v.Turn = TurnTo(c.last.Heading, v.HeadingDeg)
	c.vectored = true
	c.note(fmt.Sprintf("joining the final at %.0f NM", nm), nil)
	return nm, v, nil
}

// AssignSpeed has an arrival on its procedure fly kts on the rest of its
// STAR (#443), no slower than its type's minimum there; 0 resumes the
// normal speed (ProcedureSpeedKts). The final is not touched; any
// vectors it was given stay. It returns the speed set.
func (c *ArrivalController) AssignSpeed(kts float64) (float64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.flyingProc || c.proc == nil || c.req.Circuit != nil || len(c.corners) < 3 {
		return 0, ErrNotOnProcedure
	}
	if c.holding != nil {
		return 0, ErrHolding
	}
	pos := c.last.Position
	final := len(c.corners) - 2
	k := c.cornerAhead()
	if pos == (airport.LatLon{}) || k >= final {
		return 0, ErrNotOnProcedure
	}
	minKts := MinProcedureSpeedKts
	if c.aircraft().Category == CategoryTurboprop {
		minKts = MinProcedureSpeedTurbopropKts
	}
	set := ProcedureSpeedKts
	if kts > 0 {
		set = math.Max(minKts, math.Min(kts, ProcedureSpeedKts))
	}
	plain := append([]types.SIMCONNECT_DATA_WAYPOINT(nil), c.corners[k:]...)
	for i := range plain[:final-k] {
		plain[i].KtsSpeed = set
	}
	vectors, vectored := c.vectors, c.vectored
	c.reroute(pos, plain, append([]string(nil), c.cornerNames[k:]...))
	c.vectors, c.vectored = vectors, vectored
	c.procSpeed = 0
	if kts > 0 {
		c.procSpeed = set
	}
	c.note(fmt.Sprintf("speed %.0f kt", set), nil)
	return set, nil
}
