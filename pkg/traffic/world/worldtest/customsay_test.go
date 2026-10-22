package worldtest

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestSayCustomCrewOnOwnFrequency: a scene's own words: the crew's line
// with the controller's answer as written, on a frequency no station of
// ours has (Praha Control, en route), and a controller's call answered
// "Roger" by default.
func TestSayCustomCrewOnOwnFrequency(t *testing.T) {
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
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 4}); err != nil {
		t.Fatal(err)
	}
	if !h.Until(10*time.Minute, func() bool { return len(h.World.Snapshot().Aircraft) > 0 }) {
		t.Fatal("no aircraft in 10 simulated minutes")
	}
	tail := h.World.Snapshot().Aircraft[0].Tail
	cast := map[string]string{"x": tail}
	crew := world.ScenarioAction{Do: "say", Kind: "custom", Who: "x", Args: map[string]string{
		"from": "pilot", "text": "request to dump fuel", "reply": "fuel dumping approved", "frequency": "132.89"}}
	if err := h.World.Act("LKPR", crew, cast); err != nil {
		t.Fatal(err)
	}
	atc := world.ScenarioAction{Do: "say", Kind: "custom", Who: "x", Args: map[string]string{"text": "reduce speed 180 knots"}}
	if err := h.World.Act("LKPR", atc, cast); err != nil {
		t.Fatal(err)
	}
	h.Advance(30 * time.Second)
	var gotCrew, gotReply, gotRoger bool
	for _, tx := range h.Heard() {
		switch {
		case tx.Text == "request to dump fuel":
			gotCrew = tx.Pilot && tx.Frequency == "132.89"
		case tx.Text == "fuel dumping approved":
			gotReply = !tx.Pilot && tx.Frequency == "132.89"
		case strings.HasPrefix(tx.Text, "Roger, ") && tx.Callsign == tail:
			gotRoger = tx.Pilot
		}
	}
	if !gotCrew || !gotReply || !gotRoger {
		for _, tx := range h.Heard() {
			t.Logf("%s pilot=%v %s: %s", tx.Frequency, tx.Pilot, tx.Callsign, tx.Text)
		}
		t.Errorf("crew line %v, its answer %v, roger %v", gotCrew, gotReply, gotRoger)
	}
}
