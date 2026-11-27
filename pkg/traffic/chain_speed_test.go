package traffic

import (
	"encoding/binary"
	"math"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// sentChain is the last waypoint chain written to the simulator.
func sentChain(ec *eventClient) []types.SIMCONNECT_DATA_WAYPOINT {
	w := int(engine.WaypointWireSize)
	for i := len(ec.waypoints) - 1; i >= 0; i-- {
		b := ec.waypoints[i]
		if len(b) < 2*w || len(b)%w != 0 || len(b) == int(unsafe.Sizeof(types.SIMCONNECT_DATA_INITPOSITION{})) {
			continue
		}
		var out []types.SIMCONNECT_DATA_WAYPOINT
		for k := 0; k+w <= len(b); k += w {
			f := func(o int) float64 { return math.Float64frombits(binary.LittleEndian.Uint64(b[k+o:])) }
			out = append(out, types.SIMCONNECT_DATA_WAYPOINT{Latitude: f(0), Longitude: f(8), Altitude: f(16), Flags: binary.LittleEndian.Uint32(b[k+24:]), KtsSpeed: f(28)})
		}
		if math.Abs(out[0].Latitude) <= 90 && math.Abs(out[0].Longitude) <= 180 && out[0].KtsSpeed > 0 {
			return out
		}
	}
	return nil
}

// TestApproachChainSpeeds (#1682): the STAR and approach transition MSFS
// AI flies for an arrival are no faster than the type's top speed (a DHC6:
// VMO 166 KIAS, not 250 and 180), and slow toward its approach at the
// join: the join point at the injected approach's start speed, the
// alignment point before it halfway from the transition speed. Before, a
// DHC6 came to the join at 180 kt and the approach took it over at 88.
func TestApproachChainSpeeds(t *testing.T) {
	for _, tc := range []struct {
		model  string
		topKts float64
	}{
		{"FSLTL_DHC6_WIA", 166},
		{"FSLTL A320 Air France SL", ProcedureSpeedKts},
	} {
		t.Run(tc.model, func(t *testing.T) {
			g := lkprGraph(t)
			route, err := lkprProcedures(t).Arrival("06", "GOLOP")
			if err != nil {
				t.Fatal(err)
			}
			ec := &eventClient{}
			inj := NewInjector(ec)
			ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
			c22, _ := g.Layout.ParkingIndex("C22")
			if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "06", Parking: c22, Model: tc.model, Tail: "WIA1",
				InjectApproach: true, Procedure: route, RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
				t.Fatal(err)
			}
			go func() {
				for range ctl.Events() {
				}
			}()
			now := time.Now()
			ctl.now = func() time.Time { return now }
			ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
			if !ctl.flyingProc {
				t.Fatal("not handed to MSFS AI for the procedure")
			}
			chain := sentChain(ec)
			if len(chain) < 3 {
				t.Fatalf("chain of %d points", len(chain))
			}
			for i, w := range chain {
				if w.KtsSpeed > tc.topKts+0.01 {
					t.Errorf("point %d of %d at %.0f kt, over the type's %.0f", i, len(chain), w.KtsSpeed, tc.topKts)
				}
			}
			start := approachProfileOf(ctl.req).StartKts
			trans := math.Min(ProcedureApproachSpeedKts, tc.topKts)
			n := len(chain)
			if got := chain[n-1].KtsSpeed; got > start+0.01 {
				t.Errorf("join point at %.0f kt, the approach starts at %.0f", got, start)
			}
			if got, want := chain[n-2].KtsSpeed, math.Max(start, (trans+start)/2); got > want+0.01 {
				t.Errorf("alignment point at %.0f kt, want at most %.0f (halfway from %.0f to %.0f)", got, want, trans, start)
			}
			// An aircraft slowing 2 kt/s from the alignment point's speed is
			// at the join point's by the join.
			al, jo := chain[n-2], chain[n-1]
			d := math.Hypot((jo.Latitude-al.Latitude)*111195, (jo.Longitude-al.Longitude)*111195*math.Cos(al.Latitude*math.Pi/180))
			if need := (al.KtsSpeed - jo.KtsSpeed) / 2 * (al.KtsSpeed + jo.KtsSpeed) / 2 * ktsToMS; need > d {
				t.Errorf("%.0f → %.0f kt takes %.0f m at 2 kt/s, the leg is %.0f m", al.KtsSpeed, jo.KtsSpeed, need, d)
			}
		})
	}
}
