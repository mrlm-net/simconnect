package worldtest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// uncontrolledData is dataDir's LKPR with a UNICOM its only frequency: a
// field without a tower, with its runways and stands.
func uncontrolledData(t *testing.T) string {
	t.Helper()
	from := filepath.Join(dataDir(t), "LKPR")
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
			l["frequencies"] = json.RawMessage(`[{"type":3,"hz":118100000,"name":"HENDERSON INFORMATION"}]`)
			if b, err = json.Marshal(l); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(to, f.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Dir(to)
}

// TestUncontrolledFieldSelfAnnounce: at a field without a tower our crews
// announce themselves on the common frequency and nobody clears them; the
// traffic still flows (#1296). Before, its tower and ground, on the common
// frequency, cleared them.
func TestUncontrolledFieldSelfAnnounce(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	o := Options{DataDir: uncontrolledData(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 8, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	said := func(sub string) bool {
		for _, tx := range h.Heard() {
			if tx.Position == traffic.PosTraffic && strings.Contains(tx.Text, sub) {
				return true
			}
		}
		return false
	}
	calls := []string{"taxiing to runway", "departing runway", "final runway", "clear of runway"}
	done := h.Until(90*time.Minute, func() bool {
		for _, c := range calls {
			if !said(c) {
				return false
			}
		}
		return true
	})
	for _, tx := range h.Heard() {
		switch tx.Position {
		case traffic.PosTower, traffic.PosGround, traffic.PosDelivery:
			t.Errorf("said at a field without a tower, on %s: %q", tx.Position, tx.Text)
		case traffic.PosTraffic:
			if tx.Frequency != "118.10" || !strings.HasPrefix(tx.Text, "Henderson Traffic, ") {
				t.Errorf("%q on %q", tx.Text, tx.Frequency)
			}
		}
	}
	if !done {
		for _, c := range calls {
			t.Logf("%q said: %v", c, said(c))
		}
		t.Error("not all the calls in 90 simulated minutes")
	}
}
