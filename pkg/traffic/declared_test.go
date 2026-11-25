package traffic

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// declaredGraph is LKPR's taxi graph with set applied to the named runway
// ends (synthetic declared distances and uses, #1660, #1661).
func declaredGraph(t *testing.T, set map[string]func(*airport.RunwayEnd)) *airport.Graph {
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
	for i := range l.Runways {
		for _, e := range []*airport.RunwayEnd{&l.Runways[i].Primary, &l.Runways[i].Secondary} {
			if f := set[e.Name]; f != nil {
				f(e)
			}
		}
	}
	g, err := airport.BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// declaredLineUp starts a B738 departure from C22 for end of g and runs it
// to lined up: where it stands, where its nose gear stands (the point its
// path, and so its line-up, is of), its controller and its line-up time.
func declaredLineUp(t *testing.T, g *airport.Graph, end string) (GroundPose, airport.LatLon, *TaxiController, time.Duration) {
	t.Helper()
	i, err := g.Layout.ParkingIndex("C22")
	if err != nil {
		t.Fatal(err)
	}
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj))
	if err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: end, Model: "FSLTL_B738_KLM", Tail: "T1", RollingTakeoffChance: -1, HoldForClearances: true}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	mon := DefaultTaxiRequestBase + reqOffMonitor
	st := g.Layout.Parking[i]
	var last GroundPose
	var nose airport.LatLon
	run := func(until TaxiState, n int) bool {
		for k := 0; k < n && ctl.State() != until && !ctl.State().Terminal(); k++ {
			now = now.Add(time.Second / 60)
			ctl.Handle(positionMsg(mon, 77, st.Position, 0, 0, true))
			if ctl.mover != nil {
				last = ctl.mover.Pose()
				nose = ctl.mover.Path().PointAt(last.Distance)
			}
		}
		return ctl.State() == until
	}
	run(TaxiAwaitingPushback, 60*900)
	ctl.ClearPushback()
	run(TaxiAwaitingTaxi, 60*900)
	ctl.ClearToTaxi()
	if !run(TaxiHoldingShort, 60*1800) {
		t.Fatalf("state %v, want holding short", ctl.State())
	}
	left := ctl.LineUpLeft()
	ctl.ClearToLineUp()
	if !run(TaxiLinedUp, 60*600) {
		t.Fatalf("state %v, want lined up", ctl.State())
	}
	return last, nose, ctl, left
}

// TestDeclaredTakeoffStart: a departure from an end whose take-off start is
// declared further in than its full-length entry (#1660) lines up past
// that start, where without one it lines up nearer the pavement end; its
// take-off run is figured from there to the end of its TORA.
func TestDeclaredTakeoffStart(t *testing.T) {
	const end, start, tora = "24", 450.0, 2600.0
	SetRandomSeed(1)
	p0, _, c0, left0 := declaredLineUp(t, lkprGraph(t), end)
	g := declaredGraph(t, map[string]func(*airport.RunwayEnd){end: func(e *airport.RunwayEnd) { e.TakeoffStartM, e.TORAM = start, tora }})
	SetRandomSeed(1)
	p, nose, c, left := declaredLineUp(t, g, end)
	_, e, _ := g.Layout.RunwayEnd(end)
	a0, a := alongHeading(e.Threshold, e.Heading, p0.Position), alongHeading(e.Threshold, e.Heading, p.Position)
	t.Logf("lined up %.0f m in (%.0f without the start offset); planned run %.0f m (%.0f)", a, a0, c.toConfig.AvailableM, c0.toConfig.AvailableM)
	if a0 >= start {
		t.Fatalf("without the offset lined up %.0f m in, not before %.0f: the test shows nothing", a0, start)
	}
	if a < start {
		t.Errorf("lined up %.0f m in, before the declared take-off start at %.0f", a, start)
	}
	// Aligned past the start, not held where the line-up began on the
	// runway: its nose gear, which the line-up aligns, LineUpAlignMeters
	// past it; the time it takes counts the way in to the start.
	if n := alongHeading(e.Threshold, e.Heading, nose); n < start+LineUpAlignMeters-5 {
		t.Errorf("nose gear lined up %.0f m in, short of aligned past the start (%.0f)", n, start+LineUpAlignMeters)
	}
	shift := time.Duration((start - a0) / (LineUpSpeedKts * knot) * float64(time.Second))
	if left-left0 < shift/2 {
		t.Errorf("line-up time %v with the start offset, %v without: not the %v more along the runway", left, left0, shift)
	}
	if got := c.toConfig.AvailableM; got <= 0 || got > tora+1 {
		t.Errorf("planned take-off run %.0f m, want at most the TORA %.0f", got, tora)
	}
	if c0.toConfig.AvailableM <= tora {
		t.Errorf("without TORA the planned run %.0f m is not the runway's", c0.toConfig.AvailableM)
	}
}

// TestLineUpWaypointsTakeoffStart: MSFS AI's line-up chain aligns past a
// declared take-off start (#1660); with none declared it is the same, byte
// for byte.
func TestLineUpWaypointsTakeoffStart(t *testing.T) {
	const end, start = "24", 450.0
	plain := lkprGraph(t)
	zero := declaredGraph(t, nil)
	set := declaredGraph(t, map[string]func(*airport.RunwayEnd){end: func(e *airport.RunwayEnd) { e.TakeoffStartM = start }})
	stand, err := plain.Layout.ParkingIndex("C22")
	if err != nil {
		t.Fatal(err)
	}
	chain := func(g *airport.Graph) []types.SIMCONNECT_DATA_WAYPOINT {
		r, err := g.RouteToRunway(stand, end, airport.RouteOptions{})
		if err != nil {
			t.Fatal(err)
		}
		wps, err := LineUpWaypoints(g, r)
		if err != nil {
			t.Fatal(err)
		}
		return wps
	}
	a, b := chain(plain), chain(zero)
	if fmt.Sprint(a) != fmt.Sprint(b) {
		t.Errorf("none declared: chain changed: %v, was %v", b, a)
	}
	_, e, _ := set.Layout.RunwayEnd(end)
	got := chain(set)
	align := alongHeading(e.Threshold, e.Heading, airport.LatLon{Lat: got[1].Latitude, Lon: got[1].Longitude})
	was := alongHeading(e.Threshold, e.Heading, airport.LatLon{Lat: a[1].Latitude, Lon: a[1].Longitude})
	t.Logf("aligned %.0f m in (%.0f without the start)", align, was)
	if was >= start+LineUpAlignMeters {
		t.Fatalf("without the start aligned %.0f m in already: the test shows nothing", was)
	}
	if align < start+LineUpAlignMeters-1 {
		t.Errorf("aligned %.0f m in, before the start %.0f and its alignment", align, start)
	}
}

// TestRunwayEndUseRefused: no departure from an end used for landings only,
// no arrival onto one used for take-offs only, no circuit joined to it, and
// no runway change to either (#1661).
func TestRunwayEndUseRefused(t *testing.T) {
	g := declaredGraph(t, map[string]func(*airport.RunwayEnd){
		"06": func(e *airport.RunwayEnd) { e.Use = airport.RunwayUseLanding },
		"30": func(e *airport.RunwayEnd) { e.Use = airport.RunwayUseTakeoff },
		"12": func(e *airport.RunwayEnd) { e.Use = airport.RunwayUseNone },
	})
	c22, _ := g.Layout.ParkingIndex("C22")
	ec := &eventClient{}
	for _, rwy := range []string{"06", "12"} {
		ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
		err := ctl.Start(TaxiRequest{Graph: g, Parking: c22, Runway: rwy, Model: "FSLTL_B738_KLM", Tail: "T1"})
		if !errors.Is(err, airport.ErrRunwayEndUse) {
			t.Errorf("departure from %s: %v, want ErrRunwayEndUse", rwy, err)
		}
	}
	for _, rwy := range []string{"30", "12"} {
		ctl := NewArrivalController(NewFleet(ec))
		err := ctl.Start(ArrivalRequest{Graph: g, Runway: rwy, Parking: c22, Model: "A320", Tail: "T2"})
		if !errors.Is(err, airport.ErrRunwayEndUse) {
			t.Errorf("arrival onto %s: %v, want ErrRunwayEndUse", rwy, err)
		}
		if _, _, err := CircuitJoinFor(g.Layout, rwy, CircuitConfig{}, g.Layout.Parking[c22].Position); !errors.Is(err, airport.ErrRunwayEndUse) {
			t.Errorf("circuit to %s: %v, want ErrRunwayEndUse", rwy, err)
		}
	}
	dep := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
	if err := dep.Start(TaxiRequest{Graph: g, Parking: c22, Runway: "24", Model: "FSLTL_B738_KLM", Tail: "T3"}); err != nil {
		t.Fatalf("departure from 24: %v", err)
	}
	if err := dep.ChangeRunway("06", "", nil); !errors.Is(err, airport.ErrRunwayEndUse) {
		t.Errorf("change to 06: %v, want ErrRunwayEndUse", err)
	}
	if err := dep.ChangeRunway("30", "", nil); errors.Is(err, airport.ErrRunwayEndUse) {
		t.Errorf("change to 30 (take-offs only): %v", err)
	}
}

// TestDeclaredZeroEnds: ends with every declared value 0 give the runway's
// own distances, so the take-off run and landing distance are as before.
func TestDeclaredZeroEnds(t *testing.T) {
	g := lkprGraph(t)
	for _, r := range g.Layout.Runways {
		for _, e := range []airport.RunwayEnd{r.Primary, r.Secondary} {
			if !e.ForTakeoff() || !e.ForLanding() {
				t.Errorf("%s: not for both", e.Name)
			}
			if got := e.TORA(r.Length); got != r.Length {
				t.Errorf("%s: TORA %.1f, length %.1f", e.Name, got, r.Length)
			}
			if got := e.LandingDistance(r.Length); got != r.Length-e.DisplacedM {
				t.Errorf("%s: LDA %.1f, want %.1f", e.Name, got, r.Length-e.DisplacedM)
			}
			if got := e.TakeoffRunAhead(321, r.Length); got != r.Length-321 {
				t.Errorf("%s: run ahead of 321 m %.1f", e.Name, got)
			}
		}
	}
}
