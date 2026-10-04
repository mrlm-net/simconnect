package airport

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

// DeicingPad is a remote de-icing position (#323): departures taxi onto it
// with engines running, are treated, and taxi on to the runway. MSFS
// facility data has no de-icing type for parking spots or taxi points, so
// pads come from Limits.DeicingPads (KnownLimits, or set by the caller);
// without one, aircraft are de-iced on their stand.
type DeicingPad struct {
	Name     string `json:"name"`
	Position LatLon `json:"position"`
}

// NearestNode is the taxi node (not a stand) nearest to p within
// maxMeters, e.g. the node a de-icing pad is on.
func (g *Graph) NearestNode(p LatLon, maxMeters float64) (NodeID, bool) {
	best, bd := NodeID(-1), math.Inf(1)
	for _, n := range g.Nodes {
		if n.Kind == NodeParking {
			continue
		}
		if d := calc.HaversineMeters(p.Lat, p.Lon, n.Position.Lat, n.Position.Lon); d < bd {
			best, bd = n.ID, d
		}
	}
	return best, best >= 0 && bd <= maxMeters
}
