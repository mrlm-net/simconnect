package traffic

import (
	"math"
	"sort"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Rescue standby points (#1597). Fire trucks wait on pavement with access
// to the runway: at the holding positions of its exits and entries, out
// of the runway strip (RescueStripHalfMeters), spread along it by the
// shares of the runway's length they always were (a third, half, two
// thirds; more than 3 from a fifth to four fifths, #1587), the one
// nearest the expected stop (RescueExpectedStopShare) first. A layout may
// give its own (Layout.RescueStandby): those are taken first.

// RescueExpectedStopShare: an aircraft in an emergency is expected to stop
// this share of the runway's length from the landing threshold; the
// standby point nearest it gets the first vehicle.
const RescueExpectedStopShare = 0.5

// RescueSpacingMeters: rescue vehicles keep this far apart, standing (two
// standby points at one holding position) and driving one behind another.
const RescueSpacingMeters = RescueDepotGapMeters

// RescueStripHalfMeters is the half-width of the runway strip kept clear
// of standby points for a runway width meters wide: 75 m from the
// centreline for a runway 30 m wide or more, 40 m from 18 m, 30 m below.
// A working figure, not a published one: the runway-holding positions in
// the test layouts (ten airports) lie 71 to 218 m from the centreline of
// runways 42 to 61 m wide, so 75 m keeps standby on the holding positions
// (walked out along the taxiway where one is nearer) and off the grass
// just beside the runway edge (#1597).
func RescueStripHalfMeters(width float64) float64 {
	switch {
	case width >= 30:
		return 75
	case width >= 18:
		return 40
	}
	return 30
}

// rescueSpot is a standby point candidate: where, along the runway from
// its landing threshold, and facing which way.
type rescueSpot struct {
	at     airport.LatLon
	along  float64
	facing float64
}

// rescueAxis is p along the runway end's direction from its threshold and
// across it (right positive), in metres.
func rescueAxis(end airport.RunwayEnd, p airport.LatLon) (along, cross float64) {
	d := localDist(end.Threshold, p)
	a := (localBearing(end.Threshold, p) - end.Heading) * math.Pi / 180
	return d * math.Cos(a), d * math.Sin(a)
}

// RescueStandby are the standby points for a landing on runwayEnd: n of
// them, on the layout's standby positions or the holding positions of the
// runway's exits and entries (#1597), at the shares of its length, out of
// the runway strip, at least RescueSpacingMeters apart, the one nearest
// the expected stop first; each faces the runway. A layout without
// holding positions out of the strip: beside the runway, past the strip.
func RescueStandby(l *airport.Layout, runwayEnd string, n int) []GroundPose {
	rwy, end, ok := l.RunwayEnd(runwayEnd)
	if !ok || n <= 0 {
		return nil
	}
	shares := []float64{1.0 / 3, 0.5, 2.0 / 3}[:min(n, 3)]
	if n > 3 {
		shares = shares[:0]
		for i := 0; i < n; i++ {
			shares = append(shares, 0.2+0.6*float64(i)/float64(n-1))
		}
	}
	var given []rescueSpot
	for _, p := range l.RescueStandbyFor(runwayEnd) {
		along, _ := rescueAxis(end, p.Position)
		given = append(given, rescueSpot{p.Position, along, p.Heading})
	}
	var derived []rescueSpot
	if len(given) < n {
		derived = rescueHolds(l, rwy, end, given)
	}
	used := make([]bool, len(given)+len(derived))
	pool := append(append([]rescueSpot(nil), given...), derived...)
	pick := func(from, to int, target float64) int {
		best, bestD := -1, math.Inf(1)
		for i := from; i < to; i++ {
			if d := math.Abs(pool[i].along - target); !used[i] && d < bestD {
				best, bestD = i, d
			}
		}
		return best
	}
	var chosen []rescueSpot
	var left []float64
	for _, share := range shares {
		target := rwy.Length * share
		i := pick(0, len(given), target)
		if i < 0 {
			i = pick(len(given), len(pool), target)
		}
		if i < 0 {
			left = append(left, share)
			continue
		}
		used[i] = true
		chosen = append(chosen, pool[i])
	}
	side := rescueExitSide(l, runwayEnd)
	aside := math.Max(rwy.Width/2+RescueStandbyAsideMeters, RescueStripHalfMeters(rwy.Width)+RescueSpacingMeters)
	for _, share := range left {
		p := offsetHeading(end.Threshold, end.Heading, rwy.Length*share)
		p = offsetHeading(p, end.Heading+side, aside)
		chosen = append(chosen, rescueSpot{p, rwy.Length * share, math.Mod(end.Heading-side+360, 360)})
	}
	stop := rwy.Length * RescueExpectedStopShare
	sort.SliceStable(chosen, func(i, j int) bool {
		return math.Abs(chosen[i].along-stop) < math.Abs(chosen[j].along-stop)
	})
	out := make([]GroundPose, 0, len(chosen))
	for _, s := range chosen {
		out = append(out, GroundPose{Position: s.at, Heading: s.facing})
	}
	return out
}

// RescueStandbyAsideMeters: without a holding position to wait at, standby
// points lie at least this far beside the runway's edge (and past the
// strip).
const RescueStandbyAsideMeters = 60.0

// rescueExitSide is the side (±90° off the landing direction) most of the
// runway's exits leave on: right unless most are left.
func rescueExitSide(l *airport.Layout, runwayEnd string) float64 {
	g, err := airport.BuildGraph(l)
	if err != nil {
		return 90
	}
	exits, err := g.RunwayExits(runwayEnd)
	if err != nil || len(exits) == 0 {
		return 90
	}
	left := 0
	for _, x := range exits {
		if x.Side == airport.ExitLeft {
			left++
		}
	}
	if left*2 > len(exits) {
		return -90
	}
	return 90
}

// rescueTurnoffs are the ways on and off the runway: the exits for a
// landing on either end (the entries for the other).
func rescueTurnoffs(g *airport.Graph, rwy airport.Runway) []airport.RunwayExit {
	var out []airport.RunwayExit
	for _, name := range []string{rwy.Primary.Name, rwy.Secondary.Name} {
		if xs, err := g.RunwayExits(name); err == nil {
			out = append(out, xs...)
		}
	}
	return out
}

// rescueHolds are the standby candidates of the runway's exits and
// entries: each one's holding position (the ILS one, else the exit's
// first node off the runway, when it has none), walked out along the
// taxiway until past the runway strip; a second and third further out by
// RescueSpacingMeters for more vehicles than holding positions. None
// within RescueSpacingMeters of another or of taken.
func rescueHolds(l *airport.Layout, rwy airport.Runway, end airport.RunwayEnd, taken []rescueSpot) []rescueSpot {
	g, err := airport.BuildGraph(l)
	if err != nil {
		return nil
	}
	strip := RescueStripHalfMeters(rwy.Width)
	var lines [][]airport.LatLon
	for _, x := range rescueTurnoffs(g, rwy) {
		start := x.HoldShort
		if start < 0 {
			start = x.ILSHoldShort
		}
		if start < 0 {
			start = x.Node
		}
		if line := rescueWalkOut(g, end, start, strip); len(line) > 0 {
			lines = append(lines, line)
		}
	}
	far := func(p airport.LatLon, have []rescueSpot) bool {
		for _, s := range have {
			if localDist(s.at, p) < RescueSpacingMeters {
				return false
			}
		}
		return true
	}
	have := append([]rescueSpot(nil), taken...)
	var out []rescueSpot
	for k := 0; k < 3; k++ {
		for _, line := range lines {
			p, ok := rescueAlongLine(line, float64(k)*RescueSpacingMeters)
			if !ok || !far(p, have) {
				continue
			}
			along, cross := rescueAxis(end, p)
			if math.Abs(cross) <= strip || along < 0 || along > rwy.Length {
				continue
			}
			facing := math.Mod(end.Heading-math.Copysign(90, cross)+360, 360)
			s := rescueSpot{p, along, facing}
			have, out = append(have, s), append(out, s)
		}
	}
	return out
}

// rescueWalkOut is the way out from node along the taxiways (never a
// runway path), each step to the neighbour farthest from the centreline,
// from the first point past strip on, at most rescueWalkMeters on; nil
// when it never gets past.
func rescueWalkOut(g *airport.Graph, end airport.RunwayEnd, node airport.NodeID, strip float64) []airport.LatLon {
	off := func(n airport.NodeID) float64 {
		_, c := rescueAxis(end, g.Nodes[n].Position)
		return math.Abs(c)
	}
	var line []airport.LatLon
	walked := 0.0
	seen := map[airport.NodeID]bool{node: true}
	for n := node; ; {
		if off(n) > strip {
			if len(line) > 0 {
				walked += localDist(line[len(line)-1], g.Nodes[n].Position)
			}
			line = append(line, g.Nodes[n].Position)
		}
		if walked >= rescueWalkMeters || len(seen) > 12 {
			break
		}
		next, best := airport.NodeID(-1), off(n)
		for _, e := range g.Adj[n] {
			if e.Type == types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_RUNWAY || seen[e.To] {
				continue
			}
			if d := off(e.To); d > best {
				next, best = e.To, d
			}
		}
		if next < 0 {
			break
		}
		seen[next] = true
		n = next
	}
	return line
}

// rescueWalkMeters bounds the walk out from a holding position: room for
// the third vehicle at it.
const rescueWalkMeters = 2*RescueSpacingMeters + 10

// rescueAlongLine is the point d metres along line.
func rescueAlongLine(line []airport.LatLon, d float64) (airport.LatLon, bool) {
	if d == 0 {
		return line[0], true
	}
	for i := 1; i < len(line); i++ {
		seg := localDist(line[i-1], line[i])
		if d <= seg && seg > 0 {
			f := d / seg
			a, b := line[i-1], line[i]
			return airport.LatLon{Lat: a.Lat + (b.Lat-a.Lat)*f, Lon: a.Lon + (b.Lon-a.Lon)*f}, true
		}
		d -= seg
	}
	return airport.LatLon{}, false
}

// RescueWay is the way onto the runway to an aircraft: Node, a taxiway node
// just off the runway where the vehicles join the exit or entry from the
// taxiways, then Onto, the exit's nodes from there to the runway
// centreline; Ahead when it meets the runway ahead of the aircraft. A way
// with no Onto: the aircraft is off the runway, Node the taxiway node
// nearest it.
type RescueWay struct {
	Node  airport.LatLon
	Onto  []airport.LatLon
	Ahead bool
}

// RescueAccess is the way for rescue vehicles to an aircraft at at, on or
// beside runway rwy: the exit or entry meeting the runway nearest it; not
// ahead of it while it still moves (they never drive at it). Off the
// runway: by the taxiways to it.
func RescueAccess(g *airport.Graph, runwayEnd string, at airport.LatLon, moving bool) (RescueWay, bool) {
	rwy, end, ok := g.Layout.RunwayEnd(runwayEnd)
	if !ok {
		return RescueWay{}, false
	}
	if r, on := g.Layout.RunwayAt(at, 0); !on || r.Index != rwy.Index {
		return RescueWay{Node: at}, true
	}
	ac, _ := rescueAxis(end, at)
	var best airport.RunwayExit
	bestD, found := math.Inf(1), false
	for _, x := range rescueTurnoffs(g, rwy) {
		along, _ := rescueAxis(end, g.Nodes[x.RunwayNode].Position)
		if moving && along > ac {
			continue
		}
		if d := math.Abs(along - ac); d < bestD {
			best, bestD, found = x, d, true
		}
	}
	if !found {
		return RescueWay{}, false
	}
	w := RescueWay{Node: g.Nodes[best.Node].Position}
	for i := len(best.Path) - 2; i >= 0; i-- {
		w.Onto = append(w.Onto, g.Nodes[best.Path[i]].Position)
	}
	along, _ := rescueAxis(end, g.Nodes[best.RunwayNode].Position)
	w.Ahead = along > ac
	return w, true
}
