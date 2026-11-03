package worldtest

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// A taxi route over a runway nobody uses is cleared across it with the taxi,
// "cross runway 12", and the crew never stops there to ask (the user, live
// LKPR: FLBUB "taxi to and hold short of runway 06 via D, F" stopped at 12
// and waited for a crossing clearance).
func TestTaxiCrossesRunwayNotInUse(t *testing.T) {
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
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 14}); err != nil {
		t.Fatal(err)
	}
	h.Advance(60 * time.Minute)
	crossed := map[string]string{} // call sign → runway crossed with its taxi
	for _, tx := range h.Heard() {
		if !tx.Pilot && strings.Contains(tx.Text, "taxi") {
			if _, rest, ok := strings.Cut(tx.Text, ", cross runway "); ok {
				crossed[tx.Callsign] = strings.FieldsFunc(rest, func(r rune) bool { return r == ',' || r == ' ' })[0]
			}
		}
	}
	if len(crossed) == 0 {
		t.Fatal("no taxi clearance crossed a runway in an hour at LKPR")
	}
	for _, tx := range h.Heard() {
		rwy, ok := crossed[tx.Callsign]
		if ok && strings.Contains(tx.Text, "cross runway "+rwy) && !strings.Contains(tx.Text, "taxi") && !tx.Pilot {
			t.Errorf("%s cleared across %s with its taxi, cleared again: %q", tx.Callsign, rwy, tx.Text)
		}
		if ok && tx.Pilot && strings.Contains(tx.Text, "holding short of runway "+rwy) {
			t.Errorf("%s cleared across %s with its taxi, stopped there: %q", tx.Callsign, rwy, tx.Text)
		}
	}
	t.Logf("crossed with the taxi: %v", crossed)
}
