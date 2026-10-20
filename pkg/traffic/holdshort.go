package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// HoldShortAheadMeters: past a give-way stop, how far along its route an
// aircraft looks for the taxiway it would turn onto or cross there; ground
// may say "hold short of (taxiway)" for it instead of "give way to".
var HoldShortAheadMeters = 60.0

// near is the segment of the route nearest p and the distance along the
// route there, the whole route searched (the tracker not moved on).
func (t *routeTracker) near(p airport.LatLon) (seg int, along float64) {
	pts := t.route.Points
	best := math.Inf(1)
	seg = -1
	for i := 1; i < len(pts); i++ {
		ax, az := meters(p, pts[i-1])
		bx, bz := meters(p, pts[i])
		dx, dz := bx-ax, bz-az
		l2 := dx*dx + dz*dz
		f := 0.0
		if l2 > 0 {
			f = math.Max(0, math.Min(1, -(ax*dx+az*dz)/l2))
		}
		cx, cz := ax+dx*f, az+dz*f
		if d := cx*cx + cz*cz; d < best {
			best, seg, along = d, i-1, t.cum[i-1]+f*(t.cum[i]-t.cum[i-1])
		}
	}
	return seg, along
}

// enteredAfter is the taxiway the route turns onto or crosses within
// HoldShortAheadMeters past p, "" when it stays on the one it is on at p
// (or none of them has a name).
func (t *routeTracker) enteredAfter(p airport.LatLon) string {
	if t == nil || len(t.route.Points) < 2 {
		return ""
	}
	seg, along := t.near(p)
	if seg < 0 {
		return ""
	}
	here := t.taxiwayAt(seg)
	end := t.segmentAt(along + HoldShortAheadMeters)
	for i := seg + 1; i <= end && i < len(t.route.Edges); i++ {
		if n := t.route.Edges[i].Name; n != "" && n != here {
			return n
		}
	}
	return ""
}

// giveWayHoldShort is the taxiway short of which an aircraft giving way
// stops ("" not giving way, or not short of another taxiway).
func (d *groundDrive) giveWayHoldShort(t *routeTracker) string {
	if d.givingWay == 0 || !d.hasGiveWayAt {
		return ""
	}
	return t.enteredAfter(d.giveWayAt)
}
