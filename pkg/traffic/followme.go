package traffic

import (
	"errors"
	"math"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Follow-me car (#890): it meets an arrival FollowMeLeadMeters past its
// vacate stop on its taxi path, driving there from its depot along the
// vehicle roads once the aircraft is down, and waits. As the aircraft taxis
// it leads it along the same path, FollowMeGapMeters ahead, the aircraft
// never closer (ArrivalRequest.FollowMe holds it); FollowMeLeaveMeters
// before the stand it pulls aside and drives home, round the aircraft where
// its way would cross it. An aircraft that waits FollowMeWaitMax for a car
// that does not come goes on without it. Estimates, tuned by eye.
const (
	FollowMeLeadMeters  = 100.0
	FollowMeGapMeters   = 45.0
	FollowMeLeaveMeters = 120.0
	FollowMeAsideMeters = 30.0
	FollowMeKts         = 20.0
	FollowMeWaitMax     = 4 * time.Minute
	// followMeSlackMeters: the car drives on once the aircraft is this
	// close to its stop behind it, beyond StopApproachMeters: closer, the
	// aircraft brakes exactly onto the stop and crawls behind the car.
	followMeSlackMeters = StopApproachMeters + 20
)

// FollowMeService leads an arrival to its stand (#890). The arrival calls
// Attach once it is down with its taxi path and where on it the car is to
// wait, Update on every frame with the path it drives now and how far
// along it the aircraft is, until Done, and Remove if it is cancelled.
// SimObjectFollowMe is the built-in implementation.
type FollowMeService interface {
	// Handle consumes the car's own SimConnect messages.
	Handle(msg engine.Message) bool
	// Attach sends the car to wait at meet along path.
	Attach(path *GroundPath, meet float64) error
	// Update drives it: path is the aircraft's way now, s how far along it
	// the aircraft is.
	Update(path *GroundPath, s float64, dt float64) error
	// Lead is how far along the aircraft's path it may go; ok is false
	// when the car leads no more (or never did).
	Lead() (at float64, ok bool)
	// Done reports that the car has gone home and needs no more updates.
	Done() bool
	// Remove takes it away at once.
	Remove() error
}

type followMePhase int

const (
	followMeDriving followMePhase = iota // from the depot to the meeting point
	followMeWaiting                      // at the meeting point
	followMeLeading                      // ahead of the aircraft along its path
	followMeLeaving                      // aside and home
)

// SimObjectFollowMe is a follow-me car (GSX FSDT_FollowMe_Hilux or
// FSDT_FollowMe_class_B) spawned as a simulated object and driven by the
// Injector (FollowMeService).
type SimObjectFollowMe struct {
	client engine.Client
	inj    *Injector
	title  string
	reqID  uint32
	prof   MotionProfile // the aircraft's
	// Layout is the airport, for the way from the depot and back. nil, or
	// no depot or road: it appears at the meeting point and drives off
	// ahead.
	Layout *airport.Layout
	vehicleYield

	mu       sync.Mutex
	objectID uint32
	created  bool
	early    bool
	done     bool
	err      error
	phase    followMePhase
	pose     GroundPose
	arrive   *GroundMover // from the depot
	away     *GroundMover // aside and home
	path     *GroundPath  // the aircraft's, while waiting and leading
	s, v     float64      // along path, and its speed (m/s) while leading
	meet     float64
	depot    airport.LatLon
	hasDepot bool
}

var _ FollowMeService = (*SimObjectFollowMe)(nil)

// NewSimObjectFollowMe creates a follow-me car of the given ground vehicle
// title for an aircraft moving with prof; reqID is the request ID of its
// creation.
func NewSimObjectFollowMe(client engine.Client, inj *Injector, title string, reqID uint32, prof MotionProfile) *SimObjectFollowMe {
	return &SimObjectFollowMe{client: client, inj: inj, title: title, reqID: reqID, prof: prof}
}

// Title is the car's model.
func (f *SimObjectFollowMe) Title() string { return f.title }

// ObjectID is the car's simulated object, 0 until it has been created.
func (f *SimObjectFollowMe) ObjectID() uint32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.objectID
}

func (f *SimObjectFollowMe) Attach(path *GroundPath, meet float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.path, f.meet, f.s = path, meet, meet
	f.pose = pathPose(path, meet)
	if way, depot, ok := f.inbound(); ok {
		f.arrive, f.depot, f.hasDepot = NewGroundMoverFrom(way, followMeProfile(), localBearing(way.PointAt(0), way.PointAt(math.Min(5, way.Length()))), 0), depot, true
		f.pose = f.arrive.Pose()
	} else {
		f.phase = followMeWaiting
	}
	f.created = true
	return f.client.AICreateSimulatedObject(f.title, types.SIMCONNECT_DATA_INITPOSITION{
		Latitude: f.pose.Position.Lat, Longitude: f.pose.Position.Lon, Heading: f.pose.Heading, OnGround: 1,
	}, f.reqID)
}

// inbound is the way to the meeting point: from the nearest depot along the
// vehicle roads to FollowMeLeadMeters/2 short of it on the aircraft's path,
// then along the path onto it.
func (f *SimObjectFollowMe) inbound() (*GroundPath, airport.LatLon, bool) {
	if f.Layout == nil {
		return nil, airport.LatLon{}, false
	}
	before := f.path.PointAt(math.Max(0, f.meet-FollowMeLeadMeters/2))
	d, ok := nearestDepot(f.Layout, before)
	if !ok {
		return nil, airport.LatLon{}, false
	}
	// It waits near the runway exits, as follow-me cars do: it starts from
	// the vehicle road nearest the meeting point (a route from there to
	// itself joins that road), not from its depot far off (live distances
	// at LKPR: some 3 km, 6 minutes).
	r, err := f.Layout.VehicleRoute(before, before)
	if err != nil || len(r) < 2 {
		return nil, airport.LatLon{}, false
	}
	r = r[1:]
	if len(r) < 2 {
		r = append([]airport.LatLon{offsetHeading(before, localBearing(f.path.PointAt(f.meet), before), FollowMeAsideMeters)}, r...)
	}
	// The mover drives the front axle: it ends a wheelbase past the meeting
	// point, the car's reference point on it.
	end := f.path.PointAt(math.Min(f.path.Length(), f.meet+followMeProfile().WheelbaseMeters))
	way, err := NewArcPath(append(r, end), followMeProfile(), 8)
	if err != nil {
		return nil, airport.LatLon{}, false
	}
	return way, d, true
}

func (f *SimObjectFollowMe) Handle(msg engine.Message) bool {
	if msg.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID {
		return false
	}
	m := msg.AsAssignedObjectID()
	if uint32(m.DwRequestID) != f.reqID {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	id := uint32(m.DwObjectID)
	switch {
	case !f.created:
		return false
	case f.objectID != 0 && f.done:
		return false
	case f.objectID != 0:
		if id != f.objectID {
			f.err = f.client.AIRemoveObject(id, f.reqID) // a second one from a retry (#90)
		}
		return true
	case f.done || f.early:
		f.objectID, f.done = id, true
		f.err = f.client.AIRemoveObject(id, f.reqID)
		return true
	}
	f.objectID = id
	f.err = f.inj.Takeover(f.objectID)
	return true
}

func (f *SimObjectFollowMe) Update(path *GroundPath, s float64, dt float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.done || f.objectID == 0 {
		return nil
	}
	if err := f.err; err != nil {
		f.err = nil
		return err
	}
	if path != nil && path != f.path && f.phase != followMeLeaving {
		// A new way for the aircraft (a re-route): the car's place on it.
		f.meet = nearestAlong(path, f.pathPoint(), s)
		f.s, f.path = math.Max(f.meet, s), path
	}
	switch f.phase {
	case followMeDriving:
		f.check(f.arrive)
		f.pose = f.arrive.Step(dt)
		if f.pose.Arrived {
			f.arrive, f.phase = nil, followMeWaiting
			f.pose = pathPose(f.path, f.s)
		}
	case followMeWaiting, followMeLeading:
		// Ahead of the aircraft by the gap: on once it comes within
		// followMeSlackMeters of where the car holds it (it stops there: the
		// car must not wait for it to come closer), waiting when it stops.
		want := math.Max(0, math.Min((s+FollowMeGapMeters+followMeSlackMeters-f.s)/3, FollowMeKts*0.514444))
		step := followMeProfile().Accel * dt
		f.v = math.Max(f.v-step*2, math.Min(f.v+step, want))
		if f.v > 0.1 {
			f.phase = followMeLeading
		}
		f.s = math.Min(f.s+f.v*dt, f.path.Length())
		f.pose = pathPose(f.path, f.s)
		f.pose.GroundSpeedKts = f.v / 0.514444
		if f.path.Length()-f.s <= FollowMeLeaveMeters {
			f.leave(pathPose(f.path, s))
		}
	case followMeLeaving:
		f.check(f.away)
		f.pose = f.away.Step(dt)
		if f.pose.Arrived {
			return f.finish()
		}
	}
	return f.place()
}

// leave turns the car aside off the aircraft's way and home, round the
// aircraft (at aircraft) where its way would cross it.
func (f *SimObjectFollowMe) leave(aircraft GroundPose) {
	f.phase = followMeLeaving
	p, h := f.pose.Position, f.pose.Heading
	side := 1.0 // aside to the right, unless the stand lies that way
	if end := f.path.PointAt(f.path.Length()); math.Sin((localBearing(p, end)-h)*math.Pi/180) > 0 {
		side = -1
	}
	p1 := offsetHeading(offsetHeading(p, h, FollowMeAsideMeters/2), h+side*90, FollowMeAsideMeters/3)
	p2 := offsetHeading(offsetHeading(p, h, FollowMeAsideMeters), h+side*90, FollowMeAsideMeters)
	pts := []airport.LatLon{p, p1, p2}
	if f.hasDepot {
		if route, err := f.Layout.VehicleRoute(p2, f.depot); err == nil {
			pts = append(pts, route[1:]...)
		}
	} else {
		pts = append(pts, offsetHeading(p2, h+side*90, FuelDriveOffMeters))
	}
	way, err := NewArcPath(aroundAircraft(pts, aircraft, f.prof), followMeProfile(), 8)
	if err != nil {
		f.done = true
		return
	}
	f.away = NewGroundMoverFrom(way, followMeProfile(), h, f.v/0.514444)
}

// pathPoint is where the car is on the aircraft's path now.
func (f *SimObjectFollowMe) pathPoint() airport.LatLon {
	if f.phase == followMeDriving {
		return f.path.PointAt(f.meet)
	}
	return f.pose.Position
}

func (f *SimObjectFollowMe) place() error {
	road := f.arrive
	if road == nil {
		road = f.away
	}
	shown := lane(f.pose, road)
	f.report(f.objectID, shown, followMeProfile().WheelbaseMeters+2)
	err := f.inj.PlaceMoving(f.objectID, shown)
	if errors.Is(err, ErrGroundUnknown) {
		return nil
	}
	return err
}

func (f *SimObjectFollowMe) finish() error {
	f.done = true
	obj := f.objectID
	f.forget(obj)
	f.inj.Forget(obj)
	return f.client.AIRemoveObject(obj, f.reqID)
}

// Lead is how far along the aircraft's path it may go: FollowMeGapMeters
// behind the car, or behind the meeting point while the car drives there.
func (f *SimObjectFollowMe) Lead() (float64, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.created || f.done || f.phase == followMeLeaving {
		return 0, false
	}
	if f.phase == followMeDriving || f.objectID == 0 {
		return f.meet - FollowMeGapMeters, true
	}
	return f.s - FollowMeGapMeters, true
}

// Leading reports that the car waits for or leads the aircraft.
func (f *SimObjectFollowMe) Leading() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.objectID != 0 && !f.done && (f.phase == followMeWaiting || f.phase == followMeLeading)
}

// Driving reports that the car is on its way in or home: the aircraft's
// frames move it, so they come at the full rate meanwhile.
func (f *SimObjectFollowMe) Driving() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.objectID != 0 && !f.done && f.phase != followMeWaiting
}

func (f *SimObjectFollowMe) Done() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.done
}

func (f *SimObjectFollowMe) Remove() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.objectID == 0 {
		f.done = true
		return nil
	}
	if f.done {
		return nil
	}
	return f.finish()
}

// Track is where it is and the way it still drives; ok is false before it
// is created or once removed.
func (f *SimObjectFollowMe) Track() (pose GroundPose, route []airport.LatLon, ok bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.objectID == 0 || f.done {
		return GroundPose{}, nil, false
	}
	m := f.arrive
	if m == nil {
		m = f.away
	}
	if m != nil {
		pts := m.Path().Points()
		near, best := 0, math.Inf(1)
		for i, p := range pts {
			if d := localDist(p, f.pose.Position); d < best {
				near, best = i, d
			}
		}
		route = append([]airport.LatLon{f.pose.Position}, pts[min(near+1, len(pts)):]...)
	}
	return f.pose, route, true
}

// followMeProfile moves a follow-me car: a short wheelbase, brisk.
func followMeProfile() MotionProfile {
	p := DefaultMotionProfile()
	p.WheelbaseMeters, p.RefAheadMeters = 3, 0
	p.CruiseKts, p.MinTurnKts, p.Accel, p.Decel = FollowMeKts, 4, 1.2, 1.5
	p.SpanMeters, p.TailMeters = 2, 2
	return p
}

// pathPose is the pose at s along path, facing along it.
func pathPose(path *GroundPath, s float64) GroundPose {
	a, b := path.PointAt(math.Max(0, s-2)), path.PointAt(math.Min(path.Length(), s+2))
	return GroundPose{Position: path.PointAt(s), Heading: localBearing(a, b), Distance: s}
}

// nearestAlong is how far along path, from from on, the point nearest p
// is.
func nearestAlong(path *GroundPath, p airport.LatLon, from float64) float64 {
	best, at := math.Inf(1), from
	for s := math.Max(0, from); s <= path.Length(); s += 2 {
		if d := localDist(path.PointAt(s), p); d < best {
			best, at = d, s
		}
	}
	return at
}
