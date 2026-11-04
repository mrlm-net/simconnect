package worldtest

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// TestGoAroundRejoinKeepsSpacing (#1454): on LKPR 06 the arrival ahead goes
// around on its short final and is sequenced again behind the others on
// LOMK8T, coming in from the north-west, one of them sent direct to the
// final or not: none closer than 3 NM and 1000 ft, the conflict watch
// quiet (live, DLH299 after its go-around and AFR971 direct to the final:
// 2.0 NM and 235 ft at 4600 ft).
func TestGoAroundRejoinKeepsSpacing(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	for _, direct := range []bool{true, false} {
		t.Run(fmt.Sprintf("direct=%v", direct), func(t *testing.T) { goAroundRejoin(t, direct) })
	}
}

func goAroundRejoin(t *testing.T, sendDirect bool) {
	log := &lockedBuf{}
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	var out io.Writer = log
	if testing.Verbose() {
		out = io.MultiWriter(log, os.Stderr)
	}
	o.World.Output = out
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	h.World.UseRunway("LKPR", "06")
	off := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 10, Generator: &off}); err != nil {
		t.Fatal(err)
	}
	sta := h.Clock.Now().Add(25 * time.Minute)
	if err := h.World.AddFlights([]traffic.Flight{
		{Callsign: "DLH299", Airline: "DLH", Type: "A320", Origin: "EDDM", Destination: "LKPR", STD: sta.Add(-60 * time.Minute), STA: sta, DistanceNM: 192},
		{Callsign: "AFR971", Airline: "AFR", Type: "A320", Origin: "LFPG", Destination: "LKPR", STD: sta.Add(-100 * time.Minute), STA: sta.Add(12 * time.Minute), DistanceNM: 480},
		{Callsign: "TVS947", Airline: "TVS", Type: "B738", Origin: "LIRF", Destination: "LKPR", STD: sta.Add(-110 * time.Minute), STA: sta.Add(16 * time.Minute), DistanceNM: 500},
	}); err != nil {
		t.Fatal(err)
	}
	_, end06, ok := h.Layout.RunwayEnd("06")
	if !ok {
		t.Fatal("no runway 06")
	}
	thr := end06.Threshold
	tails := []string{"DLH299", "AFR971", "TVS947"}
	wentAround, direct := false, false
	worst := ""
	for range 6 * 60 {
		h.Advance(10 * time.Second)
		var air []string
		for _, c := range tails {
			if a, ok := h.Aircraft(c); ok && a.Position.Lat != 0 && !a.OnGround {
				air = append(air, c)
			}
		}
		for i := range air {
			for j := i + 1; j < len(air); j++ {
				a, _ := h.Aircraft(air[i])
				b, _ := h.Aircraft(air[j])
				nm := calc.HaversineNM(a.Position.Lat, a.Position.Lon, b.Position.Lat, b.Position.Lon)
				ft := a.AltFt - b.AltFt
				if ft < 0 {
					ft = -ft
				}
				if nm < 3 && ft < 1000 && worst == "" {
					worst = fmt.Sprintf("%s and %s %.1f NM, %.0f ft at %s", air[i], air[j], nm, ft, h.Clock.Now().Format("15:04:05"))
				}
			}
		}
		if wentAround && testing.Verbose() {
			for _, c := range air {
				a, _ := h.Aircraft(c)
				t.Logf("%s %-6s %-12s %5.0f ft hdg %3.0f %.4f,%.4f %4.1f NM %s", h.Clock.Now().Format("15:04:05"), c, a.State, a.AltFt, a.Heading, a.Position.Lat, a.Position.Lon,
					calc.HaversineNM(a.Position.Lat, a.Position.Lon, thr.Lat, thr.Lon), a.Procedure)
			}
		}
		d, ok := h.Aircraft("DLH299")
		if ok && !wentAround && d.Runway == "06" && d.State == "approaching" && d.Position.Lat != 0 && !d.OnGround &&
			calc.HaversineNM(d.Position.Lat, d.Position.Lon, thr.Lat, thr.Lon) < 3 {
			if _, err := h.World.Do(http.MethodPost, "/api/approach/LKPR/DLH299/goaround", nil); err != nil {
				t.Fatal(err)
			}
			wentAround = true
		}
		if wentAround && sendDirect && !direct {
			if a, ok := h.Aircraft("AFR971"); ok && a.State == "approaching" {
				if _, err := h.World.Do(http.MethodPost, "/api/approach/LKPR/AFR971/direct", nil); err == nil {
					direct = true
				}
			}
		}
		down := 0
		for _, c := range tails {
			if a, ok := h.Aircraft(c); ok && a.OnGround && a.State != "approaching" {
				down++
			}
		}
		if down == len(tails) {
			break
		}
	}
	if !wentAround {
		t.Fatal("DLH299 never on its final to go around")
	}
	if sendDirect && !direct {
		t.Error("AFR971 never sent direct to the final")
	}
	var conflicts []string
	for _, l := range strings.Split(log.String(), "\n") {
		if strings.Contains(l, "conflict:") || strings.Contains(l, "separation:") {
			conflicts = append(conflicts, l)
		}
	}
	// Planned apart: the conflict watch never sees them meet.
	if worst != "" || len(conflicts) > 0 {
		t.Errorf("lost separation: %q; conflict watch:\n%s", worst, strings.Join(conflicts, "\n"))
	}
	// Told to by the tower, or saying so on an uncontrolled field.
	around := map[string]bool{}
	for _, tx := range h.Heard() {
		if l := strings.ToLower(tx.Text); strings.Contains(l, "go around") || strings.Contains(l, "going around") {
			around[tx.Callsign] = true
		}
	}
	if len(around) != 1 || !around["DLH299"] {
		t.Errorf("gone around: %v, want only DLH299", around)
	}
}
