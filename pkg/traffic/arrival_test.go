//go:build windows
// +build windows

package traffic

import (
	"errors"
	"math"
	"testing"
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
	p, err := PlanArrival(g, "24", c22, 5, nil, airport.RouteOptions{}, false)
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
	// Stand: nose-in at crawl speed, last waypoint past the stand.
	stand, junction := p.Route.Points[len(p.Route.Points)-1], p.Route.Points[len(p.Route.Points)-2]
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
		p, err := PlanArrival(g, end, c22, 6, nil, airport.RouteOptions{}, true)
		if err != nil {
			t.Fatalf("RWY %s: %v", end, err)
		}
		for _, w := range p.Waypoints {
			if w.Flags&onGround != 0 && (w.Flags&aglFlag == 0 || w.Altitude != 0) {
				t.Fatalf("RWY %s: ground waypoint not at 0 ft AGL: %+v", end, w)
			}
		}
	}
	if _, err := PlanArrival(g, "18", c22, 5, nil, airport.RouteOptions{}, false); !errors.Is(err, airport.ErrUnknownRunway) {
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
	*(*arrivalMonitor)(unsafe.Pointer(&buf[off])) = arrivalMonitor{p.Lat, p.Lon, agl, hdg, kts, g, -300}
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
	// Gear handle first, then the waypoint chain.
	if len(fc.waypoints) != 2 || len(fc.waypoints[0]) != 8 || len(fc.waypoints[1])/engine.WaypointWireSize != len(p.Waypoints) {
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
	// Walk the taxi-in route point by point, as 1 Hz updates would.
	for _, q := range pts[len(p.Exit.Path)-1 : len(pts)-2] {
		ctl.Handle(arrivalPositionMsg(mon, 77, q, 12, 300, 15, true))
	}
	ctl.Handle(arrivalPositionMsg(mon, 77, pts[len(pts)-2], 12, 35, 3, true)) // at the PARKING path
	ctl.Handle(arrivalPositionMsg(mon, 77, pts[len(pts)-1], 12, 35, 0.2, true))

	states, evs := arrivalStates(ctl.Events())
	want := []ArrivalState{ArrivalSpawning, ArrivalApproaching, ArrivalLanding, ArrivalRollout, ArrivalTaxiing, ArrivalParking, ArrivalParked}
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
