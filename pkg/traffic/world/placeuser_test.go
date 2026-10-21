package world

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestUserStandBlocked: a stand held by one of ours keeps the user off it;
// the user's own aircraft detected there does not.
func TestUserStandBlocked(t *testing.T) {
	b, err := os.ReadFile("../../airport/testdata/LKPR.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	g, err := airport.BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	stand := -1
	for i, p := range l.Parking {
		if p.Size() != airport.StandNone {
			stand = i
			break
		}
	}
	a := traffic.NewStandAllocator(nil, g)
	if why := userStandBlocked(a, l, stand, 7); why != "" {
		t.Fatalf("free stand blocked: %s", why)
	}
	if err := a.Occupy(stand, "CSA1", 0); err != nil {
		t.Fatal(err)
	}
	if why := userStandBlocked(a, l, stand, 7); !strings.Contains(why, "CSA1") {
		t.Errorf("held by CSA1: %q", why)
	}
}
