package traffic

import (
	"cmp"
	"math"
	"slices"
	"strings"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// PushRoute is a stand's pushback as a route: the main gear's way back
// (Points, from the stand), a tow forward after it (Tow, mostly none), and
// where it ends: the nose there (Nose) facing Facing (true degrees), as ATC
// says it (Said: "east"). A standard push as planned (StandardPush) or one
// drawn by hand (SetCustomPush).
type PushRoute struct {
	Points []airport.LatLon `json:"points"`
	Tow    []airport.LatLon `json:"tow,omitempty"`
	Nose   airport.LatLon   `json:"nose"`
	Facing float64          `json:"facing"`
	Said   string           `json:"said"`
}

// StandardPush is the stand's standard push (PlanStandardPushes) for an
// aircraft of model's size, planned now if it is not yet: the push a
// departure from the stand takes whatever its runway. False when the stand
// has none (it faces out, or no pose fits). The planning itself is as
// departures do it, unchanged.
func StandardPush(g *airport.Graph, stand int, model string) (PushRoute, bool) {
	if g == nil || stand < 0 || stand >= len(g.Layout.Parking) {
		return PushRoute{}, false
	}
	PlanStandardPushes(g, model, []int{stand})
	v, ok := standardPushes.Load(standardKey{g, stand})
	std, _ := v.(*pushPose)
	if !ok || std == nil {
		return PushRoute{}, false
	}
	// The push to it from a departure's planning: for the runway ends that
	// voted for it, the first whose push ends there.
	rwys := slices.Clone(g.Layout.Runways)
	slices.SortStableFunc(rwys, func(a, b airport.Runway) int { return cmp.Compare(b.Length, a.Length) })
	for _, r := range rwys[:min(2, len(rwys))] {
		for _, end := range []airport.RunwayEnd{r.Primary, r.Secondary} {
			p, err := PlanPush(TaxiRequest{Graph: g, Parking: stand, Model: model, Runway: end.Name})
			if err != nil || localDist(p.Pose, std.nose) > 10 || math.Abs(headingDiff(p.Heading, std.heading)) > 30 {
				continue
			}
			return pushRouteOf(p.Push, p.Tow, p.Pose, p.Heading), true
		}
	}
	return PushRoute{}, false
}

func pushRouteOf(push, tow []airport.LatLon, nose airport.LatLon, heading float64) PushRoute {
	facing := heading
	if len(tow) > 1 {
		facing = localBearing(tow[len(tow)-2], tow[len(tow)-1])
	}
	return PushRoute{Points: push, Tow: tow, Nose: nose, Facing: facing, Said: CompassName8(facing)}
}

// customPushes are the pushes drawn by hand, by ICAO and stand name.
var customPushes sync.Map

type customKey struct{ icao, stand string }

func customKeyOf(icao, stand string) customKey {
	return customKey{strings.ToUpper(strings.TrimSpace(icao)), strings.ToUpper(strings.TrimSpace(stand))}
}

// SetCustomPush makes r the push from stand (its name, "S6") at icao for
// every departure planned from now on: flown as drawn instead of the
// planned one, its facing said (PushFacingSaid). r with no Points is an
// end pose only (Nose, Facing): the push is planned to it (#493). r.Said
// is set from r.Facing when empty.
func SetCustomPush(icao, stand string, r PushRoute) {
	if r.Said == "" {
		r.Said = CompassName8(r.Facing)
	}
	customPushes.Store(customKeyOf(icao, stand), r)
}

// ClearCustomPush goes back to the planned push from stand at icao.
func ClearCustomPush(icao, stand string) { customPushes.Delete(customKeyOf(icao, stand)) }

// CustomPush is the push drawn for stand at icao, if any.
func CustomPush(icao, stand string) (PushRoute, bool) {
	v, ok := customPushes.Load(customKeyOf(icao, stand))
	if !ok {
		return PushRoute{}, false
	}
	return v.(PushRoute), true
}

// customPushTo plans the departure's push as drawn for its stand: the
// drawn way, ending on the taxiway edge under its nose that runs its way,
// and the taxi-out from there. False without one, or when its end is on no
// taxiway the taxi-out can start from (the planned push then).
func (c *TaxiController) customPushTo() bool {
	g := c.req.Graph
	r, ok := CustomPush(g.Layout.ICAO, g.Layout.Parking[c.req.Parking].Label())
	if !ok || len(r.Points) < 2 {
		return false
	}
	// The edge the nose ends on, facing along it.
	best, bestD := pushPose{}, math.Inf(1)
	for a := range g.Adj {
		pa := g.Nodes[a].Position
		for _, e := range g.Adj[a] {
			if !pushEdge(g, e) {
				continue
			}
			pb := g.Nodes[e.To].Position
			if math.Abs(headingDiff(localBearing(pa, pb), r.Facing)) > 45 {
				continue
			}
			along := calc.AlongTrackMeters(pa.Lat, pa.Lon, pb.Lat, pb.Lon, r.Nose.Lat, r.Nose.Lon)
			if along < -customPushEdgeMeters || along > localDist(pa, pb)+customPushEdgeMeters {
				continue
			}
			if d := math.Abs(calc.CrossTrackMeters(pa.Lat, pa.Lon, pb.Lat, pb.Lon, r.Nose.Lat, r.Nose.Lon)); d < bestD {
				best, bestD = pushPose{nose: r.Nose, heading: r.Facing, from: airport.NodeID(a), to: e.To}, d
			}
		}
	}
	if bestD > customPushEdgeMeters {
		return false
	}
	out := c.poseRoute(best, map[[2]airport.NodeID]*airport.Route{})
	if out == nil {
		return false
	}
	full, err := g.RouteFromNodes(append([]airport.NodeID{best.from}, out.Nodes...))
	if err != nil {
		return false
	}
	full.Runway, full.RunwayEnd, full.Entry, full.HoldShort, full.Tight = out.Runway, out.RunwayEnd, out.Entry, out.HoldShort, out.Tight
	best.out = out
	c.route, c.pushJunction, c.pushPts, c.towPts, c.pushPose = full, 0, slices.Clone(r.Points), slices.Clone(r.Tow), &best
	return true
}

// configuredPoses is poses kept to the stand's configured end pose (#493):
// a custom push with no points, only where the nose ends (Nose) and its
// facing, is planned like any push but to there — the poses within
// customPushEdgeMeters of it facing within customPoseDeg. All of poses
// without one, or when none is near it.
func (c *TaxiController) configuredPoses(poses []pushPose) []pushPose {
	g := c.req.Graph
	r, ok := CustomPush(g.Layout.ICAO, g.Layout.Parking[c.req.Parking].Label())
	if !ok || len(r.Points) >= 2 || r.Nose == (airport.LatLon{}) {
		return poses
	}
	var near []pushPose
	for _, p := range poses {
		if localDist(p.nose, r.Nose) <= customPushEdgeMeters && math.Abs(headingDiff(p.heading, r.Facing)) <= customPoseDeg {
			near = append(near, p)
		}
	}
	if len(near) == 0 {
		return poses
	}
	return near
}

// customPoseDeg: a configured end pose is met facing within this of it.
const customPoseDeg = 30.0

// customPushEdgeMeters: a drawn push ends on a taxiway edge within this of
// its nose.
const customPushEdgeMeters = 25.0
