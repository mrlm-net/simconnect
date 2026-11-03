package worldtest

import (
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestFrequencyKeptOverBlockChange: the radar frequency in use changes by the
// three-hour block (#772); an arrival already working radar stays on its
// frequency, not moved without a word (live, LKPR 2026-11-03 12:00 UTC:
// OKCMJ was suddenly on 118.31 instead of 119.01).
func TestFrequencyKeptOverBlockChange(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10,
		Start: time.Date(2026, 11, 3, 11, 45, 0, 0, time.UTC)}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	off := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 10, Generator: &off}); err != nil {
		t.Fatal(err)
	}
	sta := h.Clock.Now().Add(30 * time.Minute)
	if err := h.World.AddFlights([]traffic.Flight{
		{Callsign: "DLH1A", Type: "A320", Origin: "EDDF", Destination: "LKPR", STD: sta.Add(-60 * time.Minute), STA: sta, DistanceNM: 220},
	}); err != nil {
		t.Fatal(err)
	}
	h.Until(45*time.Minute, func() bool {
		a, ok := h.Aircraft("DLH1A")
		return ok && a.OnGround
	})
	freqs := map[string]bool{}
	before, after := 0, 0
	for _, tx := range h.Heard() {
		if tx.Callsign != "DLH1A" || tx.Pilot || tx.Position != traffic.PosApproach {
			continue
		}
		freqs[tx.Frequency] = true
		if tx.At.Hour() < 12 {
			before++
		} else {
			after++
		}
	}
	if before == 0 || after == 0 {
		t.Fatalf("approach calls %d before 12:00 UTC, %d after: the case not covered", before, after)
	}
	if len(freqs) != 1 {
		t.Errorf("approach calls to DLH1A on %v, want one frequency", freqs)
	}
}
