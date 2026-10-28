package worldtest

import (
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestSlowLeaderFastFollower: a B738 behind a TBM9 sent direct to the
// final 20 NM behind it (live, LKPR: LOT229 behind OKBJZ, cleared direct
// to the final 36 NM out) loses its delay before the final and lands
// behind it, 3 NM or more apart all the way, not sent around. Before, the
// TBM9 was predicted 1m20s early 18 NM out, and the B738 joined the final
// ahead of it with 5 minutes to lose and nothing to lose them on: 2.7 NM
// apart, it landed first.
func TestSlowLeaderFastFollower(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	was := DefaultAircraftTitles
	DefaultAircraftTitles = append(slices.Clone(was), "FSLTL_GA_TBM9_ZZZ")
	defer func() { DefaultAircraftTitles = was }()
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	off := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 10, Generator: &off}); err != nil {
		t.Fatal(err)
	}
	sta := h.Clock.Now().Add(25 * time.Minute)
	if err := h.World.AddFlights([]traffic.Flight{
		{Callsign: "OKBJZ", Type: "TBM9", Origin: "LZKZ", Destination: "LKPR", STD: sta.Add(-80 * time.Minute), STA: sta, DistanceNM: 349},
		{Callsign: "LOT229", Airline: "RYR", Type: "B738", Origin: "EPWA", Destination: "LKPR", STD: sta.Add(-60 * time.Minute), STA: sta.Add(time.Minute), DistanceNM: 339},
	}); err != nil {
		t.Fatal(err)
	}
	sent, landed, minNM := false, "", 99.0
	for range 6 * 40 {
		h.Advance(10 * time.Second)
		tbm, ok1 := h.Aircraft("OKBJZ")
		jet, ok2 := h.Aircraft("LOT229")
		if !ok1 || !ok2 || tbm.Position.Lat == 0 || jet.Position.Lat == 0 {
			continue
		}
		if landed == "" && (tbm.OnGround || jet.OnGround) {
			landed = map[bool]string{true: "OKBJZ", false: "LOT229"}[tbm.OnGround]
		}
		if landed != "" {
			break
		}
		d := calc.HaversineNM(tbm.Position.Lat, tbm.Position.Lon, jet.Position.Lat, jet.Position.Lon)
		minNM = min(minNM, d)
		if !sent && d < 20 {
			sent = true
			if _, err := h.World.Do(http.MethodPost, "/api/approach/LKPR/LOT229/direct", nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !sent || landed == "" {
		t.Fatalf("never sent direct (%v) or nobody landed in 40 minutes", sent)
	}
	around := slices.ContainsFunc(h.Heard(), func(tx traffic.Transmission) bool {
		return tx.Callsign == "LOT229" && strings.Contains(strings.ToLower(tx.Text), "go around")
	})
	if landed != "OKBJZ" || minNM < 3 || around {
		t.Errorf("first down %s, closest %.1f NM, B738 sent around %v: want the TBM9 first, 3 NM or more, no go-around", landed, minNM, around)
	}
}
