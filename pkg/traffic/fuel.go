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

// FuelService refuels a departure on its stand (#582). The departure calls
// Attach while it waits for its pushback (with time for the refuelling
// before the tug comes), Update on every frame until Done (leave turns true
// once the refuelling is over or the push is cleared) and Remove if it is
// cancelled. SimObjectFuelTruck is the built-in implementation.
type FuelService interface {
	// Handle consumes the vehicle's own SimConnect messages.
	Handle(msg engine.Message) bool
	// Attach brings the vehicle to the aircraft, parked at pose.
	Attach(pose GroundPose) error
	// Update drives it in, keeps it at the wing, and away once leave.
	Update(pose GroundPose, leave bool, dt float64) error
	// Fuelling reports that it is at the wing, refuelling.
	Fuelling() bool
	// Clear reports that it is off the wing (or never came): the push may
	// start.
	Clear() bool
	// Done reports that it has left and needs no more updates.
	Done() bool
	// Remove takes it away at once.
	Remove() error
}

// SimObjectFuelTruck is a fuel truck (or a hydrant dispenser) spawned as a
// simulated object and driven by the Injector, like SimObjectTug: from the
// nearest vehicle depot along the vehicle roads to the aircraft's right
// wing, where it parks alongside the fuselage facing the way the aircraft
// does, then on past the wing and back home, where it is removed.
type SimObjectFuelTruck struct {
	client engine.Client
	inj    *Injector
	title  string
	reqID  uint32
	prof   MotionProfile // the aircraft's: its span and main gear
	// Layout is the airport, for the way from the depot and back (as
	// SimObjectTug.Layout). nil, or no depot or road: it appears at the
	// wing and drives off ahead.
	Layout *airport.Layout
	// SideMeters is how far right of the aircraft's axis it parks; 0 uses
	// FuelTruckSideShare of the span (at least FuelTruckMinSideMeters).
	SideMeters float64
	// Spot, when set, is where it parks instead (FuelSpot): GPUSpot drives
	// a ground power unit to the nose (#832).
	Spot         func(pose GroundPose, prof MotionProfile) GroundPose
	vehicleYield // gives way to aircraft on its way (SetTraffic)

	mu        sync.Mutex
	objectID  uint32
	pose      GroundPose
	arrive    *GroundMover // driving in; nil once at the wing
	away      *GroundMover // driving off
	depot     airport.LatLon
	hasDepot  bool
	homing    bool
	clearDist float64 // driving off: past the aircraft this far along its way
	err       error
	done      bool
}

// NewSimObjectFuelTruck creates a fuel vehicle of the given ground vehicle
// title for an aircraft moving with prof; reqID is the SimConnect request
// ID of its creation.
func NewSimObjectFuelTruck(client engine.Client, inj *Injector, title string, reqID uint32, prof MotionProfile) *SimObjectFuelTruck {
	return &SimObjectFuelTruck{client: client, inj: inj, title: title, reqID: reqID, prof: prof}
}

// ObjectID is the vehicle's simulated object, 0 until it has been created.
func (f *SimObjectFuelTruck) ObjectID() uint32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.objectID
}

// FuelSpot is where a fuel vehicle parks for an aircraft at pose with
// profile prof: FuelTruckAheadMeters ahead of the main gear, side meters
// right of the axis (0: FuelTruckSideShare of the span, at least
// FuelTruckMinSideMeters), facing the way the aircraft does (Attach may
// turn it round, to come in the way the road does).
func FuelSpot(pose GroundPose, prof MotionProfile, side float64) GroundPose {
	if side <= 0 {
		side = math.Max(FuelTruckMinSideMeters, prof.SpanMeters*FuelTruckSideShare)
	}
	main := offsetHeading(pose.Position, pose.Heading+180, prof.RefAheadMeters)
	p := offsetHeading(offsetHeading(main, pose.Heading, FuelTruckAheadMeters), pose.Heading+90, side)
	return GroundPose{Position: p, Heading: normDeg(pose.Heading)}
}

func (f *SimObjectFuelTruck) Attach(pose GroundPose) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	spot := FuelSpot(pose, f.prof, f.SideMeters)
	if f.Spot != nil {
		spot = f.Spot(pose, f.prof)
	}
	f.pose = spot
	if path, depot, ok := f.inbound(&spot); ok {
		f.arrive, f.depot, f.hasDepot = NewGroundMoverFrom(path, fuelRoadProfile(), localBearing(path.PointAt(0), path.PointAt(math.Min(5, path.Length()))), 0), depot, true
		f.pose = f.arrive.Pose()
	}
	return f.client.AICreateSimulatedObject(f.title, types.SIMCONNECT_DATA_INITPOSITION{
		Latitude: f.pose.Position.Lat, Longitude: f.pose.Position.Lon, Heading: f.pose.Heading, OnGround: 1,
	}, f.reqID)
}

// inbound is the way in: from the nearest depot along the vehicle roads to
// a point FuelApproachMeters before the spot, then straight along the
// fuselage onto it. It comes from the nose or the tail, whichever way the
// road's last leg runs (at LKPR the stand roads come in at the nose: it
// parks facing the tail), and spot turns to face that way.
func (f *SimObjectFuelTruck) inbound(spot *GroundPose) (*GroundPath, airport.LatLon, bool) {
	if f.Layout == nil {
		return nil, airport.LatLon{}, false
	}
	var (
		route   []airport.LatLon
		depot   airport.LatLon
		heading float64
		best    = math.Inf(1)
	)
	for _, h := range []float64{spot.Heading, normDeg(spot.Heading + 180)} {
		before := offsetHeading(spot.Position, h+180, FuelApproachMeters)
		d, ok := nearestDepot(f.Layout, before)
		if !ok {
			return nil, airport.LatLon{}, false
		}
		r, err := f.Layout.VehicleRoute(d, before)
		if err != nil || len(r) < 2 {
			continue
		}
		if off := math.Abs(headingDiff(localBearing(r[len(r)-2], r[len(r)-1]), h)); off < best {
			route, depot, heading, best = r, d, h, off
		}
	}
	if route == nil {
		return nil, airport.LatLon{}, false
	}
	spot.Heading = heading
	// The mover drives the front axle along the path: it ends a wheelbase
	// ahead, the truck's reference point on the spot.
	front := offsetHeading(spot.Position, spot.Heading, fuelProfile().WheelbaseMeters)
	path, err := NewArcPath(append(route, front), fuelRoadProfile(), 8)
	if err != nil {
		return nil, airport.LatLon{}, false
	}
	return path, depot, true
}

func (f *SimObjectFuelTruck) Handle(msg engine.Message) bool {
	if msg.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID {
		return false
	}
	m := msg.AsAssignedObjectID()
	if uint32(m.DwRequestID) != f.reqID {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objectID = uint32(m.DwObjectID)
	f.err = f.inj.Takeover(f.objectID)
	return true
}

func (f *SimObjectFuelTruck) Update(pose GroundPose, leave bool, dt float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.done || f.objectID == 0 {
		return nil
	}
	if err := f.err; err != nil {
		f.err = nil
		return err
	}
	if f.arrive != nil {
		f.check(f.arrive)
		f.pose = f.arrive.Step(dt)
		if f.pose.Arrived {
			f.arrive = nil
		}
		return f.place()
	}
	if f.away == nil {
		if !leave {
			return f.place()
		}
		// On along the fuselage until FuelLeaveAheadMeters past the nose or
		// the tail (whichever way it faces), then home along the roads, or
		// off to the side without a depot: never across the aircraft.
		p, h := offsetHeading(f.pose.Position, f.pose.Heading, fuelProfile().WheelbaseMeters), f.pose.Heading
		f.clearDist = f.clearOf(pose, p)
		ahead := offsetHeading(p, h, f.clearDist)
		pts := []airport.LatLon{p, ahead}
		if f.hasDepot {
			if route, err := f.Layout.VehicleRoute(ahead, f.depot); err == nil {
				pts, f.homing = append(pts, route...), true
			}
		}
		if !f.homing {
			pts = append(pts, offsetHeading(ahead, h+FuelDriveOffTurnDeg, FuelDriveOffMeters))
		}
		path, err := NewArcPath(pts, fuelRoadProfile(), 8)
		if err != nil {
			return f.finish()
		}
		f.away = NewGroundMoverFrom(path, fuelRoadProfile(), h, 0)
	}
	f.check(f.away)
	f.pose = f.away.Step(dt)
	if f.pose.Arrived {
		return f.finish()
	}
	return f.place()
}

// clearOf is how far a truck at p, facing its heading along the aircraft
// at pose, drives on to be FuelLeaveAheadMeters past the aircraft's nose or
// tail.
func (f *SimObjectFuelTruck) clearOf(pose GroundPose, p airport.LatLon) float64 {
	along := localDist(pose.Position, p) * math.Cos((localBearing(pose.Position, p)-pose.Heading)*math.Pi/180)
	if math.Abs(headingDiff(f.pose.Heading, pose.Heading)) < 90 {
		nose := f.prof.WheelbaseMeters - f.prof.RefAheadMeters + fuelNoseMeters
		return math.Max(FuelLeaveAheadMeters, nose-along+FuelLeaveAheadMeters)
	}
	return math.Max(FuelLeaveAheadMeters, along+f.prof.TailMeters+FuelLeaveAheadMeters)
}

// fuelNoseMeters: the nose sits about this far ahead of the nose gear.
const fuelNoseMeters = 5.0

func (f *SimObjectFuelTruck) place() error {
	// On the roads in its lane; reported to the other vehicles.
	road := f.arrive
	if road == nil {
		road = f.away
	}
	shown := lane(f.pose, road)
	f.report(f.objectID, shown, fuelProfile().WheelbaseMeters+3)
	err := f.inj.PlaceMoving(f.objectID, shown)
	if errors.Is(err, ErrGroundUnknown) {
		return nil
	}
	return err
}

func (f *SimObjectFuelTruck) finish() error {
	f.done = true
	obj := f.objectID
	f.forget(obj)
	f.inj.Forget(obj)
	return f.client.AIRemoveObject(obj, f.reqID)
}

// Fuelling reports that it is parked at the wing.
func (f *SimObjectFuelTruck) Fuelling() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.objectID != 0 && !f.done && f.arrive == nil && f.away == nil
}

// Clear reports that it is not at the wing: still driving in (it turns
// back), on its way off, or gone.
func (f *SimObjectFuelTruck) Clear() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.done || f.objectID == 0 {
		return true
	}
	// Past the aircraft's nose or tail (clearOf).
	return f.away != nil && f.away.Pose().Distance > f.clearDist
}

// Track is where it is and the way it still drives; ok is false before it
// is created or once removed.
func (f *SimObjectFuelTruck) Track() (pose GroundPose, route []airport.LatLon, ok bool) {
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

func (f *SimObjectFuelTruck) Done() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.done
}

func (f *SimObjectFuelTruck) Remove() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.done || f.objectID == 0 {
		f.done = true
		return nil
	}
	return f.finish()
}

// fuelProfile moves a fuel truck: a long wheelbase, slow and steady.
func fuelProfile() MotionProfile {
	p := DefaultMotionProfile()
	p.WheelbaseMeters, p.RefAheadMeters = 5, 0
	p.CruiseKts, p.MinTurnKts, p.Accel, p.Decel = FuelTruckKts, 3, 0.4, 0.6
	p.SpanMeters, p.TailMeters = 3, 5
	return p
}

// fuelRoadProfile drives it on the vehicle roads.
func fuelRoadProfile() MotionProfile {
	p := fuelProfile()
	p.CruiseKts = FuelTruckKts
	return p
}

// Fuel trucks (#582, SimObjectFuelTruck). The vehicle parks
// FuelTruckAheadMeters ahead of the main gear, FuelTruckSideShare of the
// span (at least FuelTruckMinSideMeters) right of the axis, facing the way
// the aircraft does; it comes in along the fuselage from
// FuelApproachMeters behind that spot and leaves FuelLeaveAheadMeters on
// before turning home (or FuelDriveOffTurnDeg off for FuelDriveOffMeters
// without a depot), at FuelTruckKts. Estimates, tuned by eye.
const (
	FuelTruckSideShare     = 0.3
	FuelTruckMinSideMeters = 7.0
	FuelTruckAheadMeters   = 2.0
	FuelApproachMeters     = 25.0
	FuelLeaveAheadMeters   = 15.0
	FuelDriveOffTurnDeg    = 60.0
	FuelDriveOffMeters     = 40.0
	FuelTruckKts           = 12.0
)

// Refuelling times (TaxiRequest.Fuel), estimates: it starts FuelStartDelay
// after the departure starts waiting on its stand (a schedule's departure
// starts ManagerOptions.DepartureLead before its STD) and lasts FuelServiceTime (twice for a widebody), varied by
// DwellJitter; it must be off the wing FuelClearMargin before the tug comes
// (TugLeadTime before the push), so it comes only with FuelMinService left
// for it. The push waits up to FuelClearTimeout for it to leave, then it is
// removed.
const (
	FuelStartDelay   = 30 * time.Second
	FuelServiceTime  = 8 * time.Minute
	FuelMinService   = 3 * time.Minute
	FuelClearMargin  = time.Minute
	FuelClearTimeout = 2 * time.Minute
)

// Ground power units (#832): a GPU cart parks GPUAheadMeters ahead of the
// nose gear and GPUSideMeters right of the axis, facing the way the
// aircraft does, by the external power receptacle under the nose. Driven
// like a fuel truck (SimObjectFuelTruck with Spot GPUSpot). Estimates,
// tuned by eye.
const (
	GPUAheadMeters = 1.0
	GPUSideMeters  = 3.0
)

// GPUSpot is where a GPU parks for an aircraft at pose with profile prof.
func GPUSpot(pose GroundPose, prof MotionProfile) GroundPose {
	main := offsetHeading(pose.Position, pose.Heading+180, prof.RefAheadMeters)
	nose := offsetHeading(main, pose.Heading, prof.WheelbaseMeters)
	p := offsetHeading(offsetHeading(nose, pose.Heading, GPUAheadMeters), pose.Heading+90, GPUSideMeters)
	return GroundPose{Position: p, Heading: normDeg(pose.Heading)}
}
