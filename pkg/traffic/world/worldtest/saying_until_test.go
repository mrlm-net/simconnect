package worldtest

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestSayingUntil: a clearance given is shown as queued until it has been said,
// and no longer after (#1312).
func TestSayingUntil(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	h, err := Start(Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 6}); err != nil {
		t.Fatal(err)
	}
	var dep world.ControlView
	if !h.Until(10*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "departure" && !v.Done && v.ATC == "ground" && v.Request == "pushback" {
				dep = v
				return true
			}
		}
		return false
	}) {
		t.Fatal("no departure asking to push in 10 simulated minutes")
	}
	// `upto` needs its limit, and the offer and the refusal say so (#1308).
	if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/upto", dep.ID), nil); err == nil || !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "node") {
		t.Errorf("upto with no limit: %v", err)
	}
	if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/pushback", dep.ID), nil); err != nil {
		t.Fatal(err)
	}
	now := h.Clock.Now()
	v, _ := h.Aircraft(dep.Tail)
	if v.SayingUntil == nil || !v.SayingUntil.After(now) {
		t.Fatalf("%s just cleared at %v: saying until %v", dep.Tail, now, v.SayingUntil)
	}
	h.Advance(v.SayingUntil.Sub(now) + 5*time.Second)
	if v, _ := h.Aircraft(dep.Tail); v.SayingUntil != nil && !v.SayingUntil.After(h.Clock.Now()) {
		t.Errorf("%s all said: saying until %v, now %v", dep.Tail, v.SayingUntil, h.Clock.Now())
	}
}
