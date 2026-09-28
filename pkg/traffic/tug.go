//go:build windows
// +build windows

package traffic

import (
	"errors"
	"math"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// PushbackTug shows the tug of an injected pushback (#304). The departure
// calls Attach while the aircraft waits for pushback, Update on every frame
// until Done (with the aircraft's pose; pushing is false once the push has
// ended) and Remove if the departure is cancelled. SimObjectTug is the
// built-in implementation; a third-party integration (e.g. GSX) can take
// its place.
type PushbackTug interface {
	// Handle consumes the tug's own SimConnect messages.
	Handle(msg engine.Message) bool
	// Attach brings the tug to the aircraft, parked at pose.
	Attach(pose GroundPose) error
	// Update moves the tug with the aircraft while pushing, and away after.
	Update(pose GroundPose, pushing bool, dt float64) error
	// Done reports that the tug has left and needs no more updates.
	Done() bool
	// Remove takes the tug away at once.
	Remove() error
}

// SimObjectTug is a pushback tug spawned as a simulated object (a GSX
// FSDT_Pushback_* model, for example) and driven by the Injector: it sits on
// the nose gear during the push, then drives off and is removed.
type SimObjectTug struct {
	client engine.Client
	inj    *Injector
	title  string
	reqID  uint32
	prof   MotionProfile // the aircraft's: where its nose gear is
	// AheadMeters is how far the tug's reference point is ahead of the
	// aircraft's nose gear; YawDeg turns the tug against the aircraft
	// heading (0: the tug faces the way the aircraft does).
	AheadMeters, YawDeg float64

	mu        sync.Mutex
	objectID  uint32
	pose      GroundPose // last placed
	away      *GroundMover
	reversing bool    // backing off the nose, before driving away
	bar       float64 // tow bar direction, from the nose wheel to the tug
	haveBar   bool
	lastNose  airport.LatLon // nose gear at the last bar update
	haveNose  bool
	waitLeft  float64 // seconds to the drive-off after the push
	err       error   // from the takeover, reported by Update
	done      bool
}

// NewSimObjectTug creates a tug of the given ground vehicle title for an
// aircraft moving with prof; reqID is the SimConnect request ID of its
// creation (one per tug).
func NewSimObjectTug(client engine.Client, inj *Injector, title string, reqID uint32, prof MotionProfile) *SimObjectTug {
	return &SimObjectTug{client: client, inj: inj, title: title, reqID: reqID, prof: prof,
		AheadMeters: TugAheadMeters, YawDeg: TugYawDeg, waitLeft: TugDisconnectSeconds}
}

// ObjectID is the tug's simulated object, 0 until it has been created.
func (t *SimObjectTug) ObjectID() uint32 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.objectID
}

// at is where the tug is for an aircraft pose.
func (t *SimObjectTug) at(pose GroundPose) GroundPose {
	nose := NoseGear(pose.Position, pose.Heading, t.prof)
	bar := pose.Heading
	if t.haveBar {
		bar = t.bar
	}
	return GroundPose{Position: offsetHeading(nose, bar, t.AheadMeters), Heading: normDeg(bar + t.YawDeg), GroundSpeedKts: pose.GroundSpeedKts}
}

// steer turns the tow bar with the push: the tug pushes the nose wheel along
// the bar, so the bar points against the nose wheel's direction of travel
// (swinging out in the arc), within TugMaxBarDeg of the aircraft axis and
// eased over TugBarSeconds.
func (t *SimObjectTug) steer(pose GroundPose, dt float64) {
	nose := NoseGear(pose.Position, pose.Heading, t.prof)
	if !t.haveNose {
		t.lastNose, t.haveNose = nose, true
		return
	}
	if localDist(t.lastNose, nose) < 0.05 {
		return // too little movement for a direction
	}
	want := localBearing(t.lastNose, nose) + 180
	rel := math.Max(-TugMaxBarDeg, math.Min(TugMaxBarDeg, headingDiff(pose.Heading, want)))
	want = pose.Heading + rel
	k := 1 - math.Exp(-dt/TugBarSeconds)
	t.bar = normDeg(t.bar + headingDiff(t.bar, want)*k)
	t.lastNose = nose
}

func normDeg(d float64) float64 {
	for d < 0 {
		d += 360
	}
	for d >= 360 {
		d -= 360
	}
	return d
}

func (t *SimObjectTug) Attach(pose GroundPose) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.bar, t.haveBar = pose.Heading, true
	t.pose = t.at(pose)
	return t.client.AICreateSimulatedObject(t.title, types.SIMCONNECT_DATA_INITPOSITION{
		Latitude: t.pose.Position.Lat, Longitude: t.pose.Position.Lon, Heading: t.pose.Heading, OnGround: 1,
	}, t.reqID)
}

func (t *SimObjectTug) Handle(msg engine.Message) bool {
	if msg.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID {
		return false
	}
	m := msg.AsAssignedObjectID()
	if uint32(m.DwRequestID) != t.reqID {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.objectID = uint32(m.DwObjectID)
	// Frozen and placed by the injector, like the aircraft; an error comes
	// back from the next Update.
	t.err = t.inj.Takeover(t.objectID)
	return true
}

func (t *SimObjectTug) Update(pose GroundPose, pushing bool, dt float64) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.done || t.objectID == 0 {
		return nil
	}
	if err := t.err; err != nil {
		t.err = nil
		return err
	}
	if pushing {
		t.steer(pose, dt)
		t.pose = t.at(pose)
		return t.place()
	}
	// Disconnected: stand a moment, back away from the nose (the tug faces
	// the aircraft), then turn off to the side and leave.
	if t.away == nil {
		if t.waitLeft -= dt; t.waitLeft > 0 {
			return t.place()
		}
		p := t.pose.Position
		back := offsetHeading(p, t.pose.Heading+180, TugBackOffMeters)
		path, err := NewArcPath([]airport.LatLon{p, back}, tugProfile(), 6)
		if err != nil {
			return t.finish()
		}
		t.away, t.reversing = NewPushbackMover(path, tugProfile(), t.pose.Heading), true
	}
	t.pose = t.away.Step(dt)
	if !t.pose.Arrived {
		return t.place()
	}
	if !t.reversing {
		return t.finish()
	}
	// Backed off: drive forward, turning TugDriveOffTurnDeg away.
	p, h := t.pose.Position, t.pose.Heading
	ahead := offsetHeading(p, h, 5)
	off := offsetHeading(ahead, h+TugDriveOffTurnDeg, TugDriveOffMeters)
	path, err := NewArcPath([]airport.LatLon{p, ahead, off}, tugProfile(), 6)
	if err != nil {
		return t.finish()
	}
	t.away, t.reversing = NewGroundMoverFrom(path, tugProfile(), h, 0), false
	return t.place()
}

func (t *SimObjectTug) place() error {
	err := t.inj.PlaceMoving(t.objectID, t.pose)
	if errors.Is(err, ErrGroundUnknown) {
		return nil
	}
	return err
}

// finish removes the tug once it has driven off.
func (t *SimObjectTug) finish() error {
	t.done = true
	obj := t.objectID
	t.inj.Forget(obj)
	return t.client.AIRemoveObject(obj, t.reqID)
}

func (t *SimObjectTug) Done() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.done
}

func (t *SimObjectTug) Remove() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.done || t.objectID == 0 {
		t.done = true
		return nil
	}
	return t.finish()
}

// tugProfile moves a tug: short wheelbase, brisk but not fast.
func tugProfile() MotionProfile {
	p := DefaultMotionProfile()
	p.WheelbaseMeters, p.RefAheadMeters = 3, 0
	p.CruiseKts, p.MinTurnKts, p.Accel, p.Decel = TugDriveOffKts, 3, 0.6, 0.8
	p.SpanMeters, p.TailMeters = 3, 3
	return p
}
