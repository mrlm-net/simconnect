//go:build windows
// +build windows

package traffic

import (
	"errors"
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
func injectedDeparture(t *testing.T, req TaxiRequest, opts ...TaxiOption) (*TaxiController, *eventClient, func(until TaxiState, maxFrames int) bool, *time.Time) {
	t.Helper()
	g := lkprGraph(t)
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewTaxiController(NewFleet(ec), append([]TaxiOption{TaxiWithInjector(inj)}, opts...)...)
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
	// (The push takes this aircraft's random draw of speeds and pauses,
	// #343: up to about three minutes.)
	if !run(TaxiAwaitingTaxi, 60*240) {
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
	at := StandPoint(stand, 0)
	if d := calc.HaversineMeters(all[0].Latitude, all[0].Longitude, at.Lat, at.Lon); d > 1 || math.Abs(headingDiff(all[0].Heading, stand.Heading)) > 2 {
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

// TestTaxiControllerProgressiveTaxi: ClearUpTo stops the aircraft with its
// nose gear on the chosen route node; a further ClearUpTo moves it on,
// ClearToTaxi removes the limit; a node behind is refused (#322).
func TestTaxiControllerProgressiveTaxi(t *testing.T) {
	ctl, _, run, now := injectedDeparture(t, TaxiRequest{HoldForClearances: true})
	if !run(TaxiAwaitingPushback, 60*60) {
		t.Fatal(ctl.State())
	}
	ctl.ClearPushback()
	if !run(TaxiAwaitingTaxi, 60*600) {
		t.Fatal(ctl.State())
	}
	route := ctl.Route()
	first, second := route.Nodes[len(route.Nodes)/3], route.Nodes[2*len(route.Nodes)/3]
	if err := ctl.ClearUpTo(first); err != nil {
		t.Fatal(err)
	}
	holdAt := func(node airport.NodeID) {
		t.Helper()
		for i := 0; i < 60*600 && !(ctl.last.AtLimit && ctl.last.LimitNode == node); i++ {
			run(TaxiComplete, 1)
		}
		if !ctl.last.AtLimit || ctl.last.LimitNode != node {
			t.Fatalf("not holding at node %d: %+v", node, ctl.last)
		}
		pose := ctl.mover.Pose()
		nose := NoseGear(pose.Position, pose.Heading, DefaultMotionProfile())
		want := ctl.req.Graph.Nodes[node].Position
		if d := calc.HaversineMeters(nose.Lat, nose.Lon, want.Lat, want.Lon); d > 3 {
			t.Errorf("nose gear %.1f m from the clearance limit", d)
		}
		*now = now.Add(time.Minute)
		run(TaxiComplete, 60*20)
		if !ctl.last.AtLimit || ctl.mover.Pose().Distance != pose.Distance {
			t.Fatal("moved past the clearance limit")
		}
	}
	holdAt(first)
	if err := ctl.ClearUpTo(route.Nodes[1]); err == nil {
		t.Error("a node behind the aircraft was accepted")
	}
	if err := ctl.ClearUpTo(second); err != nil {
		t.Fatal(err)
	}
	holdAt(second)
	ctl.ClearToTaxi()
	if !run(TaxiHoldingShort, 60*900) || ctl.last.HoldingShortOf != ctl.runway.Name() {
		t.Fatalf("state %v (%s), want holding short of the runway", ctl.State(), ctl.last.HoldingShortOf)
	}
}

// TestTaxiControllerLimitPassedDuringPush: a clearance limit given during
// the pushback that is behind the aircraft when the taxi starts is reported with ErrNotOnRoute when
// the taxi starts, and the aircraft holds there instead of taxiing on
// without a limit; ClearToTaxi releases it (#337).
func TestTaxiControllerLimitPassedDuringPush(t *testing.T) {
	ctl, _, run, now := injectedDeparture(t, TaxiRequest{HoldForClearances: true})
	if !run(TaxiAwaitingPushback, 60*60) {
		t.Fatal(ctl.State())
	}
	ctl.ClearPushback()
	if !run(TaxiPushback, 60*60) {
		t.Fatal(ctl.State())
	}
	// A limit the aircraft is past when the taxi starts: the stand's own
	// node (ClearUpTo refuses it up front; a limit given before the push
	// can end up behind the same way on stands where the push passes
	// route nodes).
	passed := ctl.Route().Nodes[0]
	ctl.mu.Lock()
	ctl.pendingLimit, ctl.hasPendingLimit, ctl.taxiCleared = passed, true, true
	ctl.mu.Unlock()
	var got error
	for i := 0; i < 60*900 && got == nil; i++ {
		run(TaxiComplete, 1)
		for len(ctl.Events()) > 0 {
			if ev := <-ctl.Events(); ev.Err != nil {
				got = ev.Err
			}
		}
	}
	if !errors.Is(got, ErrNotOnRoute) {
		t.Fatalf("no ErrNotOnRoute event (state %v, err %v)", ctl.State(), got)
	}
	at := ctl.mover.Pose().Distance
	*now = now.Add(time.Minute)
	run(TaxiComplete, 60*30)
	if moved := ctl.mover.Pose().Distance - at; moved > 1 {
		t.Fatalf("taxied %.0f m without a clearance limit", moved)
	}
	ctl.ClearToTaxi()
	if !run(TaxiHoldingShort, 60*900) {
		t.Fatalf("state %v, want holding short after ClearToTaxi", ctl.State())
	}
}

// TestTaxiControllerAdopts: with TaxiRequest.ObjectID the controller spawns
// nothing and departs the aircraft already on the stand: pushback, taxi,
// hold short (#293).
func TestTaxiControllerAdopts(t *testing.T) {
	g := lkprGraph(t)
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj))
	c22, _ := g.Layout.ParkingIndex("C22")
	now := time.Now()
	ctl.now = func() time.Time { return now }
	if err := ctl.Start(TaxiRequest{Graph: g, Parking: c22, Runway: "24", Model: "FSLTL A320 Air France SL", Tail: "CSA9", ObjectID: 88}); err != nil {
		t.Fatal(err)
	}
	if len(ec.spawned) != 0 || ctl.ObjectID() != 88 || ctl.State() != TaxiAwaitingPushback {
		t.Fatalf("spawned %d, object %d, state %v", len(ec.spawned), ctl.ObjectID(), ctl.State())
	}
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 88, 1200, 12))
	stand := g.Layout.Parking[c22]
	for i := 0; i < 60*1200 && ctl.State() != TaxiHoldingShort && !ctl.State().Terminal(); i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(positionMsg(DefaultTaxiRequestBase+reqOffMonitor, 88, stand.Position, 0, 0, true))
	}
	if ctl.State() != TaxiHoldingShort {
		t.Fatalf("state %v, want holding short", ctl.State())
	}
}

// TestPushbackFitsStands: at every LKPR stand with a pushback the push
// starts straight along the stand axis, turns no tighter than
// PushbackMinArcMeters and ends facing along the taxiway; the swing
// reaching into a neighbouring stand is logged.
func TestPushbackFitsStands(t *testing.T) {
	g := lkprGraph(t)
	prof := DefaultMotionProfile()
	n := 0
	for _, p := range g.Layout.Parking {
		if p.Radius < 15 || standFacesOut(g, p.Index) {
			continue
		}
		if _, err := g.RouteToRunway(p.Index, "24", airport.RouteOptions{}); err != nil {
			continue
		}
		ec := &eventClient{}
		ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
		if err := ctl.Start(TaxiRequest{Graph: g, Parking: p.Index, Runway: "24", Model: "A320"}); err != nil {
			t.Fatalf("%s: %v", p.Label(), err)
		}
		if err := ctl.startPushback(); err != nil {
			t.Errorf("%s: pushback: %v", p.Label(), err)
			continue
		}
		n++
		pts := ctl.mover.Path().Points()
		start := ctl.mover.Pose()
		if d := math.Abs(headingDiff(start.Heading, p.Heading)); d > 1 {
			t.Errorf("%s: push starts %.1f° off the stand heading", p.Label(), d)
		}
		// Tightest radius from the heading change over 4 m of path.
		straight, minR, s := -1.0, math.Inf(1), 0.0
		for i := 1; i+1 < len(pts); i++ {
			s += localDist(pts[i-1], pts[i])
			j := i
			for j+1 < len(pts) && localDist(pts[i], pts[j]) < 4 {
				j++
			}
			turn := math.Abs(headingDiff(localBearing(pts[i-1], pts[i]), localBearing(pts[j-1], pts[j])))
			if turn > 1 {
				if straight < 0 {
					straight = s
				}
				minR = math.Min(minR, localDist(pts[i], pts[j])/(turn*math.Pi/180))
			}
		}
		base := standIntrusion(g, p.Index, []airport.LatLon{offsetHeading(pts[0], p.Heading, 1), pts[0]}, prof)
		in := standIntrusion(g, p.Index, pts, prof)
		t.Logf("%-4s straight %5.1f m, tightest %5.1f m, length %5.1f m, neighbour intrusion %+.1f m (parked %+.1f)", p.Label(), straight, minR, ctl.mover.Path().Length(), in, base)
		if minR < PushbackMinArcMeters-3 {
			t.Errorf("%s: pushback turns on %.1f m", p.Label(), minR)
		}
	}
	if n == 0 {
		t.Fatal("no pushback stands")
	}
}

// TestTaxiControllerCancelAfterComplete: Cancel still removes the aircraft
// once the departure is complete (handed to MSFS AI), and only once.
func TestTaxiControllerCancelAfterComplete(t *testing.T) {
	ctl, ec, run, _ := injectedDeparture(t, TaxiRequest{RollingTakeoffChance: -1})
	go func() {
		for range ctl.Events() {
		}
	}()
	if !run(TaxiComplete, 60*3600) {
		t.Fatalf("ended %v", ctl.State())
	}
	for i := 0; i < 2; i++ {
		if err := ctl.Cancel(); err != nil {
			t.Fatal(err)
		}
	}
	if len(ec.removed) != 1 || ec.removed[0] != 77 || ctl.State() != TaxiComplete {
		t.Errorf("removed=%v state=%v", ec.removed, ctl.State())
	}
}

// TestPushbackFacesRoute: after the pushback the aircraft never faces away
// from its taxi-out — at every LKPR pushback stand, the way on from the nose
// is at most a turn onto the taxiway (LKPR C17 used to end facing opposite:
// the tail went onto the branch the route continued on).
func TestPushbackFacesRoute(t *testing.T) {
	g := lkprGraph(t)
	n := 0
	for _, p := range g.Layout.Parking {
		if p.Radius < 15 || standFacesOut(g, p.Index) {
			continue
		}
		if _, err := g.RouteToRunway(p.Index, "24", airport.RouteOptions{}); err != nil {
			continue
		}
		ec := &eventClient{}
		inj := NewInjector(ec)
		ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj))
		if err := ctl.Start(TaxiRequest{Graph: g, Parking: p.Index, Runway: "24", Model: "A320", RollingTakeoffChance: -1}); err != nil {
			t.Fatal(err)
		}
		now := time.Now()
		ctl.now = func() time.Time { return now }
		ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
		go func() {
			for range ctl.Events() {
			}
		}()
		for i := 0; i < 60*1200 && ctl.State() != TaxiTaxiing && !ctl.State().Terminal(); i++ {
			now = now.Add(time.Second / 60)
			ctl.Handle(positionMsg(DefaultTaxiRequestBase+reqOffMonitor, 77, p.Position, 0, 0, true))
		}
		if ctl.State() != TaxiTaxiing {
			t.Errorf("%s: %v", p.Label(), ctl.State())
			continue
		}
		n++
		pose := ctl.mover.Pose()
		nose := NoseGear(pose.Position, pose.Heading, DefaultMotionProfile())
		ahead := ctl.mover.Path().PointAt(math.Min(25, ctl.mover.Path().Length()))
		if d := math.Abs(headingDiff(pose.Heading, localBearing(nose, ahead))); d > 110 {
			if knownBesideJunction[p.Label()] {
				t.Logf("%s: after the push the taxi-out lies %.0f° off the nose (known, #341)", p.Label(), d)
				continue
			}
			t.Errorf("%s: after the push the taxi-out lies %.0f° off the nose", p.Label(), d)
		}
	}
	if n < 20 {
		t.Fatalf("only %d stands checked", n)
	}
}

// knownBesideJunction are LKPR stands whose junction lies beside the stand:
// the push cannot yet leave them facing the taxi-out (#341).
var knownBesideJunction = map[string]bool{}

// TestDepartureRoutesBySize: the departure routes for its aircraft — a 777
// from LKPR B14 keeps off the code C taxilanes JO and JB and leaves by J;
// an A320 from C17 is pushed onto JB's side and leaves by JB, the nearest.
func TestDepartureRoutesBySize(t *testing.T) {
	g := lkprGraph(t)
	for _, c := range []struct {
		stand, model string
		want, not    string
	}{
		{"B14", "FSLTL B77W Emirates", "J", "JO"},
		{"C17", "FSLTL A320 Air France SL", "JB", ""},
	} {
		pi, _ := g.Layout.ParkingIndex(c.stand)
		ec := &eventClient{}
		ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
		if err := ctl.Start(TaxiRequest{Graph: g, Parking: pi, Runway: "24", Model: c.model, Profile: MotionProfileFor(c.model)}); err != nil {
			t.Fatal(err)
		}
		tw := ctl.Route().Taxiways
		if !slices.Contains(tw, c.want) || (c.not != "" && (slices.Contains(tw, c.not) || slices.Contains(tw, "JB"))) {
			t.Errorf("%s %s via %v, want %s", c.model, c.stand, tw, c.want)
		}
		t.Logf("%s from %s via %v (tight %v)", c.model, c.stand, tw, ctl.Route().Tight)
	}
}

// TestPushbackFitsAircraft: a 777 at LKPR B14 is not pushed onto JO (code C)
// but on straight back to J, and leaves along it.
func TestPushbackFitsAircraft(t *testing.T) {
	g := lkprGraph(t)
	pi, _ := g.Layout.ParkingIndex("B14")
	model := "Asobo PassiveAircraft B777-300ER"
	ec := &eventClient{}
	ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
	if err := ctl.Start(TaxiRequest{Graph: g, Parking: pi, Runway: "24", Model: model, Profile: MotionProfileFor(model)}); err != nil {
		t.Fatal(err)
	}
	for _, e := range ctl.route.Edges[:ctl.pushJunction+1] {
		if e.Name == "JO" || e.Name == "JB" {
			t.Fatalf("pushed along %s", e.Name)
		}
	}
	if ctl.pushTurn || !ctl.havePushBranch {
		t.Fatalf("push plan: turn %v branch %v", ctl.pushTurn, ctl.havePushBranch)
	}
	if err := ctl.startPushback(); err != nil {
		t.Fatal(err)
	}
	pose := ctl.mover.Pose()
	for !pose.Arrived {
		pose = ctl.mover.Step(0.5)
	}
	nose := NoseGear(pose.Position, pose.Heading, MotionProfileFor(model))
	ahead := ctl.route.Points[min(len(ctl.route.Points)-1, ctl.pushJunction+3)]
	if d := math.Abs(headingDiff(pose.Heading, localBearing(nose, ahead))); d > 45 {
		t.Errorf("after the push the route lies %.0f° off the nose", d)
	}
}

// Handed to MSFS AI after the injected climb, a departure's SID still to
// fly is its ClimbRoute from where it is: the whole chain from the runway's
// end, less as it flies on; nothing before the hand-over.
func TestClimbRoute(t *testing.T) {
	g := lkprGraph(t)
	rwy, end, _ := g.Layout.RunwayEnd("24")
	far := rwy.Primary.Threshold
	if end.Name == rwy.Primary.Name {
		far = rwy.Secondary.Threshold
	}
	sid, err := lkprProcedures(t).ResolveSID("VOZ4A", "24", "", far, g.Layout.Altitude)
	if err != nil {
		t.Fatal(err)
	}
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{Departure: sid})
	if ctl.ClimbRoute(far) != nil {
		t.Fatal("a climb route on the stand")
	}
	if !run(TaxiComplete, 60*1500) {
		t.Fatalf("state %v", ctl.State())
	}
	all := ctl.ClimbRoute(far)
	if len(all) < 3 {
		t.Fatalf("after the hand-over: %d points", len(all))
	}
	if later := ctl.ClimbRoute(all[len(all)/2]); len(later) == 0 || len(later) >= len(all) {
		t.Errorf("halfway: %d of %d points still to fly", len(later), len(all))
	}
}
