package worldtest

import (
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestApproachHeading: a controller takes an arrival off its STAR on a heading,
// said and read back, and it flies it; it refuses no heading; and the arrival is
// put back on its way (#1414).
func TestApproachHeading(t *testing.T) {
	h := startLKPR(t)
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 3, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	var arr world.ControlView
	if !h.Until(20*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "arrival" && v.State == "approaching" && v.AltFt > 9000 {
				arr = v
				return true
			}
		}
		return false
	}) {
		t.Fatal("no arrival on its STAR above 9000 ft")
	}
	do := func(action string, body map[string]any) error {
		_, err := h.World.Do(http.MethodPost, "/api/approach/LKPR/"+arr.Tail+"/"+action, body)
		return err
	}
	// In the landing sequence a moment after it appears.
	var err error
	if !h.Until(2*time.Minute, func() bool {
		err = do("heading", map[string]any{"heading": 180})
		return err == nil
	}) {
		t.Fatal(err)
	}
	h.Advance(time.Second)
	if !h.Said(arr.Tail, "heading 180") {
		t.Error("the heading not said")
	}
	h.Advance(2 * time.Minute)
	v, ok := h.Aircraft(arr.Tail)
	if !ok {
		t.Fatal("gone")
	}
	// LKPR's magnetic variation is a few degrees.
	if off := math.Abs(math.Mod(v.Heading-180+540, 360) - 180); off > 15 {
		t.Errorf("flying %.0f° two minutes after heading 180", v.Heading)
	}
	if v.Instruction != "heading 180" {
		t.Errorf("its card says %q", v.Instruction)
	}
	if err := do("heading", map[string]any{}); err == nil {
		t.Error("a heading of nothing taken")
	}
	if err := do("direct", nil); err != nil {
		t.Errorf("not put back on its way: %v", err)
	}
}
