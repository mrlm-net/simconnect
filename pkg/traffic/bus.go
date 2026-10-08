package traffic

import (
	"errors"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Passenger buses (#887): on a remote stand, buses bring the passengers out
// to the stairs. A bus (GSX FSDT_Cobus_3000 or FSDT_neoplan_bus, MSFS "Bus
// Apron 02") drives from its depot along the vehicle roads to a point
// BusApproachMeters ahead of its spot, on the nose side, then along the
// fuselage onto the spot, BusSideMeters left of the axis facing the tail:
// the first BusAftMeters behind the front left door, each next one
// BusGapMeters further forward, clear of the wing. Told to leave, it turns
// round away from the aircraft (BusTurnMeters) and drives home. Where a way
// would cross the aircraft it goes round its nose or tail. One bus for a
// small airliner, BusesLarge from BusLargeSpanMeters of span (A320, 737 and
// up). Estimates, tuned by eye.
const (
	BusSideMeters      = 15.0
	BusAftMeters       = 4.0
	BusGapMeters       = 20.0
	BusApproachMeters  = 30.0
	BusTurnMeters      = 14.0
	BusKts             = 12.0
	BusLargeSpanMeters = 30.0
	BusesLarge         = 2
	// busClearMeters: a way passing the aircraft keeps this far off its nose,
	// tail and wingtips.
	busClearMeters = 15.0
)

// Boarding (TaxiRequest.Buses): the buses come no earlier than
// BusBoardingTime before the stairs leave (StandServiceClearMargin before
// the tug comes), BusAfterStairs after the stairs were sent, each next one
// once the one before is at its spot, and leave BusLeaveBeforeStairs before
// the stairs do.
const (
	BusBoardingTime      = 15 * time.Minute
	BusAfterStairs       = time.Minute
	BusLeaveBeforeStairs = time.Minute
)

// Deboarding (TaxiRequest.Deboard, a turnaround's departure): the buses come
// BusAfterStairs after the stairs and stay BusDeboardTime at the aircraft;
// each boarding bus comes only once the deboarding bus on its spot is home.
const BusDeboardTime = 5 * time.Minute

// BusesFor is how many buses an aircraft moving with prof gets.
func BusesFor(prof MotionProfile) int {
	if prof.SpanMeters >= BusLargeSpanMeters {
		return BusesLarge
	}
	return 1
}

// BusSpot is where bus n (0: the first) parks for an aircraft at pose with
// profile prof: BusSideMeters left of the axis, BusAftMeters behind the
// front left door, each next one BusGapMeters further forward, facing the
// tail.
func BusSpot(n int) func(pose GroundPose, prof MotionProfile) GroundPose {
	return func(pose GroundPose, prof MotionProfile) GroundPose {
		main := offsetHeading(pose.Position, pose.Heading+180, prof.RefAheadMeters)
		nose := offsetHeading(main, pose.Heading, prof.WheelbaseMeters)
		door := offsetHeading(nose, pose.Heading+180, StairsDoorAftShare*prof.WheelbaseMeters)
		p := offsetHeading(door, pose.Heading, float64(n)*BusGapMeters-BusAftMeters)
		return GroundPose{Position: offsetHeading(p, pose.Heading-90, BusSideMeters), Heading: normDeg(pose.Heading + 180)}
	}
}

// busOwner is who holds bus n of a departure in the fleet: the call sign,
// then call sign/2, /3 for the others.
func busOwner(tail string, n int) string {
	if n == 0 {
		return tail
	}
	return tail + "/" + strconv.Itoa(n+1)
}

// SimObjectBus is a passenger bus spawned as a simulated object and driven
// by the Injector (BusSpot, BusSideMeters). It is a FuelService so the
// departure drives it like its stairs.
type SimObjectBus struct {
	client engine.Client
	inj    *Injector
	title  string
	reqID  uint32
	n      int
	prof   MotionProfile // the aircraft's
	// Layout is the airport, for the way from the depot and back. nil, or
	// no depot or road: it appears at its spot and drives off.
	Layout *airport.Layout
	vehicleYield

	mu       sync.Mutex
	objectID uint32
	pose     GroundPose
	aircraft GroundPose
	arrive   *GroundMover // driving in; nil once at the spot
	away     *GroundMover // driving home
	depot    airport.LatLon
	hasDepot bool
	created  bool // Attach asked the simulator for it
	early    bool // told to leave before it existed
	err      error
	done     bool
}

var _ FuelService = (*SimObjectBus)(nil)

// NewSimObjectBus creates bus n (0: the first) of the given ground vehicle
// title for an aircraft moving with prof; reqID is the request ID of its
// creation.
func NewSimObjectBus(client engine.Client, inj *Injector, title string, reqID uint32, n int, prof MotionProfile) *SimObjectBus {
	return &SimObjectBus{client: client, inj: inj, title: title, reqID: reqID, n: n, prof: prof}
}

// Title is the bus's model.
func (b *SimObjectBus) Title() string { return b.title }

// ObjectID is the bus's simulated object, 0 until it has been created.
func (b *SimObjectBus) ObjectID() uint32 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.objectID
}

func (b *SimObjectBus) Attach(pose GroundPose) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	spot := BusSpot(b.n)(pose, b.prof)
	b.pose, b.aircraft = spot, pose
	if path, depot, ok := b.inbound(spot); ok {
		b.arrive, b.depot, b.hasDepot = NewGroundMoverFrom(path, busProfile(), localBearing(path.PointAt(0), path.PointAt(math.Min(5, path.Length()))), 0), depot, true
		b.pose = b.arrive.Pose()
	}
	b.created = true
	return b.client.AICreateSimulatedObject(b.title, types.SIMCONNECT_DATA_INITPOSITION{
		Latitude: b.pose.Position.Lat, Longitude: b.pose.Position.Lon, Heading: b.pose.Heading, OnGround: 1,
	}, b.reqID)
}

// inbound is the way in: from the nearest depot along the vehicle roads to
// BusApproachMeters ahead of the spot (the nose side), round the aircraft
// where the way would cross it, then along the fuselage onto the spot.
func (b *SimObjectBus) inbound(spot GroundPose) (*GroundPath, airport.LatLon, bool) {
	if b.Layout == nil {
		return nil, airport.LatLon{}, false
	}
	before := offsetHeading(spot.Position, spot.Heading+180, BusApproachMeters)
	d, ok := nearestDepot(b.Layout, before)
	if !ok {
		return nil, airport.LatLon{}, false
	}
	r, err := b.Layout.VehicleRoute(d, before)
	if err != nil || len(r) < 2 {
		return nil, airport.LatLon{}, false
	}
	// The mover drives the front axle: it ends a wheelbase ahead, the bus's
	// reference point on the spot.
	front := offsetHeading(spot.Position, spot.Heading, busProfile().WheelbaseMeters)
	// Straight in over the last BusApproachMeters: through a point halfway,
	// so the turn onto it stays before it.
	half := offsetHeading(spot.Position, spot.Heading+180, BusApproachMeters/2)
	pts := aroundAircraft(append(r, half, front), b.aircraft, b.prof)
	path, err := NewArcPath(pts, busProfile(), 10)
	if err != nil {
		return nil, airport.LatLon{}, false
	}
	return path, d, true
}

// outbound is the way home: round to the right, away from the aircraft on
// its left, back the way it came, then along the roads to its depot (or
// off ahead without one), round the aircraft where needed.
func (b *SimObjectBus) outbound() (*GroundPath, error) {
	p, h := b.pose.Position, b.pose.Heading
	out := h + 90 // the bus faces the tail on the aircraft's left: away is its right
	p1 := offsetHeading(offsetHeading(p, h, BusTurnMeters/2), out, BusTurnMeters/2)
	p2 := offsetHeading(p, out, BusTurnMeters)
	p3 := offsetHeading(p2, h+180, BusApproachMeters)
	pts := []airport.LatLon{p, p1, p2, p3}
	if b.hasDepot {
		if route, err := b.Layout.VehicleRoute(p3, b.depot); err == nil {
			pts = append(pts, route[1:]...)
		}
	} else {
		pts = append(pts, offsetHeading(p3, h+180, FuelDriveOffMeters))
	}
	return NewArcPath(aroundAircraft(pts, b.aircraft, b.prof), busProfile(), BusTurnMeters/2)
}

func (b *SimObjectBus) Handle(msg engine.Message) bool {
	if msg.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID {
		return false
	}
	m := msg.AsAssignedObjectID()
	if uint32(m.DwRequestID) != b.reqID {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	id := uint32(m.DwObjectID)
	switch {
	case !b.created:
		return false // not sent for: another bus on its request ID (deboarding, then boarding)
	case b.objectID != 0 && b.done:
		return false // home: this one is the next bus's
	case b.objectID != 0:
		if id != b.objectID {
			b.err = b.client.AIRemoveObject(id, b.reqID) // a second one from a retry (#90)
		}
		return true
	case b.done || b.early:
		// Removed or told to leave before it existed: away at once (#96).
		b.objectID, b.done = id, true
		b.err = b.client.AIRemoveObject(id, b.reqID)
		return true
	}
	b.objectID = id
	b.err = b.inj.Takeover(b.objectID)
	return true
}

func (b *SimObjectBus) Update(pose GroundPose, leave bool, dt float64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.objectID == 0 && leave {
		b.early = true
	}
	if b.done || b.objectID == 0 {
		return nil
	}
	if err := b.err; err != nil {
		b.err = nil
		return err
	}
	if b.arrive != nil {
		b.check(b.arrive)
		b.pose = b.arrive.Step(dt)
		if b.pose.Arrived {
			// Square to the fuselage: the last turn leaves it a few degrees
			// off (as the stairs).
			b.arrive, b.pose.Heading = nil, BusSpot(b.n)(b.aircraft, b.prof).Heading
		}
		return b.place()
	}
	if b.away == nil {
		if !leave {
			return b.place()
		}
		path, err := b.outbound()
		if err != nil {
			return b.finish()
		}
		b.away = NewGroundMoverFrom(path, busProfile(), b.pose.Heading, 0)
	}
	b.check(b.away)
	b.pose = b.away.Step(dt)
	if b.pose.Arrived {
		return b.finish()
	}
	return b.place()
}

func (b *SimObjectBus) place() error {
	road := b.arrive
	if road == nil {
		road = b.away
	}
	shown := lane(b.pose, road)
	b.report(b.objectID, shown, busProfile().WheelbaseMeters+6)
	err := b.inj.PlaceMoving(b.objectID, shown)
	if errors.Is(err, ErrGroundUnknown) {
		return nil
	}
	return err
}

func (b *SimObjectBus) finish() error {
	b.done = true
	obj := b.objectID
	b.forget(obj)
	b.inj.Forget(obj)
	return b.client.AIRemoveObject(obj, b.reqID)
}

// Fuelling reports that the bus is at its spot (FuelService).
func (b *SimObjectBus) Fuelling() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.objectID != 0 && !b.done && b.arrive == nil && b.away == nil
}

// Clear reports that the bus is away from its spot: leaving, never came,
// or gone.
func (b *SimObjectBus) Clear() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.done || b.objectID == 0 || b.away != nil && b.away.Pose().Distance > BusTurnMeters
}

func (b *SimObjectBus) Done() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.done
}

func (b *SimObjectBus) Remove() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.done || b.objectID == 0 {
		b.done = true
		return nil
	}
	return b.finish()
}

// Track is where it is and the way it still drives; ok is false before it
// is created or once removed.
func (b *SimObjectBus) Track() (pose GroundPose, route []airport.LatLon, ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.objectID == 0 || b.done {
		return GroundPose{}, nil, false
	}
	m := b.arrive
	if m == nil {
		m = b.away
	}
	if m != nil {
		pts := m.Path().Points()
		near, best := 0, math.Inf(1)
		for i, p := range pts {
			if d := localDist(p, b.pose.Position); d < best {
				near, best = i, d
			}
		}
		route = append([]airport.LatLon{b.pose.Position}, pts[min(near+1, len(pts)):]...)
	}
	return b.pose, route, true
}

// busProfile moves a bus: a long wheelbase, slow.
func busProfile() MotionProfile {
	p := DefaultMotionProfile()
	p.WheelbaseMeters, p.RefAheadMeters = 6, 0
	p.CruiseKts, p.MinTurnKts, p.Accel, p.Decel = BusKts, 3, 0.4, 0.6
	p.SpanMeters, p.TailMeters = 3, 7
	return p
}

// aroundAircraft is pts with a way round the aircraft at pose wherever a
// leg would cross it (its fuselage from tail to nose, or its wings): past
// its nose or tail, whichever is nearer the leg, busClearMeters off.
func aroundAircraft(pts []airport.LatLon, pose GroundPose, prof MotionProfile) []airport.LatLon {
	main := offsetHeading(pose.Position, pose.Heading+180, prof.RefAheadMeters)
	nose := offsetHeading(main, pose.Heading, prof.WheelbaseMeters+fuelNoseMeters)
	tail := offsetHeading(main, pose.Heading+180, prof.TailMeters)
	half := prof.SpanMeters / 2
	wingL, wingR := offsetHeading(main, pose.Heading-90, half+2), offsetHeading(main, pose.Heading+90, half+2)
	axisA, axisB := offsetHeading(tail, pose.Heading+180, busClearMeters), offsetHeading(nose, pose.Heading, busClearMeters)
	out := []airport.LatLon{pts[0]}
	for i := 1; i < len(pts); i++ {
		a, c := pts[i-1], pts[i]
		near := pointSegDist(nose, a, c) < busClearMeters || pointSegDist(tail, a, c) < busClearMeters
		if !near && !segmentsCross(a, c, axisA, axisB) && !segmentsCross(a, c, wingL, wingR) {
			out = append(out, c)
			continue
		}
		// Round the nearer end, on a's side first, then c's.
		mid := airport.LatLon{Lat: (a.Lat + c.Lat) / 2, Lon: (a.Lon + c.Lon) / 2}
		end := axisB
		if localDist(mid, axisA) < localDist(mid, axisB) {
			end = axisA
		}
		sideOf := func(p airport.LatLon) float64 {
			if math.Sin((localBearing(main, p)-pose.Heading)*math.Pi/180) < 0 {
				return -90
			}
			return 90
		}
		reach := half + busClearMeters
		sa, sc := sideOf(a), sideOf(c)
		out = append(out, offsetHeading(end, pose.Heading+sa, reach))
		if sa != sc {
			out = append(out, offsetHeading(end, pose.Heading+sc, reach))
		}
		out = append(out, c)
	}
	return out
}

// pointSegDist is the distance from p to the segment a–b, in meters.
func pointSegDist(p, a, b airport.LatLon) float64 {
	ab := localDist(a, b)
	if ab < 0.01 {
		return localDist(a, p)
	}
	rel := (localBearing(a, p) - localBearing(a, b)) * math.Pi / 180
	along := localDist(a, p) * math.Cos(rel)
	switch {
	case along <= 0:
		return localDist(a, p)
	case along >= ab:
		return localDist(b, p)
	}
	return math.Abs(localDist(a, p) * math.Sin(rel))
}
