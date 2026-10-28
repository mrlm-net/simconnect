package worldtest

import (
	"os"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// staffedPair starts LKPR with approach staffed in mode and two arrivals
// due a minute apart on the same STAR, a TBM9 and a B738 behind it, their
// STARs and then approaches cleared by the player as they ask: the B738
// catches the TBM9 up, a conflict of the player's. It returns once both
// are cleared for the approach.
func staffedPair(t *testing.T, mode string) (*Harness, [2]string) {
	t.Helper()
	if testing.Short() {
		t.Skip("runs the World")
	}
	was := DefaultAircraftTitles
	DefaultAircraftTitles = append(slices.Clone(was), "FSLTL_GA_TBM9_ZZZ")
	t.Cleanup(func() { DefaultAircraftTitles = was })
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	h.World.SetStaffed([]world.StaffedStation{{ICAO: "LKPR", Position: "approach", Mode: mode}})
	off := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 10, Generator: &off}); err != nil {
		t.Fatal(err)
	}
	pair := [2]string{"OKBJZ", "LOT229"}
	sta := h.Clock.Now().Add(25 * time.Minute)
	if err := h.World.AddFlights([]traffic.Flight{
		{Callsign: pair[0], Type: "TBM9", Origin: "LZKZ", Destination: "LKPR", STD: sta.Add(-80 * time.Minute), STA: sta, DistanceNM: 349},
		{Callsign: pair[1], Airline: "RYR", Type: "B738", Origin: "EPWA", Destination: "LKPR", STD: sta.Add(-60 * time.Minute), STA: sta.Add(time.Minute), DistanceNM: 339},
	}); err != nil {
		t.Fatal(err)
	}
	cleared := map[string]bool{}
	if !h.Until(40*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if !slices.Contains(pair[:], v.Tail) {
				continue
			}
			for _, act := range []string{"clearance", "approach"} {
				if cleared[v.Tail+act] || !slices.Contains(v.Actions, act) || act == "approach" && !cleared[v.Tail+"clearance"] {
					continue
				}
				if err := h.World.Clear(v.ID, act); err == nil {
					cleared[v.Tail+act] = true
				}
			}
		}
		return cleared[pair[0]+"approach"] && cleared[pair[1]+"approach"]
	}) {
		t.Fatalf("%v not both cleared for the approach in 40 minutes", pair)
	}
	return h, pair
}

// aiTold are the instructions the AI said to cs after the player's
// clearances: sequence speeds, levels, crossing restrictions, headings,
// traffic information.
func aiTold(h *Harness, cs string) []traffic.Transmission {
	var out []traffic.Transmission
	for _, tx := range h.Heard() {
		if tx.Callsign != cs || tx.Pilot {
			continue
		}
		switch tx.Intent {
		case traffic.IntentSequence, traffic.IntentLevel, traffic.IntentCrossLevel, traffic.IntentTrafficInfo, traffic.IntentSpeed, traffic.IntentHeading:
			out = append(out, tx)
		}
	}
	return out
}

// TestStaffedAssist: two arrivals of a staffed approach (assist) in a
// conflict get nothing from the AI; the player has an alert while the
// conflict is predicted (#1096). Before, the conflict watch slowed the
// second one itself on the player's frequency ("number 2, for spacing
// reduce speed to 220 knots").
func TestStaffedAssist(t *testing.T) {
	h, pair := staffedPair(t, "assist")
	alerted := false
	for range 10 * 6 {
		h.Advance(10 * time.Second)
		for _, a := range h.World.Alerts() {
			if a.Staffed[pair[0]] == world.ModeAssist && a.Staffed[pair[1]] == world.ModeAssist && a.Deadline.IsZero() && !a.SafetyNet {
				alerted = true
			}
		}
	}
	if !alerted {
		t.Error("no alert for the pair in 10 minutes")
	}
	for _, cs := range pair {
		for _, tx := range aiTold(h, cs) {
			t.Errorf("the AI told %s on a staffed approach: %q", cs, tx.Text)
		}
	}
}

// TestStaffedSilent: in silent mode, no alert either.
func TestStaffedSilent(t *testing.T) {
	h, pair := staffedPair(t, "silent")
	for range 10 * 6 {
		h.Advance(10 * time.Second)
		if a := h.World.Alerts(); len(a) > 0 {
			t.Fatalf("alert on a silent station: %+v", a[0])
		}
	}
	for _, cs := range pair {
		for _, tx := range aiTold(h, cs) {
			t.Errorf("the AI told %s on a staffed approach: %q", cs, tx.Text)
		}
	}
}

// TestStaffedSafetyNet: the player doing nothing, the safety net resolves
// the conflict itself, not before its deadline, and marks what it says.
func TestStaffedSafetyNet(t *testing.T) {
	h, pair := staffedPair(t, "safety-net")
	var deadline time.Time
	for range 10 * 6 {
		h.Advance(10 * time.Second)
		for _, a := range h.World.Alerts() {
			if a.Staffed[pair[0]] == world.ModeSafetyNet && !a.Deadline.IsZero() {
				deadline = a.Deadline
			}
		}
		if !deadline.IsZero() && h.Clock.Now().After(deadline.Add(time.Minute)) {
			break
		}
	}
	if deadline.IsZero() {
		t.Fatal("no safety net alert with a deadline in 10 minutes")
	}
	var told []traffic.Transmission
	for _, cs := range pair {
		told = append(told, aiTold(h, cs)...)
	}
	if len(told) == 0 {
		t.Fatalf("the safety net did nothing by a minute past its deadline %s", deadline.Format("15:04:05"))
	}
	for _, tx := range told {
		if tx.At.Before(deadline) {
			t.Errorf("said before the deadline %s: %q at %s", deadline.Format("15:04:05"), tx.Text, tx.At.Format("15:04:05"))
		}
		if tx.Params["by"] != world.ModeSafetyNet {
			t.Errorf("not marked as the safety net's: %q", tx.Text)
		}
	}
}
