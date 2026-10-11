package traffic

import (
	"math"
	"testing"
	"time"
)

// A departure lines up facing down the runway before its roll starts (live
// at EDDM 26L: lined up 3.5° off, the heading snapped as the take-off
// began): held lined up, or rolling straight on.
func TestLineUpAligned(t *testing.T) {
	for _, c := range []struct{ icao, stand, rwy string }{{"EDDM", "214", "26L"}, {"EDDM", "214", "26R"}, {"EDDM", "214", "08R"}, {"LKPR", "C22", "24"}} {
		for _, rolling := range []bool{false, true} {
			g := airportGraph(t, c.icao)
			i, err := g.Layout.ParkingIndex(c.stand)
			if err != nil {
				t.Fatal(err)
			}
			ec := &eventClient{}
			inj := NewInjector(ec)
			ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj))
			if err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: c.rwy, Model: "FSLTL_B738_KLM", Tail: "T1", RollingTakeoffChance: -1, HoldForClearances: true}); err != nil {
				t.Fatal(err)
			}
			now := time.Now()
			ctl.now = func() time.Time { return now }
			ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
			inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
			mon := DefaultTaxiRequestBase + reqOffMonitor
			st := g.Layout.Parking[i]
			var last GroundPose
			frame := func() {
				now = now.Add(time.Second / 60)
				ctl.Handle(positionMsg(mon, 77, st.Position, 0, 0, true))
				if ctl.mover != nil {
					last = ctl.mover.Pose()
				}
			}
			run := func(until TaxiState, n int) bool {
				for k := 0; k < n && ctl.State() != until && !ctl.State().Terminal(); k++ {
					frame()
				}
				return ctl.State() == until
			}
			run(TaxiAwaitingPushback, 60*900)
			ctl.ClearPushback()
			run(TaxiAwaitingTaxi, 60*900)
			ctl.ClearToTaxi()
			if !run(TaxiHoldingShort, 60*1800) {
				t.Fatalf("%s %s: state %v, want holding short", c.icao, c.rwy, ctl.State())
			}
			want := TaxiLinedUp
			if rolling {
				ctl.ClearForTakeoff()
				want = TaxiDeparting
			} else {
				ctl.ClearToLineUp()
			}
			if !run(want, 60*600) {
				t.Fatalf("%s %s: state %v, want %v", c.icao, c.rwy, ctl.State(), want)
			}
			if off := math.Abs(headingDiff(last.Heading, ctl.end.Heading)); off > 1 {
				t.Errorf("%s %s rolling %v: %.1f° off the runway as the roll begins", c.icao, c.rwy, rolling, off)
			}
		}
	}
}
