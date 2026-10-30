package worldtest

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestMainEntranceFirst: with one departure from A5 heading for runway
// 24 at A, two from LKPR's C stands still taxi to A, the main entrance at
// its end; Z, the shorter taxi from there, is the overflow once A's line
// is full (three). Live, TVS1517 and TVS322 both went to Z, BAW1334 the
// only one heading for A.
func TestMainEntranceFirst(t *testing.T) {
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
	stand := func(label string) int {
		for i, p := range h.Layout.Parking {
			if p.Label() == label {
				return i
			}
		}
		t.Fatalf("no stand %s", label)
		return -1
	}
	pair := []string{"TVS1517", "TVS322"}
	for _, r := range []world.SpawnRequest{
		{Kind: "departure", ICAO: "LKPR", Stand: stand("A5"), Runway: "24", Model: "FSLTL A320 IAE BAW British Airways", Tail: "BAW1334", PushInMin: 1},
		{Kind: "departure", ICAO: "LKPR", Stand: stand("C27"), Runway: "24", Model: "FSLTL_FAIB_B738_TVS-Skytravel_TSOC", Tail: pair[0], PushInMin: 3},
		{Kind: "departure", ICAO: "LKPR", Stand: stand("C31"), Runway: "24", Model: "FSLTL_FAIB_B738_TVS-Skytravel_TSOC", Tail: pair[1], PushInMin: 3},
	} {
		if _, err := h.World.Do(http.MethodPost, "/api/control", r); err != nil {
			t.Fatalf("%s: %v", r.Tail, err)
		}
	}
	taxi := map[string]string{}
	if !h.Until(30*time.Minute, func() bool {
		for _, tx := range h.Heard() {
			if !tx.Pilot && tx.Intent == traffic.IntentTaxi && (tx.Callsign == pair[0] || tx.Callsign == pair[1]) {
				taxi[tx.Callsign] = tx.Text
			}
		}
		return len(taxi) == 2
	}) {
		t.Fatalf("not both cleared to taxi in 30 minutes: %v", taxi)
	}
	for _, cs := range pair {
		route, _, _ := strings.Cut(taxi[cs], ", follow ") // said with the route when one is ahead
		if !strings.HasSuffix(route, ", A") && !strings.HasSuffix(route, " via A") {
			t.Errorf("%s: %q, want runway 24 at A", cs, taxi[cs])
		}
	}
}
