//go:build windows
// +build windows

package traffic

import (
	"math"
	"slices"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// injectedDeparture starts an injected departure from C22 to runway 24 and
// returns a function that runs sim frames until a state (or a frame limit).
func injectedDeparture(t *testing.T, req TaxiRequest) (*TaxiController, *eventClient, func(until TaxiState, maxFrames int) bool, *time.Time) {
	t.Helper()
	g := lkprGraph(t)
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj))
	c22, _ := g.Layout.ParkingIndex("C22")
	if req.Runway == "" {
		req.Runway = "24"
	}
	req.Graph, req.Parking, req.Model, req.Tail = g, c22, "FSLTL A320 Air France SL", "CSA8"
	if err := ctl.Start(req); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	mon := DefaultTaxiRequestBase + reqOffMonitor
	stand := g.Layout.Parking[c22]
	run := func(until TaxiState, maxFrames int) bool {
		for i := 0; i < maxFrames && ctl.State() != until && !ctl.State().Terminal(); i++ {
			now = now.Add(time.Second / 60)
			ctl.Handle(positionMsg(mon, 77, stand.Position, 0, 0, true)) // a frame tick
		}
		return ctl.State() == until
	}
	return ctl, ec, run, &now
}

func placements(ec *eventClient) []types.SIMCONNECT_DATA_INITPOSITION {
	var out []types.SIMCONNECT_DATA_INITPOSITION
	for _, b := range ec.waypoints {
		if len(b) == int(unsafe.Sizeof(types.SIMCONNECT_DATA_INITPOSITION{})) {
			var q types.SIMCONNECT_DATA_INITPOSITION
			copy(unsafe.Slice((*byte)(unsafe.Pointer(&q)), len(b)), b)
			out = append(out, q)
		}
	}
	return out
}

func TestTaxiControllerInjectedDeparture(t *testing.T) {
	ctl, ec, run, _ := injectedDeparture(t, TaxiRequest{RollingTakeoffChance: -1})
	var states []TaxiState
	done := make(chan struct{})
	go func() {
		for ev := range ctl.Events() {
			if len(states) == 0 || states[len(states)-1] != ev.State {
				states = append(states, ev.State)
			}
		}
		close(done)
	}()
	if !run(TaxiAwaitingTaxi, 60*120) {
		t.Fatalf("state %v, want awaiting taxi after the push", ctl.State())
	}
	pushed := ctl.mover.Pose()
	route := ctl.Route()
	taxiDir := localBearing(route.Points[1], route.Points[2])
	if hd := math.Abs(headingDiff(pushed.Heading, taxiDir)); hd > 30 {
		t.Errorf("after the push facing %.0f°, taxi direction %.0f°", pushed.Heading, taxiDir)
	}
	if !run(TaxiLinedUp, 60*900) {
		t.Fatalf("state %v, want lined up", ctl.State())
	}
	if hd := math.Abs(headingDiff(ctl.mover.Pose().Heading, ctl.end.Heading)); hd > 3 {
		t.Errorf("lined up %.1f° off the runway heading", hd)
	}
	if !run(TaxiComplete, 60*300) {
		t.Fatalf("state %v, want complete", ctl.State())
	}
	<-done
	want := []TaxiState{TaxiSpawning, TaxiAwaitingPushback, TaxiPushback, TaxiAwaitingTaxi, TaxiTaxiing, TaxiHoldingShort, TaxiLiningUp, TaxiLinedUp, TaxiDeparting, TaxiComplete}
	if !slices.Equal(states, want) {
		t.Errorf("states %v, want %v", states, want)
	}
	all := placements(ec)
	maxStep := 0.0
	for i := 1; i < len(all); i++ {
		maxStep = math.Max(maxStep, calc.HaversineMeters(all[i-1].Latitude, all[i-1].Longitude, all[i].Latitude, all[i].Longitude))
	}
	if maxStep > 1.5 {
		t.Errorf("largest move between frames %.2f m", maxStep)
	}
	ev := strings.Join(ec.events, " ")
	order := []string{"BEACON_LIGHTS_SET=1", "TAXI_LIGHTS_SET=1", "STROBES_SET=1", "LANDING_LIGHTS_SET=1", "TAXI_LIGHTS_SET=0"}
	at := 0
	for _, o := range order {
		i := strings.Index(ev[at:], o)
		if i < 0 {
			t.Fatalf("lights %v: %s missing or out of order", ec.events, o)
		}
		at += i
	}
	if !strings.Contains(strings.Join(ec.events, " "), "FREEZE_LATITUDE_LONGITUDE_SET=0") {
		t.Error("not released to MSFS AI for the climb-out")
	}
	t.Logf("%d placements, largest step %.2f m; states %v", len(all), maxStep, states)
}

func TestTaxiControllerInjectedGates(t *testing.T) {
	ctl, _, run, now := injectedDeparture(t, TaxiRequest{HoldForClearances: true})
	// Each gate holds until its clearance.
	for _, gate := range []struct {
		state TaxiState
		clear func()
	}{
		{TaxiAwaitingPushback, ctl.ClearPushback},
		{TaxiAwaitingTaxi, ctl.ClearToTaxi},
		{TaxiHoldingShort, ctl.ClearToLineUp},
		{TaxiLinedUp, func() { ctl.ClearForTakeoff() }},
	} {
		if !run(gate.state, 60*900) {
			t.Fatalf("state %v, want %v", ctl.State(), gate.state)
		}
		*now = now.Add(time.Minute)
		if run(TaxiComplete, 60*30) || ctl.State() != gate.state {
			t.Fatalf("left %v without a clearance (now %v)", gate.state, ctl.State())
		}
		gate.clear()
	}
	if !run(TaxiComplete, 60*300) {
		t.Fatalf("state %v, want complete", ctl.State())
	}
}

var _ = airport.LatLon{}

// TestTaxiControllerInjectedRollingTakeoff: line-up and take-off cleared
// together, the aircraft rolls into the take-off without stopping on the
// runway; take-off flaps are set while taxiing and retracted before the
// hand-over to MSFS AI.
func TestTaxiControllerInjectedRollingTakeoff(t *testing.T) {
	ctl, ec, run, _ := injectedDeparture(t, TaxiRequest{RollingTakeoffChance: 1})
	var states []TaxiState
	done := make(chan struct{})
	go func() {
		for ev := range ctl.Events() {
			if len(states) == 0 || states[len(states)-1] != ev.State {
				states = append(states, ev.State)
			}
		}
		close(done)
	}()
	if !run(TaxiComplete, 60*1500) {
		t.Fatalf("state %v, want complete", ctl.State())
	}
	<-done
	if slices.Contains(states, TaxiLinedUp) {
		t.Errorf("stopped lined up on a rolling take-off: %v", states)
	}
	flapsMax, flapsLast := 0.0, -1.0
	for _, b := range ec.waypoints {
		if len(b) == 32 { // four flap surfaces
			v := *(*float64)(unsafe.Pointer(&b[0]))
			flapsMax, flapsLast = math.Max(flapsMax, v), v
		}
	}
	if flapsMax != TakeoffFlapsPct || flapsLast != 0 {
		t.Errorf("flaps up to %.0f%%, last %.0f%%; want %.0f then retracted", flapsMax, flapsLast, TakeoffFlapsPct)
	}
	t.Logf("states %v", states)
}

// TestTaxiControllerInjectedDepartureSweep runs injected departures from a
// sample of LKPR stands to every runway end: each must reach the climb-out
// without failing or jumping.
func TestTaxiControllerInjectedDepartureSweep(t *testing.T) {
	g := lkprGraph(t)
	ok, total := 0, 0
	for _, p := range g.Layout.Parking {
		if p.Index%9 != 0 || p.Radius < 15 {
			continue
		}
		for _, end := range []string{"06", "24", "12", "30"} {
			if _, err := g.RouteToRunway(p.Index, end, airport.RouteOptions{}); err != nil {
				continue
			}
			total++
			ec := &eventClient{}
			inj := NewInjector(ec)
			ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj))
			if err := ctl.Start(TaxiRequest{Graph: g, Parking: p.Index, Runway: end, Model: "A320", RollingTakeoffChance: -1}); err != nil {
				t.Errorf("%s → %s: start: %v", p.Label(), end, err)
				continue
			}
			now := time.Now()
			ctl.now = func() time.Time { return now }
			ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
			inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
			errs := make(chan error, 1)
			go func() {
				var last error
				for ev := range ctl.Events() {
					if ev.Err != nil {
						last = ev.Err
					}
				}
				errs <- last
			}()
			checked := map[TaxiState]bool{}
			for i := 0; i < 60*3600 && !ctl.State().Terminal(); i++ {
				now = now.Add(time.Second / 60)
				ctl.Handle(positionMsg(DefaultTaxiRequestBase+reqOffMonitor, 77, p.Position, 0, 0, true))
				switch s := ctl.State(); {
				case s == TaxiLinedUp && !checked[s]:
					checked[s] = true
					if hd := math.Abs(headingDiff(ctl.mover.Pose().Heading, ctl.end.Heading)); hd > 3 {
						t.Errorf("%s → %s: lined up %.1f° off the runway", p.Label(), end, hd)
					}
				case s == TaxiTaxiing && !checked[s]:
					checked[s] = true
					path := ctl.mover.Path().Points()
					start := ctl.mover.Pose()
					ahead := path[min(len(path)-1, len(path)/20+1)]
					if d := math.Abs(headingDiff(start.Heading, localBearing(NoseGear(start.Position, start.Heading, DefaultMotionProfile()), ahead))); d > 120 { // a 90° turn off the junction is normal
						t.Errorf("%s → %s: after the push facing %.0f° off the way to taxi", p.Label(), end, d)
					}
				}
			}
			if ctl.State() != TaxiComplete {
				var err error
				if ctl.State().Terminal() {
					err = <-errs
				}
				t.Errorf("%s → %s: ended %v (%v)", p.Label(), end, ctl.State(), err)
				continue
			}
			all := placements(ec)
			for i := 1; i < len(all); i++ {
				if d := calc.HaversineMeters(all[i-1].Latitude, all[i-1].Longitude, all[i].Latitude, all[i].Longitude); d > 1.5 {
					t.Errorf("%s → %s: jumped %.2f m at placement %d/%d", p.Label(), end, d, i, len(all))
					break
				}
			}
			ok++
		}
	}
	t.Logf("%d of %d departures complete", ok, total)
}

// TestTaxiControllerInjectedFaceOutStand: from a self-manoeuvring stand the
// departure starts without a pushback and taxis straight out.
func TestTaxiControllerInjectedFaceOutStand(t *testing.T) {
	g := lkprGraph(t)
	var stand airport.Parking
	for _, p := range g.Layout.Parking {
		if standFacesOut(g, p.Index) && p.Radius >= 15 {
			if r, err := g.RouteToRunway(p.Index, "24", airport.RouteOptions{}); err == nil && leadInAhead(g, p.Index, r.Points[1]) {
				stand = p
				break
			}
		}
	}
	if stand.Radius == 0 {
		t.Skip("no face-out stand routable to 24")
	}
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj))
	if err := ctl.Start(TaxiRequest{Graph: g, Parking: stand.Index, Runway: "24", Model: "A320", RollingTakeoffChance: -1}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	var states []TaxiState
	done := make(chan struct{})
	go func() {
		for ev := range ctl.Events() {
			if len(states) == 0 || states[len(states)-1] != ev.State {
				states = append(states, ev.State)
			}
		}
		close(done)
	}()
	for i := 0; i < 60*3600 && !ctl.State().Terminal(); i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(positionMsg(DefaultTaxiRequestBase+reqOffMonitor, 77, stand.Position, 0, 0, true))
	}
	<-done
	if ctl.State() != TaxiComplete || slices.Contains(states, TaxiPushback) {
		t.Fatalf("%s: states %v, want complete without a pushback", stand.Label(), states)
	}
	all := placements(ec)
	if d := calc.HaversineMeters(all[0].Latitude, all[0].Longitude, stand.Position.Lat, stand.Position.Lon); d > 1 || math.Abs(headingDiff(all[0].Heading, stand.Heading)) > 2 {
		t.Errorf("first placement %.1f m from the stand, heading %.0f (stand %.0f)", d, all[0].Heading, stand.Heading)
	}
	t.Logf("%s: %v", stand.Label(), states)
}

// standFacesOut reports whether any lead-in of a stand lies ahead of it
// (candidates for turn-around arrivals and push-less departures).
func standFacesOut(g *airport.Graph, parking int) bool {
	n, ok := g.ParkingNode(parking)
	if !ok {
		return false
	}
	for _, e := range g.Adj[n] {
		if leadInAhead(g, parking, g.Nodes[e.To].Position) {
			return true
		}
	}
	return false
}
