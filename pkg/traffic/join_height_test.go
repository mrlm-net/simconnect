package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

// MSFS AI 4,000 ft above the final at its join (live MSFS AI comes in
// high too; offline, DLH1691 handed over at FL289 36 NM out): more than
// the hand-over can fade by the gate, so it is not joined to go round at
// once (cleared to land, then "going around, approach not stable"). It
// stays on its procedure, given the track miles to come down first: out
// and back to its join, a way at least as long as its descent needs, and
// its approach not judged at all. 300 ft above, as MSFS AI brings them
// in, it joins as before (#1681 follow-up).
func TestJoinTooHighExtends(t *testing.T) {
	for _, c := range []struct {
		above float64
		joins bool
	}{{300, true}, {4000, false}} {
		SetRandomSeed(1681)
		g := lkprGraph(t)
		route, err := lkprProcedures(t).Arrival("24", "LOMKI")
		if err != nil {
			t.Fatal(err)
		}
		ec := &eventClient{}
		now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
		inj := NewInjector(ec)
		inj.now = func() time.Time { return now }
		ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj), ArrivalWithSeed(1681))
		ctl.now = func() time.Time { return now }
		c17, _ := g.Layout.ParkingIndex("C17")
		if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c17, Model: "B738", Tail: "DLH1691",
			InjectApproach: true, RollThroughChance: -1, Procedure: route}); err != nil {
			t.Fatal(err)
		}
		go func() {
			for range ctl.Events() {
			}
		}()
		ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
		mon := DefaultArrivalRequestBase + arrReqMonitor
		p := ctl.Plan()
		ctl.mu.Lock()
		joinAt, joinM := ctl.proc.Join, ctl.proc.JoinMeters
		pathFt := NewApproachMover(p.End.LandingThreshold(), p.End.Heading, joinM, ctl.approachProfile()).Pose().HeightFt
		ctl.mu.Unlock()
		runwayFt := p.Runway.Altitude / 0.3048
		ctl.Handle(flownMsg(mon, 77, joinAt, runwayFt+pathFt+c.above, pathFt+c.above, p.End.Heading, 160))
		ctl.mu.Lock()
		joined, onProc := ctl.approach != nil, ctl.flyingProc
		unstable, round := ctl.last.Unstable, ctl.last.CrewGoAround
		wps := ctl.proc.Waypoints
		ctl.mu.Unlock()
		SetRandomSeed(0)
		if joined != c.joins {
			t.Errorf("%.0f ft above the final at the join: joined %v, want %v", c.above, joined, c.joins)
			continue
		}
		if c.joins {
			continue
		}
		if !onProc || unstable || round {
			t.Errorf("%.0f ft high: on its procedure %v, unstable %v, crew round %v; want on it, neither", c.above, onProc, unstable, round)
		}
		way := 0.0
		for i := 1; i < len(wps); i++ {
			way += calc.HaversineNM(wps[i-1].Latitude, wps[i-1].Longitude, wps[i].Latitude, wps[i].Longitude)
		}
		if need := c.above / ProcedureDescentFtPerNm; way < need {
			t.Errorf("%.0f ft high: %.1f NM back to its join, its descent needs %.1f", c.above, way, need)
		}
		last := wps[len(wps)-1]
		if d := calc.HaversineMeters(last.Latitude, last.Longitude, joinAt.Lat, joinAt.Lon); d > 50 {
			t.Errorf("%.0f ft high: its way ends %.0f m from its join", c.above, d)
		}
	}
}
