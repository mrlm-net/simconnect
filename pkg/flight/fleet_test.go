//go:build windows

package flight

import (
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/camera"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type fleetClient struct {
	creates []string // title
	reqs    []uint32
	removed []uint32
}

func (c *fleetClient) AICreateNonATCAircraft(title, _ string, _ types.SIMCONNECT_DATA_INITPOSITION, req uint32) error {
	c.creates, c.reqs = append(c.creates, title), append(c.reqs, req)
	return nil
}
func (c *fleetClient) AIRemoveObject(obj, _ uint32) error {
	c.removed = append(c.removed, obj)
	return nil
}

type fleetInj struct {
	fakeInjector
	taken, forgot []uint32
}

func (f *fleetInj) Takeover(obj uint32) error { f.taken = append(f.taken, obj); return nil }
func (f *fleetInj) Forget(obj uint32)         { f.forgot = append(f.forgot, obj) }

func assigned(req, obj uint32) engine.Message {
	m := &types.SIMCONNECT_RECV_ASSIGNED_OBJECT_ID{SIMCONNECT_RECV: types.SIMCONNECT_RECV{DwID: types.DWORD(types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID)},
		DwRequestID: types.DWORD(req), DwObjectID: types.DWORD(obj)}
	return engine.Message{SIMCONNECT_RECV: &m.SIMCONNECT_RECV}
}

// TestGhostFleet: on the player's clock, an aircraft is created when its
// time comes, flown once the sim gives its object, removed when its track
// ends; the budget takes the nearest; a seek back brings the earlier ones
// back and removes the later; an object given after it stopped being due
// is removed at once; Stop removes all.
func TestGhostFleet(t *testing.T) {
	player := &Track{Samples: []Sample{{T: 100, Lat: 50, Lon: 14}, {T: 200, Lat: 50, Lon: 14}}}
	sc := &Scene{Aircraft: []*SceneAircraft{
		{Key: "NEAR", Title: "A320", Track: sceneTrack(100, 30, 50.01)}, // 100…129
		{Key: "FAR", Title: "B738", Track: sceneTrack(110, 30, 51)},     // 110…139
		{Key: "LATE", Title: "C172", Track: sceneTrack(150, 20, 50.02)}, // 150…169
	}}
	c, inj := &fleetClient{}, &fleetInj{}
	p := NewPlayer(player)
	f := NewGhostFleet(c, inj, sc, p, FleetOptions{Budget: 2, Fallback: func(*SceneAircraft) string { return "FALLBACK" }})
	now := time.Now()
	p.Seek(5, now) // scene 105: NEAR only
	f.Tick(now)
	if !slices.Equal(c.creates, []string{"A320"}) {
		t.Fatalf("at 105 created %v", c.creates)
	}
	f.Handle(assigned(c.reqs[0], 501))
	f.Tick(now)
	if len(inj.taken) != 1 || len(inj.poses) == 0 {
		t.Fatalf("not flown: taken %v, %d poses", inj.taken, len(inj.poses))
	}
	p.Seek(15, now) // 115: NEAR and FAR
	f.Tick(now)
	if f.Live() != 2 || c.creates[len(c.creates)-1] != "B738" {
		t.Errorf("at 115: %d live, created %v", f.Live(), c.creates)
	}
	farReq := c.reqs[len(c.reqs)-1]
	p.Seek(55, now) // 155: LATE only (NEAR and FAR ended)
	f.Tick(now)
	if !slices.Contains(c.removed, 501) || f.Live() != 1 || c.creates[len(c.creates)-1] != "C172" {
		t.Errorf("at 155: removed %v, %d live, created %v", c.removed, f.Live(), c.creates)
	}
	f.Handle(assigned(farReq, 777)) // FAR's object, too late
	if !slices.Contains(c.removed, 777) {
		t.Error("an object no longer due kept")
	}
	// Its own title refused: after fleetRetry, once as the fallback.
	f.Tick(now.Add(fleetRetry + time.Second))
	if c.creates[len(c.creates)-1] != "FALLBACK" {
		t.Errorf("no fallback: %v", c.creates)
	}
	lateReq := c.reqs[len(c.reqs)-1]
	f.Handle(assigned(lateReq, 601))
	f.Stop()
	if !slices.Contains(c.removed, 601) || f.Live() != 0 {
		t.Errorf("stop: removed %v, %d live", c.removed, f.Live())
	}
}

// TestGhostFleetsShareConnection: two fleets on one connection, their ID
// ranges overlapping, each takes only the objects of its own requests and
// leaves the other's alone, and one past its range too (#1285: any object
// in its range not waited for was removed, the other fleet's included).
// Owns and Objects tell its aircraft.
func TestGhostFleetsShareConnection(t *testing.T) {
	player := &Track{Samples: []Sample{{T: 100, Lat: 50, Lon: 14}, {T: 200, Lat: 50, Lon: 14}}}
	sc := func(key string) *Scene {
		return &Scene{Aircraft: []*SceneAircraft{{Key: key, Title: "A320", Track: sceneTrack(100, 50, 50.01)}}}
	}
	ca, cb := &fleetClient{}, &fleetClient{}
	pa, pb := NewPlayer(player), NewPlayer(player)
	a := NewGhostFleet(ca, &fleetInj{}, sc("A"), pa, FleetOptions{IDBase: 0x7E00})
	b := NewGhostFleet(cb, &fleetInj{}, sc("B"), pb, FleetOptions{IDBase: 0x7E10})
	now := time.Now()
	pa.Seek(5, now)
	pb.Seek(5, now)
	a.Tick(now)
	b.Tick(now)
	msgB := assigned(cb.reqs[0], 902)
	if a.Handle(msgB) {
		t.Error("fleet A took fleet B's object")
	}
	if !b.Handle(msgB) || !b.Owns(902) || !slices.Equal(b.Objects(), []uint32{902}) {
		t.Errorf("fleet B: owns 902 %v, objects %v", b.Owns(902), b.Objects())
	}
	if a.Handle(assigned(0x7E00+FleetIDs, 903)) {
		t.Error("an object one past fleet A's range taken")
	}
	if len(ca.removed) > 0 || a.Owns(902) {
		t.Errorf("fleet A removed %v, owns 902 %v", ca.removed, a.Owns(902))
	}
}

// TestFilmOnAGhost: a film's subject is flown though it is the furthest and over
// the budget, and a shot cued on it follows its object: held until the object is
// there, on it once it is, held again when it is gone, on the new one after a seek.
func TestFilmOnAGhost(t *testing.T) {
	player := &Track{Samples: []Sample{{T: 100, Lat: 50, Lon: 14}, {T: 200, Lat: 50, Lon: 14}}}
	sc := &Scene{Aircraft: []*SceneAircraft{
		{Key: "NEAR", Title: "A320", Track: sceneTrack(100, 30, 50.01)},
		{Key: "FAR", Title: "B738", Track: sceneTrack(110, 30, 51)}, // 110…139
	}}
	c, inj := &fleetClient{}, &fleetInj{}
	p := NewPlayer(player)
	f := NewGhostFleet(c, inj, sc, p, FleetOptions{Budget: 1})
	now := time.Now()
	shot := func(o uint32) camera.Shot { return camera.Flyover(o, camera.Size{Span: 34, Length: 37}, 1, 8*time.Second) }
	tl := camera.NewTimeline(camera.Cue{At: 15 * time.Second, Shot: camera.OnObject(40*time.Second, func() uint32 { return f.ObjectOf("FAR") }, shot)})

	p.Seek(15, now) // scene 115: both due, the budget takes the nearest
	f.Tick(now)
	if !slices.Equal(c.creates, []string{"A320"}) {
		t.Fatalf("without Keep created %v", c.creates)
	}
	if f.ObjectOf("FAR") != 0 {
		t.Error("an object for an aircraft not flown")
	}
	f.Keep("FAR")
	f.Tick(now)
	if c.creates[len(c.creates)-1] != "B738" || f.Live() != 1 {
		t.Fatalf("Keep: created %v, %d live", c.creates, f.Live())
	}
	if _, _, ok := tl.PoseAt(16 * time.Second); ok {
		t.Error("a pose before the object is there")
	}
	f.Handle(assigned(c.reqs[len(c.reqs)-1], 777))
	pose, _, ok := tl.PoseAt(16 * time.Second)
	if !ok || pose != shot(777).PoseAt(time.Second) {
		t.Errorf("on 777: %v, %+v", ok, pose)
	}
	if f.ObjectOf("FAR") != 777 {
		t.Errorf("ObjectOf %d", f.ObjectOf("FAR"))
	}
	p.Seek(45, now) // 145: over
	f.Tick(now)
	if _, _, ok := tl.PoseAt(16 * time.Second); ok || f.ObjectOf("FAR") != 0 {
		t.Errorf("gone: pose %v, object %d", ok, f.ObjectOf("FAR"))
	}
	p.Seek(15, now) // back: created again, another object
	f.Tick(now)
	f.Handle(assigned(c.reqs[len(c.reqs)-1], 888))
	if pose, _, ok := tl.PoseAt(16 * time.Second); !ok || pose != shot(888).PoseAt(time.Second) {
		t.Errorf("on 888: %v, %+v", ok, pose)
	}
}
