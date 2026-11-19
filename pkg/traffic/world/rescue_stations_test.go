package world

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// stationClient records where each vehicle appears, by request ID.
type stationClient struct {
	rescueClient
	at  []airport.LatLon
	req []uint32
}

func (c *stationClient) AICreateSimulatedObject(title string, p types.SIMCONNECT_DATA_INITPOSITION, req uint32) error {
	c.created = append(c.created, title)
	c.at = append(c.at, airport.LatLon{Lat: p.Latitude, Lon: p.Longitude})
	c.req = append(c.req, req)
	return nil
}

// rescueBed is a control center at LKPR (layout l) offering titles, with
// CSA1 coming in to runway 24 nm out.
func rescueBed(t *testing.T, l *airport.Layout, titles []string) (*controlCenter, *stationClient, *World) {
	t.Helper()
	w := New(Options{DataDir: t.TempDir()})
	g, err := airport.BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	c := &stationClient{}
	cc := &controlCenter{core: w.st.core, log: w.st.core.log, clock: traffic.NewSimClock(), client: c, inj: traffic.NewInjector(c),
		ids: traffic.NewIDBlocks(1000, 2000, controlIDBlock, 4), items: map[int]*controlled{},
		graph: func(string) (*airport.Graph, error) { return g, nil }}
	cc.addGroundTitles(titles)
	return cc, c, w
}

// rescueComing sends name's crew: it declares kind and comes within reach
// of LKPR 24.
func rescueComing(t *testing.T, cc *controlCenter, w *World, l *airport.Layout, name string, kind traffic.EmergencyKind, now time.Time) *rescueCrew {
	t.Helper()
	_, end, _ := l.RunwayEnd("24")
	w.PriorityFor(name, kind)
	lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, end.Heading, -10*1852)
	a := traffic.TrackedAircraft{Airport: "LKPR", Updated: now}
	a.Tail, a.Position, a.Heading, a.GroundKts = name, airport.LatLon{Lat: lat, Lon: lon}, end.Heading, 180
	cc.rescueStep(now, map[string]bool{name: true}, []traffic.TrackedAircraft{a})
	return cc.rescue.by[name]
}

var rescueTestTitles = []string{"Truck Fire Airport Medium", "Truck Fire Short", "Truck Utility Europe Medic Box"}

// TestRescueNoCountAsBefore: a layout giving no fire station's count
// sends rescueFireTrucks (2) fire trucks at once, each from the depot
// nearest its standby point, and an ambulance for a medical: as before
// #1587.
func TestRescueNoCountAsBefore(t *testing.T) {
	l := lkprLayout(t)
	if len(l.FireStations()) != 0 {
		t.Fatalf("LKPR gives fire stations %v", l.FireStations())
	}
	cc, c, w := rescueBed(t, l, rescueTestTitles)
	now := time.Now()
	crew := rescueComing(t, cc, w, l, "CSA1", traffic.EmergencyFire, now)
	if crew == nil || len(crew.trucks) != 2 || rescueFireTrucks != 2 {
		t.Fatalf("crew %+v, want 2 fire trucks", crew)
	}
	// From one depot the second waits for the first to move off (#1597).
	for i, handled := 0, 0; i < 3000 && len(c.created) < 2; i++ {
		for ; handled < len(c.req); handled++ {
			cc.rescueHandle(assignedObject(c.req[handled], 700+c.req[handled]))
		}
		now = now.Add(100 * time.Millisecond)
		cc.rescueFrame(now)
	}
	if len(c.created) != 2 || c.created[0] != "Truck Fire Airport Medium" || c.created[1] != "Truck Fire Short" {
		t.Fatalf("created %v", c.created)
	}
	for _, tr := range crew.trucks {
		if tr.Depot != nil || tr.Waiting() {
			t.Errorf("default crew: depot %v, waiting %v", tr.Depot, tr.Waiting())
		}
	}
	c.created = nil
	if crew := rescueComing(t, cc, w, l, "CSA2", traffic.EmergencyMedical, now); crew == nil || len(c.created) != 1 || c.created[0] != "Truck Utility Europe Medic Box" {
		t.Errorf("medical: created %v, want an ambulance", c.created)
	}
}

// TestRescueStationCount: a fire station giving 4 sends 4 to any emergency
// but a medical (here hydraulic, as a crash or fire), from
// it; they appear one by one, each once the one before is
// RescueDepotGapMeters out, never on top of one another, and each drives
// to its own standby point.
func TestRescueStationCount(t *testing.T) {
	l := lkprLayout(t)
	depots := l.VehicleDepots()
	if len(depots) == 0 {
		t.Skip("no vehicle depot at LKPR")
	}
	l.Parking[depots[0]].RescueVehicles = 4
	if fs := l.FireStations(); len(fs) != 1 || fs[0] != depots[0] {
		t.Fatalf("fire stations %v", fs)
	}
	station := l.Parking[depots[0]].Position
	cc, c, w := rescueBed(t, l, rescueTestTitles)
	now := time.Now()
	crew := rescueComing(t, cc, w, l, "CSA1", traffic.EmergencyHydraulic, now)
	if crew == nil || len(crew.trucks) != 4 {
		t.Fatalf("crew %+v, want 4", crew)
	}
	if len(c.created) != 1 {
		t.Fatalf("appeared at once: %v, want 1 (the rest wait at the station)", c.created)
	}
	for _, tr := range crew.trucks {
		if tr.Depot == nil || *tr.Depot != station {
			t.Fatalf("truck from %v, want the station %v", tr.Depot, station)
		}
	}
	handled := 0
	for step := 0; step < 3000 && len(c.created) < 4; step++ {
		for ; handled < len(c.req); handled++ {
			if !cc.rescueHandle(assignedObject(c.req[handled], 700+c.req[handled])) {
				t.Fatalf("creation answer %d not taken", c.req[handled])
			}
		}
		now = now.Add(100 * time.Millisecond)
		n := len(c.created)
		cc.rescueFrame(now)
		if len(c.created) > n {
			for _, tr := range crew.trucks {
				if p, _, ok := tr.Track(); ok {
					if d := calc.HaversineMeters(p.Position.Lat, p.Position.Lon, c.at[0].Lat, c.at[0].Lon); d < traffic.RescueDepotGapMeters-1 {
						t.Fatalf("vehicle %d appeared with another %.1f m off where they appear", len(c.created), d)
					}
				}
			}
		}
	}
	if len(c.created) != 4 {
		t.Fatalf("created %d of 4", len(c.created))
	}
	for i, p := range c.at {
		if d := calc.HaversineMeters(p.Lat, p.Lon, c.at[0].Lat, c.at[0].Lon); d > 1 || calc.HaversineMeters(p.Lat, p.Lon, station.Lat, station.Lon) > 50 {
			t.Errorf("vehicle %d appeared %.1f m from the first, or not at the station", i, d)
		}
	}
}

// TestRescueMedicalReduced: with fire stations, a medical gets the reduced
// set from the station nearest the runway: an ambulance, or one fire
// truck when the simulator has no ambulance.
func TestRescueMedicalReduced(t *testing.T) {
	for _, tc := range []struct {
		titles []string
		want   string
	}{
		{rescueTestTitles, "Truck Utility Europe Medic Box"},
		{rescueTestTitles[:2], "Truck Fire Airport Medium"},
	} {
		l := lkprLayout(t)
		depots := l.VehicleDepots()
		if len(depots) == 0 {
			t.Skip("no vehicle depot at LKPR")
		}
		for _, d := range depots {
			l.Parking[d].RescueVehicles = 3
		}
		cc, c, w := rescueBed(t, l, tc.titles)
		crew := rescueComing(t, cc, w, l, "CSA1", traffic.EmergencyMedical, time.Now())
		if crew == nil || len(crew.trucks) != 1 || len(c.created) != 1 || c.created[0] != tc.want {
			t.Fatalf("medical (titles %v): created %v, want just %s", tc.titles, c.created, tc.want)
		}
		_, end, _ := l.RunwayEnd(crew.runway)
		near, best := -1, 1e18
		for _, d := range depots {
			p := l.Parking[d].Position
			if m := calc.HaversineMeters(p.Lat, p.Lon, end.Threshold.Lat, end.Threshold.Lon); m < best {
				near, best = d, m
			}
		}
		if tr := crew.trucks[0]; tr.Depot == nil || *tr.Depot != l.Parking[near].Position {
			t.Errorf("medical from %v, want the nearest station %v", tr.Depot, l.Parking[near].Position)
		}
	}
}

// TestRescueTwoStations: two fire stations (2 and 3) both send theirs to a
// fire: 5 vehicles, 2 from one and 3 from the other, the first of each
// leaving at once.
func TestRescueTwoStations(t *testing.T) {
	l := lkprLayout(t)
	depots := l.VehicleDepots()
	if len(depots) < 2 {
		t.Skip("fewer than 2 vehicle depots at LKPR")
	}
	a, b := depots[0], depots[len(depots)-1]
	pa, pb := l.Parking[a].Position, l.Parking[b].Position
	if calc.HaversineMeters(pa.Lat, pa.Lon, pb.Lat, pb.Lon) < traffic.RescueDepotGapMeters {
		t.Skip("depots too close together")
	}
	l.Parking[a].RescueVehicles, l.Parking[b].RescueVehicles = 2, 3
	cc, c, w := rescueBed(t, l, rescueTestTitles)
	crew := rescueComing(t, cc, w, l, "CSA1", traffic.EmergencyFire, time.Now())
	if crew == nil || len(crew.trucks) != 5 {
		t.Fatalf("crew %+v, want 5", crew)
	}
	from := map[airport.LatLon]int{}
	for _, tr := range crew.trucks {
		if tr.Depot == nil {
			t.Fatal("truck with no station")
		}
		from[*tr.Depot]++
	}
	if from[pa] != 2 || from[pb] != 3 {
		t.Errorf("from the stations %v, want 2 from %v and 3 from %v", from, pa, pb)
	}
	if len(c.created) != 2 {
		t.Errorf("appeared at once %d, want 2 (the first of each station)", len(c.created))
	}
	seen := map[airport.LatLon]bool{}
	for _, p := range c.at {
		for _, s := range []airport.LatLon{pa, pb} {
			if calc.HaversineMeters(p.Lat, p.Lon, s.Lat, s.Lon) < 50 {
				seen[s] = true
			}
		}
	}
	if !seen[pa] || !seen[pb] {
		t.Errorf("first vehicles at %v, want one at each station", c.at)
	}
}
