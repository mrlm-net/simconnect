package worldtest

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
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
