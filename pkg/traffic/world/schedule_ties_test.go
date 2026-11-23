package world

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestScheduleTiesOptIn: ScheduleSettings sends ties only when given; the
// ties set are kept by upper-case code, an empty set clears them; the
// game's error names the tie's region (#1678).
func TestScheduleTiesOptIn(t *testing.T) {
	b, err := json.Marshal(ScheduleSettings{Enabled: true, ICAO: "XNCM"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "ties") {
		t.Errorf("no ties sent as %s", b)
	}
	c := &core{}
	if c.airportTies() != nil || (*core)(nil).airportTies() != nil {
		t.Error("ties before any set")
	}
	c.setAirportTies(map[string]traffic.AirportTie{" xncm ": {Like: "tncm"}})
	if got := c.airportTies(); got["XNCM"] != (traffic.AirportTie{Like: "TNCM"}) {
		t.Errorf("ties kept as %v", got)
	}
	c.setAirportTies(map[string]traffic.AirportTie{})
	if c.airportTies() != nil {
		t.Error("an empty set does not clear the ties")
	}
	for _, c := range []struct {
		icao string
		tie  traffic.AirportTie
		want string
	}{
		{"LKPR", traffic.AirportTie{}, "LK"},
		{"XNCM", traffic.AirportTie{Like: "TNCM"}, "TN"},
		{"XNCM", traffic.AirportTie{Region: "TF", Like: "TNCM"}, "TF"},
	} {
		if got := tieRegion(c.icao, c.tie); got != c.want {
			t.Errorf("%s %+v: region %q, want %q", c.icao, c.tie, got, c.want)
		}
	}
	if err := noAirlineError("XNCM", nil, traffic.AirportTie{Like: "TNCM"}); !strings.Contains(err.Error(), `"TN"`) {
		t.Errorf("game error %q does not name region TN", err)
	}
}
