package world

import (
	"slices"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Stop bars (#1532): the engine owns the clearances, so it tells a client
// where each aircraft is held and which runway holding points its
// clearances take it past. Node is the taxi graph node (airport.NodeID, the
// taxi point's index in Layout.TaxiPoints): Graph.HoldShortNodes and
// Layout.HoldShortPoints name the same points.

// HoldPoint is a runway holding point: where a stop bar stands.
type HoldPoint struct {
	Node     airport.NodeID `json:"node"`
	Position airport.LatLon `json:"position"`
	// Runway is the runway it protects ("06/24"), "" when the point is near
	// none.
	Runway string `json:"runway,omitempty"`
	// ILS: an ILS (CAT II/III) holding point, else an ordinary one.
	ILS bool `json:"ils,omitempty"`
}

// HoldBar is a holding point's stop bar: released (off) while a clearance
// takes an aircraft past it, until the aircraft has passed; protected (on)
// otherwise.
type HoldBar struct {
	HoldPoint
	Released bool `json:"released"`
	// By are the call signs cleared past it.
	By []string `json:"by,omitempty"`
}

// holdPointOf is graph node n as a holding point; false when it is none.
func holdPointOf(g *airport.Graph, n airport.NodeID) (HoldPoint, bool) {
	if g == nil || n < 0 || int(n) >= len(g.Nodes) || g.Nodes[n].Kind != airport.NodeHoldShort {
		return HoldPoint{}, false
	}
	node := g.Nodes[n]
	p := HoldPoint{Node: n, Position: node.Position}
	if node.HoldShort != nil {
		p.Runway, p.ILS = node.HoldShort.Name, node.HoldShort.ILS
	}
	if int(n) < len(g.Layout.TaxiPoints) && g.Layout.TaxiPoints[n].IsILSHoldShort() {
		p.ILS = true
	}
	return p, true
}

// holdPointsView turns an event's holding points into the view's.
func holdPointsView(g *airport.Graph, at airport.NodeID, past []airport.NodeID) (*HoldPoint, []HoldPoint) {
	var held *HoldPoint
	if p, ok := holdPointOf(g, at); ok {
		held = &p
	}
	var out []HoldPoint
	for _, n := range past {
		if p, ok := holdPointOf(g, n); ok {
			out = append(out, p)
		}
	}
	return held, out
}

// sameHolds reports whether two views hold and release the same points.
func sameHolds(a, b ControlView) bool {
	if (a.HoldAt == nil) != (b.HoldAt == nil) || a.HoldAt != nil && a.HoldAt.Node != b.HoldAt.Node {
		return false
	}
	return slices.EqualFunc(a.ClearedPast, b.ClearedPast, func(x, y HoldPoint) bool { return x.Node == y.Node })
}

// holdBars is every runway holding point of g's airport with its stop bar,
// released where one of our aircraft there is cleared past it.
func (cc *controlCenter) holdBars(g *airport.Graph) []HoldBar {
	by := map[airport.NodeID][]string{}
	cc.mu.Lock()
	for _, it := range cc.items {
		it.mu.Lock()
		if it.ICAO == g.Layout.ICAO {
			for _, p := range it.view.ClearedPast {
				by[p.Node] = append(by[p.Node], it.Tail)
			}
		}
		it.mu.Unlock()
	}
	cc.mu.Unlock()
	out := []HoldBar{}
	for i := range g.Nodes {
		if p, ok := holdPointOf(g, airport.NodeID(i)); ok {
			tails := by[p.Node]
			slices.Sort(tails)
			out = append(out, HoldBar{HoldPoint: p, Released: len(tails) > 0, By: tails})
		}
	}
	return out
}
