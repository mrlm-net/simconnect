package worldtest

import (
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestVacateSaidOnce: naming an arrival's exit is one call from the tower
// with the exit in it, and the arrival leaves by it; without a node it is
// a 400, as for upto (#1319).
func TestVacateSaidOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 6}); err != nil {
		t.Fatal(err)
	}
	var arr world.ControlView
	if !h.Until(30*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "arrival" && !v.Done && slices.Contains(v.Actions, "vacate") {
				arr = v
				return true
			}
		}
		return false
	}) {
		t.Fatal("no arrival offered vacate in 30 simulated minutes")
	}
	g, err := airport.BuildGraph(h.Layout)
	if err != nil {
		t.Fatal(err)
	}
	exits, err := g.RunwayExits(arr.Runway)
	if err != nil || len(exits) == 0 {
		t.Fatalf("exits of %s: %v, %v", arr.Runway, exits, err)
	}
	x := exits[len(exits)-1]
	if err := h.World.Clear(arr.ID, "vacate"); err == nil || !strings.Contains(err.Error(), "400") {
		t.Errorf("no node: %v, want 400", err)
	}
	if err := h.World.Clear(arr.ID, fmt.Sprintf("vacate?node=%d", x.Node)); err != nil {
		t.Fatal(err)
	}
	h.Advance(20 * time.Second)
	var said []string
	for _, tx := range h.Heard() {
		if tx.Callsign == arr.Tail && !tx.Pilot && tx.Intent == traffic.IntentVacate {
			said = append(said, string(tx.Position)+": "+tx.Text)
		}
	}
	if len(said) != 1 || !strings.HasPrefix(said[0], "tower: ") || !strings.HasSuffix(said[0], "via "+x.Taxiway) {
		t.Fatalf("said %q, want the tower once, with %s", said, x.Taxiway)
	}
	if err := h.World.Clear(arr.ID, "land"); err != nil {
		t.Fatal(err)
	}
	if !h.Until(30*time.Minute, func() bool { return h.Said(arr.Tail, "vacated at "+x.Taxiway) }) {
		t.Errorf("did not report vacated at %s", x.Taxiway)
	}
}

// TestVacateOnTheRoll: on the landing roll vacate is on offer; an exit behind
// or out of reach is refused saying why; a far one is taken with the speed
// carrying on, and the aircraft leaves there. A side alone takes an exit on
// that side (#1330).
func TestVacateOnTheRoll(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 3, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	g, err := airport.BuildGraph(h.Layout)
	if err != nil {
		t.Fatal(err)
	}
	var arr world.ControlView
	if !h.Until(30*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "arrival" && v.State == "rollout" && v.GroundSpeed > 90 {
				arr = v
				return true
			}
		}
		return false
	}) {
		t.Fatal("no arrival on the roll")
	}
	if !slices.Contains(arr.Actions, "vacate") {
		t.Errorf("vacate not on offer on the roll: %v", arr.Actions)
	}
	exits, err := g.RunwayExits(arr.Runway)
	if err != nil {
		t.Fatal(err)
	}
	_, end, _ := h.Layout.RunwayEnd(arr.Runway)
	along := calc.HaversineMeters(end.Threshold.Lat, end.Threshold.Lon, arr.Position.Lat, arr.Position.Lon)
	post := func(q string) error {
		_, err := h.World.Do("POST", fmt.Sprintf("/api/control/%d/vacate?%s", arr.ID, q), nil)
		return err
	}
	var near, behind *airport.RunwayExit
	for i := range exits {
		x := &exits[i]
		if x.Along < along-100 {
			behind = x
		} else if near == nil && x.Along > along+60 {
			near = x
		}
	}
	t.Logf("%s at %.0f kt, %.0f m along %s; %d exits", arr.Tail, arr.GroundSpeed, along, arr.Runway, len(exits))
	if behind != nil {
		if err := post(fmt.Sprintf("node=%d", behind.Node)); err == nil || !strings.Contains(err.Error(), "past") {
			t.Errorf("exit behind it: %v", err)
		}
	}
	if near != nil && near.Along-along < 200 {
		if err := post(fmt.Sprintf("node=%d", near.Node)); err == nil || !strings.Contains(err.Error(), "too fast") {
			t.Errorf("exit %.0f m ahead at %.0f kt: %v", near.Along-along, arr.GroundSpeed, err)
		}
	}
	far := exits[len(exits)-1]
	was, _ := h.Aircraft(arr.Tail)
	if err := post(fmt.Sprintf("node=%d", far.Node)); err != nil {
		t.Fatal(err)
	}
	h.Advance(time.Second / 5)
	if v, ok := h.Aircraft(arr.Tail); !ok || v.State != "rollout" || math.Abs(v.GroundSpeed-was.GroundSpeed) > 5 {
		t.Errorf("after the new exit: %s at %.0f kt, was %.0f", v.State, v.GroundSpeed, was.GroundSpeed)
	}
	var off world.ControlView
	if !h.Until(5*time.Minute, func() bool {
		v, ok := h.Aircraft(arr.Tail)
		off = v
		return ok && v.State != "rollout"
	}) {
		t.Fatalf("still on the roll: %+v", off)
	}
	if d := calc.HaversineMeters(off.Position.Lat, off.Position.Lon, g.Nodes[far.Node].Position.Lat, g.Nodes[far.Node].Position.Lon); d > 200 {
		t.Errorf("left the runway %.0f m from the exit named (%s)", d, off.State)
	}

	// The next one, a side alone.
	var next world.ControlView
	if !h.Until(60*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "arrival" && v.Tail != arr.Tail && v.State == "rollout" {
				next = v
				return true
			}
		}
		return false
	}) {
		t.Fatal("no second arrival on the roll")
	}
	arr = next
	side := far.Side.String()
	if err := post("side=" + side); err != nil {
		t.Fatalf("side %s: %v", side, err)
	}
	if v, _ := h.Aircraft(arr.Tail); !strings.Contains(v.Instruction, "vacate "+side) {
		t.Errorf("instruction %q, want side %s", v.Instruction, side)
	}
}

// TestVacateTooTight: an exit whose only path bends back sharply is refused as
// too tight to vacate, on the final as on the roll (#1330).
func TestVacateTooTight(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	h.World.UseRunway("LKPR", "12")
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 3, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	g, err := airport.BuildGraph(h.Layout)
	if err != nil {
		t.Fatal(err)
	}
	_, end, _ := h.Layout.RunwayEnd("12")
	exits, err := g.RunwayExits("12")
	if err != nil {
		t.Fatal(err)
	}
	node := airport.NodeID(-1)
	for _, x := range exits {
		if g.ExitTight(x, end.Heading, airport.RouteOptions{}) == "" {
			continue
		}
		onClean := slices.ContainsFunc(exits, func(y airport.RunwayExit) bool {
			return g.ExitTight(y, end.Heading, airport.RouteOptions{}) == "" && (y.Node == x.Node || y.RunwayNode == x.Node || slices.Contains(y.Path, x.Node))
		})
		if !onClean {
			node = x.Node
		}
	}
	if node < 0 {
		t.Fatal("no exit off LKPR 12 only by a hook: the data changed")
	}
	var arr world.ControlView
	if !h.Until(30*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "arrival" && v.Runway == "12" && slices.Contains(v.Actions, "vacate") {
				arr = v
				return true
			}
		}
		return false
	}) {
		t.Fatal("no arrival for 12 offered vacate")
	}
	_, err = h.World.Do("POST", fmt.Sprintf("/api/control/%d/vacate?node=%d", arr.ID, node), nil)
	if err == nil || !strings.Contains(err.Error(), "too tight") {
		t.Errorf("a hooked exit: %v", err)
	}
}
