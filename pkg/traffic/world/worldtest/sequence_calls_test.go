package worldtest

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestSequenceCallsPhrased: a busy hour of arrivals has no call that is a
// landing number alone and none saying "for spacing" twice (#1428; live,
// LKPR: "Skytravel 149, number 2"; "number 3, for spacing reduce speed to
// 180 knots, vectoring for ILS approach runway 06, fly heading 125, for
// spacing").
func TestSequenceCallsPhrased(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	h, err := Start(Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	off := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 12, Generator: &off}); err != nil {
		t.Fatal(err)
	}
	sta := h.Clock.Now().Add(25 * time.Minute)
	var fl []traffic.Flight
	for i, cs := range []string{"TVS149", "DLH1A", "CSA2B", "EZY3C", "WZZ4D", "RYR5E", "AFR6F", "KLM7G"} {
		at := sta.Add(time.Duration(i%2) * 10 * time.Second)
		fl = append(fl, traffic.Flight{Callsign: cs, Type: "A320", Origin: "EDDF", Destination: "LKPR", STD: at.Add(-60 * time.Minute), STA: at, DistanceNM: 220})
	}
	if err := h.World.AddFlights(fl); err != nil {
		t.Fatal(err)
	}
	h.Advance(50 * time.Minute)
	bare := regexp.MustCompile(`^[^,]+, number \d+$`)
	seq := 0
	for _, tx := range h.Heard() {
		if tx.Pilot || tx.Position != traffic.PosApproach {
			continue
		}
		if strings.Contains(tx.Text, ", number ") {
			seq++
		}
		if bare.MatchString(tx.Text) {
			t.Errorf("a number alone: %q", tx.Text)
		}
		if strings.Count(tx.Text, "for spacing") > 1 {
			t.Errorf("\"for spacing\" twice: %q", tx.Text)
		}
	}
	if seq == 0 {
		t.Fatal("no sequence numbers said: nothing checked")
	}
	t.Logf("%d calls with a number", seq)
}
