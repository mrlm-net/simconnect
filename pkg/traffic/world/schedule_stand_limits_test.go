package world

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestScheduleSettingsStandLimitsOptIn: ScheduleSettings' StartStands and
// NightStops are sent only when set, so a host that leaves them 0 keeps
// what is set (#1677).
func TestScheduleSettingsStandLimitsOptIn(t *testing.T) {
	b, err := json.Marshal(ScheduleSettings{Enabled: true, ICAO: "TNCM"})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"startStands", "nightStops"} {
		if strings.Contains(string(b), k) {
			t.Errorf("zero settings send %s: %s", k, b)
		}
	}
	b, _ = json.Marshal(ScheduleSettings{StartStands: 4, NightStops: -1})
	if !strings.Contains(string(b), `"startStands":4`) || !strings.Contains(string(b), `"nightStops":-1`) {
		t.Errorf("set: %s", b)
	}
}
