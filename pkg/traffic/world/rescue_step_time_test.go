package world

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// The rescue crews' searches on the connection's goroutine (#1670): a
// crew's send and its vehicles' going home, each timed on the step that
// does it, at a large field, against slowStep. Before, the send built the
// airport's graph twice and searched each vehicle's way out there, and
// going home searched each vehicle's way home there.

// stepTimeGridSide: the synthetic field is LKPR with a square of
// stepTimeGridSide² taxi points stepTimeGridM apart joined to it east of
// the field, a large field's graph and more.
const (
	stepTimeGridSide = 300
	stepTimeGridM    = 20
)

// stepTimeLargeField is LKPR with the square (stepTimeGridSide) joined to
// its easternmost taxiway point.
func stepTimeLargeField(t *testing.T) *airport.Layout {
	t.Helper()
	b, err := os.ReadFile("../../airport/testdata/LKPR.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	// The taxiways: TAXI (1) or, in 2024 data such as this, PATH (4).
	east, eastX, kind := int32(-1), float32(-1e9), int32(0)
	var maxX, minZ float32 = -1e9, 1e9
	for _, p := range raw.TaxiPaths {
		if p.Type != int32(types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_TAXI) && p.Type != int32(types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_PATH) {
			continue
		}
		if q := raw.TaxiPoints[p.Start]; q.BiasX > eastX {
			east, eastX, kind = p.Start, q.BiasX, p.Type
		}
	}
	for _, q := range raw.TaxiPoints {
		maxX, minZ = max(maxX, q.BiasX), min(minZ, q.BiasZ)
	}
	if east < 0 {
		t.Fatal("no taxiway at LKPR")
	}
	first := int32(len(raw.TaxiPoints))
	at := func(i, j int) int32 { return first + int32(i*stepTimeGridSide+j) }
	for i := 0; i < stepTimeGridSide; i++ {
		for j := 0; j < stepTimeGridSide; j++ {
			raw.TaxiPoints = append(raw.TaxiPoints, airport.RawTaxiPoint{Type: 1,
				BiasX: maxX + 1000 + float32(i*stepTimeGridM), BiasZ: minZ + float32(j*stepTimeGridM)})
		}
	}
	link := func(a, b int32) {
		raw.TaxiPaths = append(raw.TaxiPaths, airport.RawTaxiPath{Type: kind, Width: 20, Start: a, End: b})
	}
	for i := 0; i < stepTimeGridSide; i++ {
		for j := 0; j < stepTimeGridSide; j++ {
			if i > 0 {
				link(at(i-1, j), at(i, j))
			}
			if j > 0 {
				link(at(i, j-1), at(i, j))
			}
		}
	}
	link(east, at(0, 0))
	l, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// stepTimeOffline is icao's layout from the simconnect-data clone
// ($SIMCONNECT_DATA, else beside this checkout); the test skips without.
func stepTimeOffline(t *testing.T, icao string) *airport.Layout {
	t.Helper()
	dir := os.Getenv("SIMCONNECT_DATA")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "..", "simconnect-data")
	}
	b, err := os.ReadFile(filepath.Join(dir, icao, "layout.json"))
	if err != nil {
		t.Skipf("no offline %s at %s (SIMCONNECT_DATA)", icao, dir)
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// stepTimeGraph is l's graph, its build timed and logged.
func stepTimeGraph(t *testing.T, name string, l *airport.Layout) *airport.Graph {
	t.Helper()
	t0 := time.Now()
	g, err := airport.BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: BuildGraph %v (%d taxi points, %d paths, %d stands)", name, time.Since(t0).Round(10*time.Microsecond), len(l.TaxiPoints), len(l.TaxiPaths), len(l.Parking))
	return g
}

// stepTimeRun sends a fire crew for an aircraft coming in to l, as the
// connection's control center does, and sends it home as its vehicles
// drive out to standby: each step timed against slowStep.
func stepTimeRun(t *testing.T, name string, l *airport.Layout) {
	g := stepTimeGraph(t, name, l)
	w := New(Options{DataDir: t.TempDir()})
	c := &stationClient{}
	cc := newControlCenter(c, w.st.core)
	cc.graph = func(string) (*airport.Graph, error) { return g, nil }
	cc.addGroundTitles(rescueTestTitles)
	_, end, ok := l.RunwayEnd(l.Runways[0].Primary.Name)
	if !ok {
		t.Fatal("no runway")
	}
	const tail = "CSA1"
	w.PriorityFor(tail, traffic.EmergencyFire)
	now := time.Now()
	lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, end.Heading, -10*1852)
	a := traffic.TrackedAircraft{Airport: l.ICAO, Updated: now}
	a.Tail, a.Position, a.Heading, a.GroundKts, a.AGLFt = tail, airport.LatLon{Lat: lat, Lon: lon}, end.Heading, 180, 3000

	t0 := time.Now()
	cc.rescueStep(now, map[string]bool{tail: true}, []traffic.TrackedAircraft{a})
	send := time.Since(t0)

	crew := func() *rescueCrew {
		cc.rescue.mu.Lock()
		defer cc.rescue.mu.Unlock()
		return cc.rescue.by[tail]
	}
	// Frames until the crew is out and both vehicles drive.
	handled := 0
	frame := func() {
		for ; handled < len(c.req); handled++ {
			cc.rescueHandle(assignedObject(c.req[handled], 700+c.req[handled]))
		}
		now = now.Add(100 * time.Millisecond)
		cc.rescueFrame(now)
	}
	for until := time.Now().Add(2 * time.Minute); crew() == nil && time.Now().Before(until); {
		frame()
		time.Sleep(20 * time.Millisecond)
	}
	cr := crew()
	if cr == nil || len(cr.trucks) != 2 {
		t.Fatalf("%s: send took %v; no crew of 2 out", name, send)
	}
	for i := 0; i < 3000 && (len(c.created) < 2 || handled < 2); i++ {
		frame()
	}
	for i := 0; i < 30; i++ {
		frame()
	}
	driving := 0
	for _, tr := range cr.trucks {
		if p, route, ok := tr.Track(); ok && len(route) >= 2 && p.GroundSpeedKts > 1 {
			driving++
		}
	}
	if driving == 0 {
		t.Fatalf("%s: no vehicle driving out to standby: nothing checked", name)
	}

	a.Updated = now
	t0 = time.Now()
	cc.rescueStep(now, map[string]bool{}, []traffic.TrackedAircraft{a})
	home := time.Since(t0)
	t.Logf("%s: the step sending the crew took %v, the one sending %d driving home %v (slowStep %v)", name, send.Round(10*time.Microsecond), driving, home.Round(10*time.Microsecond), slowStep)
	if send > slowStep {
		t.Errorf("%s: the step sending the crew took %v, over slowStep %v: traffic frames held back", name, send, slowStep)
	}
	if home > slowStep {
		t.Errorf("%s: the step sending the crew home took %v, over slowStep %v: traffic frames held back", name, home, slowStep)
	}
	// Its searches let end before the test does.
	for i := 0; i < 100; i++ {
		frame()
		time.Sleep(20 * time.Millisecond)
	}
}

// TestRescueStepTimeLargeField: on a synthetic large field.
func TestRescueStepTimeLargeField(t *testing.T) {
	stepTimeRun(t, "LKPR + grid", stepTimeLargeField(t))
}

// TestRescueStepTimeEDDF: at EDDF from simconnect-data (skipped without
// it); LKPR's graph build logged beside it.
func TestRescueStepTimeEDDF(t *testing.T) {
	stepTimeGraph(t, "LKPR", lkprLayout(t))
	stepTimeRun(t, "EDDF", stepTimeOffline(t, "EDDF"))
}
