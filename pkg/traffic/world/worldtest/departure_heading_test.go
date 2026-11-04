package worldtest

import (
	"fmt"
	"math"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestStaffedDepartureHeading: on a staffed departure the player turns a
// departure handed over onto a heading, said, and it flies it (#1414).
func TestStaffedDepartureHeading(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	h, err := Start(Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	h.World.SetStaffed([]world.StaffedStation{{ICAO: "LKPR", Position: "departure", Mode: "silent"}})
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 3, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	var dep world.ControlView
	// Past a stand delay the first one may ask for (up to 15 minutes,
	// WORLDTEST_SEED=1257411435651082003: TVS1985 asked to push at 19
	// minutes).
	if !h.Until(45*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "departure" && slices.Contains(v.Actions, "heading") {
				dep = v
				return true
			}
		}
		return false
	}) {
		t.Fatal("no departure on the staffed departure offered a heading")
	}
	if dep.ActionParams["heading"] != "heading" {
		t.Errorf("its parameter: %v", dep.ActionParams)
	}
	if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/heading", dep.ID), nil); err == nil {
		t.Error("a heading of nothing taken")
	}
	if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/heading?heading=90", dep.ID), nil); err != nil {
		t.Fatalf("heading: %v", err)
	}
	h.Advance(time.Second)
	if !h.Said(dep.Tail, "heading 090") {
		t.Error("the heading not said")
	}
	// A level given in the turn does not undo it.
	if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/climb?level=24000", dep.ID), nil); err != nil {
		t.Fatalf("climb: %v", err)
	}
	h.Advance(2 * time.Minute)
	v, ok := h.Aircraft(dep.Tail)
	if !ok {
		t.Fatal("gone")
	}
	// LKPR's magnetic variation is a few degrees.
	if off := math.Abs(math.Mod(v.Heading-90+540, 360) - 180); off > 15 {
		t.Errorf("flying %.0f° two minutes after heading 090", v.Heading)
	}
	// A speed, then its planned ones again.
	if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/speed?kts=100", dep.ID), nil); err == nil {
		t.Error("100 kt taken")
	}
	was := v.GroundSpeed
	if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/speed?kts=200", dep.ID), nil); err != nil {
		t.Fatalf("speed: %v", err)
	}
	h.Advance(90 * time.Second)
	if !h.Said(dep.Tail, "200 knots") {
		t.Error("the speed not said")
	}
	if v, _ := h.Aircraft(dep.Tail); math.Abs(v.GroundSpeed-200) > 10 {
		t.Errorf("at %.0f kt 90 s after speed 200 (was %.0f)", v.GroundSpeed, was)
	}
	if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/speed?kts=0", dep.ID), nil); err != nil {
		t.Fatalf("resume: %v", err)
	}
	h.Advance(90 * time.Second)
	if v, _ := h.Aircraft(dep.Tail); v.GroundSpeed < was-10 {
		t.Errorf("at %.0f kt after resuming, was %.0f", v.GroundSpeed, was)
	}
}
