package worldtest

import (
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// tunedHarness is a World at start with its settings in dir, its schedule
// on at LKPR.
func tunedHarness(t *testing.T, data, dir string, start time.Time, tune world.ScheduleTuning) *Harness {
	t.Helper()
	if testing.Short() {
		t.Skip("runs the World")
	}
	o := Options{DataDir: data, ICAO: "LKPR", Manual: true, FrameRate: 5, Start: start}
	o.World.DataDir = dir
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	if err := h.World.SetScheduleTuning(tune); err != nil {
		t.Fatal(err)
	}
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 40, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	return h
}

// cargoOnBoard counts the board's flights of the default cargo airlines.
func cargoOnBoard(t *testing.T, h *Harness) int {
	t.Helper()
	n := 0
	for _, f := range nightBoard(t, h) {
		for _, a := range traffic.DefaultScheduleConfig().Airlines {
			if a.Cargo && strings.HasPrefix(f.Callsign, a.ICAO) {
				n++
			}
		}
	}
	return n
}

// TestScheduleTuningNight: the settings change the night's trigger, are
// kept in the local file over a restart, and a bad value changes nothing
// (#1303).
func TestScheduleTuningNight(t *testing.T) {
	dir, night := t.TempDir(), time.Date(2026, 10, 20, 1, 30, 0, 0, time.UTC)
	none := -1
	h := tunedHarness(t, dataDir(t), dir, night, world.ScheduleTuning{NightStops: &none})
	bad := 2.0
	if err := h.World.SetScheduleTuning(world.ScheduleTuning{NightWave: &bad}); err == nil {
		t.Error("nightWave 2 taken")
	}
	if got := h.World.ScheduleTuning(); got.NightStops == nil || *got.NightStops != -1 || got.NightWave != nil {
		t.Errorf("after a refused change: %+v", got)
	}
	h.Advance(20 * time.Minute)
	for _, f := range nightBoard(t, h) {
		if f.NightStop {
			t.Errorf("%s a night stop with none wanted", f.Callsign)
		}
	}
	var v struct {
		Local, Effective struct {
			NightStops     *int     `json:"nightStops"`
			MorningWaveMin *float64 `json:"morningWaveMin"`
		}
	}
	if err := h.World.Get("/api/schedule/tuning", &v); err != nil {
		t.Fatal(err)
	}
	if v.Local.NightStops == nil || *v.Local.NightStops != -1 || v.Local.MorningWaveMin != nil || v.Effective.MorningWaveMin == nil || *v.Effective.MorningWaveMin != 120 {
		t.Errorf("GET: %+v", v)
	}
	h.Close()

	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 5, Start: night}
	o.World.DataDir = dir
	h2, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h2.Close()
	if got := h2.World.ScheduleTuning(); got.NightStops == nil || *got.NightStops != -1 {
		t.Errorf("after a restart: %+v", got)
	}
}

// TestScheduleTuningLead: night stops call delivery the DepartureLead set
// before their STD: the trigger follows the setting.
func TestScheduleTuningLead(t *testing.T) {
	lead := 25.0
	h := tunedHarness(t, dataDir(t), t.TempDir(), time.Date(2026, 10, 20, 1, 30, 0, 0, time.UTC), world.ScheduleTuning{DepartureLeadMin: &lead})
	std := map[string]time.Time{}
	called := map[string]time.Time{}
	h.Until(2*time.Hour, func() bool {
		for _, f := range nightBoard(t, h) {
			if f.NightStop {
				std[f.Callsign] = f.STD
			}
		}
		for _, tx := range h.Heard() {
			if _, ok := std[tx.Callsign]; ok && tx.Pilot && tx.Intent == traffic.IntentRequestClearance && called[tx.Callsign].IsZero() {
				called[tx.Callsign] = tx.At
			}
		}
		return len(called) >= 2
	})
	if len(called) < 2 {
		t.Fatalf("%d night stops called delivery by 05:30", len(called))
	}
	for cs, at := range called {
		before := std[cs].Sub(at)
		t.Logf("%s called %v before its STD", cs, before)
		if before > 25*time.Minute+time.Second || before < 15*time.Minute {
			t.Errorf("%s called %v before its STD, want about 25 min", cs, before)
		}
	}
}

// TestScheduleTuningCargo: the cargo share: none at 0, many at 10.
func TestScheduleTuningCargo(t *testing.T) {
	noon := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	zero, ten := 0.0, 10.0
	h := tunedHarness(t, dataDir(t), t.TempDir(), noon, world.ScheduleTuning{CargoShare: &zero})
	h.Advance(time.Minute)
	if n := cargoOnBoard(t, h); n != 0 {
		t.Errorf("share 0: %d cargo flights", n)
	}
	h.Close()
	h = tunedHarness(t, dataDir(t), t.TempDir(), noon, world.ScheduleTuning{CargoShare: &ten})
	h.Advance(time.Minute)
	if n := cargoOnBoard(t, h); n == 0 {
		t.Error("share 10: no cargo flights on the board")
	}
}

// TestScheduleTuningSelfAnnounceOff: switched off, a field without a tower
// has the tower and ground it had, on the common frequency.
func TestScheduleTuningSelfAnnounceOff(t *testing.T) {
	off := false
	h := tunedHarness(t, uncontrolledData(t), t.TempDir(), time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC), world.ScheduleTuning{SelfAnnounce: &off})
	heard := func(p traffic.Position) bool {
		for _, tx := range h.Heard() {
			if tx.Position == p {
				return true
			}
		}
		return false
	}
	if !h.Until(30*time.Minute, func() bool { return heard(traffic.PosDelivery) || heard(traffic.PosGround) }) {
		t.Error("no delivery or ground call in 30 simulated minutes")
	}
	if heard(traffic.PosTraffic) {
		t.Error("self-announced with it switched off")
	}
}
