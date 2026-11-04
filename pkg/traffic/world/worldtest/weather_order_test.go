package worldtest

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestWeatherAskedAfterCheckInAnswer: a crew asks for the weather only once
// its check-in is answered, and the next the controller says to it is the
// weather (live, TVS827 at LKPR: "request weather" while tower's "report
// ready for departure" waited on the busy frequency, the weather after it).
func TestWeatherAskedAfterCheckInAnswer(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	seed := uint64(weatherOrderSeed)
	if s, err := strconv.ParseUint(os.Getenv("ZZ_SEED"), 10, 64); err == nil {
		seed = s
	}
	h, err := Start(Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10, Seed: seed})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 20}); err != nil {
		t.Fatal(err)
	}
	h.Advance(60 * time.Minute)
	heard := h.Heard()
	asked := 0
	for i, tx := range heard {
		if !tx.Pilot || tx.Intent != traffic.IntentRequestWeather {
			continue
		}
		asked++
		for _, next := range heard[i+1:] {
			if next.Pilot || next.Callsign != tx.Callsign {
				continue
			}
			if next.Intent == traffic.IntentWeather {
				break
			}
			// Something more pressing may come first (a speed for spacing),
			// never the answer to its check-in.
			if next.Intent == traffic.IntentDepartureOrder || next.Intent == traffic.IntentArrivalClearance {
				t.Errorf("%s asked for the weather at %s before its check-in was answered: %q", tx.Callsign, tx.At.Format("15:04:05"), next.Text)
				break
			}
		}
	}
	if asked == 0 {
		t.Errorf("seed %d: no crew asked for the weather", seed)
	}
}

// weatherOrderSeed: a draw with a crew asking for the weather on a busy
// tower frequency.
const weatherOrderSeed = 1
