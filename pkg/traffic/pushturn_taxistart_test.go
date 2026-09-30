//go:build windows
// +build windows

package traffic

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// The taxi starts along the nose after every push: no corner and no pivot
// on the spot in its first 30 m (#492; LKPR, live: at A1 a CS300 was
// pushed straight and turned 90° where it stood; at B14, B15 and A7 the
// push left the nose off the way out; at B9 the taxi began with a leg
// backwards). The departures are driven for real, from spawn to the taxi,
// at every stand the allocator gives each type (SuitableStands; GA ramps
// aside), both runways, three types. A smooth turn onto a taxiway (LKPR C22) is not a corner: at
// most taxiStartCornerDeg in any 5 m.
func TestTaxiStartsAlongNose(t *testing.T) {
	if testing.Short() {
		t.Skip("drives every stand's departure")
	}
	const taxiStartCornerDeg = 35
	g := lkprGraph(t)
	bad, total := 0, 0
	for i, st := range g.Layout.Parking {
		switch st.Type {
		case types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_SMALL,
			types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_MEDIUM, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_LARGE,
			types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_EXTRA:
			continue
		}
		for _, m := range []string{"FSLTL_B738_RYR", "FSLTL_SBAI_BCS3_CSA-Lines", "FSLTL_E190_LOT"} {
			for _, rwy := range []string{"24", "06"} {
				ec := &eventClient{}
				inj := NewInjector(ec)
				ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj))
				if err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: rwy, Model: m, Tail: "T1", RollingTakeoffChance: -1}); err != nil {
					continue
				}
				// Only stands the allocator gives this type (StandAllocator.Assign).
				if !slices.Contains(g.Layout.SuitableStands(ctl.profile().SpanMeters/2), i) {
					continue
				}
				now := time.Now()
				ctl.now = func() time.Time { return now }
				ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
				inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
				mon := DefaultTaxiRequestBase + reqOffMonitor
				for f := 0; f < 60*600 && ctl.State() != TaxiTaxiing && !ctl.State().Terminal(); f++ {
					now = now.Add(time.Second / 60)
					ctl.Handle(positionMsg(mon, 77, st.Position, 0, 0, true))
				}
				if ctl.State() != TaxiTaxiing {
					t.Errorf("%s %s runway %s: never taxied (%v)", st.Label(), m, rwy, ctl.State())
					continue
				}
				total++
				pose, path := ctl.mover.Pose(), ctl.mover.Path()
				prev, worst := pose.Heading, 0.0
				for s := pose.Distance; s < pose.Distance+30 && s+5 < path.Length(); s += 5 {
					h := localBearing(path.PointAt(s), path.PointAt(s+5))
					worst = math.Max(worst, math.Abs(headingDiff(prev, h)))
					prev = h
				}
				if worst > taxiStartCornerDeg {
					bad++
					t.Errorf("%s %s runway %s: the taxi turns %.0f° in 5 m within its first 30 m", st.Label(), m, rwy, worst)
				}
			}
		}
	}
	if total < 150 {
		t.Errorf("only %d departures reached the taxi", total)
	}
	t.Logf("%d of %d taxi starts with a corner", bad, total)
}
