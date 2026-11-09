package traffic

import (
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// tickClient answers each monitor request with the definition last asked
// for it, as the simulator would.
type tickClient struct {
	eventClient
	def map[uint32]uint32 // request: definition
}

func (c *tickClient) RequestDataOnSimObject(req, def, obj uint32, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin, interval, limit uint32) error {
	if c.def == nil {
		c.def = map[uint32]uint32{}
	}
	c.def[req] = def
	return c.eventClient.RequestDataOnSimObject(req, def, obj, period, flags, origin, interval, limit)
}

// tickMsg is a tick answer: the simulation time alone.
func tickMsg(req, def, obj uint32, simTime float64) engine.Message {
	var hdr types.SIMCONNECT_RECV_SIMOBJECT_DATA
	off := int(unsafe.Offsetof(hdr.DwData))
	buf := make([]byte, off+8)
	h := (*types.SIMCONNECT_RECV_SIMOBJECT_DATA)(unsafe.Pointer(&buf[0]))
	h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA)
	h.DwRequestID, h.DwObjectID, h.DwDefineID = types.DWORD(req), types.DWORD(obj), types.DWORD(def)
	*(*float64)(unsafe.Pointer(&buf[off])) = simTime
	return engine.Message{SIMCONNECT_RECV: (*types.SIMCONNECT_RECV)(unsafe.Pointer(&buf[0]))}
}

// A departure placed by the injector is clocked by the simulation time
// alone, not the dozen SimVars of its full monitor; the lights it reports
// are the ones commanded (beacon on for the push), and its full monitor is
// read first, before anything is placed.
func TestDepartureTickCommandedLights(t *testing.T) {
	g := lkprGraph(t)
	c := &tickClient{}
	inj := NewInjector(c)
	ctl := NewTaxiController(NewFleet(c), TaxiWithInjector(inj))
	c22, _ := g.Layout.ParkingIndex("C22")
	req := TaxiRequest{Runway: "24", Graph: g, Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA8", RollingTakeoffChance: -1}
	if err := ctl.Start(req); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	mon := DefaultTaxiRequestBase + reqOffMonitor
	full, tick := DefaultTaxiDefinitionBase+defOffMonitor, DefaultTaxiDefinitionBase+defOffTick
	if c.def[mon] != full {
		t.Fatalf("first monitor request with definition %d, want the full %d", c.def[mon], full)
	}
	stand := g.Layout.Parking[c22]
	fulls, ticks := 0, 0
	simTime := 100.0
	for k := 0; k < 60*240 && ctl.State() < TaxiTaxiing; k++ {
		now = now.Add(time.Second / 60)
		simTime += 1.0 / 60
		if c.def[mon] == tick {
			ticks++
			ctl.Handle(tickMsg(mon, tick, 77, simTime))
		} else {
			fulls++
			// The simulator's full monitor: every light off.
			ctl.Handle(positionMsg(mon, 77, stand.Position, 0, 0, true))
		}
	}
	if ctl.State() < TaxiTaxiing {
		t.Fatalf("state %v, want taxiing after the push", ctl.State())
	}
	if fulls > 2 || ticks < 100 {
		t.Errorf("%d full monitor answers, %d ticks: want the tick once placed", fulls, ticks)
	}
	if ctl.simNow != simTime {
		t.Errorf("simulation time %.3f, want the tick's %.3f", ctl.simNow, simTime)
	}
	l, ok := inj.ShownLights(77)
	if !ok || !l.Beacon {
		t.Fatalf("commanded lights %+v (%v), want the beacon on", l, ok)
	}
	if ctl.last.Lights != l {
		t.Errorf("reported lights %+v, want the commanded %+v", ctl.last.Lights, l)
	}
	if p, _, ok := inj.Shown(77); !ok || ctl.tickMonitor(0).Latitude != p.Latitude || ctl.tickMonitor(0).Heading != p.Heading {
		t.Error("tick position not where it was placed")
	}
}

// A fully injected arrival is clocked by the tick from final to its stand,
// lands and parks as with the full monitor.
func TestArrivalTickToStand(t *testing.T) {
	g := lkprGraph(t)
	c := &tickClient{}
	inj := NewInjector(c)
	ctl := NewArrivalController(NewFleet(c), ArrivalWithInjector(inj))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA7",
		InjectApproach: true, RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	p := ctl.Plan()
	mon := DefaultArrivalRequestBase + arrReqMonitor
	tick := DefaultArrivalDefinitionBase + arrDefTick
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	fulls, ticks := 0, 0
	for i := 0; i < 60*1200 && ctl.State() != ArrivalParked; i++ {
		now = now.Add(time.Second / 60)
		if c.def[mon] == tick {
			ticks++
			ctl.Handle(tickMsg(mon, tick, 77, float64(i)/60))
		} else {
			fulls++
			ctl.Handle(arrivalPositionMsg(mon, 77, p.End.Threshold, 0, 0, 0, false))
		}
	}
	if ctl.State() != ArrivalParked {
		t.Fatalf("stuck in %v", ctl.State())
	}
	t.Logf("%d full, %d ticks", fulls, ticks)
	if fulls > 2 || ticks < 1000 {
		t.Errorf("%d full monitor answers, %d ticks: want the tick once placed", fulls, ticks)
	}
}
