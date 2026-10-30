package worldtest

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestReadyForTaxiContact: a stand worked by an apron station, the taxi by
// ground: the push is cleared and worked on the apron's frequency, then
// the crew is told "when ready for taxi contact" ground, after the push's
// readback, and asks ground for taxi (#1268). Not handed over mid-push.
func TestReadyForTaxiContact(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	// The manual clock at midday: on the wall clock, at night the schedule
	// had no departure in its first 10 minutes.
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	o.World.DataDir = t.TempDir() // the stations set here
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 6}); err != nil {
		t.Fatal(err)
	}
	var dep world.ControlView
	if !h.Until(10*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "departure" && v.State == traffic.TaxiAwaitingPushback.String() && v.Position.Lat != 0 && !heardPush(h, v.Tail) {
				dep = v
				return true
			}
		}
		return false
	}) {
		t.Fatal("no departure on a stand in 10 simulated minutes")
	}
	// The apron: 20 m round its stand.
	const apron, ground = "121.79", "121.91"
	dLat, dLon := 20.0/111320, 20.0/(111320*math.Cos(dep.Position.Lat*math.Pi/180))
	p := dep.Position
	st := []traffic.Station{
		{Position: traffic.PosGround, Name: "Ruzyne Apron", Freq: apron, Area: []airport.LatLon{
			{Lat: p.Lat - dLat, Lon: p.Lon - dLon}, {Lat: p.Lat - dLat, Lon: p.Lon + dLon}, {Lat: p.Lat + dLat, Lon: p.Lon + dLon}, {Lat: p.Lat + dLat, Lon: p.Lon - dLon}}},
		{Position: traffic.PosGround, Name: "Ruzyne Ground", Freq: ground},
	}
	if _, err := h.World.Do(http.MethodPut, "/api/stations?icao=LKPR", st); err != nil {
		t.Fatal(err)
	}
	if !h.Until(20*time.Minute, func() bool {
		return slices.ContainsFunc(h.Heard(), func(tx traffic.Transmission) bool { return tx.Callsign == dep.Tail && tx.Intent == traffic.IntentTaxi })
	}) {
		t.Fatalf("%s never cleared to taxi", dep.Tail)
	}
	var seq []string
	for _, tx := range h.Heard() {
		if tx.Callsign != dep.Tail || tx.Position != traffic.PosGround {
			continue
		}
		seq = append(seq, fmt.Sprintf("%s %s %s", tx.Frequency, tx.Intent, tx.Text))
	}
	at := func(pred func(string) bool) int { return slices.IndexFunc(seq, pred) }
	push := at(func(s string) bool { return strings.HasPrefix(s, apron+" pushback") })
	when := at(func(s string) bool {
		return strings.Contains(s, " contact ") && strings.Contains(s, "when ready for taxi contact Ruzyne Ground "+ground)
	})
	ask := at(func(s string) bool { return strings.HasPrefix(s, ground+" request_taxi") })
	plain := at(func(s string) bool {
		return strings.Contains(s, string(traffic.IntentContact)+" ") && !strings.Contains(s, "when ready") && strings.Contains(s, "Ruzyne Ground")
	})
	if push < 0 || when < push || ask < when || plain >= 0 {
		t.Errorf("%s: push %d, when ready %d, request %d, plain contact %d; heard:\n%s", dep.Tail, push, when, ask, plain, strings.Join(seq, "\n"))
	}
}

func heardPush(h *Harness, tail string) bool {
	return slices.ContainsFunc(h.Heard(), func(tx traffic.Transmission) bool {
		return tx.Callsign == tail && tx.Intent == traffic.IntentRequestPushback
	})
}
