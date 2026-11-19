package worldtest

import (
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// staffedDeparture starts a World at LKPR with positions staffed, spawns
// one departure from C22 to runway 24 and gives it every clearance offered
// that give accepts, one each 3 s, until it is done (at most 40 minutes).
// It returns the harness, the World's log and the departure's states seen.
func staffedDeparture(t *testing.T, positions []string, give func(v world.ControlView, action string) bool) (*Harness, *lockedBuf, []string) {
	t.Helper()
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	log := &lockedBuf{}
	var out io.Writer = log
	if testing.Verbose() {
		out = io.MultiWriter(log, os.Stderr)
	}
	o.World.Output = out
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	var st []world.StaffedStation
	for _, p := range positions {
		st = append(st, world.StaffedStation{ICAO: "LKPR", Position: p})
	}
	h.World.SetStaffed(st)
	stand := -1
	for i, p := range h.Layout.Parking {
		if p.Label() == "C22" {
			stand = i
		}
	}
	if _, err := h.World.Do(http.MethodPost, "/api/control", world.SpawnRequest{Kind: "departure", ICAO: "LKPR", Stand: stand, Runway: "24", Procedure: true,
		Model: "FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery", Tail: "CSA8"}); err != nil {
		t.Fatal(err)
	}
	var states []string
	var given time.Time
	order := []string{"clearance", "handoff", "pushstart", "taxi", "takeoff", "lineup"}
	if !h.Until(40*time.Minute, func() bool {
		v, ok := h.Aircraft("CSA8")
		if !ok {
			return false
		}
		if len(states) == 0 || states[len(states)-1] != v.State {
			states = append(states, v.State)
		}
		if v.Done {
			return true
		}
		if now := traffic.Now(); now.Sub(given) >= 3*time.Second {
			for _, a := range order {
				if slices.Contains(v.Actions, a) && give(v, a) {
					given = now
					if err := h.World.Clear(v.ID, a); err != nil {
						t.Logf("%s refused: %v", a, err)
					}
					break
				}
			}
		}
		return false
	}) {
		t.Fatalf("CSA8 not done in 40 minutes: states %v", states)
	}
	return h, log, states
}

// TestRollingTakeoff (#1568): taxiing up to its runway with the runway free,
// a departure is offered the take-off clearance with the tower; given, it
// rolls through the holding point onto the runway and takes off without
// stopping, smoothly and with no conflict.
func TestRollingTakeoff(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	rolled := false
	h, log, states := staffedDeparture(t, []string{"delivery", "ground", "tower"}, func(v world.ControlView, a string) bool {
		switch a {
		case "takeoff":
			if v.State == "taxiing" {
				rolled = true
				return true
			}
			return false
		case "lineup":
			return false
		}
		return true
	})
	if !rolled {
		t.Fatalf("take-off never offered while taxiing: states %v", states)
	}
	if slices.Contains(states, "holding short") || slices.Contains(states, "lined up") {
		t.Errorf("stopped on the way onto the runway: states %v", states)
	}
	if !slices.Contains(states, "departing") {
		t.Errorf("never departed: states %v", states)
	}
	for _, l := range strings.Split(log.String(), "\n") {
		if strings.Contains(l, "conflict:") || strings.Contains(l, "separation:") {
			t.Errorf("conflict watch: %s", l)
		}
	}
	for _, j := range h.Sim.Jumps() {
		if j.Tail == "CSA8" {
			t.Errorf("jump: %+v", j)
		}
	}
}

// TestDepartureHandoverBeforeDone (#1572): with every position staffed and
// the tower player never handing the departure over once airborne, the
// tower still hands it to departure, with the readback, and the crew
// checks in with departure, before it is handed to the simulator's AI.
// Before, it climbed away silent.
func TestDepartureHandoverBeforeDone(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	h, _, states := staffedDeparture(t, []string{"delivery", "ground", "tower", "departure", "approach"}, func(v world.ControlView, a string) bool {
		return a != "lineup" && !(a == "handoff" && (v.State == "departing" || v.State == "complete"))
	})
	heard := h.Heard()
	from := slices.IndexFunc(heard, func(tx traffic.Transmission) bool {
		return tx.Callsign == "CSA8" && !tx.Pilot && strings.Contains(strings.ToLower(tx.Text), "cleared for take-off")
	})
	if from < 0 {
		t.Fatalf("never cleared for take-off: states %v", states)
	}
	handover, readback, checkIn := false, false, false
	for _, tx := range heard[from:] {
		if tx.Callsign != "CSA8" {
			continue
		}
		switch {
		case !tx.Pilot && tx.Position == traffic.PosTower && tx.Intent == traffic.IntentContact:
			handover = true
		case handover && tx.Pilot && tx.Position == traffic.PosTower:
			readback = true
		case tx.Pilot && tx.Position == traffic.PosDeparture && tx.Intent == traffic.IntentCheckIn:
			checkIn = true
		}
	}
	if !handover || !readback || !checkIn {
		t.Errorf("after take-off: tower hand-over %v, readback %v, departure check-in %v", handover, readback, checkIn)
	}
}
