package worldtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// noFrequencies is data's LKPR with no frequency at all, as a generated
// field: no tower, no advisory station.
func noFrequencies(t *testing.T, data string) string {
	t.Helper()
	file := filepath.Join(data, "LKPR", "layout.json")
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var l map[string]json.RawMessage
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	l["frequencies"] = json.RawMessage(`[]`)
	if b, err = json.Marshal(l); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return data
}

// towerData is data's LKPR with a tower for certain: data without one (no
// frequencies, as older dumps) gets Ruzyne's, as the full data has them.
func towerData(t *testing.T, data string) string {
	t.Helper()
	from := filepath.Join(data, "LKPR")
	to := filepath.Join(t.TempDir(), "LKPR")
	if err := os.MkdirAll(to, 0o755); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(from)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(from, f.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if f.Name() == "layout.json" {
			var l map[string]json.RawMessage
			if err := json.Unmarshal(b, &l); err != nil {
				t.Fatal(err)
			}
			var freqs []struct {
				Type int `json:"type"`
			}
			_ = json.Unmarshal(l["frequencies"], &freqs)
			tower := false
			for _, fr := range freqs {
				tower = tower || fr.Type == 6
			}
			if !tower {
				l["frequencies"] = json.RawMessage(`[{"type":8,"hz":118310000,"name":"RUZYNE"},{"type":1,"hz":122160000,"name":"LKPR"},{"type":7,"hz":120060000,"name":"RUZYNE"},{"type":5,"hz":121910000,"name":"RUZYNE"},{"type":6,"hz":134560000,"name":"RUZYNE"}]`)
				if b, err = json.Marshal(l); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := os.WriteFile(filepath.Join(to, f.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Dir(to)
}

// landingArrival and landingDeparture are landingBed's flights.
const (
	landingArrival   = "CSA401"
	landingDeparture = "CSA402"
)

// landingBed flies an IFR arrival into data's LKPR at sim rate rate, a
// departure ready as it comes, until the arrival is off the runway, has gone
// around twice or 90 minutes have passed; the harness and the World's log.
func landingBed(t *testing.T, data string, rate float64) (*Harness, *lockedLog) {
	t.Helper()
	if testing.Short() {
		t.Skip("runs the World")
	}
	start := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	logs := &lockedLog{}
	o := Options{DataDir: data, ICAO: "LKPR", Manual: true, FrameRate: 10, Start: start}
	o.World.Output = logs
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	h.Sim.SetSimRate(rate)
	h.World.SetWeather(&nav.Weather{WindDirTrue: 240, WindKts: 5, VisibilityM: 9999, QNHhPa: 1013, TempC: 15})
	yes, no := true, false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 10, IFR: &yes, VFR: &no, Generator: &no}); err != nil {
		t.Fatal(err)
	}
	sta := start.Add(30 * time.Minute)
	flights := []traffic.Flight{
		{Callsign: landingArrival, Origin: "LKKB", Destination: "LKPR", Type: "A320", STD: sta.Add(-time.Hour), STA: sta},
		{Callsign: landingDeparture, Origin: "LKPR", Destination: "EDDF", Type: "A320", STD: start.Add(20 * time.Minute), STA: start.Add(80 * time.Minute)},
	}
	if _, err := h.World.Do(http.MethodPost, "/api/flights", flights); err != nil {
		t.Fatal(err)
	}
	h.Until(90*time.Minute, func() bool {
		return landed(h, landingArrival) || goneAround(h, landingArrival) >= 2
	})
	return h, logs
}

// landed: tail reported off the runway.
func landed(h *Harness, tail string) bool {
	return h.Said(tail, "clear of runway") || h.Said(tail, "vacated")
}

// goneAround counts tail's go-arounds heard.
func goneAround(h *Harness, tail string) int {
	n := 0
	for _, tx := range h.Heard() {
		if tx.Callsign == tail && strings.Contains(tx.Text, "going around") {
			n++
		}
	}
	return n
}

// landedFirstTime: the arrival landed on its first approach with no
// go-around, and the tower dropped no landing grant for it; its log on a
// failure.
func landedFirstTime(t *testing.T, h *Harness, logs *lockedLog) {
	t.Helper()
	tail := landingArrival
	if n := goneAround(h, tail); n > 0 {
		t.Errorf("%s went around %d times", tail, n)
	}
	if !landed(h, tail) {
		t.Errorf("%s did not land", tail)
	}
	if n := strings.Count(logs.String(), tail+" crew: going around"); n > 0 {
		t.Errorf("%s: the crew went around %d times", tail, n)
	}
	if n := strings.Count(logs.String(), tail+" tower: land no longer granted"); n > 0 {
		t.Errorf("%s: a landing grant dropped %d times", tail, n)
	}
	if t.Failed() {
		n := 0
		for _, l := range strings.Split(logs.String(), "\n") {
			if strings.Contains(l, tail) && !strings.Contains(l, "agenda:") && n < 80 {
				t.Log(l)
				n++
			}
		}
	}
}

// TestUncontrolledArrivalLandsUncleared: at a field without a tower an IFR
// arrival cancels IFR and lands on its own traffic calls at its first
// approach, with no landing clearance and no go-around for one, at any sim
// rate; nothing said on tower (#1596). Before, at a rate whose ticks were
// further apart than a grant stays fresh, the tower's landing grant was
// dropped every time and the crew went around for "no landing clearance"
// until the flight was cancelled.
func TestUncontrolledArrivalLandsUncleared(t *testing.T) {
	for _, c := range []struct {
		name string
		data func(t *testing.T) string
	}{
		{"advisory", uncontrolledData},
		{"no frequencies", func(t *testing.T) string { return noFrequencies(t, uncontrolledData(t)) }},
	} {
		for _, rate := range []float64{1, 4} {
			t.Run(fmt.Sprintf("%s rate %g", c.name, rate), func(t *testing.T) {
				h, logs := landingBed(t, c.data(t), rate)
				h.Close() // one World at a time: the clock is process-wide
				if !h.Said(landingArrival, "cancel IFR") {
					t.Errorf("%s did not cancel IFR", landingArrival)
				}
				landedFirstTime(t, h, logs)
				nobodyLocal(t, h)
				// Still one at a time: no take-off from its final call until
				// it is off the runway.
				var final time.Time
				for _, tx := range h.Heard() {
					switch {
					case tx.Callsign == landingArrival && strings.Contains(tx.Text, "final runway") && final.IsZero():
						final = tx.At
					case tx.Callsign == landingArrival && strings.Contains(tx.Text, "clear of runway"):
						final = time.Time{}
					case tx.Callsign == landingDeparture && strings.Contains(tx.Text, "departing runway") && !final.IsZero():
						t.Errorf("%s took off at %v, %s on its final since %v", landingDeparture, tx.At, landingArrival, final)
					}
				}
			})
		}
	}
}

// TestTowerArrivalStillCleared: at a towered field the arrival is cleared to
// land and lands at its first approach, IFR not cancelled, at any sim rate.
// Before, at rate 4 the landing clearance was older than a grant stays fresh
// whenever its turn came, dropped every time, and the arrival went around
// for no landing clearance until it was cancelled (#1596).
func TestTowerArrivalStillCleared(t *testing.T) {
	for _, rate := range []float64{1, 2, 4} {
		t.Run(fmt.Sprintf("rate %g", rate), func(t *testing.T) {
			h, logs := landingBed(t, towerData(t, dataDir(t)), rate)
			h.Close() // one World at a time: the clock is process-wide
			landedFirstTime(t, h, logs)
			if !h.Said(landingArrival, "cleared to land") {
				t.Errorf("%s not cleared to land at a towered field", landingArrival)
			}
			if h.Said(landingArrival, "cancel IFR") {
				t.Errorf("%s cancelled IFR at a towered field", landingArrival)
			}
			if n := strings.Count(logs.String(), "no longer granted when its turn came"); n > 0 {
				t.Errorf("%d grants dropped", n)
			}
		})
	}
}
