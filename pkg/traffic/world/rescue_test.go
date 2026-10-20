package world

import (
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// rescueClient records the vehicles created and removed.
type rescueClient struct {
	engine.Client
	created []string
	removed int
}

func (c *rescueClient) AICreateSimulatedObject(title string, _ types.SIMCONNECT_DATA_INITPOSITION, _ uint32) error {
	c.created = append(c.created, title)
	return nil
}
func (c *rescueClient) AIRemoveObject(uint32, uint32) error { c.removed++; return nil }
func (c *rescueClient) AddToDataDefinition(uint32, string, string, types.SIMCONNECT_DATATYPE, float32, uint32) error {
	return nil
}
func (c *rescueClient) RequestDataOnSimObject(uint32, uint32, uint32, types.SIMCONNECT_PERIOD, types.SIMCONNECT_DATA_REQUEST_FLAG, uint32, uint32, uint32) error {
	return nil
}
func (c *rescueClient) GetLastSentPacketID() (uint32, error)          { return 0, nil }
func (c *rescueClient) MapClientEventToSimEvent(uint32, string) error { return nil }
func (c *rescueClient) TransmitClientEvent(uint32, uint32, uint32, uint32, types.SIMCONNECT_EVENT_FLAG) error {
	return nil
}
func (c *rescueClient) AIReleaseControl(uint32, uint32) error { return nil }
func (c *rescueClient) SetDataOnSimObject(uint32, uint32, types.SIMCONNECT_DATA_SET_FLAG, uint32, uint32, unsafe.Pointer) error {
	return nil
}

func assignedObject(req, obj uint32) engine.Message {
	m := &types.SIMCONNECT_RECV_ASSIGNED_OBJECT_ID{DwRequestID: types.DWORD(req), DwObjectID: types.DWORD(obj)}
	m.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID)
	return engine.Message{SIMCONNECT_RECV: &m.SIMCONNECT_RECV}
}

// TestRescueCrew: an aircraft with priority (a fire) gets the fire trucks
// as it comes within rescueDispatchNM, not before and only once; they
// follow it once down, go home a while after it stopped, and their IDs
// come back; a medical gets an ambulance.
func TestRescueCrew(t *testing.T) {
	w := New(Options{DataDir: t.TempDir()})
	l := lkprLayout(t)
	g, err := airport.BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	c := &rescueClient{}
	cc := &controlCenter{core: w.st.core, log: w.st.core.log, clock: traffic.NewSimClock(), client: c, inj: traffic.NewInjector(c),
		ids: traffic.NewIDBlocks(1000, 2000, controlIDBlock, 4), items: map[int]*controlled{},
		graph: func(string) (*airport.Graph, error) { return g, nil }}
	cc.addGroundTitles([]string{"Truck Fire Airport Medium", "Truck Fire Short", "Truck Utility Europe Medic Box"})
	_, end, _ := l.RunwayEnd("24")
	w.PriorityFor("CSA1", traffic.EmergencyFire)
	at := func(nm float64, ground bool, kts float64, now time.Time) traffic.TrackedAircraft {
		lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, end.Heading, -nm*1852)
		a := traffic.TrackedAircraft{Airport: "LKPR", Updated: now}
		a.Tail, a.Position, a.Heading, a.OnGround, a.GroundKts = "CSA1", airport.LatLon{Lat: lat, Lon: lon}, end.Heading, ground, kts
		return a
	}
	prio := map[string]bool{"CSA1": true}
	now := time.Now()
	cc.rescueStep(now, prio, []traffic.TrackedAircraft{at(30, false, 250, now)})
	if len(c.created) != 0 {
		t.Fatalf("sent at 30 NM: %v", c.created)
	}
	cc.rescueStep(now, prio, []traffic.TrackedAircraft{at(15, false, 200, now)})
	cc.rescueStep(now, prio, []traffic.TrackedAircraft{at(14, false, 200, now)})
	if len(c.created) != rescueFireTrucks || c.created[0] != "Truck Fire Airport Medium" {
		t.Fatalf("created %v, want %d fire trucks once", c.created, rescueFireTrucks)
	}
	crew := cc.rescue.by["CSA1"]
	if crew == nil || crew.runway != cc.runwayFor(g, true, -1) {
		t.Fatalf("crew %+v", crew)
	}
	for i, tr := range crew.trucks {
		tr.Handle(assignedObject(2000+uint32(i), 600+uint32(i)))
	}
	now = now.Add(5 * time.Minute)
	flare := at(0.05, false, 130, now)
	flare.AGLFt = 30
	cc.rescueStep(now, prio, []traffic.TrackedAircraft{flare})
	if crew.state != rescueStandby {
		t.Fatalf("in the flare, state %d: following before the touchdown", crew.state)
	}
	now = now.Add(5 * time.Second)
	cc.rescueStep(now, prio, []traffic.TrackedAircraft{at(-0.3, true, 100, now)})
	if crew.state != rescueFollowing {
		t.Fatalf("down, state %d", crew.state)
	}
	// Taxiing at 15 kt while the simulator says 0 kt (AI on the ground):
	// still moving.
	s := -0.3
	for i := 0; i < 90; i++ {
		now, s = now.Add(time.Second), s-15.0/3600
		cc.rescueStep(now, prio, []traffic.TrackedAircraft{at(s, true, 0, now)})
	}
	if crew.state != rescueFollowing || !crew.stoppedAt.IsZero() {
		t.Fatalf("taxiing at 0 kt said, state %d, stopped since %v", crew.state, crew.stoppedAt)
	}
	now = now.Add(time.Second)
	cc.rescueStep(now, prio, []traffic.TrackedAircraft{at(s, true, 0, now)})
	now = now.Add(rescueStay)
	cc.rescueStep(now, prio, []traffic.TrackedAircraft{at(s, true, 0, now)})
	if crew.state != rescueHome {
		t.Fatalf("stopped %v, state %d", rescueStay, crew.state)
	}
	for _, tr := range crew.trucks {
		_ = tr.Remove()
	}
	cc.rescueStep(now, prio, nil)
	if len(cc.rescue.by) != 0 {
		t.Error("crew kept after all home")
	}
	if _, _, err := cc.ids.Acquire(); err != nil {
		t.Errorf("IDs not given back: %v", err)
	}

	w.PriorityFor("CSA2", traffic.EmergencyMedical)
	c.created = nil
	a := at(10, false, 180, now)
	a.Tail = "CSA2"
	cc.rescueStep(now, map[string]bool{"CSA2": true}, []traffic.TrackedAircraft{a})
	if len(c.created) != 1 || c.created[0] != "Truck Utility Europe Medic Box" {
		t.Errorf("medical: created %v, want an ambulance", c.created)
	}
}
