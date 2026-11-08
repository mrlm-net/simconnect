package traffic

import (
	"math"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// placeClient records each object's last INITPOSITION.
type placeClient struct {
	eventClient
	last map[uint32]types.SIMCONNECT_DATA_INITPOSITION
}

func (c *placeClient) AICreateSimulatedObject(string, types.SIMCONNECT_DATA_INITPOSITION, uint32) error {
	return nil
}

func (c *placeClient) SetDataOnSimObject(def, obj uint32, f types.SIMCONNECT_DATA_SET_FLAG, n, size uint32, p unsafe.Pointer) error {
	if def == DefaultInjectDefinitionBase+injDefPosition {
		if c.last == nil {
			c.last = map[uint32]types.SIMCONNECT_DATA_INITPOSITION{}
		}
		c.last[obj] = *(*types.SIMCONNECT_DATA_INITPOSITION)(p)
	}
	return c.eventClient.SetDataOnSimObject(def, obj, f, n, size, p)
}

// The tug stays on the nose wheel through the whole push: as written to the
// sim in each frame, its reference point stays TugAheadMeters from the
// aircraft's nose gear (within 2 cm) while pushing, braking, stopping and
// through the wait before it disconnects. Placed from the aircraft's pose
// of the frame before, it trailed by a frame's travel, and the gap closed
// as the push braked: the tug slid on the nose wheel (live, 2026-11-07).
func TestTugStaysOnTheNoseWheelThroughTheStop(t *testing.T) {
	g := lkprGraph(t)
	c := &placeClient{}
	inj := NewInjector(c)
	ctl := NewTaxiController(NewFleet(c), TaxiWithInjector(inj))
	c22, _ := g.Layout.ParkingIndex("C22")
	model := "FSLTL_B738_KLM"
	tug := NewSimObjectTug(c, inj, DefaultTugTitle, 9001, MotionProfileFor(model))
	if err := ctl.Start(TaxiRequest{Graph: g, Parking: c22, Runway: "24", Model: model, Tail: "KLM1", RollingTakeoffChance: -1, Tug: tug}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	mon := DefaultTaxiRequestBase + reqOffMonitor
	stand := g.Layout.Parking[c22]
	frame := func() {
		now = now.Add(time.Second / 60)
		ctl.Handle(positionMsg(mon, 77, stand.Position, 0, 0, true))
	}
	assigned := false
	checked, braking, waiting := 0, 0, 0
	worst := 0.0
	lastKts := 0.0
	for i := 0; i < 60*600 && ctl.State() < TaxiTaxiing && !ctl.State().Terminal(); i++ {
		frame()
		if !assigned && tug.ObjectID() == 0 && ctl.tugAttached {
			ctl.Handle(assignedMsg(9001, 55))
			inj.Handle(groundMsg(DefaultInjectRequestBase+3, 55, 1200, 3))
			assigned = true
			continue
		}
		if ctl.State() == TaxiAwaitingPushback && !ctl.pushCleared {
			ctl.ClearPushback()
		}
		tug.mu.Lock()
		on := tug.objectID != 0 && tug.arrive == nil && tug.away == nil
		prof := tug.prof
		tug.mu.Unlock()
		ac, okA := c.last[77]
		tp, okT := c.last[55]
		if !on || !okA || !okT || ctl.mover == nil {
			continue
		}
		nose := NoseGear(airport.LatLon{Lat: ac.Latitude, Lon: ac.Longitude}, ac.Heading, prof)
		d := math.Abs(localDist(nose, airport.LatLon{Lat: tp.Latitude, Lon: tp.Longitude}) - TugAheadMeters)
		kts := ctl.mover.Pose().GroundSpeedKts
		switch {
		case ctl.State() == TaxiPushback && kts < lastKts:
			braking++
		case ctl.State() > TaxiPushback:
			waiting++
		}
		lastKts = kts
		checked++
		if d > worst {
			worst = d
		}
		if d > 0.02 {
			t.Fatalf("frame %d (%v, %.2f kt): tug %.3f m off its place on the nose gear", i, ctl.State(), kts, d)
		}
	}
	if checked < 600 || braking < 30 || waiting < 60 {
		t.Fatalf("checked %d frames (%d braking, %d waiting after the push), state %v", checked, braking, waiting, ctl.State())
	}
	t.Logf("%d frames (%d braking, %d after the push): at most %.3f m off", checked, braking, waiting, worst)
}

var _ engine.Client = (*placeClient)(nil)
