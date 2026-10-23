package worldtest

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestStaffedDeliveryGivesTheClearance: with delivery staffed, a departure
// asks for its clearance and the World does not answer (#1156); the view
// names delivery, the position that may clear it (#1155), and offers the
// "clearance" action, which gives it. Delivery unstaffed, the World gives
// the next one itself.
func TestStaffedDeliveryGivesTheClearance(t *testing.T) {
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
	h.World.SetStaffed([]world.StaffedStation{{ICAO: "LKPR", Position: "delivery", Mode: "silent"}})
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 6}); err != nil {
		t.Fatal(err)
	}
	cleared := func(tail string) bool {
		for _, tx := range h.Heard() {
			if tx.Callsign == tail && tx.Intent == traffic.IntentDepartureClearance {
				return true
			}
		}
		return false
	}
	var dep world.ControlView
	if !h.Until(10*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "departure" && !v.Done && slices.Contains(v.Actions, "clearance") {
				dep = v
				return true
			}
		}
		return false
	}) {
		t.Fatal("no departure waiting for its clearance in 10 simulated minutes")
	}
	h.Advance(30 * time.Second)
	if cleared(dep.Tail) {
		t.Fatalf("%s cleared by the World on a staffed delivery", dep.Tail)
	}
	for _, v := range h.World.Snapshot().Aircraft {
		if v.Tail == dep.Tail && v.ATC != "delivery" {
			t.Errorf("%s shown with %s, still delivery's", v.Tail, v.ATC)
		}
	}
	if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/clearance?as=delivery", dep.ID), nil); err != nil {
		t.Fatalf("clearance as delivery: %v", err)
	}
	if !h.Until(time.Minute, func() bool { return cleared(dep.Tail) }) {
		t.Fatalf("%s: the player's clearance never said", dep.Tail)
	}
	// Unstaffed: the World clears those still waiting, and the next.
	h.World.SetStaffed(nil)
	h.Advance(2 * time.Minute)
	for _, v := range h.World.Snapshot().Aircraft {
		if v.Kind == "departure" && slices.Contains(v.Actions, "clearance") {
			t.Errorf("%s still waits for its clearance, delivery unstaffed", v.Tail)
		}
	}
}

// TestCheapReadAccessors: Now, PlayerPlace and Vehicles answer as the
// Snapshot does, without its views (an app polling twice a second).
func TestCheapReadAccessors(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 6}); err != nil {
		t.Fatal(err)
	}
	if !h.Until(10*time.Minute, func() bool { return len(h.World.Snapshot().Aircraft) > 0 }) {
		t.Fatal("no aircraft")
	}
	if d := h.World.Now().Sub(h.World.Snapshot().At); d < -time.Second || d > time.Second {
		t.Errorf("Now %v off the snapshot's time", d)
	}
	if h.World.PlayerPlace() != h.World.Snapshot().Player {
		t.Error("player place differs")
	}
	want := map[string]int{}
	for _, v := range h.World.Snapshot().Aircraft {
		if len(v.Vehicles) > 0 {
			want[v.Tail] = len(v.Vehicles)
		}
	}
	got := h.World.Vehicles()
	for tail, n := range want {
		if len(got[tail]) != n {
			t.Errorf("%s: %d vehicles, the snapshot has %d", tail, len(got[tail]), n)
		}
	}
}

// TestHarnessesOneAfterAnother: a World per subtest, each with its own
// output, closed as the next starts (an app's offline scenario tests):
// nothing shared between them races (-race).
func TestHarnessesOneAfterAnother(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	for i := range 3 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
			o.World.Output = io.Discard
			h, err := Start(o)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(h.Close)
			if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 8}); err != nil {
				t.Fatal(err)
			}
			h.Until(5*time.Minute, func() bool { return len(h.World.Snapshot().Aircraft) > 2 })
			h.Advance(time.Minute)
		})
	}
}

// TestStaffedDeliveryAmendsTheClearance: the player's clearance off
// another runway with a lower initial climb (#1164) re-plans the departure
// (the SID off that runway) and says what was applied; a SID not off that
// runway is refused, the clearance still waiting.
func TestStaffedDeliveryAmendsTheClearance(t *testing.T) {
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
	h.World.SetStaffed([]world.StaffedStation{{ICAO: "LKPR", Position: "delivery", Mode: "silent"}})
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 6}); err != nil {
		t.Fatal(err)
	}
	var dep world.ControlView
	if !h.Until(10*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "departure" && !v.Done && slices.Contains(v.Actions, "clearance") {
				dep = v
				return true
			}
		}
		return false
	}) {
		t.Fatal("no departure waiting for its clearance in 10 simulated minutes")
	}
	to := "06"
	if dep.Runway == "06" {
		to = "24"
	}
	if err := h.World.GiveClearance(dep.ID, world.ClearanceAmend{Runway: to, SID: "NOSUCH1A"}); err == nil {
		t.Fatal("a SID not off the runway accepted")
	}
	if err := h.World.GiveClearance(dep.ID, world.ClearanceAmend{Runway: to, LevelFt: 4000}); err != nil {
		t.Fatalf("amended clearance: %v", err)
	}
	var said *traffic.Transmission
	if !h.Until(time.Minute, func() bool {
		for _, tx := range h.Heard() {
			if tx.Callsign == dep.Tail && tx.Intent == traffic.IntentDepartureClearance {
				said = &tx
				return true
			}
		}
		return false
	}) {
		t.Fatalf("%s: the amended clearance never said", dep.Tail)
	}
	t.Logf("%s filed runway %s %s: %q", dep.Tail, dep.Runway, dep.Procedure, said.Text)
	if said.Params[traffic.ParamRunway] != to || said.Params[traffic.ParamLevel] != "4000 feet" {
		t.Errorf("said runway %q level %q, want %s and 4000 feet", said.Params[traffic.ParamRunway], said.Params[traffic.ParamLevel], to)
	}
	for _, v := range h.World.Snapshot().Aircraft {
		if v.Tail == dep.Tail && (v.Runway != to || (v.Procedure != "" && said.Params[traffic.ParamSID] == "")) {
			t.Errorf("%s planned runway %s %s, said %q", v.Tail, v.Runway, v.Procedure, said.Params[traffic.ParamSID])
		}
	}
}

// TestApproachDescend: an arrival on its STAR told to descend (#1164) is
// said so, against the transition altitude; a level above it is refused.
func TestApproachDescend(t *testing.T) {
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
	f := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 6, VFR: &f}); err != nil {
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
		t.Fatal("no arrival on its STAR above 9000 ft in 20 simulated minutes")
	}
	if err := h.World.ApproachDescend("LKPR", arr.Tail, arr.AltFt+2000); err == nil {
		t.Error("a climb accepted as a descent")
	}
	ft := float64(int(arr.AltFt-2000) / 1000 * 1000)
	if err := h.World.ApproachDescend("LKPR", arr.Tail, ft); err != nil {
		t.Fatalf("%s at %.0f ft, descend to %.0f: %v", arr.Tail, arr.AltFt, ft, err)
	}
	if !h.Until(time.Minute, func() bool {
		for _, tx := range h.Heard() {
			if tx.Callsign == arr.Tail && tx.Intent == traffic.IntentLevel && tx.Params[traffic.ParamClimb] == "descend" {
				t.Logf("%q", tx.Text)
				return true
			}
		}
		return false
	}) {
		t.Fatalf("%s: no descent said", arr.Tail)
	}
}
