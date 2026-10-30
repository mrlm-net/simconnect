package worldtest

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// nightFlight is a flight on the schedule's board (GET /api/schedule).
type nightFlight struct {
	Callsign  string    `json:"callsign"`
	Kind      string    `json:"kind"`
	STD       time.Time `json:"std"`
	STA       time.Time `json:"sta"`
	Status    string    `json:"status"`
	TurnTo    string    `json:"turnTo"`
	NightStop bool      `json:"nightStop"`
}

func nightBoard(t *testing.T, h *Harness) []nightFlight {
	t.Helper()
	b, err := h.World.Do(http.MethodGet, "/api/schedule", nil)
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Flights []nightFlight `json:"flights"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v.Flights
}

func nightHarness(t *testing.T, start time.Time) *Harness {
	t.Helper()
	if testing.Short() {
		t.Skip("runs the World")
	}
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 5, Start: start}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 40, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	return h
}

// TestNightStops: at 03:30 at LKPR, in the night, departures of the
// morning wave stand on their stands, quiet, and each calls delivery only
// DepartureLead (10 min) before its STD, then pushes (#1294). Before, the
// stands stayed empty until 10 minutes before each STD.
func TestNightStops(t *testing.T) {
	h := nightHarness(t, time.Date(2026, 10, 20, 1, 30, 0, 0, time.UTC)) // 03:30 CEST
	h.Advance(20 * time.Minute)
	parked := map[string]bool{}
	for _, v := range h.World.Snapshot().Aircraft {
		if v.Kind == "departure" && v.State == traffic.TaxiAwaitingPushback.String() {
			parked[v.Tail] = true
		}
	}
	std := map[string]time.Time{}
	for _, f := range nightBoard(t, h) {
		if f.NightStop {
			std[f.Callsign] = f.STD
		}
	}
	stops := 0
	for cs := range parked {
		if _, ok := std[cs]; ok {
			stops++
		}
	}
	t.Logf("03:50: %d departures on their stands, %d of them night stops", len(parked), stops)
	if stops < 3 {
		t.Fatalf("%d night stops on their stands at 03:50 (of %d departures)", stops, len(parked))
	}
	pushed := map[string]bool{}
	h.Until(2*time.Hour, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if _, ok := std[v.Tail]; ok && v.State != traffic.TaxiAwaitingPushback.String() && v.State != "spawning" {
				pushed[v.Tail] = true
			}
		}
		return len(pushed) >= 2
	})
	if len(pushed) < 2 {
		t.Errorf("%d night stops pushed by 05:50", len(pushed))
	}
	for _, tx := range h.Heard() {
		at, ok := std[tx.Callsign]
		if ok && tx.Pilot && tx.Intent == traffic.IntentRequestClearance && tx.At.Before(at.Add(-10*time.Minute-time.Second)) {
			t.Errorf("%s called delivery at %s, STD %s", tx.Callsign, tx.At.Format("15:04"), at.Format("15:04"))
		}
	}
}

// TestOvernightArrivalStays: an arrival landing late in the evening turns
// into a departure of the morning wave and stays on its stand through the
// night, not removed 20 minutes after it parked (#1294).
func TestOvernightArrivalStays(t *testing.T) {
	h := nightHarness(t, time.Date(2026, 10, 20, 20, 30, 0, 0, time.UTC)) // 22:30 CEST
	morning := time.Date(2026, 10, 21, 3, 0, 0, 0, time.UTC)              // 05:00 CEST
	parkedAt := map[string]time.Time{}
	stayed := ""
	h.Until(3*time.Hour, func() bool {
		now := h.Clock.Now()
		on := map[string]bool{}
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "arrival" && v.State == "parked" {
				on[v.Tail] = true
				if parkedAt[v.Tail].IsZero() {
					parkedAt[v.Tail] = now
				}
			}
		}
		std := map[string]time.Time{}
		turn := map[string]string{}
		for _, f := range nightBoard(t, h) {
			switch f.Kind {
			case "departure":
				std[f.Callsign] = f.STD
			case "arrival":
				turn[f.Callsign] = f.TurnTo
			}
		}
		for cs, at := range parkedAt {
			if d := turn[cs]; on[cs] && now.Sub(at) >= time.Hour && d != "" && !std[d].Before(morning) {
				stayed = cs + " → " + d + " at " + std[d].Format("15:04")
			}
		}
		return stayed != ""
	})
	if stayed == "" {
		t.Fatalf("no arrival on its stand overnight, an hour after parking and leaving in the morning wave, by 01:30 (%d parked)", len(parkedAt))
	}
	t.Logf("overnight: %s", stayed)
}
