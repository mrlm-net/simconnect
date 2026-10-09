package world

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestPlayerRunwayAnswer: the player's ATC asking to clear the user
// aircraft for take-off gets "hold position", number 2, while
// one of ours is lined up (live, OKRAX on 24 when the player was cleared),
// "free" on an empty runway, and the copy leaves the real controller as it
// was.
func TestPlayerRunwayAnswer(t *testing.T) {
	now := time.Date(2026, 10, 9, 16, 56, 0, 0, time.UTC)
	rc := traffic.NewRunwayController(traffic.RunwayControllerOptions{})
	okrax := traffic.RunwayUser{Callsign: "OKRAX", Wake: traffic.WakeFor("PC12"), Phase: traffic.RunwayLinedUp}
	rc.Decide(now, []traffic.RunwayUser{okrax})
	player := traffic.RunwayUser{Callsign: "CEF007", Wake: traffic.WakeFor("A320"), Phase: traffic.RunwayHoldingShort}
	tw := &towers{}

	d := rc.Clone().Decide(now.Add(time.Second), []traffic.RunwayUser{okrax, player})
	a := playerAnswer(tw, PlayerTakeoff, "CEF007", d)
	if a.Free || a.Say != "hold position" || a.Number != 2 {
		t.Errorf("OKRAX lined up: %+v, want hold position, number 2", a)
	}

	d = rc.Clone().Decide(now.Add(2*time.Second), []traffic.RunwayUser{player})
	if a := playerAnswer(tw, PlayerTakeoff, "CEF007", d); !a.Free {
		t.Errorf("empty runway: %+v, want free", a)
	}
	// Asking changed nothing: the real controller never saw CEF007 queue.
	d = rc.Decide(now.Add(3*time.Second), []traffic.RunwayUser{okrax})
	if _, waits := d.Waiting["CEF007"]; waits {
		t.Error("the query changed the runway's state")
	}
}

func TestTrafficSaid(t *testing.T) {
	for why, want := range map[string]string{
		"OKRAX on the runway":    "traffic on the runway",
		"AFR1 on a 2.4 NM final": "traffic on a 2 mile final",
		"AFR1 lands in 1m10s":    "landing traffic",
		"1m behind OKRAX":        "departing traffic",
		"number 2 for departure": "",
		"":                       "",
	} {
		if got := trafficSaid(why); got != want {
			t.Errorf("%q: %q, want %q", why, got, want)
		}
	}
}
