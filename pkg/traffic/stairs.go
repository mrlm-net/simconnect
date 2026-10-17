package traffic

import (
	"errors"
	"math"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Boarding stairs (#831) at a remote stand: a stairs vehicle (GSX
// FSDT_Staircase_*, MSFS ASO_Boarding_Stairs) drives from its depot along
// the vehicle roads to StairsApproachMeters out from the aircraft's front
// left door, then square in to the door, facing the fuselage; it stays
// while the aircraft waits, and when told to leave backs straight out the
// way it came before driving home. It is a FuelService (Attach, Update with
// leave, Done, Remove) so the departure drives it like its fuel truck.
type SimObjectStairs struct {
	client engine.Client
	inj    *Injector
	title  string
	reqID  uint32
	prof   MotionProfile // the aircraft's
	// Layout is the airport, for the way from the depot and back. nil, or
	// no depot or road: it appears at the door and drives off.
	Layout *airport.Layout
	// DoorAftMeters is how far the door is behind the nose gear (0:
	// StairsDoorAftShare of the wheelbase); SideMeters how far left of
	// the axis the stairs' reference point stands (0: StairsSideMeters).
	DoorAftMeters, SideMeters float64
	// Door, when set, is the model's own main door (#1026: the aircraft's
	// files, addons.DoorsByType): DoorAheadMeters ahead of its reference
	// point (behind: negative). It wins over DoorAftMeters.
	Door            bool
	DoorAheadMeters float64
	vehicleYield

	mu       sync.Mutex
	objectID uint32
	pose     GroundPose
	spot     GroundPose   // at the door: where it stops, square to it
	arrive   *GroundMover // driving in; nil once at the door
	back     *GroundMover // backing out
	away     *GroundMover // driving home
	depot    airport.LatLon
	hasDepot bool
	err      error
	done     bool
}

var _ FuelService = (*SimObjectStairs)(nil)

// NewSimObjectStairs creates stairs of the given ground vehicle title for
// an aircraft moving with prof; reqID is the request ID of their creation.
func NewSimObjectStairs(client engine.Client, inj *Injector, title string, reqID uint32, prof MotionProfile) *SimObjectStairs {
	return &SimObjectStairs{client: client, inj: inj, title: title, reqID: reqID, prof: prof}
}

// Boarding stairs (SimObjectStairs): the door StairsDoorAftShare of the
// wheelbase behind the nose gear, the stairs' reference point
// StairsSideMeters left of the axis facing the fuselage, coming square in
// from StairsApproachMeters out at StairsKts. Estimates, tuned by eye.
const (
	StairsDoorAftShare   = 0.15
	StairsSideMeters     = 6.0
	StairsApproachMeters = 15.0
	StairsKts            = 8.0
	// stairsTurnMeters: the radius of the quarter turn backing away.
	stairsTurnMeters = 6.0
)

// StairsSpot is where stairs stand for an aircraft at pose (its reference
// point, as the departure gives it) with profile prof: at the front left
// door, facing the fuselage (the aircraft's heading +90).
func StairsSpot(pose GroundPose, prof MotionProfile, doorAft, side float64) GroundPose {
	if doorAft <= 0 {
		doorAft = StairsDoorAftShare * prof.WheelbaseMeters
	}
	if side <= 0 {
		side = StairsSideMeters
	}
	main := offsetHeading(pose.Position, pose.Heading+180, prof.RefAheadMeters)
	nose := offsetHeading(main, pose.Heading, prof.WheelbaseMeters)
	door := offsetHeading(nose, pose.Heading+180, doorAft)
	return GroundPose{Position: offsetHeading(door, pose.Heading-90, side), Heading: normDeg(pose.Heading + 90)}
}

func (s *SimObjectStairs) Attach(pose GroundPose) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	spot := StairsSpot(pose, s.prof, s.DoorAftMeters, s.SideMeters)
	if s.Door {
		spot = StairsSpotAt(pose, s.DoorAheadMeters, s.SideMeters)
	}
	s.pose, s.spot = spot, spot
	if path, depot, ok := s.inbound(spot); ok {
		s.arrive, s.depot, s.hasDepot = NewGroundMoverFrom(path, stairsProfile(), localBearing(path.PointAt(0), path.PointAt(math.Min(5, path.Length()))), 0), depot, true
		s.pose = s.arrive.Pose()
	}
	return s.client.AICreateSimulatedObject(s.title, types.SIMCONNECT_DATA_INITPOSITION{
		Latitude: s.pose.Position.Lat, Longitude: s.pose.Position.Lon, Heading: s.pose.Heading, OnGround: 1,
	}, s.reqID)
}

// StairsSpotAt is where stairs stand for an aircraft at pose whose door is
// ahead meters ahead of its reference point (the model's own, #1026):
// side meters left of the axis (0: StairsSideMeters), facing the fuselage.
func StairsSpotAt(pose GroundPose, ahead, side float64) GroundPose {
	if side <= 0 {
		side = StairsSideMeters
	}
	door := offsetHeading(pose.Position, pose.Heading, ahead)
	return GroundPose{Position: offsetHeading(door, pose.Heading-90, side), Heading: normDeg(pose.Heading + 90)}
}

// outPoint is StairsApproachMeters out from spot, the way it backs to.
func outPoint(spot GroundPose) airport.LatLon {
	return offsetHeading(spot.Position, spot.Heading+180, StairsApproachMeters)
}

// inbound is the way in: from the nearest depot along the vehicle roads to
// the point out from the door, then square in to it.
func (s *SimObjectStairs) inbound(spot GroundPose) (*GroundPath, airport.LatLon, bool) {
	if s.Layout == nil {
		return nil, airport.LatLon{}, false
	}
	out := outPoint(spot)
	d, ok := nearestDepot(s.Layout, out)
	if !ok {
		return nil, airport.LatLon{}, false
	}
	r, err := s.Layout.VehicleRoute(d, out)
	if err != nil || len(r) < 2 {
		return nil, airport.LatLon{}, false
	}
	// The mover drives the front axle: it ends a wheelbase ahead, the
	// stairs' reference point on the spot.
	front := offsetHeading(spot.Position, spot.Heading, stairsProfile().WheelbaseMeters)
	path, err := NewArcPath(append(r, front), stairsProfile(), 6)
	if err != nil {
		return nil, airport.LatLon{}, false
	}
	return path, d, true
}

func (s *SimObjectStairs) Handle(msg engine.Message) bool {
	if msg.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID {
		return false
	}
	m := msg.AsAssignedObjectID()
	if uint32(m.DwRequestID) != s.reqID {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Removed before it existed, or a second one from a retry: away at once (#90).
	if id := uint32(m.DwObjectID); s.done || s.objectID != 0 && s.objectID != id {
		if s.objectID == 0 {
			s.objectID = id
		}
		s.err = s.client.AIRemoveObject(id, s.reqID)
		return true
	}
	s.objectID = uint32(m.DwObjectID)
	s.err = s.inj.Takeover(s.objectID)
	return true
}

func (s *SimObjectStairs) Update(pose GroundPose, leave bool, dt float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done || s.objectID == 0 {
		return nil
	}
	if err := s.err; err != nil {
		s.err = nil
		return err
	}
	switch {
	case s.arrive != nil:
		s.check(s.arrive)
		// Squared to the door on the way in: the short wheelbase leaves it a
		// few degrees off the last turn (live: 108° to the fuselage, not 90).
		s.pose = squaredIn(s.arrive.Step(dt), s.arrive, s.spot.Heading)
		if s.pose.Arrived {
			s.arrive = nil
		}
		return s.place()
	case s.back == nil && s.away == nil:
		if !leave {
			return s.place()
		}
		// Straight back out the way it came, facing the fuselage still, then
		// on back round a quarter turn, to leave forward along the fuselage
		// (turned about on the spot, it swung 180° in a frame).
		out, back := outPoint(s.pose), normDeg(s.pose.Heading+180)
		side := 1.0 // to the side its depot lies, else right
		if s.hasDepot && math.Sin((localBearing(s.pose.Position, s.depot)-s.pose.Heading)*math.Pi/180) < 0 {
			side = -1
		}
		corner := offsetHeading(out, back, stairsTurnMeters)
		path, err := NewArcPath([]airport.LatLon{s.pose.Position, out, corner, offsetHeading(corner, back+side*90, stairsTurnMeters)}, stairsProfile(), stairsTurnMeters)
		if err != nil {
			return s.finish()
		}
		s.back = NewPushbackMover(path, stairsProfile(), s.pose.Heading)
	}
	if s.back != nil {
		s.pose = s.back.Step(dt)
		if !s.pose.Arrived {
			return s.place()
		}
		s.back = nil
		// Then home, or off to the side without a depot, from the front axle
		// (the mover puts it a wheelbase behind its path's start).
		h := s.pose.Heading
		p := offsetHeading(s.pose.Position, h, stairsProfile().WheelbaseMeters)
		pts := []airport.LatLon{p, offsetHeading(p, h, 5)}
		if s.hasDepot {
			if route, err := s.Layout.VehicleRoute(pts[1], s.depot); err == nil {
				pts = append(pts, route...)
			}
		} else {
			pts = append(pts, offsetHeading(pts[1], h, FuelDriveOffMeters))
		}
		path, err := NewArcPath(pts, stairsProfile(), 6)
		if err != nil {
			return s.finish()
		}
		s.away = NewGroundMoverFrom(path, stairsProfile(), h, 0)
	}
	s.check(s.away)
	s.pose = s.away.Step(dt)
	if s.pose.Arrived {
		return s.finish()
	}
	return s.place()
}

func (s *SimObjectStairs) place() error {
	road := s.arrive
	if road == nil {
		road = s.away
	}
	shown := lane(s.pose, road)
	s.report(s.objectID, shown, stairsProfile().WheelbaseMeters+3)
	err := s.inj.PlaceMoving(s.objectID, shown)
	if errors.Is(err, ErrGroundUnknown) {
		return nil
	}
	return err
}

func (s *SimObjectStairs) finish() error {
	s.done = true
	obj := s.objectID
	s.forget(obj)
	s.inj.Forget(obj)
	return s.client.AIRemoveObject(obj, s.reqID)
}

// Fuelling reports that the stairs stand at the door (FuelService).
func (s *SimObjectStairs) Fuelling() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.objectID != 0 && !s.done && s.arrive == nil && s.back == nil && s.away == nil
}

// Clear reports that the stairs are away from the door: backed out, or
// never came, or gone.
func (s *SimObjectStairs) Clear() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.done || s.objectID == 0 || s.away != nil
}

func (s *SimObjectStairs) Done() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.done
}

func (s *SimObjectStairs) Remove() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done || s.objectID == 0 {
		s.done = true
		return nil
	}
	return s.finish()
}

// stairsProfile moves stairs: a short wheelbase, slow.
func stairsProfile() MotionProfile {
	p := DefaultMotionProfile()
	p.WheelbaseMeters, p.RefAheadMeters = 3, 0
	p.CruiseKts, p.MinTurnKts, p.Accel, p.Decel = StairsKts, 2, 0.3, 0.5
	p.SpanMeters, p.TailMeters = 3, 3
	return p
}
