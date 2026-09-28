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
	req.Graph, req.Parking, req.Runway, req.Model, req.Tail = g, c22, "24", "FSLTL A320 Air France SL", "CSA8"
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
	ctl, ec, run, _ := injectedDeparture(t, TaxiRequest{})
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
