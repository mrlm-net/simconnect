package traffic

import (
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// noFarExitsGraph is LKPR with every way off runway 06/24 within farM of
// the 06 threshold closed (synthetic: LKPR has exits at both ends of both
// runways, so a take-off on 24 always has one ahead). A rejected take-off
// on 24 stopped past the last open exit has 1 km and more ahead to turn in
// and none ahead to leave by.
func noFarExitsGraph(t *testing.T, farM float64) *airport.Graph {
	t.Helper()
	b, err := os.ReadFile("../airport/testdata/LKPR.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	rwy, end, ok := l.RunwayEnd("06")
	if !ok {
		t.Fatal("no 06")
	}
	near := func(i int32) bool {
		if i < 0 || int(i) >= len(l.TaxiPoints) {
			return false
		}
		p := l.TaxiPoints[i].Position
		a := alongHeading(end.Threshold, end.Heading, p)
		far := offsetHeading(end.Threshold, end.Heading, a)
		return a > -50 && a < farM && calc.HaversineMeters(p.Lat, p.Lon, far.Lat, far.Lon) < rwy.Width/2+60
	}
	closed := 0
	for i, p := range raw.TaxiPaths {
		if types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE(p.Type) == types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_RUNWAY {
			continue
		}
		if near(p.Start) || near(p.End) {
			raw.TaxiPaths[i].Type = int32(types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_CLOSED)
			closed++
		}
	}
	if closed == 0 {
		t.Fatal("no way off 06/24 near 06 to close")
	}
	if l, err = airport.BuildLayout(raw); err != nil {
		t.Fatal(err)
	}
	g, err := airport.BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// TestRejectNoExitAheadTurnsRound (#1685): a rejected take-off stopped past
// the last exit ahead (as on a backtrack field, or near the far end) turns
// round on the runway and backtracks to an exit behind, then taxis to the
// holding point, smoothly; before, the departure failed on the runway.
func TestRejectNoExitAheadTurnsRound(t *testing.T) {
	const rwy = "24"
	g := noFarExitsGraph(t, 1000)
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj))
	c22, err := g.Layout.ParkingIndex("C22")
	if err != nil {
		t.Fatal(err)
	}
	if err := ctl.Start(TaxiRequest{Graph: g, Parking: c22, Runway: rwy, Model: "FSLTL A320 Air France SL", Tail: "CSA8", HoldForClearances: true}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	mon := DefaultTaxiRequestBase + reqOffMonitor
	stand := g.Layout.Parking[c22]
	frame := func() {
		now = now.Add(time.Second / 60)
		ctl.Handle(positionMsg(mon, 77, stand.Position, 0, 0, true))
	}

	prof := ctl.profile()
	exits, err := g.RunwayExits(rwy)
	if err != nil {
		t.Fatal(err)
	}
	last := 0.0
	for _, e := range exits {
		if e.Angle <= 120 && len(e.Path) >= 2 {
			last = math.Max(last, e.Along)
		}
	}
	// Stopped past the last exit ahead, with room ahead to turn round.
	along := last + prof.WheelbaseMeters + 30
	r, a0 := endTurnRadius(ctl.runway)
	if room := ctl.runway.Length - along; room < a0+2*r+rejectRoomM {
		t.Fatalf("%s: last exit %.0f m in, %.0f m left past it: the synthetic runway has no room to turn", rwy, last, room)
	}
	thr, hdg := ctl.end.Threshold, ctl.end.Heading
	pos := offsetHeading(thr, hdg, along)
	if err := ctl.vacateAfterReject(TakeoffPose{Position: pos, Heading: hdg}); err != nil {
		t.Fatalf("%s: stopped %.0f m in past its last exit (%.0f): %v", rwy, along, last, err)
	}
	if ctl.State() != TaxiTaxiing || ctl.mover == nil {
		t.Fatalf("%s: state %v, want taxiing off", rwy, ctl.State())
	}
	prev := ctl.mover.Pose()
	start, total := prev.Position, ctl.mover.Path().Length()
	turned := false
	for i := 0; i < 60*1800 && ctl.State() == TaxiTaxiing; i++ {
		if h := ctl.last.HoldingShortOf; h != "" && h != ctl.runway.Name() {
			ctl.ClearToCross() // a runway on its way back: crossed
		}
		frame()
		if ctl.mover == nil || ctl.State() != TaxiTaxiing {
			break // at the holding point: its own pose from here
		}
		p := ctl.mover.Pose()
		step := calc.HaversineMeters(prev.Position.Lat, prev.Position.Lon, p.Position.Lat, p.Position.Lon)
		if lim := math.Max(prev.GroundSpeedKts, p.GroundSpeedKts)*ktsToMS/60 + 0.05; step > lim {
			t.Fatalf("%s: jumped %.2f m in a frame at %.1f kt (%.0f m along)", rwy, step, p.GroundSpeedKts, p.Distance)
		}
		if d := math.Abs(math.Mod(p.Heading-hdg+540, 360) - 180); d > 170 {
			turned = true // facing back down the runway
		}
		if p.GroundSpeedKts > BacktrackKts+0.5 && turned && onRunwaySurface(ctl.runway, p.Position) {
			t.Errorf("%s: %.1f kt backtracking on the runway, over %.0f", rwy, p.GroundSpeedKts, BacktrackKts)
			break
		}
		prev = p
	}
	if !turned {
		t.Errorf("%s: never faced back down the runway", rwy)
	}
	if ctl.State() != TaxiHoldingShort || ctl.last.HoldingShortOf != ctl.runway.Name() {
		t.Errorf("%s: %v (%s) after %.0f of %.0f m, want holding short of the runway again", rwy, ctl.State(), ctl.last.HoldingShortOf, prev.Distance, total)
	}
	if a := alongHeading(thr, hdg, prev.Position); math.Abs(alongHeading(thr, hdg, start)-along) > 30 || a >= along {
		t.Errorf("%s: ended %.0f m in, not back behind where it stopped (%.0f)", rwy, a, along)
	}
	t.Logf("%s: stopped %.0f m in (last exit %.0f), round and back %.0f m to the holding point", rwy, along, last, total)
}

// rejectRoomM: the test's stop leaves at least this much more than the
// turn itself ahead.
const rejectRoomM = 50.0
