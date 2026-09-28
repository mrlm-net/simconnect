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
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

const aglFlag = uint32(types.SIMCONNECT_WAYPOINT_ALTITUDE_IS_AGL)

func TestRequiredRollout(t *testing.T) {
	hs, std := RequiredRollout(ExitHighSpeedKts), RequiredRollout(ExitSpeedKts)
	if hs < 1400 || hs > 1900 || std <= hs {
		t.Errorf("RequiredRollout: high-speed %.0f m, standard %.0f m", hs, std)
	}
}

func TestPlanArrivalC22On24(t *testing.T) {
	g := lkprGraph(t)
	c22, _ := g.Layout.ParkingIndex("C22")
	p, err := PlanArrival(g, "24", c22, ArrivalOptions{SpawnNm: 5})
	if err != nil {
		t.Fatal(err)
	}
	if p.Exit.Along < RequiredRollout(ExitSpeed(p.Exit)) {
		t.Errorf("exit %s at %.0f m is before the required rollout", p.Exit.Taxiway, p.Exit.Along)
	}
	thr := p.End.Threshold
	if d := calc.HaversineMeters(thr.Lat, thr.Lon, p.Spawn.Latitude, p.Spawn.Longitude); math.Abs(d-5*1852) > 5*1852*0.005 { // WGS84 vs sphere
		t.Errorf("spawn %.0f m from the threshold, want 5 nm", d)
	}
	if p.Spawn.Heading != p.End.Heading || p.Spawn.OnGround != 0 {
		t.Errorf("spawn %+v", p.Spawn)
	}

	wps := p.Waypoints
	firstGround := -1
	for i, w := range wps {
		if w.Flags&onGround != 0 {
			firstGround = i
			break
		}
	}
	if firstGround != 5 { // 4, 3, 2, 1 nm and the threshold
		t.Fatalf("first ground waypoint at %d, want 5", firstGround)
	}
	if w := wps[firstGround-1]; w.Altitude != ThresholdCrossingFt || w.Flags&aglFlag == 0 {
		t.Errorf("threshold waypoint %+v", w)
	}
	if d := calc.HaversineMeters(thr.Lat, thr.Lon, wps[firstGround].Latitude, wps[firstGround].Longitude); math.Abs(d-TouchdownMeters) > TouchdownMeters*0.005 {
		t.Errorf("touchdown waypoint %.0f m past the threshold", d)
	}
	for i, w := range wps[firstGround:] {
		if w.Flags&onGround == 0 || w.Flags&reverse != 0 {
			t.Fatalf("ground waypoint %d flags %x", i, w.Flags)
		}
	}
	// Rollout slows monotonically to the exit speed at the exit's runway node.
	rn := p.Route.Points[0]
	exitIdx := -1
	for i := firstGround; i < len(wps); i++ {
		if calc.HaversineMeters(rn.Lat, rn.Lon, wps[i].Latitude, wps[i].Longitude) < 0.5 {
			exitIdx = i
			break
		}
		if i > firstGround && wps[i].KtsSpeed > wps[i-1].KtsSpeed {
			t.Errorf("rollout speeds up at waypoint %d", i)
		}
	}
	if exitIdx < 0 || wps[exitIdx].KtsSpeed != ExitSpeed(p.Exit) {
		t.Fatalf("no exit waypoint at the runway node with the exit speed")
	}
	// Stand: nose-in at crawl speed to the stop point (radius − nose offset
	// ahead of the circle centre along the stand heading), last waypoint past it.
	centre, junction := p.Route.Points[len(p.Route.Points)-1], p.Route.Points[len(p.Route.Points)-2]
	standInfo := g.Layout.Parking[c22]
	if d := calc.HaversineMeters(centre.Lat, centre.Lon, p.Stop.Lat, p.Stop.Lon); math.Abs(d-(standInfo.Radius-DefaultNoseOffsetMeters)) > 0.5 {
		t.Errorf("stop %.1f m ahead of the centre, want %.1f", d, standInfo.Radius-DefaultNoseOffsetMeters)
	}
	stand := p.Stop
	// The landing chain ends at the vacate stop; the taxi-in chain ends at the stand.
	v := p.Route.Points[p.VacateIndex]
	if d := calc.HaversineMeters(v.Lat, v.Lon, wps[len(wps)-1].Latitude, wps[len(wps)-1].Longitude); d > 0.5 {
		t.Errorf("landing chain ends %.1f m from the vacate stop", d)
	}
	if off := math.Abs(calc.CrossTrackMeters(p.Runway.Primary.Threshold.Lat, p.Runway.Primary.Threshold.Lon, p.Runway.Secondary.Threshold.Lat, p.Runway.Secondary.Threshold.Lon, v.Lat, v.Lon)); off < p.Runway.Width/2+RunwayClearMeters {
		t.Errorf("vacate stop only %.0f m from the runway centreline", off)
	}
	wps = p.TaxiWaypoints
	n := len(wps)
	if d := calc.HaversineMeters(stand.Lat, stand.Lon, wps[n-2].Latitude, wps[n-2].Longitude); d > 0.5 {
		t.Errorf("second-last waypoint %.1f m from the stand", d)
	}
	over := calc.HaversineMeters(stand.Lat, stand.Lon, wps[n-1].Latitude, wps[n-1].Longitude)
	if math.Abs(over-StandOvershootMeters) > 0.5 ||
		calc.HaversineMeters(junction.Lat, junction.Lon, wps[n-1].Latitude, wps[n-1].Longitude) <= calc.HaversineMeters(junction.Lat, junction.Lon, stand.Lat, stand.Lon) {
		t.Errorf("overshoot waypoint %.1f m from the stand, not beyond it", over)
	}
	if wps[n-1].KtsSpeed != StandApproachSpeedKts || wps[n-2].KtsSpeed != StandApproachSpeedKts {
		t.Errorf("stand approach speeds %.0f / %.0f", wps[n-2].KtsSpeed, wps[n-1].KtsSpeed)
	}
	t.Logf("RWY 24 → C22: exit %s at %.0f m, %d waypoints, taxi-in %.0f m via %v", p.Exit.Taxiway, p.Exit.Along, n, p.Route.Length, p.Route.Taxiways)
}

func TestPlanArrivalAllRunwaysAndGroundAGL(t *testing.T) {
	g := lkprGraph(t)
	c22, _ := g.Layout.ParkingIndex("C22")
	for _, end := range []string{"24", "06", "12", "30"} {
		p, err := PlanArrival(g, end, c22, ArrivalOptions{SpawnNm: 6, GroundAGL: true})
		if err != nil {
			t.Fatalf("RWY %s: %v", end, err)
		}
		for _, w := range p.Waypoints {
			if w.Flags&onGround != 0 && (w.Flags&aglFlag == 0 || w.Altitude != 0) {
				t.Fatalf("RWY %s: ground waypoint not at 0 ft AGL: %+v", end, w)
			}
		}
	}
	if _, err := PlanArrival(g, "18", c22, ArrivalOptions{SpawnNm: 5}); !errors.Is(err, airport.ErrUnknownRunway) {
		t.Errorf("unknown runway: %v", err)
	}
	if _, err := TaxiInWaypoints(g, &airport.Route{}); !errors.Is(err, ErrNotStandRoute) {
		t.Errorf("empty route: %v", err)
	}
}

func arrivalPositionMsg(req, obj uint32, p airport.LatLon, agl, hdg, kts float64, ground bool) engine.Message {
	var hdr types.SIMCONNECT_RECV_SIMOBJECT_DATA
	off := int(unsafe.Offsetof(hdr.DwData))
	buf := make([]byte, off+int(unsafe.Sizeof(arrivalMonitor{})))
	h := (*types.SIMCONNECT_RECV_SIMOBJECT_DATA)(unsafe.Pointer(&buf[0]))
	h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA)
	h.DwRequestID, h.DwObjectID = types.DWORD(req), types.DWORD(obj)
	g := 0.0
	if ground {
		g = 1
	}
	*(*arrivalMonitor)(unsafe.Pointer(&buf[off])) = arrivalMonitor{p.Lat, p.Lon, agl, hdg, kts, g, -300, [5]float64{}, 0, 0}
	return engine.Message{SIMCONNECT_RECV: (*types.SIMCONNECT_RECV)(unsafe.Pointer(&buf[0]))}
}

func arrivalStates(ch <-chan ArrivalEvent) ([]ArrivalState, []ArrivalEvent) {
	var out []ArrivalState
	var evs []ArrivalEvent
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return out, evs
			}
			evs = append(evs, ev)
			if len(out) == 0 || out[len(out)-1] != ev.State {
				out = append(out, ev.State)
			}
		default:
			return out, evs
		}
	}
}

func startArrival(t *testing.T) (*ArrivalController, *fakeClient) {
	t.Helper()
	g := lkprGraph(t)
	fc := &fakeClient{}
	ctl := NewArrivalController(NewFleet(fc))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA2"}); err != nil {
		t.Fatal(err)
	}
	return ctl, fc
}

func TestArrivalControllerFullArrival(t *testing.T) {
	ctl, fc := startArrival(t)
	p := ctl.Plan()
	if len(fc.spawned) != 1 || fc.spawned[0].OnGround != 0 || fc.spawned[0].Heading != p.End.Heading {
		t.Fatalf("spawn %+v", fc.spawned)
	}
	req, mon := DefaultArrivalRequestBase, DefaultArrivalRequestBase+arrReqMonitor
	if !ctl.Handle(assignedMsg(req, 77)) {
		t.Fatal("spawn not handled")
	}
	// Gear handle, lights, then the waypoint chain.
	// Gear handle, lights, then the landing chain.
	if len(fc.waypoints) != 3 || len(fc.waypoints[0]) != 8 || len(fc.waypoints[1]) != 40 || len(fc.waypoints[2])/engine.WaypointWireSize != len(p.Waypoints) {
		t.Fatalf("SetDataOnSimObject calls: %d (sizes %v)", len(fc.waypoints), []int{len(fc.waypoints[0])})
	}
	thr := p.End.Threshold
	onFinal := func(m float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(thr.Lat, thr.Lon, math.Mod(p.End.Heading+180, 360), m)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	onRunway := func(m float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(thr.Lat, thr.Lon, p.End.Heading, m)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	pts := p.Route.Points
	ctl.Handle(arrivalPositionMsg(mon, 77, onFinal(7000), 1200, p.End.Heading, 160, false))
	ctl.Handle(arrivalPositionMsg(mon, 77, onFinal(300), 120, p.End.Heading, 140, false))
	ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(600), 12, p.End.Heading, 125, true))
	ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(1500), 12, p.End.Heading, 50, true))
	now := time.Now()
	ctl.now = func() time.Time { return now }
	step := func(q airport.LatLon, kts float64) {
		ctl.Handle(arrivalPositionMsg(mon, 77, q, 12, 300, kts, true))
		now = now.Add(time.Second)
	}
	// Roll off the runway to the vacate stop, as 1 Hz updates would.
	for _, q := range pts[len(p.Exit.Path)-1 : p.VacateIndex+1] {
		step(q, 15)
	}
	// Stopped there (still reporting 5 kt, as MSFS AI does after stopping):
	// after-landing lights, then the dwell before taxiing on.
	for i := 0; i < StationarySeconds+2; i++ {
		step(pts[p.VacateIndex], 5)
	}
	if ctl.State() != ArrivalAwaitingTaxi {
		t.Fatalf("state %v at the vacate stop, want awaiting taxi", ctl.State())
	}
	taxiSent := len(fc.waypoints)
	ctl.ClearToTaxi()
	if len(fc.waypoints) <= taxiSent || len(fc.waypoints[len(fc.waypoints)-1])/engine.WaypointWireSize != len(p.TaxiWaypoints) {
		t.Fatal("ClearToTaxi did not send the taxi-in chain")
	}
	for _, q := range pts[p.VacateIndex : len(pts)-1] {
		step(q, 15)
	}
	// On the stop mark, then standing still for longer than StationarySeconds.
	for i := 0; i < StationarySeconds+2; i++ {
		step(p.Stop, 3)
	}

	states, evs := arrivalStates(ctl.Events())
	want := []ArrivalState{ArrivalSpawning, ArrivalApproaching, ArrivalLanding, ArrivalRollout, ArrivalVacating, ArrivalAwaitingTaxi, ArrivalTaxiing, ArrivalParking, ArrivalParked}
	if len(states) != len(want) {
		t.Fatalf("states %v, want %v", states, want)
	}
	for i := range want {
		if states[i] != want[i] {
			t.Fatalf("states %v, want %v", states, want)
		}
	}
	last := evs[len(evs)-1]
	if last.Err != nil || math.Abs(last.Touchdown-600) > 5 {
		t.Errorf("final event %+v", last)
	}
	if fc.periods[len(fc.periods)-1] != types.SIMCONNECT_PERIOD_NEVER {
		t.Error("monitoring not stopped when parked")
	}
}

func TestArrivalControllerNoTouchdown(t *testing.T) {
	ctl, _ := startArrival(t)
	p := ctl.Plan()
	mon := DefaultArrivalRequestBase + arrReqMonitor
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 5))
	thr := p.End.Threshold
	lat, lon := calc.DisplaceByHeading(thr.Lat, thr.Lon, p.End.Heading, 100)
	ctl.Handle(arrivalPositionMsg(mon, 5, airport.LatLon{Lat: lat, Lon: lon}, 12, p.End.Heading, 132, false))
	lat, lon = calc.DisplaceByHeading(thr.Lat, thr.Lon, p.End.Heading, p.Runway.Length+500)
	ctl.Handle(arrivalPositionMsg(mon, 5, airport.LatLon{Lat: lat, Lon: lon}, 12, p.End.Heading, 132, false))
	_, evs := arrivalStates(ctl.Events())
	last := evs[len(evs)-1]
	if last.State != ArrivalFailed || !errors.Is(last.Err, ErrNoTouchdown) {
		t.Errorf("last event %v %v, want failed / ErrNoTouchdown", last.State, last.Err)
	}
}

func TestArrivalControllerCancelAndErrors(t *testing.T) {
	ctl, fc := startArrival(t)
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 9))
	if err := ctl.Cancel(); err != nil || len(fc.removed) != 1 || ctl.State() != ArrivalCancelled {
		t.Errorf("cancel: err=%v removed=%v state=%v", err, fc.removed, ctl.State())
	}
	if err := ctl.Start(ArrivalRequest{}); !errors.Is(err, ErrAlreadyStarted) {
		t.Errorf("restart: %v", err)
	}
	if err := NewArrivalController(NewFleet(&fakeClient{})).Start(ArrivalRequest{Model: "x"}); !errors.Is(err, ErrBadTaxiRequest) {
		t.Errorf("no graph: %v", err)
	}
}

// TestArrivalControllerHybrid: MSFS AI lands, the injector takes over clear
// of the runway without a jump, holds at the vacate stop with taxi lights,
// and parks aligned on the stop mark (#309).
func TestArrivalControllerHybrid(t *testing.T) {
	g := lkprGraph(t)
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA3", HoldForClearance: true}); err != nil {
		t.Fatal(err)
	}
	p := ctl.Plan()
	req, mon := DefaultArrivalRequestBase, DefaultArrivalRequestBase+arrReqMonitor
	ctl.Handle(assignedMsg(req, 77))
	thr := p.End.Threshold
	onRunway := func(m float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(thr.Lat, thr.Lon, p.End.Heading, m)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(-300), 120, p.End.Heading, 140, false))
	ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(600), 12, p.End.Heading, 125, true))
	if ec.periods[len(ec.periods)-1] != types.SIMCONNECT_PERIOD_SIM_FRAME {
		t.Fatal("not reading every frame after touchdown")
	}
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))

	// Roll along the exit with MSFS AI until clear of the runway.
	pts := p.Route.Points
	var at airport.LatLon
	var hdg float64
	for i := 1; i < len(pts) && ctl.State() == ArrivalRollout; i++ {
		at, hdg = pts[i], calc.BearingDegrees(pts[i-1].Lat, pts[i-1].Lon, pts[i].Lat, pts[i].Lon)
		ctl.Handle(arrivalPositionMsg(mon, 77, at, 12, hdg, 20, true))
		now = now.Add(time.Second / 60)
	}
	if ctl.State() != ArrivalVacating || !inj.Driven(77) {
		t.Fatalf("state %v, driven %v after leaving the runway", ctl.State(), inj.Driven(77))
	}
	placed := func() []types.SIMCONNECT_DATA_INITPOSITION {
		var out []types.SIMCONNECT_DATA_INITPOSITION
		for _, b := range ec.waypoints {
			if len(b) == int(unsafe.Sizeof(types.SIMCONNECT_DATA_INITPOSITION{})) && b[len(b)-8] == 1 { // OnGround
				var q types.SIMCONNECT_DATA_INITPOSITION
				copy(unsafe.Slice((*byte)(unsafe.Pointer(&q)), len(b)), b)
				out = append(out, q)
			}
		}
		return out
	}
	first := placed()
	if len(first) != 1 {
		t.Fatalf("%d placements at takeover, want 1", len(first))
	}
	// No jump at the switch: placed where the aircraft was, same heading.
	if d := calc.HaversineMeters(at.Lat, at.Lon, first[0].Latitude, first[0].Longitude); d > 0.5 {
		t.Errorf("takeover moved the aircraft %.2f m", d)
	}
	if math.Abs(headingDiff(first[0].Heading, hdg)) > 1 {
		t.Errorf("takeover heading %.1f, was %.1f", first[0].Heading, hdg)
	}

	frames := func(until ArrivalState, max int) {
		for i := 0; i < max && ctl.State() != until; i++ {
			now = now.Add(time.Second / 60)
			ctl.Handle(arrivalPositionMsg(mon, 77, at, 12, hdg, 0, true)) // positions are ignored now
		}
		if ctl.State() != until {
			t.Fatalf("state %v, want %v", ctl.State(), until)
		}
	}
	frames(ArrivalAwaitingTaxi, 60*120)
	ec.events = nil
	now = now.Add(time.Minute)
	ctl.Handle(arrivalPositionMsg(mon, 77, at, 12, hdg, 0, true))
	if ctl.State() != ArrivalAwaitingTaxi {
		t.Fatal("did not hold for clearance")
	}
	ctl.ClearToTaxi()
	frames(ArrivalParked, 60*900)

	all := placed()
	for i := 1; i < len(all); i++ {
		d := calc.HaversineMeters(all[i-1].Latitude, all[i-1].Longitude, all[i].Latitude, all[i].Longitude)
		if d > 0.6 { // 20 kt at 60 Hz is 0.17 m per frame
			t.Fatalf("aircraft jumped %.2f m between frames %d and %d", d, i-1, i)
		}
	}
	last := all[len(all)-1]
	if d := calc.HaversineMeters(p.Stop.Lat, p.Stop.Lon, last.Latitude, last.Longitude); d > 1 {
		t.Errorf("parked %.2f m from the stop mark", d)
	}
	if hd := math.Abs(headingDiff(last.Heading, g.Layout.Parking[c22].Heading)); hd > 3 {
		t.Errorf("parked %.1f° off the stand heading", hd)
	}
	// The taxi light comes on while waiting; taxi-in from D crosses runway
	// 12/30: strobes and landing lights on
	// while crossing, off again after it, then taxi and beacon off on the stand.
	if want := []string{"TAXI_LIGHTS_SET=1", "STROBES_SET=1", "LANDING_LIGHTS_SET=1", "STROBES_SET=0", "LANDING_LIGHTS_SET=0", "BEACON_LIGHTS_SET=0", "TAXI_LIGHTS_SET=0"}; !equalStrings(ec.events, want) {
		t.Errorf("lights after clearance %v, want %v", ec.events, want)
	}
	if len(ctl.crossZones) != 1 {
		t.Errorf("crossing zones %v, want one (12/30 between its hold-short lines)", ctl.crossZones)
	}
	t.Logf("%d placements, crossing zones %v", len(all), ctl.crossZones)
}

// TestArrivalControllerHybridRunwayTakeover: the injector takes over during
// the rollout, brakes to the high-speed exit speed, keeps landing lights and
// strobes on while on the runway and switches them off once clear (#309).
func TestArrivalControllerHybridRunwayTakeover(t *testing.T) {
	g := lkprGraph(t)
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA4", RollThroughChance: -1}); err != nil {
		t.Fatal(err)
	}
	p := ctl.Plan()
	mon := DefaultArrivalRequestBase + arrReqMonitor
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	thr := p.End.Threshold
	onRunway := func(m float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(thr.Lat, thr.Lon, p.End.Heading, m)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(-300), 120, p.End.Heading, 140, false))
	ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(700), 12, p.End.Heading, 125, true))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	// MSFS AI brakes: 1.5 m/s² from 125 kt, reported every frame.
	s, v := 700.0, 125*ktsToMS
	for ctl.State() == ArrivalRollout && !inj.Driven(77) && s < p.Exit.Along {
		now = now.Add(time.Second / 60)
		v = math.Max(v-1.5/60, 0)
		s += v / 60
		ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(s), 12, p.End.Heading, v/ktsToMS, true))
	}
	if !inj.Driven(77) || ctl.State() != ArrivalRollout {
		t.Fatalf("not taken over on the runway: state %v at %.0f m, %.0f kt", ctl.State(), s, v/ktsToMS)
	}
	if v/ktsToMS > TakeoverKts+0.1 {
		t.Errorf("taken over at %.0f kt", v/ktsToMS)
	}
	lightsAt := func(evts []string, name string) string {
		for i := len(evts) - 1; i >= 0; i-- {
			if strings.HasPrefix(evts[i], name) {
				return evts[i]
			}
		}
		return ""
	}
	if lightsAt(ec.events, "LANDING_LIGHTS_SET") != "LANDING_LIGHTS_SET=1" || lightsAt(ec.events, "STROBES_SET") != "STROBES_SET=1" {
		t.Errorf("runway lights at takeover: %v", ec.events)
	}
	exitNode := p.Route.Points[0]
	var atExit float64
	for i := 0; i < 60*300 && ctl.State() != ArrivalAwaitingTaxi; i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(s), 12, p.End.Heading, 0, true))
		if atExit == 0 && ctl.last.Position.Lat != 0 && calc.HaversineMeters(exitNode.Lat, exitNode.Lon, ctl.last.Position.Lat, ctl.last.Position.Lon) < 15 {
			atExit = ctl.last.GroundSpeed
		}
		landing := lightsAt(ec.events, "LANDING_LIGHTS_SET")
		if (ctl.State() == ArrivalRollout || ctl.State() == ArrivalVacating) && landing == "LANDING_LIGHTS_SET=0" {
			t.Fatalf("landing lights off before stopping (%v)", ctl.State())
		}
		if ctl.State() == ArrivalVacating && lightsAt(ec.events, "STROBES_SET") != "STROBES_SET=0" {
			t.Fatal("strobes still on clear of the runway")
		}
	}
	if ctl.State() != ArrivalAwaitingTaxi {
		t.Fatalf("state %v, want awaiting taxi", ctl.State())
	}
	if atExit < 20 || atExit > InjectExitHighSpeedKts+1 {
		t.Errorf("%.1f kt at the exit, want about %.0f", atExit, InjectExitHighSpeedKts)
	}
	// The taxi light TaxiLightDelay after the landing lights went off.
	stopped := len(ec.events)
	for i := 0; i < 60*3; i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(s), 12, p.End.Heading, 0, true))
		if i == 60 && len(ec.events) != stopped {
			t.Errorf("taxi light after 1 s: %v", ec.events[stopped:])
		}
	}
	for _, want := range []string{"LANDING_LIGHTS_SET=0", "STROBES_SET=0", "TAXI_LIGHTS_SET=1"} {
		if !slices.Contains(ec.events, want) {
			t.Errorf("lights %v, want %s", ec.events, want)
		}
	}
	t.Logf("taken over at %.0f kt %.0f m past the threshold; %.1f kt at the exit; lights %v", v/ktsToMS, s, atExit, ec.events)
}

// TestArrivalControllerHybridRollThrough: with a rolling clearance the
// aircraft only slows to about RollThroughKts at the vacate point, switches
// landing lights off there and the taxi light on while rolling (#309).
func TestArrivalControllerHybridRollThrough(t *testing.T) {
	g := lkprGraph(t)
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA5", RollThroughChance: 1}); err != nil {
		t.Fatal(err)
	}
	p := ctl.Plan()
	mon := DefaultArrivalRequestBase + arrReqMonitor
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	thr := p.End.Threshold
	onRunway := func(m float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(thr.Lat, thr.Lon, p.End.Heading, m)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(-300), 120, p.End.Heading, 140, false))
	ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(700), 12, p.End.Heading, 125, true))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	s, v := 700.0, 125*ktsToMS
	slowest, taxiOnAt, landingOffAt, crawl := math.Inf(1), time.Time{}, time.Time{}, 0
	for i := 0; i < 60*900 && ctl.State() != ArrivalParked; i++ {
		now = now.Add(time.Second / 60)
		if !inj.Driven(77) {
			v = math.Max(v-1.5/60, 0)
			s += v / 60
		}
		n := len(ec.events)
		ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(s), 12, p.End.Heading, v/ktsToMS, true))
		for _, e := range ec.events[n:] {
			switch {
			case e == "LANDING_LIGHTS_SET=0" && landingOffAt.IsZero() && ctl.State() >= ArrivalAwaitingTaxi:
				landingOffAt = now
			case e == "TAXI_LIGHTS_SET=1" && taxiOnAt.IsZero():
				taxiOnAt = now
			}
		}
		if ctl.State() >= ArrivalVacating && ctl.State() <= ArrivalTaxiing && inj.Driven(77) {
			if ctl.last.GroundSpeed == 0 && ctl.State() != ArrivalParking {
				t.Fatalf("stopped in %v on a rolling clearance", ctl.State())
			}
			slowest = math.Min(slowest, ctl.last.GroundSpeed)
			if ctl.last.GroundSpeed < 1 {
				crawl++
			}
		}
	}
	if ctl.State() != ArrivalParked {
		t.Fatalf("state %v, want parked", ctl.State())
	}
	if slowest > RollThroughKts+1 {
		t.Errorf("slowest %.2f kt at the vacate point, want about %.1f", slowest, RollThroughKts)
	}
	if d := taxiOnAt.Sub(landingOffAt); landingOffAt.IsZero() || d < TaxiLightDelay || d > TaxiLightDelay+time.Second {
		t.Errorf("taxi light %v after the landing lights went off, want %v", d, TaxiLightDelay)
	}
	// One brief dip, not a crawl (live: twice ~20 s below 1 kt).
	if s := float64(crawl) / 60; s > 8 {
		t.Errorf("%.1f s below 1 kt, want a brief dip", s)
	}
	t.Logf("slowest %.2f kt, %.1f s below 1 kt; taxi light %v after landing lights off", slowest, float64(crawl)/60, taxiOnAt.Sub(landingOffAt))
}

// TestArrivalControllerHoldAtCrossings: with HoldAtCrossings the aircraft
// stops short of runway 12/30 without runway lights until ClearToCross; a
// clearance given early means it does not stop (#309).
func TestArrivalControllerHoldAtCrossings(t *testing.T) {
	for _, early := range []bool{false, true} {
		g := lkprGraph(t)
		ec := &eventClient{}
		inj := NewInjector(ec)
		ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
		c22, _ := g.Layout.ParkingIndex("C22")
		if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA6",
			RollThroughChance: -1, HoldAtCrossings: true, AfterLandingDwell: time.Second}); err != nil {
			t.Fatal(err)
		}
		p := ctl.Plan()
		mon := DefaultArrivalRequestBase + arrReqMonitor
		ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
		thr := p.End.Threshold
		at := func(m float64) airport.LatLon {
			lat, lon := calc.DisplaceByHeading(thr.Lat, thr.Lon, p.End.Heading, m)
			return airport.LatLon{Lat: lat, Lon: lon}
		}
		now := time.Now()
		ctl.now = func() time.Time { return now }
		ctl.Handle(arrivalPositionMsg(mon, 77, at(-300), 120, p.End.Heading, 140, false))
		ctl.Handle(arrivalPositionMsg(mon, 77, at(700), 12, p.End.Heading, 125, true))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
		s, v := 700.0, 125*ktsToMS
		frame := func() {
			now = now.Add(time.Second / 60)
			if !inj.Driven(77) {
				v = math.Max(v-1.5/60, 0)
				s += v / 60
			}
			ctl.Handle(arrivalPositionMsg(mon, 77, at(s), 12, p.End.Heading, v/ktsToMS, true))
		}
		held := false
		for i := 0; i < 60*900 && ctl.State() != ArrivalParked; i++ {
			frame()
			if early && ctl.State() == ArrivalTaxiing && ctl.nextCross == 0 && len(ctl.crossZones) > 0 {
				ctl.ClearToCross()
			}
			if ctl.State() == ArrivalHoldingShort && !held {
				held = true
				if ctl.last.HoldingShortOf != "12/30" {
					t.Errorf("holding short of %q, want 12/30", ctl.last.HoldingShortOf)
				}
				if strings.Contains(strings.Join(ec.events, " "), "STROBES_SET=1 LANDING_LIGHTS_SET=1 STROBES_SET=0") {
					t.Error("crossing lights before the crossing clearance")
				}
				if l := ctl.lights; l.Strobe || l.Landing || ctl.crossing {
					t.Errorf("runway lights while holding short: %v", l)
				}
				stop := ctl.mover.Pose().Distance
				if z := ctl.crossZones[0]; stop > z.from || stop < z.from-HoldShortStopMeters-2 {
					t.Errorf("nose gear at %.1f m, hold line at %.1f m", stop, z.from)
				}
				for range 60 * 30 {
					frame()
				}
				if ctl.State() != ArrivalHoldingShort || ctl.mover.Pose().Distance != stop {
					t.Fatal("moved without a crossing clearance")
				}
				ctl.ClearToCross()
			}
		}
		if ctl.State() != ArrivalParked {
			t.Fatalf("early=%v: state %v, want parked", early, ctl.State())
		}
		if held == early {
			t.Errorf("early=%v: held short %v", early, held)
		}
		if !slices.Contains(ec.events, "STROBES_SET=1") {
			t.Errorf("early=%v: no crossing lights: %v", early, ec.events)
		}
	}
}

// TestArrivalControllerInjectedApproach: the whole arrival injected — glide
// path, flare, a soft touchdown, de-rotation, and the hand-over to the
// injected rollout without a jump (#318).
func TestArrivalControllerInjectedApproach(t *testing.T) {
	g := lkprGraph(t)
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA7",
		InjectApproach: true, RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
		t.Fatal(err)
	}
	p := ctl.Plan()
	mon := DefaultArrivalRequestBase + arrReqMonitor
	type result struct {
		states []ArrivalState
		evs    []ArrivalEvent
	}
	done := make(chan result)
	go func() {
		var r result
		for ev := range ctl.Events() { // until the controller finishes
			r.evs = append(r.evs, ev)
			if len(r.states) == 0 || r.states[len(r.states)-1] != ev.State {
				r.states = append(r.states, ev.State)
			}
		}
		done <- r
	}()
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	if !inj.Driven(77) || ctl.State() != ArrivalApproaching {
		t.Fatalf("not taken over on final: %v", ctl.State())
	}
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	for i := 0; i < 60*1200 && ctl.State() != ArrivalParked; i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 77, p.End.Threshold, 0, 0, 0, false)) // a frame tick
	}
	if ctl.State() != ArrivalParked {
		t.Fatalf("stuck in %v (approach %v, mover %v, pose %+v)", ctl.State(), ctl.approach != nil, ctl.mover != nil, ctl.last)
	}
	r := <-done
	states, evs := r.states, r.evs
	want := []ArrivalState{ArrivalSpawning, ArrivalApproaching, ArrivalLanding, ArrivalRollout, ArrivalVacating, ArrivalAwaitingTaxi, ArrivalTaxiing, ArrivalParking, ArrivalParked}
	if !slices.Equal(states, want) {
		t.Fatalf("states %v, want %v", states, want)
	}
	var td ArrivalEvent
	for _, e := range evs {
		if e.State == ArrivalRollout && e.Touchdown != 0 {
			td = e
			break
		}
	}
	if td.TouchdownFpm > -80 || td.TouchdownFpm < -200 || td.Touchdown < 250 || td.Touchdown > 800 {
		t.Errorf("touchdown %.0f m past the threshold at %.0f fpm", td.Touchdown, td.TouchdownFpm)
	}
	var placed []types.SIMCONNECT_DATA_INITPOSITION
	for _, b := range ec.waypoints {
		if len(b) == int(unsafe.Sizeof(types.SIMCONNECT_DATA_INITPOSITION{})) {
			var q types.SIMCONNECT_DATA_INITPOSITION
			copy(unsafe.Slice((*byte)(unsafe.Pointer(&q)), len(b)), b)
			placed = append(placed, q)
		}
	}
	maxPitch, maxJump, maxHdg := 0.0, 0.0, 0.0
	for i, q := range placed {
		maxPitch = math.Max(maxPitch, -q.Pitch)
		if i > 0 {
			a := placed[i-1]
			maxJump = math.Max(maxJump, calc.HaversineMeters(a.Latitude, a.Longitude, q.Latitude, q.Longitude))
			maxHdg = math.Max(maxHdg, math.Abs(headingDiff(a.Heading, q.Heading)))
		}
	}
	if maxJump > 1.5 { // 150 kt at 60 Hz is 1.3 m per frame
		t.Errorf("largest move between frames %.2f m", maxJump)
	}
	if maxHdg > 1 {
		t.Errorf("heading jumped %.2f° between frames", maxHdg)
	}
	if maxPitch < 5 || maxPitch > 6 {
		t.Errorf("flare pitch %.1f°, want about 5.5", maxPitch)
	}
	spoilMax := 0.0
	for _, b := range ec.waypoints {
		switch len(b) {
		case 24:
			spoilMax = math.Max(spoilMax, *(*float64)(unsafe.Pointer(&b[0])))
		}
	}
	if spoilMax != 100 || ctl.spoilers.pct != 0 {
		t.Errorf("spoilers max %.0f now %.0f; want 100 then stowed", spoilMax, ctl.spoilers.pct)
	}
	if ctl.flapsPct != 0 {
		t.Errorf("flaps %.0f%% on the stand, want retracted", ctl.flapsPct)
	}
	t.Logf("touchdown %.0f m at %.0f fpm; %d placements, largest step %.2f m, heading step %.2f°, flare pitch %.1f°",
		td.Touchdown, td.TouchdownFpm, len(placed), maxJump, maxHdg, maxPitch)
}

// TestArrivalControllerInjectedSweep flies fully injected arrivals to a
// sample of LKPR stands from every runway end: each must park without
// failing or jumping, aligned with its stand.
func TestArrivalControllerInjectedSweep(t *testing.T) {
	g := lkprGraph(t)
	ok, total := 0, 0
	for _, p := range g.Layout.Parking {
		// Only stands an A320 (36 m span) fits; stand suitability is #291.
		if (p.Index%11 != 0 && !standFacesOut(g, p.Index)) || p.Radius < 18 {
			continue
		}
		for _, end := range []string{"06", "24", "12", "30"} {
			ec := &eventClient{}
			inj := NewInjector(ec)
			ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
			if err := ctl.Start(ArrivalRequest{Graph: g, Runway: end, Parking: p.Index, Model: "A320", InjectApproach: true,
				RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
				continue // no route from this runway to the stand
			}
			total++
			now := time.Now()
			ctl.now = func() time.Time { return now }
			ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
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
			mon := DefaultArrivalRequestBase + arrReqMonitor
			for i := 0; i < 60*3600 && !ctl.State().Terminal(); i++ {
				now = now.Add(time.Second / 60)
				ctl.Handle(arrivalPositionMsg(mon, 77, p.Position, 0, 0, 0, false))
			}
			if ctl.State() != ArrivalParked {
				var err error
				if ctl.State().Terminal() {
					err = <-errs
				}
				t.Errorf("%s → %s: ended %v (%v)", end, p.Label(), ctl.State(), err)
				continue
			}
			all := placements(ec)
			for i := 1; i < len(all); i++ {
				if d := calc.HaversineMeters(all[i-1].Latitude, all[i-1].Longitude, all[i].Latitude, all[i].Longitude); d > 1.5 {
					t.Errorf("%s → %s: jumped %.2f m at placement %d/%d", end, p.Label(), d, i, len(all))
					break
				}
			}
			last := all[len(all)-1]
			if hd := math.Abs(headingDiff(last.Heading, p.Heading)); hd > 3 {
				t.Errorf("%s → %s: parked %.1f° off the stand heading", end, p.Label(), hd)
			}
			stop := ctl.Plan().Stop
			if d := calc.HaversineMeters(stop.Lat, stop.Lon, last.Latitude, last.Longitude); d > 1 {
				t.Errorf("%s → %s: parked %.1f m from the stop mark", end, p.Label(), d)
			}
			ok++
		}
	}
	t.Logf("%d of %d arrivals parked", ok, total)
}
