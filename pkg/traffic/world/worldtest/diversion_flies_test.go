package worldtest

import (
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// lockedLog is the World's console output, read while the World writes it.
type lockedLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *lockedLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *lockedLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// TestNoClearanceDiversionFlies: with tower and ground staffed and no landing
// clearance ever given, the crew that announces its diversion on the third
// go-around flies it (#1411) and is gone, never landing. The diversion was
// flown from the sequence's absorption, which skips an aircraft on a staffed
// station: the arrival stays the staffed tower's after a crew go-around (the
// player hands it over), so it flew the go-around back onto the final and
// landed without a clearance, the crew no longer checking (live, VECTOR.PRO
// at LKPR, KLM504: "diverting to LKMB", then "landing → rollout"; #1544).
func TestNoClearanceDiversionFlies(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	logs := &lockedLog{}
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	o.World.Output = logs
	if testing.Verbose() {
		o.World.Output = io.MultiWriter(logs, os.Stderr)
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	h.World.SetStaffed([]world.StaffedStation{{ICAO: "LKPR", Position: "tower"}, {ICAO: "LKPR", Position: "ground"}})
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 4, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	tail, diverting := "", ""
	goArounds := func() (n int) {
		for _, tx := range h.Heard() {
			if tx.Callsign == tail && strings.Contains(tx.Text, "going around, no landing clearance") {
				n++
				if strings.Contains(tx.Text, "diverting to ") {
					diverting = tx.Text
				}
			}
		}
		return n
	}
	view := func() (world.ControlView, bool) {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "arrival" && (tail == "" || v.Tail == tail) {
				tail = v.Tail
				return v, true
			}
		}
		return world.ControlView{}, false
	}
	landed := func(v world.ControlView) bool { return v.State == "landing" || v.State == "rollout" }
	if !h.Until(4*time.Hour, func() bool {
		v, ok := view()
		if !ok {
			return false
		}
		if landed(v) {
			t.Fatalf("%s landed with no landing clearance before diverting", tail)
		}
		goArounds()
		return diverting != ""
	}) {
		t.Fatalf("%s: %d go-arounds and no diversion", tail, goArounds())
	}
	t.Log(diverting)
	field := h.Layout
	far := 0.0
	gone := h.Until(30*time.Minute, func() bool {
		v, ok := view()
		if !ok {
			return true
		}
		if landed(v) {
			t.Fatalf("%s landed after diverting: %s", tail, v.State)
		}
		far = max(far, calc.HaversineNM(v.Position.Lat, v.Position.Lon, field.Latitude, field.Longitude))
		return false
	})
	if !gone {
		v, _ := view()
		t.Fatalf("%s still here 30 minutes after diverting: %s at %+v, %.1f NM out at most", tail, v.State, v.Position, far)
	}
	t.Logf("%s gone, %.1f NM from the field at most", tail, far)
	flown := false
	for _, line := range strings.Split(logs.String(), "\n") {
		if f := strings.Fields(line); len(f) > 3 && f[1] == tail && f[2] == "diverted" && f[3] == "to" {
			flown = true
		}
	}
	if !flown {
		t.Errorf("%s gone without flying its diversion (no \"diverted to\" in the log), %.1f NM out at most", tail, far)
	}
	if n := goArounds(); n != 3 {
		t.Errorf("%d go-arounds, want 3", n)
	}
}
