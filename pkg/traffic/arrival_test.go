//go:build windows
// +build windows

package traffic

import (
	"errors"
	"math"
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
	// Taxi-in from D crosses runway 12/30: strobes and landing lights on
	// while crossing, off again after it, then taxi and beacon off on the stand.
	if want := []string{"STROBES_SET=1", "LANDING_LIGHTS_SET=1", "STROBES_SET=0", "LANDING_LIGHTS_SET=0", "BEACON_LIGHTS_SET=0", "TAXI_LIGHTS_SET=0"}; !equalStrings(ec.events, want) {
		t.Errorf("lights after clearance %v, want %v", ec.events, want)
	}
	t.Logf("%d placements", len(all))
}
