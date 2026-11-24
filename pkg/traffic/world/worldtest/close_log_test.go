package worldtest

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// slowCancelLog is a log whose "→ cancelled" lines take a while to write,
// as a busy machine's do: a World's last lines come late.
type slowCancelLog struct {
	lockedBuf
	delay time.Duration
}

func (l *slowCancelLog) Write(p []byte) (int, error) {
	if strings.Contains(string(p), "→ cancelled") {
		time.Sleep(l.delay)
	}
	return l.lockedBuf.Write(p)
}

// TestCloseLogsDone (#1701): a World's aircraft removed as it stops are
// logged ("→ cancelled") before Close returns, none after: the World
// started next took them into its own log (TestScheduleEnrouteArrivalsFitStands
// run after TestStaffedTowerLandStands saw that test's LKPR flights
// cancelled).
func TestCloseLogsDone(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	log := &slowCancelLog{delay: 100 * time.Millisecond}
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	o.World.Output = log
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	stand := func(label string) int {
		for i, p := range h.Layout.Parking {
			if p.Label() == label {
				return i
			}
		}
		h.Close()
		t.Fatalf("no stand %s", label)
		return -1
	}
	tails := []string{"BAW1334", "TVS1517", "TVS322"}
	for i, r := range []world.SpawnRequest{
		{Kind: "departure", ICAO: "LKPR", Stand: stand("A5"), Runway: "24", Model: "FSLTL A320 IAE BAW British Airways", Tail: tails[0], PushInMin: 30},
		{Kind: "departure", ICAO: "LKPR", Stand: stand("C27"), Runway: "24", Model: "FSLTL_FAIB_B738_TVS-Skytravel_TSOC", Tail: tails[1], PushInMin: 30},
		{Kind: "departure", ICAO: "LKPR", Stand: stand("C31"), Runway: "24", Model: "FSLTL_FAIB_B738_TVS-Skytravel_TSOC", Tail: tails[2], PushInMin: 30},
	} {
		if _, err := h.World.Do(http.MethodPost, "/api/control", r); err != nil {
			h.Close()
			t.Fatalf("%s: %v", tails[i], err)
		}
	}
	if !h.Until(2*time.Minute, func() bool {
		spawned := 0
		for _, v := range h.World.Snapshot().Aircraft {
			if v.State != "spawning" && v.State != "idle" {
				spawned++
			}
		}
		return spawned == len(tails)
	}) {
		h.Close()
		t.Fatalf("not all %d spawned in 2 minutes", len(tails))
	}
	h.Close()
	atClose := log.String()
	time.Sleep(time.Duration(len(tails)+5) * log.delay)
	// Its aircraft's lines; not the process's own, once for all Worlds
	// ("model sources: … titles on disk", read in the background).
	for _, l := range strings.Split(strings.TrimPrefix(log.String(), atClose), "\n") {
		for _, cs := range tails {
			if strings.Contains(l, "  "+cs+" ") {
				t.Errorf("logged after Close returned: %s", l)
			}
		}
	}
	for _, cs := range tails {
		found := false
		for _, l := range strings.Split(atClose, "\n") {
			if strings.Contains(l, "  "+cs+" ") && strings.Contains(l, "→ cancelled") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no \"→ cancelled\" line by Close", cs)
		}
	}
}
