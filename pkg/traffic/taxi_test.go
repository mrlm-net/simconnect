//go:build windows
// +build windows

package traffic

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// fakeClient implements the engine.Client methods the taxi controller uses;
// any other method panics through the nil embedded interface.
type fakeClient struct {
	engine.Client
	defs      map[uint32][]string
	spawned   []types.SIMCONNECT_DATA_INITPOSITION
	released  []uint32
	removed   []uint32
	waypoints [][]byte
	periods   []types.SIMCONNECT_PERIOD
}

func (f *fakeClient) AddToDataDefinition(def uint32, name, unit string, _ types.SIMCONNECT_DATATYPE, _ float32, _ uint32) error {
	if f.defs == nil {
		f.defs = map[uint32][]string{}
	}
	f.defs[def] = append(f.defs[def], name)
	return nil
}
func (f *fakeClient) AICreateNonATCAircraft(_, _ string, p types.SIMCONNECT_DATA_INITPOSITION, _ uint32) error {
	f.spawned = append(f.spawned, p)
	return nil
}
func (f *fakeClient) AICreateNonATCAircraftEX1(_, _, _ string, p types.SIMCONNECT_DATA_INITPOSITION, _ uint32) error {
	f.spawned = append(f.spawned, p)
	return nil
}
func (f *fakeClient) AIReleaseControl(obj, _ uint32) error {
	f.released = append(f.released, obj)
	return nil
}
func (f *fakeClient) AIRemoveObject(obj, _ uint32) error {
	f.removed = append(f.removed, obj)
	return nil
}
func (f *fakeClient) SetDataOnSimObject(_, _ uint32, _ types.SIMCONNECT_DATA_SET_FLAG, n, size uint32, p unsafe.Pointer) error {
	f.waypoints = append(f.waypoints, append([]byte(nil), unsafe.Slice((*byte)(p), n*size)...))
	return nil
}
func (f *fakeClient) RequestDataOnSimObject(_, _, _ uint32, period types.SIMCONNECT_PERIOD, _ types.SIMCONNECT_DATA_REQUEST_FLAG, _, _, _ uint32) error {
	f.periods = append(f.periods, period)
	return nil
}

func lkprGraph(t *testing.T) *airport.Graph {
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
	g, err := airport.BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func c22Route(t *testing.T, g *airport.Graph, rwy string) *airport.Route {
	t.Helper()
	c22, err := g.Layout.ParkingIndex("C22")
	if err != nil {
		t.Fatal(err)
	}
	r, err := g.RouteToRunway(c22, rwy, airport.RouteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

const reverse = uint32(types.SIMCONNECT_WAYPOINT_REVERSE)
const onGround = uint32(types.SIMCONNECT_WAYPOINT_ON_GROUND)

func TestTaxiWaypointsC22To24(t *testing.T) {
	g := lkprGraph(t)
	r := c22Route(t, g, "24")
	wps, err := TaxiWaypoints(g, r)
	if err != nil {
		t.Fatal(err)
	}
	nrev := 0
	for nrev < len(wps) && wps[nrev].Flags&reverse != 0 {
		nrev++
	}
	for i, w := range wps {
		if (w.Flags&reverse != 0) != (i < nrev) {
			t.Errorf("waypoint %d reverse flag out of place", i)
		}
		if w.Flags&onGround == 0 {
			t.Errorf("waypoint %d not ON_GROUND", i)
		}
		if w.KtsSpeed <= 0 || w.KtsSpeed > TaxiSpeedKts {
			t.Errorf("waypoint %d speed %.1f", i, w.KtsSpeed)
		}
	}
	// Pushback: one straight reverse leg to the junction (MSFS AI cannot steer
	// in reverse), then the first forward waypoint far enough away to turn in.
	junction := r.Points[1]
	if nrev != 1 {
		t.Fatalf("%d reverse waypoints, want 1", nrev)
	}
	if d := calc.HaversineMeters(wps[0].Latitude, wps[0].Longitude, junction.Lat, junction.Lon); d > 0.01 {
		t.Errorf("pushback ends %.2f m from the junction", d)
	}
	if d := calc.HaversineMeters(junction.Lat, junction.Lon, wps[1].Latitude, wps[1].Longitude); d < TurnInMeters-0.5 || d > TurnInMeters+MaxWaypointSpacingMeters {
		t.Errorf("first forward waypoint %.1f m from the junction, want at least %.0f", d, TurnInMeters)
	}
	// Forward legs: spacing and the final hold-short point.
	for i := nrev + 2; i < len(wps); i++ {
		if d := calc.HaversineMeters(wps[i-1].Latitude, wps[i-1].Longitude, wps[i].Latitude, wps[i].Longitude); d > MaxWaypointSpacingMeters+0.5 {
			t.Errorf("waypoints %d→%d are %.0f m apart", i-1, i, d)
		}
	}
	last, hold := wps[len(wps)-1], r.Points[len(r.Points)-1]
	if d := calc.HaversineMeters(last.Latitude, last.Longitude, hold.Lat, hold.Lon); d > 0.01 {
		t.Errorf("last waypoint %.2f m from the hold-short", d)
	}
	if last.KtsSpeed != HoldShortApproachSpeedKts {
		t.Errorf("final approach speed %.1f, want %.1f", last.KtsSpeed, HoldShortApproachSpeedKts)
	}
	slow := 0
	for _, w := range wps[nrev:] {
		if w.KtsSpeed < TaxiSpeedKts {
			slow++
		}
	}
	if slow == 0 {
		t.Error("no waypoint slows for a turn")
	}
}

func TestTaxiWaypointsRejectsNonParkingRoute(t *testing.T) {
	g := lkprGraph(t)
	r := c22Route(t, g, "24")
	short := *r
	short.Nodes, short.Points = r.Nodes[1:], r.Points[1:]
	if _, err := TaxiWaypoints(g, &short); !errors.Is(err, ErrShortRoute) {
		t.Errorf("error = %v, want ErrShortRoute", err)
	}
}

func TestLineUpWaypoints(t *testing.T) {
	g := lkprGraph(t)
	for _, end := range []string{"24", "06", "12", "30"} {
		r := c22Route(t, g, end)
		wps, err := LineUpWaypoints(g, r)
		if err != nil {
			t.Fatal(err)
		}
		rwy, e, _ := g.Layout.RunwayEnd(end)
		if len(wps) != 5 || wps[0].Flags&onGround == 0 || wps[1].Flags&onGround == 0 || wps[2].Flags&onGround != 0 {
			t.Fatalf("%s: lineup chain flags wrong", end)
		}
		// Entry and alignment points lie on the runway centreline, in this end's direction.
		for _, w := range wps[:2] {
			d := calc.CrossTrackMeters(rwy.Primary.Threshold.Lat, rwy.Primary.Threshold.Lon, rwy.Secondary.Threshold.Lat, rwy.Secondary.Threshold.Lon, w.Latitude, w.Longitude)
			if math.Abs(d) > 2 {
				t.Errorf("%s: line-up point %.1f m off the centreline", end, d)
			}
		}
		b := calc.BearingDegrees(wps[0].Latitude, wps[0].Longitude, wps[1].Latitude, wps[1].Longitude)
		if diff := math.Abs(math.Mod(b-e.Heading+540, 360) - 180); diff > 1 {
			t.Errorf("%s: line-up bearing %.1f, runway heading %.1f", end, b, e.Heading)
		}
	}
}

// Message builders.

func assignedMsg(req, obj uint32) engine.Message {
	m := &types.SIMCONNECT_RECV_ASSIGNED_OBJECT_ID{DwRequestID: types.DWORD(req), DwObjectID: types.DWORD(obj)}
	m.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID)
	return engine.Message{SIMCONNECT_RECV: &m.SIMCONNECT_RECV}
}

func positionMsg(req, obj uint32, p airport.LatLon, hdg, kts float64, ground bool) engine.Message {
	var hdr types.SIMCONNECT_RECV_SIMOBJECT_DATA
	off := int(unsafe.Offsetof(hdr.DwData))
	buf := make([]byte, off+int(unsafe.Sizeof(taxiMonitor{})))
	h := (*types.SIMCONNECT_RECV_SIMOBJECT_DATA)(unsafe.Pointer(&buf[0]))
	h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA)
	h.DwRequestID, h.DwObjectID = types.DWORD(req), types.DWORD(obj)
	g := 0.0
	if ground {
		g = 1
	}
	*(*taxiMonitor)(unsafe.Pointer(&buf[off])) = taxiMonitor{p.Lat, p.Lon, hdg, kts, g}
	return engine.Message{SIMCONNECT_RECV: (*types.SIMCONNECT_RECV)(unsafe.Pointer(&buf[0]))}
}

func drain(ch <-chan TaxiEvent) []TaxiEvent {
	var out []TaxiEvent
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, ev)
		default:
			return out
		}
	}
}

func states(evs []TaxiEvent) []TaxiState {
	var out []TaxiState
	for _, e := range evs {
		if len(out) == 0 || out[len(out)-1] != e.State {
			out = append(out, e.State)
		}
	}
	return out
}

func startC22(t *testing.T) (*TaxiController, *fakeClient, *airport.Graph) {
	t.Helper()
	g := lkprGraph(t)
	fc := &fakeClient{}
	ctl := NewTaxiController(NewFleet(fc))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(TaxiRequest{Graph: g, Parking: c22, Runway: "24", Model: "FSLTL A320 Air France SL", Tail: "CSA123"}); err != nil {
		t.Fatal(err)
	}
	return ctl, fc, g
}

func TestTaxiControllerFullDeparture(t *testing.T) {
	ctl, fc, g := startC22(t)
	stand := g.Layout.Parking[18]
	if len(fc.spawned) != 1 || fc.spawned[0].Heading != stand.Heading || fc.spawned[0].OnGround != 1 ||
		math.Abs(fc.spawned[0].Latitude-stand.Position.Lat) > 1e-9 {
		t.Fatalf("spawn = %+v, want stand C22", fc.spawned)
	}
	if len(fc.defs[DefaultTaxiDefinitionBase]) != 1 || len(fc.defs[DefaultTaxiDefinitionBase+1]) != 5 {
		t.Fatalf("definitions = %v", fc.defs)
	}
	req, mon := DefaultTaxiRequestBase, DefaultTaxiRequestBase+reqOffMonitor

	if ctl.Handle(assignedMsg(999, 1)) {
		t.Error("handled another component's object ID")
	}
	if !ctl.Handle(assignedMsg(req, 42)) || ctl.ObjectID() != 42 {
		t.Fatal("spawn not acknowledged")
	}
	if len(fc.released) != 1 || len(fc.waypoints) != 1 || len(fc.periods) != 1 || fc.periods[0] != types.SIMCONNECT_PERIOD_SECOND {
		t.Fatalf("after spawn: released=%v waypoints=%d periods=%v", fc.released, len(fc.waypoints), fc.periods)
	}
	wantWps, _ := TaxiWaypoints(g, ctl.Route())
	if n := len(fc.waypoints[0]) / engine.WaypointWireSize; n != len(wantWps) {
		t.Errorf("sent %d waypoints, want %d", n, len(wantWps))
	}

	r := ctl.Route()
	pts := r.Points
	ctl.Handle(positionMsg(mon, 42, pts[0], 0, 2, true))  // pushing back
	ctl.Handle(positionMsg(mon, 42, pts[1], 0, 2, true))  // at the junction, still reversing
	ctl.Handle(positionMsg(mon, 42, pts[4], 0, 12, true)) // moving forward along the route
	ctl.Handle(positionMsg(mon, 42, pts[len(pts)/2], 0, 15, true))
	if ctl.Handle(positionMsg(mon, 7, pts[3], 0, 15, true)) {
		t.Error("handled another object's position")
	}
	ctl.Handle(positionMsg(mon, 42, pts[len(pts)-1], 0, 0.2, true)) // stopped at the hold-short
	if ctl.State() != TaxiHoldingShort {
		t.Fatalf("state %v, want holding short", ctl.State())
	}
	if err := ctl.ClearForTakeoff(); err != nil {
		t.Fatal(err)
	}
	if len(fc.waypoints) != 2 {
		t.Fatal("line-up waypoints not sent")
	}
	ctl.Handle(positionMsg(mon, 42, pts[len(pts)-1], 245, 80, true))
	ctl.Handle(positionMsg(mon, 42, pts[len(pts)-1], 245, 150, false))

	evs := drain(ctl.Events())
	want := []TaxiState{TaxiSpawning, TaxiPushback, TaxiTaxiing, TaxiHoldingShort, TaxiLiningUp, TaxiDeparting, TaxiComplete}
	if got := states(evs); !equalStates(got, want) {
		t.Fatalf("states %v, want %v", got, want)
	}
	var sawTaxiway, sawRemaining bool
	for _, e := range evs {
		if e.State == TaxiTaxiing && e.Taxiway != "" {
			sawTaxiway = true
		}
		if e.State == TaxiTaxiing && e.Remaining > 0 && e.Remaining < r.Length {
			sawRemaining = true
		}
	}
	if !sawTaxiway || !sawRemaining {
		t.Errorf("progress events: taxiway=%v remaining=%v", sawTaxiway, sawRemaining)
	}
	if fc.periods[len(fc.periods)-1] != types.SIMCONNECT_PERIOD_NEVER {
		t.Error("monitoring not stopped after take-off")
	}
	if _, ok := <-ctl.Events(); ok {
		t.Error("events channel not closed")
	}
	if ctl.Handle(positionMsg(mon, 42, pts[0], 0, 0, true)) {
		t.Error("handled a message after completion")
	}
}

func equalStates(a, b []TaxiState) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTaxiControllerCancel(t *testing.T) {
	ctl, fc, _ := startC22(t)
	ctl.Handle(assignedMsg(DefaultTaxiRequestBase, 42))
	if err := ctl.Cancel(); err != nil {
		t.Fatal(err)
	}
	if len(fc.removed) != 1 || fc.removed[0] != 42 || ctl.State() != TaxiCancelled {
		t.Errorf("removed=%v state=%v", fc.removed, ctl.State())
	}
	if err := ctl.Cancel(); err != nil {
		t.Error("second Cancel failed")
	}
	if err := ctl.ClearForTakeoff(); !errors.Is(err, ErrNotHoldingShort) {
		t.Errorf("ClearForTakeoff after cancel: %v", err)
	}
}

func TestTaxiControllerCancelBeforeSpawn(t *testing.T) {
	ctl, fc, _ := startC22(t)
	if err := ctl.Cancel(); err != nil || len(fc.removed) != 0 || ctl.State() != TaxiCancelled {
		t.Errorf("err=%v removed=%v state=%v", err, fc.removed, ctl.State())
	}
}

func TestTaxiControllerStuckWarning(t *testing.T) {
	ctl, _, _ := startC22(t)
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultTaxiRequestBase, 42))
	mon := DefaultTaxiRequestBase + reqOffMonitor
	p := ctl.Route().Points[0]
	ctl.Handle(positionMsg(mon, 42, p, 0, 0, true))
	now = now.Add(StuckTimeout + time.Second)
	ctl.Handle(positionMsg(mon, 42, p, 0, 0, true))
	ctl.Handle(positionMsg(mon, 42, p, 0, 0, true))
	stuck := 0
	for _, e := range drain(ctl.Events()) {
		if errors.Is(e.Err, ErrTaxiStuck) {
			stuck++
		}
	}
	if stuck != 1 || ctl.State() != TaxiPushback {
		t.Errorf("stuck warnings %d, state %v; want 1 warning, still pushback", stuck, ctl.State())
	}
}

func TestTaxiControllerStartErrors(t *testing.T) {
	g := lkprGraph(t)
	ctl := NewTaxiController(NewFleet(&fakeClient{}))
	for _, req := range []TaxiRequest{
		{Graph: nil, Model: "x", Runway: "24"},
		{Graph: g, Model: "", Runway: "24"},
		{Graph: g, Model: "x", Parking: -1, Runway: "24"},
	} {
		if err := ctl.Start(req); !errors.Is(err, ErrBadTaxiRequest) {
			t.Errorf("Start(%+v) = %v, want ErrBadTaxiRequest", req, err)
		}
	}
	if err := ctl.Start(TaxiRequest{Graph: g, Model: "x", Parking: 18, Runway: "18"}); !errors.Is(err, airport.ErrUnknownRunway) {
		t.Errorf("unknown runway: %v", err)
	}
	if err := NewTaxiController(NewFleet(nil)).Start(TaxiRequest{Graph: g, Model: "x", Parking: 18, Runway: "24"}); !errors.Is(err, ErrNotConnected) {
		t.Errorf("no client: %v", err)
	}
	if err := ctl.Start(TaxiRequest{Graph: g, Model: "x", Parking: 18, Runway: "24"}); err != nil {
		t.Fatal(err)
	}
	if err := ctl.Start(TaxiRequest{Graph: g, Model: "x", Parking: 18, Runway: "24"}); !errors.Is(err, ErrAlreadyStarted) {
		t.Errorf("second Start: %v", err)
	}
}
