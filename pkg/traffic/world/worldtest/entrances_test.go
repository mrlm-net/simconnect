package worldtest

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestDeparturesShareEndEntrances: at LKPR 24, A and Z side by side at
// its end, a busy hour's departures queue at both, full length, and all
// get away: none waits at one while the other stands empty, and the tower
// takes them from both.
func TestDeparturesShareEndEntrances(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	h.SetUser(func(u *Object) { // parked on S20, off the runways
		u.Lat, u.Lon, u.Heading, u.AltFt, u.OnGround, u.GroundKts = 50.097060, 14.284178, 168.8, h.Layout.Altitude/0.3048, true, 0
	})
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 30}); err != nil {
		t.Fatal(err)
	}
	g, err := airport.BuildGraph(h.Layout)
	if err != nil {
		t.Fatal(err)
	}
	holdName := func(id int) string {
		for _, es := range g.Adj[id] {
			if es.Name != "" {
				return es.Name
			}
		}
		return "?"
	}
	at := map[string]map[string]bool{} // entrance → departures
	departed := map[string]bool{}
	for range 40 * 60 / 10 {
		h.Advance(10 * time.Second)
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind != "departure" || v.Runway != "24" || v.Entry != "" || len(v.Nodes) == 0 {
				continue
			}
			if v.State == "holding short" || v.State == "lining up" {
				n := holdName(int(v.Nodes[len(v.Nodes)-1]))
				if at[n] == nil {
					at[n] = map[string]bool{}
				}
				at[n][v.Tail] = true
			}
			if v.State == "departing" {
				departed[v.Tail] = true
			}
		}
	}
	for n, tails := range at {
		t.Logf("held short at %s: %d", n, len(tails))
	}
	t.Logf("departed: %d", len(departed))
	if len(at) < 2 {
		t.Errorf("departures held short at %d entrance(s) of 24, want both A and Z", len(at))
	}
	for n := range at {
		if !strings.ContainsAny(n, "AZ") {
			t.Errorf("a full-length departure held short at %s", n)
		}
	}
	if len(departed) < 4 {
		t.Errorf("%d departures in 40 minutes", len(departed))
	}
}
