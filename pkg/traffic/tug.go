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
	// Layout is the airport, for the tug's way from its depot (the vehicle
	// parking spot nearest the stand, airport.Layout.VehicleDepots) to the
	// aircraft and back, on the vehicle roads (VehicleRoute). nil, or no
	// depot or road: it appears at the nose and drives off to the side.
	Layout       *airport.Layout
	vehicleYield // gives way to aircraft on its way (SetTraffic)

	mu        sync.Mutex
	objectID  uint32
	pose      GroundPose // last placed
	away      *GroundMover
	reversing bool    // backing off the nose, before driving away
	bar       float64 // tow bar direction, from the nose wheel to the tug
	haveBar   bool
	lastNose  airport.LatLon // nose gear at the last bar update
	haveNose  bool
	waitLeft  float64        // seconds to the drive-off after the push
	arrive    *GroundMover   // driving in from the depot; nil once at the nose
	depot     airport.LatLon // where it came from and goes back to
	haveDepot bool
	stand     airport.LatLon // the nose gear on the stand, before the push
	homing    bool           // driving back to the depot
	err       error          // from the takeover, reported by Update
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
	t.stand = NoseGear(pose.Position, pose.Heading, t.prof)
	// From its depot when it has one: it appears there and drives in.
	if path, depot, ok := t.inbound(pose); ok {
		t.arrive, t.depot, t.haveDepot = NewGroundMoverFrom(path, tugRoadProfile(), localBearing(path.PointAt(0), path.PointAt(math.Min(5, path.Length()))), 0), depot, true
		t.pose = t.arrive.Pose()
	}
	return t.client.AICreateSimulatedObject(t.title, types.SIMCONNECT_DATA_INITPOSITION{
		Latitude: t.pose.Position.Lat, Longitude: t.pose.Position.Lon, Heading: t.pose.Heading, OnGround: 1,
	}, t.reqID)
}

// inbound is the tug's way in for an aircraft at pose: from the nearest
// depot along the vehicle roads to a point TugApproachMeters in front of
// the nose, then straight on to the nose gear, facing the aircraft.
func (t *SimObjectTug) inbound(pose GroundPose) (*GroundPath, airport.LatLon, bool) {
	if t.Layout == nil {
		return nil, airport.LatLon{}, false
	}
	nose := NoseGear(pose.Position, pose.Heading, t.prof)
	front := offsetHeading(nose, pose.Heading, TugApproachMeters)
	depot, ok := nearestDepot(t.Layout, front)
	if !ok {
		return nil, airport.LatLon{}, false
	}
	route, err := t.Layout.VehicleRoute(depot, front)
	if err != nil {
		return nil, airport.LatLon{}, false
	}
	path, err := NewArcPath(append(route, t.pose.Position), tugRoadProfile(), 6)
	if err != nil {
		return nil, airport.LatLon{}, false
	}
	return path, depot, true
}

// nearestDepot is the vehicle parking spot of l nearest p.
func nearestDepot(l *airport.Layout, p airport.LatLon) (airport.LatLon, bool) {
	best, bestD := airport.LatLon{}, math.Inf(1)
	for _, i := range l.VehicleDepots() {
		if d := localDist(l.Parking[i].Position, p); d < bestD {
			best, bestD = l.Parking[i].Position, d
		}
	}
	return best, !math.IsInf(bestD, 1)
}

// Connected reports that the tug is at the nose: the push may start. A
// tug without a depot is there as soon as it appears.
func (t *SimObjectTug) Connected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.objectID != 0 && t.arrive == nil
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
	// Driving in from the depot; once there, on the nose.
	if t.arrive != nil {
		t.check(t.arrive)
		t.pose = t.arrive.Step(dt)
		if t.pose.Arrived {
			t.arrive, t.pose = nil, t.at(pose)
		}
		return t.place()
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
	if !t.reversing {
		t.check(t.away)
	}
	t.pose = t.away.Step(dt)
	if !t.pose.Arrived {
		return t.place()
	}
	if !t.reversing {
		return t.finish() // driven off, or home at the depot
	}
	// Backed off: home to the depot along the vehicle roads, where it has
	// one (it disappears there). With a road near the stand, back through
	// the stand the aircraft has left and onto the road: not along the
	// taxiway among the aircraft.
	if t.haveDepot && !t.homing {
		p, h := offsetHeading(t.pose.Position, t.pose.Heading, tugProfile().WheelbaseMeters), t.pose.Heading
		route, err := t.Layout.VehicleRoute(p, t.depot)
		if t.Layout.NearVehicleRoad(t.stand) {
			if via, verr := t.Layout.VehicleRoute(t.stand, t.depot); verr == nil {
				route, err = append([]airport.LatLon{p}, via...), nil
			}
		}
		if err == nil {
			if path, err := NewArcPath(route, tugRoadProfile(), 6); err == nil {
				t.away, t.reversing, t.homing = NewGroundMoverFrom(path, tugRoadProfile(), h, 0), false, true
				return t.place()
			}
		}
	}
	// Backed off: drive forward, turning TugDriveOffTurnDeg away.
	// Forward, the mover places it a wheelbase behind the path's start:
	// the path starts that far ahead, so it drives off from where it
	// stands (it jumped a wheelbase back as it turned away).
	p, h := offsetHeading(t.pose.Position, t.pose.Heading, tugProfile().WheelbaseMeters), t.pose.Heading
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
	// On the roads in its lane; reported to the other vehicles.
	var road *GroundMover
	if t.arrive != nil {
		road = t.arrive
	} else if t.away != nil && !t.reversing {
		road = t.away
	}
	shown := lane(t.pose, road)
	t.report(t.objectID, shown, tugProfile().WheelbaseMeters+1)
	err := t.inj.PlaceMoving(t.objectID, shown)
	if errors.Is(err, ErrGroundUnknown) {
		return nil
	}
	return err
}

// finish removes the tug once it has driven off.
func (t *SimObjectTug) finish() error {
	t.done = true
	obj := t.objectID
	t.forget(obj)
	t.inj.Forget(obj)
	return t.client.AIRemoveObject(obj, t.reqID)
}

// Track is where the tug is and the way it still drives (to the nose from
// its depot, or off and home after the push): none while on the nose or
// gone. ok is false before it is created or once removed.
func (t *SimObjectTug) Track() (pose GroundPose, route []airport.LatLon, ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.objectID == 0 || t.done {
		return GroundPose{}, nil, false
	}
	m := t.arrive
	if m == nil {
		m = t.away
	}
	if m != nil {
		pts := m.Path().Points()
		// From the point it is nearest on: the way still ahead.
		near, best := 0, math.Inf(1)
		for i, p := range pts {
			if d := localDist(p, t.pose.Position); d < best {
				near, best = i, d
			}
		}
		route = append([]airport.LatLon{t.pose.Position}, pts[min(near+1, len(pts)):]...)
	}
	return t.pose, route, true
}

// Clear reports that the tug is off the aircraft: backed away and driving
// off or home. The aircraft may start and taxi; the tug may still be on
// its way to the depot.
func (t *SimObjectTug) Clear() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.done || t.homing || t.away != nil && !t.reversing
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

// tugRoadProfile drives a tug on the vehicle roads, to and from its depot.
func tugRoadProfile() MotionProfile {
	p := tugProfile()
	p.CruiseKts = TugRoadKts
	return p
}

// tugProfile moves a tug: short wheelbase, brisk but not fast.
func tugProfile() MotionProfile {
	p := DefaultMotionProfile()
	p.WheelbaseMeters, p.RefAheadMeters = 3, 0
	p.CruiseKts, p.MinTurnKts, p.Accel, p.Decel = TugDriveOffKts, 3, 0.6, 0.8
	p.SpanMeters, p.TailMeters = 3, 3
	return p
}
