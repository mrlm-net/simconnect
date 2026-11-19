package traffic

import (
	"math"
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestTrimPathEnd (#1570): the stop HoldShortStopMeters before the end is
// trimmed along the path, also where the last segment is shorter than that.
func TestTrimPathEnd(t *testing.T) {
	a := airport.LatLon{Lat: 50.1, Lon: 14.2}
	b := offsetHeading(a, 90, 100)
	for _, last := range []float64{3, 7, 7.3, 20} {
		c := offsetHeading(b, 90, last)
		pts := []airport.LatLon{a, b, c}
		full := pathLen(pts)
		got := trimPathEnd(slices.Clone(pts), HoldShortStopMeters)
		if cut := full - pathLen(got); math.Abs(cut-HoldShortStopMeters) > 0.5 {
			t.Errorf("last segment %.1f m: %.2f m trimmed, want %.0f", last, cut, HoldShortStopMeters)
		}
		if d := localDist(got[len(got)-1], c); math.Abs(d-HoldShortStopMeters) > 0.5 {
			t.Errorf("last segment %.1f m: the path ends %.2f m from the hold-short, want %.0f", last, d, HoldShortStopMeters)
		}
	}
	// Too short to stop short of it: left as it is.
	short := []airport.LatLon{a, offsetHeading(a, 90, 5)}
	if got := trimPathEnd(slices.Clone(short), HoldShortStopMeters); !slices.Equal(got, short) {
		t.Errorf("a 5 m path trimmed to %v", got)
	}
}

// TestHoldShortStopShortLastSegment (#1570): a departure whose route has a
// node 3 m before the hold-short node (an OSM-derived layout, TNCM) stops
// with its nose gear HoldShortStopMeters short of it, not on the line.
func TestHoldShortStopShortLastSegment(t *testing.T) {
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{HoldForRunway: true, RollingTakeoffChance: -1})
	if !run(TaxiAwaitingTaxi, 60*240) {
		t.Fatalf("state %v, want awaiting taxi after the push", ctl.State())
	}
	r := ctl.route
	n := len(r.Points)
	hold, prev := r.Points[n-1], r.Points[n-2]
	near := offsetHeading(hold, localBearing(hold, prev), 3)
	r.Points = slices.Insert(r.Points, n-1, near)
	r.Nodes = slices.Insert(r.Nodes, n-1, r.Nodes[n-2])
	if len(r.Edges) == n-1 {
		r.Edges = slices.Insert(r.Edges, n-2, r.Edges[n-2])
	}
	ctl.track = newRouteTracker(r)
	if !run(TaxiHoldingShort, 60*900) {
		t.Fatalf("state %v, want holding short", ctl.State())
	}
	pose := ctl.mover.Pose()
	nose := NoseGear(pose.Position, pose.Heading, ctl.profile())
	if d := localDist(nose, hold); math.Abs(d-HoldShortStopMeters) > 1.5 {
		t.Errorf("nose gear stopped %.1f m from the hold-short node, want %.0f m short of it", d, HoldShortStopMeters)
	}
	if along := alongHeading(hold, localBearing(prev, hold), nose); along > -HoldShortStopMeters+1.5 {
		t.Errorf("nose gear %.1f m along past the hold-short node: on or past the line", along)
	}
}
