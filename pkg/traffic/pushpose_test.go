//go:build windows
// +build windows

package traffic

import (
	"math"
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// A push ends on a taxiway facing the way out, whatever the airport: at
// every test airport (every third stand, both ends of the longest runway)
// the push planned to a pose is one a tug can make — at most
// pushPoseMaxMeters, no turn tighter than PushbackMinArcMeters, no loop —
// and the taxi-out starts along the nose (no turn from a standstill).
// Nearly every stand gets a pose; the rest keep the older plans.
func TestPushToPoseEverywhere(t *testing.T) {
	for _, icao := range testAirports {
		g := airportGraph(t, icao)
		l := g.Layout
		var rw *airport.Runway
		for i := range l.Runways {
			if rw == nil || l.Runways[i].Length > rw.Length {
				rw = &l.Runways[i]
			}
		}
		planned, posed := 0, 0
		for i, st := range l.Parking {
			if i%3 != 0 {
				continue
			}
			switch st.Type {
			case types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_SMALL,
				types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_MEDIUM, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_LARGE,
				types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_EXTRA:
				continue
			}
			for _, rwy := range []string{rw.Primary.Name, rw.Secondary.Name} {
				ec := &eventClient{}
				ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
				if err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: rwy, Model: "FSLTL_B738_RYR", Tail: "T1", RollingTakeoffChance: -1}); err != nil {
					continue
				}
				if !slices.Contains(l.SuitableStands(ctl.profile().SpanMeters/2), i) || ctl.facesOut() {
					continue
				}
				planned++
				p := ctl.pushPose
				if p == nil {
					continue
				}
				posed++
				name := icao + " " + st.Label() + " for " + rwy
				path, err := ctl.pushPath()
				if err != nil {
					t.Errorf("%s: %v", name, err)
					continue
				}
				pts := ctl.pushPts
				if l := path.Length(); l > pushPoseMaxMeters+1 {
					t.Errorf("%s: push %.0f m", name, l)
				}
				if r := tightestTurn(pts); r < PushbackMinArcMeters-3 {
					t.Errorf("%s: push turns on %.1f m", name, r)
				}
				n := len(pts)
				net := math.Abs(headingDiff(localBearing(pts[0], pts[2]), localBearing(pts[n-3], pts[n-1])))
				if turn := totalTurn(pts); turn > net+pushMaxSwerveDeg+1 {
					t.Errorf("%s: push turns %.0f° to turn the aircraft %.0f°", name, turn, net)
				}
				// The push ends with the aircraft along the pose, the nose on it.
				end := localBearing(pts[n-3], pts[n-1]) + 180
				if d := math.Abs(headingDiff(end, p.heading)); d > 5 {
					t.Errorf("%s: push ends %.0f° off the pose", name, d)
				}
				if !p.within(pushTaxiStartMeters, pushTaxiStartDeg) {
					t.Errorf("%s: taxi-out turns off the nose at the start", name)
				}
				if ctl.route.Nodes[0] != p.from || ctl.route.Nodes[1] != p.to {
					t.Errorf("%s: route starts %v, not on the pose's edge %d→%d", name, ctl.route.Nodes[:2], p.from, p.to)
				}
			}
		}
		t.Logf("%s: %d of %d pushes to a pose", icao, posed, planned)
		if posed < planned*85/100 {
			t.Errorf("%s: only %d of %d pushes to a pose", icao, posed, planned)
		}
	}
}

// A stand faces out only if its junction lies ahead of the nose gear: the
// junction of EDDF B10 or KJFK A15 is ahead of the stand's reference point
// but under the aircraft, and they taxied off with a turn of 115°–163° from
// a standstill; they are pushed. LKPR's GA stands N52–N54 still face out.
// A push to a pose, whose route starts on a taxiway ahead of the stand,
// does not make a stand face out.
func TestFacesOutFromTheNose(t *testing.T) {
	for _, c := range []struct {
		icao, stand, rwy string
		out              bool
	}{
		{"EDDF", "B10", "25C", false}, {"KJFK", "A15", "13R", false}, {"KJFK", "B1", "13R", false}, {"KJFK", "E10", "13R", false},
		{"LKPR", "N52", "24", true}, {"LKPR", "N53", "24", true}, {"LKPR", "N54", "24", true},
	} {
		g := airportGraph(t, c.icao)
		i, err := g.Layout.ParkingIndex(c.stand)
		if err != nil {
			t.Fatal(err)
		}
		ec := &eventClient{}
		ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
		if err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: c.rwy, Model: "FSLTL_B738_RYR", Tail: "T1", RollingTakeoffChance: -1}); err != nil {
			t.Fatal(err)
		}
		if ctl.facesOut() != c.out {
			t.Errorf("%s %s: faces out %v, want %v", c.icao, c.stand, ctl.facesOut(), c.out)
		}
		if ctl.pushPose != nil && ctl.facesOut() {
			t.Errorf("%s %s: pushed to a pose, yet faces out", c.icao, c.stand)
		}
	}
}
