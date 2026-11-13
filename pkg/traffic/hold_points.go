package traffic

import (
	"slices"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Runway holding points a client lights stop bars at (#1532): where the
// clearances hold an aircraft and which holding points they take it past.
// Every value is a graph node (airport.NodeID, a taxi point's index).

// holdOnRoute is a hold-short node's distance along a path, kept while the
// path stays the same (aheadOf).
type holdOnRoute struct {
	path *GroundPath
	node airport.NodeID
	at   float64
	on   bool
}

// aheadOf reports whether the hold-short node is still ahead on the
// aircraft's path at s metres along it: false once passed, or off the path.
func (d *groundDrive) aheadOf(node airport.NodeID, s float64) bool {
	if d.mover == nil || node < 0 || int(node) >= len(d.graph.Nodes) {
		return false
	}
	p := d.mover.Path()
	if d.holdSeen.path != p || d.holdSeen.node != node {
		at, off := p.DistanceTo(d.graph.Nodes[node].Position)
		d.holdSeen = holdOnRoute{path: p, node: node, at: at, on: off <= 15}
	}
	return d.holdSeen.on && s < d.holdSeen.at
}

// setHolds stores the frame's holding points in the event fields and
// reports whether they changed (past is copied, the event keeps its own).
func setHolds(at airport.NodeID, past []airport.NodeID, lastAt *airport.NodeID, lastPast *[]airport.NodeID) bool {
	if at == *lastAt && slices.Equal(past, *lastPast) {
		return false
	}
	*lastAt, *lastPast = at, slices.Clone(past)
	return true
}

// ownHold is the departure runway's hold-short node the route ends at, -1
// when it ends elsewhere.
func (c *TaxiController) ownHold() airport.NodeID {
	if c.route == nil || len(c.route.Nodes) == 0 || c.req.Graph == nil {
		return -1
	}
	n := c.route.Nodes[len(c.route.Nodes)-1]
	if hs := c.req.Graph.Nodes[n].HoldShort; hs == nil || hs.Runway != c.runway.Index {
		return -1
	}
	return n
}

// holdPoints is where a departure is held and what it is cleared past, at
// pose: before its taxi the first hold on its route; taxiing, the crossings
// and the departure runway's holding point, which a line-up or take-off
// clearance releases until the aircraft has passed it.
func (c *TaxiController) holdPoints(pose GroundPose) (airport.NodeID, []airport.NodeID) {
	own := c.ownHold()
	released := own >= 0 && (c.lineUpCleared || c.takeoffCleared)
	switch c.state {
	case TaxiSpawning, TaxiAwaitingPushback, TaxiPushback, TaxiAwaitingTaxi:
		if at := c.firstCrossing(); at >= 0 {
			return at, nil
		}
		if released {
			return -1, []airport.NodeID{own}
		}
		return own, nil
	case TaxiTaxiing, TaxiHoldingShort:
		at, past := c.crossingHolds(pose.Distance, nil)
		if released {
			past = append(past, own)
		} else if at < 0 {
			at = own
		}
		return at, past
	case TaxiLiningUp:
		if own >= 0 && c.aheadOf(own, pose.Distance) {
			return -1, []airport.NodeID{own}
		}
	}
	return -1, nil
}

// firstCrossing is the hold-short node of the first runway crossing on the
// route, -1 none; worked out once per route.
func (c *TaxiController) firstCrossing() airport.NodeID {
	if c.route == nil || c.req.Graph == nil {
		return -1
	}
	if c.crossFirst.route != c.route {
		var holds []holdOnPath
		for i, n := range c.route.Nodes {
			if hs := c.req.Graph.Nodes[n].HoldShort; hs != nil && hs.Runway != c.runway.Index {
				holds = append(holds, holdOnPath{runway: hs.Runway, index: i, node: n})
			}
		}
		c.crossFirst.route, c.crossFirst.at = c.route, -1
		if z := crossingZones(c.req.Graph, c.route.Points, holds); len(z) > 0 {
			c.crossFirst.at = z[0].nodes[0]
		}
	}
	return c.crossFirst.at
}

// holdPoints is where an arrival is held and what it is cleared past, at
// pose: on the runway and vacating, its exit's holding point until passed
// (the landing clearance takes it off the runway); from there on, the
// crossings of its taxi-in.
func (c *ArrivalController) holdPoints(pose GroundPose) (airport.NodeID, []airport.NodeID) {
	var past []airport.NodeID
	switch c.state {
	case ArrivalRollout, ArrivalVacating:
		if n := c.exitHold(); n >= 0 && c.aheadOf(n, pose.Distance) {
			past = append(past, n)
		}
		fallthrough
	case ArrivalAwaitingTaxi, ArrivalTaxiing, ArrivalHoldingShort, ArrivalParking:
		return c.crossingHolds(pose.Distance, past)
	}
	return -1, nil
}

// exitHold is the first holding point of its exit on the route, -1 none.
func (c *ArrivalController) exitHold() airport.NodeID {
	if c.plan == nil || c.plan.Route == nil {
		return -1
	}
	for _, n := range c.plan.Route.Nodes {
		if n >= 0 && (n == c.plan.Exit.HoldShort || n == c.plan.Exit.ILSHoldShort) {
			return n
		}
	}
	return -1
}
