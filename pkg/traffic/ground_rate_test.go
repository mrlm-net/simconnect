package traffic

import (
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// rateClient records the injector's ground requests and placements in
// order, by object.
type rateClient struct {
	eventClient
	log []rateCall
}

type rateCall struct {
	obj      uint32
	ground   bool // a ground height request, else a placement
	period   types.SIMCONNECT_PERIOD
	interval uint32
}

func (c *rateClient) RequestDataOnSimObject(req, def, obj uint32, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin, interval, limit uint32) error {
	if def == DefaultInjectDefinitionBase+injDefGround {
		c.log = append(c.log, rateCall{obj: obj, ground: true, period: period, interval: interval})
	}
	return c.eventClient.RequestDataOnSimObject(req, def, obj, period, flags, origin, interval, limit)
}

func (c *rateClient) SetDataOnSimObject(def, obj uint32, flag types.SIMCONNECT_DATA_SET_FLAG, n, size uint32, p unsafe.Pointer) error {
	if def == DefaultInjectDefinitionBase+injDefPosition {
		c.log = append(c.log, rateCall{obj: obj})
	}
	return c.eventClient.SetDataOnSimObject(def, obj, flag, n, size, p)
}

// lastGround is obj's last ground request.
func (c *rateClient) lastGround(t *testing.T, obj uint32) rateCall {
	t.Helper()
	for k := len(c.log) - 1; k >= 0; k-- {
		if c.log[k].ground && c.log[k].obj == obj {
			return c.log[k]
		}
	}
	t.Fatalf("no ground request for %d", obj)
	return rateCall{}
}

func (c *rateClient) count(obj uint32, ground bool) int {
	n := 0
	for _, x := range c.log {
		if x.obj == obj && x.ground == ground {
			n++
		}
	}
	return n
}

// rateInjector is an injector on a rateClient with its own clock, objs
// taken over with their ground known.
func rateInjector(t *testing.T, objs ...uint32) (*Injector, *rateClient, *time.Time) {
	t.Helper()
	c := &rateClient{}
	inj := NewInjector(c)
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	inj.now = func() time.Time { return now }
	for k, obj := range objs {
		if err := inj.Takeover(obj); err != nil {
			t.Fatal(err)
		}
		inj.Handle(groundMsg(DefaultInjectRequestBase+2*uint32(k)+1, obj, 1200, 12.25))
	}
	return inj, c, &now
}

func isEveryFrame(r rateCall) bool {
	return r.period == types.SIMCONNECT_PERIOD_SIM_FRAME && r.interval == 0
}

// A departure standing on its stand, placed every 30th frame as Detail
// drives it: its ground read every few frames after 2 s, once a second
// after 30 s; the push brings it back to every frame in the call that
// moves it, before that placement is sent.
func TestGroundRateStandingThenMoving(t *testing.T) {
	const obj = 42
	inj, c, now := rateInjector(t, obj)
	stand := GroundPose{Position: lkpr, Heading: 214}
	frame := func(n int, pose GroundPose) {
		for k := 0; k < n; k++ {
			*now = now.Add(time.Second / 60)
			if k%30 == 0 {
				if err := inj.Place(obj, pose); err != nil {
					t.Fatal(err)
				}
			}
			inj.Handle(groundMsg(DefaultInjectRequestBase+1, obj, 1200, 12.25))
		}
	}
	frame(60, stand)
	if r := c.lastGround(t, obj); !isEveryFrame(r) {
		t.Fatalf("after 1 s standing: %+v, want every frame still", r)
	}
	frame(3*60, stand)
	if r := c.lastGround(t, obj); r.period != types.SIMCONNECT_PERIOD_SIM_FRAME || r.interval != groundStillInterval {
		t.Fatalf("after 4 s standing: %+v, want every %d frames", r, groundStillInterval+1)
	}
	frame(30*60, stand)
	if r := c.lastGround(t, obj); r.period != types.SIMCONNECT_PERIOD_SECOND {
		t.Fatalf("after 34 s standing: %+v, want once a second", r)
	}
	if n := c.count(obj, true); n != 3 {
		t.Errorf("%d ground requests standing, want 3 (each tier once)", n)
	}
	pushed := stand
	pushed.Position = offsetHeading(stand.Position, 34, 0.05)
	before := len(c.log)
	if err := inj.Place(obj, pushed); err != nil {
		t.Fatal(err)
	}
	added := c.log[before:]
	if len(added) != 2 || !added[0].ground || !isEveryFrame(added[0]) || added[1].ground {
		t.Fatalf("the first moving placement sent %+v, want the every-frame ground request, then the placement", added)
	}
}

// Parked on its stand (PlaceRest, never placed again) while the others
// move: once a second after 30 s. One only watched (MSFS AI flies it)
// stays every frame.
func TestGroundRateParkedAndWatched(t *testing.T) {
	const parked, mover, watched = 42, 43, 44
	inj, c, now := rateInjector(t, parked, mover)
	if err := inj.Watch(watched); err != nil {
		t.Fatal(err)
	}
	stand := GroundPose{Position: lkpr, Heading: 214}
	if err := inj.PlaceRest(parked, stand); err != nil {
		t.Fatal(err)
	}
	pose := GroundPose{Position: offsetHeading(lkpr, 90, 100), Heading: 90, GroundSpeedKts: 10}
	for k := 0; k < 35*60; k++ {
		*now = now.Add(time.Second / 60)
		pose.Position = offsetHeading(pose.Position, 90, 10*0.5144/60)
		if err := inj.Place(mover, pose); err != nil {
			t.Fatal(err)
		}
		inj.Handle(groundMsg(DefaultInjectRequestBase+3, mover, 1200, 12.25))
	}
	if r := c.lastGround(t, parked); r.period != types.SIMCONNECT_PERIOD_SECOND {
		t.Errorf("parked 35 s: %+v, want once a second", r)
	}
	if r := c.lastGround(t, mover); !isEveryFrame(r) || c.count(mover, true) != 1 {
		t.Errorf("moving: %+v after %d requests, want every frame, asked once", r, c.count(mover, true))
	}
	if r := c.lastGround(t, watched); !isEveryFrame(r) || c.count(watched, true) != 1 {
		t.Errorf("watched: %+v after %d requests, want every frame, asked once", r, c.count(watched, true))
	}
}

// Paused (no frames, no injector calls) with an aircraft taxiing: when the
// frames come back it is not taken for standing still.
func TestGroundRatePause(t *testing.T) {
	const obj = 42
	inj, c, now := rateInjector(t, obj)
	pose := GroundPose{Position: lkpr, Heading: 90, GroundSpeedKts: 10}
	step := func(n int) {
		for k := 0; k < n; k++ {
			*now = now.Add(time.Second / 60)
			pose.Position = offsetHeading(pose.Position, 90, 10*0.5144/60)
			if err := inj.Place(obj, pose); err != nil {
				t.Fatal(err)
			}
		}
	}
	step(60)
	*now = now.Add(40 * time.Second) // paused
	step(60)
	if n := c.count(obj, true); n != 1 {
		t.Errorf("%d ground requests across a pause, want 1 (every frame throughout)", n)
	}
}

// A vehicle that has stopped (frozen since its Takeover) is not placed
// again with the same placement; an aircraft standing still still is (its
// standing rate). Moving off, the vehicle is placed again at once.
func TestStoppedVehicleNotPlacedAgain(t *testing.T) {
	const vehicle, aircraft = 42, 43
	inj, c, now := rateInjector(t, vehicle, aircraft)
	spot := GroundPose{Position: lkpr, Heading: 120}
	parked := GroundPose{Position: offsetHeading(lkpr, 0, 80), Heading: 214}
	for k := 0; k < 4*60; k++ {
		*now = now.Add(time.Second / 60)
		if err := inj.PlaceMoving(vehicle, spot); err != nil {
			t.Fatal(err)
		}
		if err := inj.Place(aircraft, parked); err != nil {
			t.Fatal(err)
		}
	}
	if n := c.count(vehicle, false); n > 60 {
		t.Errorf("stopped vehicle placed %d times in 240 frames, want only until its height settled", n)
	}
	if n := c.count(aircraft, false); n != 4*60 {
		t.Errorf("aircraft placed %d times in 240 calls, want every one", n)
	}
	before := c.count(vehicle, false)
	*now = now.Add(time.Second / 60)
	spot.Position = offsetHeading(spot.Position, 120, 0.1)
	spot.GroundSpeedKts = 3
	if err := inj.PlaceMoving(vehicle, spot); err != nil {
		t.Fatal(err)
	}
	if c.count(vehicle, false) != before+1 {
		t.Error("moving off: not placed")
	}
}
