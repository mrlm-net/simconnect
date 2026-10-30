package worldtest

import (
	"net/http"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestVFRLeavingFieldNoCheckIn: a VFR departure leaving a field without a
// tower checks in with nobody (#1314).
func TestVFRLeavingFieldNoCheckIn(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	start := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	h, err := Start(Options{DataDir: uncontrolledData(t), ICAO: "LKPR", Manual: true, FrameRate: 10, Start: start})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	h.World.SetWeather(&nav.Weather{WindDirTrue: 240, WindKts: 5, VisibilityM: 9999, QNHhPa: 1013, TempC: 15})
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 10, IFR: &no, VFR: &no, Generator: &no}); err != nil {
		t.Fatal(err)
	}
	std := start.Add(10 * time.Minute)
	flights := []traffic.Flight{{Callsign: "OKLVE", Origin: "LKPR", Destination: "LKKB", Type: "DA62", Rules: "VFR", STD: std, STA: std.Add(10 * time.Minute)}}
	if _, err := h.World.Do(http.MethodPost, "/api/flights", flights); err != nil {
		t.Fatal(err)
	}
	if !h.Until(45*time.Minute, func() bool { return heardCall(h, "departing runway") }) {
		t.Fatal("no departure in 45 simulated minutes")
	}
	h.Advance(5 * time.Minute)
	for _, tx := range h.Heard() {
		if tx.Callsign == "OKLVE" && tx.Position == traffic.PosDeparture {
			t.Errorf("said on departure: %q", tx.Text)
		}
	}
}
