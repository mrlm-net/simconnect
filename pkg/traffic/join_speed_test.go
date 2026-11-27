package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// TestJoinSpeedSmooth (#1682): a DHC6 and an A320 fly GOLOP 06 with MSFS AI
// (here an ideal one: each leg at its point's speed, changing at most
// 2 kt/s) and the injected approach takes over at the join. No point of
// the chain is faster than the type's top speed; across the takeover and
// down to touchdown the ground speed changes by no more than about 2 kt
// per second between writes, and the first write is at the altitude MSFS
// AI had it at.
func TestJoinSpeedSmooth(t *testing.T) {
	for _, tc := range []struct {
		model  string
		topKts float64 // the type's top speed in the chain
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
			if len(chain) < 2 {
				t.Fatal("no waypoint chain sent")
			}
			for i, w := range chain {
				if w.KtsSpeed > tc.topKts+0.01 {
					t.Errorf("point %d of %d at %.0f kt, over the type's %.0f", i, len(chain), w.KtsSpeed, tc.topKts)
				}
			}
			mon := DefaultArrivalRequestBase + arrReqMonitor
			fieldFt := g.Layout.Altitude / 0.3048
			// The ideal MSFS AI along the chain, one answer a second.
			sp := ctl.proc.Spawn
			pos := airport.LatLon{Lat: sp.Latitude, Lon: sp.Longitude}
			alt, kts := sp.Altitude, chain[0].KtsSpeed
			k := 0
			var lastKts, lastAlt float64
			for s := 0; s < 3600 && ctl.flyingProc && k < len(chain); s++ {
				w := chain[k]
				to := airport.LatLon{Lat: w.Latitude, Lon: w.Longitude}
				kts += math.Max(-2, math.Min(2, w.KtsSpeed-kts))
				step := kts * ktsToMS
				d := calc.HaversineMeters(pos.Lat, pos.Lon, to.Lat, to.Lon)
				hdg := calc.BearingDegrees(pos.Lat, pos.Lon, to.Lat, to.Lon)
				if d <= step {
					pos, alt, k = to, w.Altitude, k+1
				} else {
					alt += (w.Altitude - alt) * step / d
					lat, lon := calc.DisplaceByHeading(pos.Lat, pos.Lon, hdg, step)
					pos = airport.LatLon{Lat: lat, Lon: lon}
				}
				now = now.Add(time.Second)
				lastKts, lastAlt = kts, alt
				ctl.Handle(flownMsg(mon, 77, pos, alt, alt-fieldFt, hdg, kts))
			}
			if ctl.flyingProc || ctl.approach == nil {
				t.Fatalf("no takeover at the join (point %d of %d)", k, len(chain))
			}
			inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, fieldFt, 12))
			// What the simulator shows each frame: the frame's last write (at
			// the nose wheel down the approach's and the rollout's first are
			// written in one frame, at one place).
			seen := len(placements(ec))
			var placed []types.SIMCONNECT_DATA_INITPOSITION
			for i := 0; i < 60*900 && ctl.State() != ArrivalParked; i++ {
				now = now.Add(time.Second / 60)
				ctl.Handle(arrivalPositionMsg(mon, 77, ctl.Plan().End.Threshold, 0, 0, 0, false))
				n := placements(ec)
				if len(n) > seen {
					placed, seen = append(placed, n[len(n)-1]), len(n)
				}
				if len(placed) > 0 && placed[len(placed)-1].OnGround != 0 {
					break
				}
			}
			if len(placed) < 60 {
				t.Fatalf("%d writes after the takeover", len(placed))
			}
			first := placed[0]
			if d := math.Abs(float64(first.Airspeed) - lastKts); d > 3 {
				t.Errorf("first write at %d kt, MSFS AI flew it at %.0f kt", first.Airspeed, lastKts)
			}
			if d := math.Abs(first.Altitude - lastAlt); d > 3 {
				t.Errorf("first write at %.1f ft, MSFS AI had it at %.1f ft (%+.1f ft in one write)", first.Altitude, lastAlt, first.Altitude-lastAlt)
			}
			// Written ground speed (whole knots), over each second of writes:
			// about 2 kt/s.
			for i := 1; i < len(placed) && placed[i].OnGround == 0; i++ {
				j := max(0, i-60)
				if d := math.Abs(float64(placed[i].Airspeed) - float64(placed[j].Airspeed)); d > 3 {
					t.Fatalf("writes %d to %d: %d → %d kt within a second", j, i, placed[j].Airspeed, placed[i].Airspeed)
				}
			}
			// And from the positions, second to second.
			var prevKts float64
			for i := 60; i < len(placed) && placed[i].OnGround == 0; i += 60 {
				a, b := placed[i-60], placed[i]
				v := calc.HaversineMeters(a.Latitude, a.Longitude, b.Latitude, b.Longitude) / ktsToMS
				if i > 60 && math.Abs(v-prevKts) > 3 {
					t.Fatalf("second %d after the takeover: %.0f → %.0f kt over the ground", i/60, prevKts, v)
				}
				prevKts = v
			}
			if i := 60; len(placed) > i {
				v := calc.HaversineMeters(placed[0].Latitude, placed[0].Longitude, placed[i].Latitude, placed[i].Longitude) / ktsToMS
				if math.Abs(v-lastKts) > 4 {
					t.Errorf("first second after the takeover at %.0f kt over the ground, MSFS AI flew %.0f kt", v, lastKts)
				}
			}
		})
	}
}
