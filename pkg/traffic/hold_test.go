package traffic

import (
	"encoding/binary"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func TestHoldFigures(t *testing.T) {
	if HoldSpeedKts(9000) != 230 || HoldSpeedKts(16000) != 240 || HoldSpeedKts(30000) != 265 || HoldSpeedKts(38000) != 280 {
		t.Error("holding speeds")
	}
	if HoldLegTime(14000) != time.Minute || HoldLegTime(15000) != 90*time.Second {
		t.Error("leg times")
	}
	if r := TurnRadiusNM(230); math.Abs(r-1.22) > 0.01 {
		t.Errorf("rate-one radius at 230 kt: %.2f NM", r)
	}
}

func TestHoldEntry(t *testing.T) {
	right := Hold{InboundTrue: 0}
	left := Hold{InboundTrue: 0, LeftTurns: true}
	for _, c := range []struct {
		h    Hold
		hdg  float64
		want HoldEntry
	}{
		{right, 0, EntryDirect}, {right, 90, EntryDirect}, {right, 110, EntryDirect}, {right, 300, EntryDirect},
		{right, 150, EntryTeardrop}, {right, 180, EntryTeardrop},
		{right, 200, EntryParallel}, {right, 280, EntryParallel},
		{left, 0, EntryDirect}, {left, 250, EntryDirect}, {left, 210, EntryTeardrop}, {left, 150, EntryParallel},
	} {
		if got := c.h.Entry(c.hdg); got != c.want {
			t.Errorf("left %v, heading %.0f: %v, want %v", c.h.LeftTurns, c.hdg, got, c.want)
		}
	}
}

func TestRacetrack(t *testing.T) {
	fix := airport.LatLon{Lat: 50, Lon: 14}
	h := Hold{Fix: fix, InboundTrue: 0} // right turns: the pattern lies east
	pts := h.Racetrack(5000)
	if pts[len(pts)-1] != fix {
		t.Fatal("the racetrack does not end at the fix")
	}
	length, far := 0.0, 0.0
	prev := fix
	for _, p := range pts {
		length += calc.HaversineNM(prev.Lat, prev.Lon, p.Lat, p.Lon)
		prev = p
		if p.Lon < fix.Lon-1e-6 {
			t.Errorf("point %v on the non-holding side", p)
		}
		far = math.Max(far, calc.HaversineNM(fix.Lat, fix.Lon, p.Lat, p.Lon))
	}
	leg := 230.0 / 60 // 1 min at 230 kt
	if want := 2*leg + 2*math.Pi*TurnRadiusNM(230); math.Abs(length-want) > 1 {
		t.Errorf("lap %.1f NM, want about %.1f", length, want)
	}
	if far < leg || far > leg+3 {
		t.Errorf("farthest %.1f NM from the fix", far)
	}
	// Left turns: the pattern lies west.
	for _, p := range (Hold{Fix: fix, InboundTrue: 0, LeftTurns: true}).Racetrack(5000) {
		if p.Lon > fix.Lon+1e-6 {
			t.Fatalf("left-hand hold point %v east of the fix", p)
		}
	}
}

func TestHoldStack(t *testing.T) {
	s := &HoldStack{BaseFt: 5000}
	if a := s.Assign("A"); a != 5000 {
		t.Fatalf("A at %.0f", a)
	}
	s.Assign("B")
	if c := s.Assign("C"); c != 7000 {
		t.Fatalf("C at %.0f", c)
	}
	if again := s.Assign("B"); again != 6000 {
		t.Fatalf("B again at %.0f", again)
	}
	moved := s.Release("A")
	if len(moved) != 2 || moved["B"] != 5000 || moved["C"] != 6000 {
		t.Fatalf("after A leaves: %v", moved)
	}
	if l := s.Aircraft(); len(l) != 2 || l[0].Callsign != "B" {
		t.Fatalf("stack %v", l)
	}
}

// lastWaypoint decodes the last waypoint of the last chain sent (the
// packed wire format, engine.PackWaypoints).
func lastWaypoint(ec *eventClient) types.SIMCONNECT_DATA_WAYPOINT {
	const size = engine.WaypointWireSize
	for i := len(ec.waypoints) - 1; i >= 0; i-- {
		b := ec.waypoints[i]
		if len(b) >= size && len(b)%size == 0 {
			w := b[len(b)-size:]
			f := func(o int) float64 { return math.Float64frombits(binary.LittleEndian.Uint64(w[o:])) }
			return types.SIMCONNECT_DATA_WAYPOINT{Latitude: f(0), Longitude: f(8), Altitude: f(16), Flags: binary.LittleEndian.Uint32(w[24:]), KtsSpeed: f(28)}
		}
	}
	return types.SIMCONNECT_DATA_WAYPOINT{}
}

// TestArrivalHolds: an arrival on GOLOP 4T holds at a STAR point, loops
// the wrapped racetrack after its entry lap, and goes on along the STAR
// when it leaves.
func TestArrivalHolds(t *testing.T) {
	g := lkprGraph(t)
	route, err := lkprProcedures(t).Arrival("06", "GOLOP")
	if err != nil {
		t.Fatal(err)
	}
	ec := &eventClient{}
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "06", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA8",
		InjectApproach: true, Procedure: route}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	mon := DefaultArrivalRequestBase + arrReqMonitor
	ctl.Handle(arrivalPositionMsg(mon, 77, route[0].Position, 9000, 90, 250, false))
	h, ok := ctl.HoldFix(15)
	if !ok {
		t.Fatal("no hold fix on GOLOP 4T")
	}
	if len(h.Ident) < 2 || h.Ident[:2] == "WP" {
		t.Errorf("hold fix named %q, want the STAR fix", h.Ident)
	}
	e, err := ctl.EnterHold(h, 7000)
	if err != nil {
		t.Fatal(err)
	}
	if w := lastWaypoint(ec); w.Flags&uint32(types.SIMCONNECT_WAYPOINT_WRAP_TO_FIRST) != 0 {
		t.Fatal("the entry chain wraps")
	}
	if _, err := ctl.AbsorbDelay(time.Minute); !errors.Is(err, ErrHolding) {
		t.Errorf("absorbing while holding: %v", err)
	}
	if r := ctl.ProcedureRoute(); len(r) == 0 || r[0] != h.Fix {
		t.Error("while holding the route goes on from the fix")
	}
	// A minute later, back over the fix: the racetrack loops.
	now = now.Add(2 * time.Minute)
	ctl.Handle(arrivalPositionMsg(mon, 77, h.Fix, 7000, h.InboundTrue, 230, false))
	if w := lastWaypoint(ec); w.Flags&uint32(types.SIMCONNECT_WAYPOINT_WRAP_TO_FIRST) == 0 || math.Abs(w.Altitude-7000) > 1 {
		t.Fatalf("after the %v entry lap: no wrapped racetrack at 7000 ft (%+v)", e, w)
	}
	if err := ctl.HoldAltitude(6000); err != nil {
		t.Fatal(err)
	}
	if w := lastWaypoint(ec); math.Abs(w.Altitude-6000) > 1 {
		t.Errorf("stepped down to %.0f", w.Altitude)
	}
	if err := ctl.LeaveHold(); err != nil {
		t.Fatal(err)
	}
	if _, _, holding := ctl.Holding(); holding {
		t.Fatal("still holding")
	}
	if w := lastWaypoint(ec); w.Flags&uint32(types.SIMCONNECT_WAYPOINT_WRAP_TO_FIRST) != 0 {
		t.Error("the STAR chain after the hold wraps")
	}
	if err := ctl.LeaveHold(); !errors.Is(err, ErrNotHolding) {
		t.Errorf("leaving twice: %v", err)
	}
}
