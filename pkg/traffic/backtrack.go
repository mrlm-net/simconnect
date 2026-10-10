package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Backtracking (LOWI: no taxiway reaches either threshold of 08/26, the
// nearest entries 260–530 m in): a departure not given an intersection
// enters by the branch turning toward its threshold, taxis back along the
// runway, turns round near the end (a teardrop within the runway's
// width) and lines up at the threshold for the full length. Tower says
// "enter runway 08 and backtrack, line up and wait"; the runway is taken
// meanwhile, as during any line-up.

// Backtrack shapes.
const (
	// BacktrackKts is the speed back along the runway; the turn round is
	// taken at TurnSpeedKts.
	BacktrackKts = 18.0
	// backtrackEdge: the turn's wheels keep this far inside the runway's
	// edge; backtrackMaxRadius: its radius at most (wide runways).
	backtrackEdge      = 5.0
	backtrackMinRadius = 8.0
	backtrackMaxRadius = 25.0
)

// Backtracks reports whether the departure backtracks to its threshold:
// its runway end has no entry near the threshold (airport.Graph's
// ThresholdEntry) and it was not given an intersection.
func (c *TaxiController) Backtracks() bool {
	if c.req.Entry != "" || c.req.Graph == nil || c.end.Name == "" {
		return false
	}
	return !c.req.Graph.ThresholdEntry(c.end.Name)
}

// backtrackEntry is the way onto the runway for a backtrack: from the
// route's hold-short by the entry branch turning toward the threshold
// (the opposite end's entry nearest this threshold on the same holding
// point), its points; nil none.
func (c *TaxiController) backtrackEntry() []airport.LatLon {
	g := c.req.Graph
	opposite := c.runway.Primary.Name
	if opposite == c.end.Name {
		opposite = c.runway.Secondary.Name
	}
	entries, err := g.RunwayEntries(opposite)
	if err != nil {
		return nil
	}
	hold := c.route.Nodes[len(c.route.Nodes)-1]
	best := -1
	for i, e := range entries {
		if e.HoldShort == hold && (best < 0 || e.FromThreshold > entries[best].FromThreshold) {
			best = i
		}
	}
	if best < 0 {
		return nil
	}
	e := entries[best]
	var pts []airport.LatLon
	if e.Node != hold {
		opts := c.req.Options
		opts.Via, opts.Taxiways = nil, nil
		r, err := g.Route(hold, e.Node, opts)
		if err != nil {
			return nil
		}
		pts = append(pts, r.Points[1:]...)
	}
	for _, id := range e.Path[1:] {
		pts = append(pts, g.Nodes[id].Position)
	}
	return pts
}

// startBacktrack builds the line-up with the backtrack: false when there
// is no way to (then the line-up is as from any entry).
func (c *TaxiController) startBacktrack(pose GroundPose, prof MotionProfile) bool {
	entry := c.backtrackEntry()
	if len(entry) == 0 {
		return false
	}
	thr, hdg := c.end.Threshold, c.end.Heading
	at := func(x, y float64) airport.LatLon { return offsetHeading(offsetHeading(thr, hdg, x), hdg+90, y) }
	onRunway := entry[len(entry)-1]
	along := alongHeading(thr, hdg, onRunway)
	r := math.Min(math.Max(c.runway.Width/2-backtrackEdge, backtrackMinRadius), backtrackMaxRadius)
	a0 := r + 5 // the turn's centre down the runway from the threshold
	nose := NoseGear(pose.Position, pose.Heading, prof)
	pts := append([]airport.LatLon{nose}, entry...)
	// Back along the centreline, over to one side for the turn's start.
	if along > a0+3*r {
		pts = append(pts, at(a0+2*r, 0))
	}
	backStart := len(pts) - 1
	// The teardrop: round from the right side of the runway (as it faces
	// the take-off) through the threshold end to the left, then aligned.
	for deg := 90.0; deg <= 270; deg += 15 {
		phi := deg * math.Pi / 180
		pts = append(pts, at(a0+r*math.Cos(phi), r*math.Sin(phi)))
	}
	turnEnd := len(pts) - 1
	// Back onto the centreline on a gentle slope, then straight along it to
	// the line-up point: aligned with the runway when it stops there.
	join := at(a0+2.5*r, 0)
	align := at(a0+2.5*r+15, 0)
	far := at(math.Max(a0+2.5*r+65, c.runwayLength), 0)
	pts = append(pts, join, align, far)
	path, err := NewGroundPath(pts, prof)
	if err != nil {
		c.fail(err)
		return true
	}
	cum := func(i int) float64 { return pathLen(pts[:i+1]) }
	path.LimitRange(cum(backStart), cum(turnEnd), BacktrackKts, prof.Decel)
	path.LimitRange(cum(backStart+1), cum(turnEnd), TurnSpeedKts, prof.Decel)
	c.alignDist = cum(turnEnd+2)
	alignKts := LineUpSpeedKts
	if c.takeoffCleared {
		alignKts = LineUpRollingKts
	}
	path.LimitRange(cum(turnEnd), path.Length(), alignKts, prof.Decel)
	c.mover = NewGroundMoverFrom(path, prof, pose.Heading, pose.GroundSpeedKts)
	if !c.takeoffCleared {
		c.mover.HoldAt(c.alignDist)
	}
	c.lastStep = c.now()
	c.ignoreRunway = c.runway.Index
	c.last.HoldingShortOf = ""
	c.setInjectedLights(lightsLineUp, "lights line-up (strobes)")
	c.note("backtracking to the threshold", nil)
	c.setState(TaxiLiningUp, nil)
	return true
}
