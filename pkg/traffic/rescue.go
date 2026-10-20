package traffic

import (
	"errors"
	"math"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Rescue vehicles (the emergency add-on, part 1): fire and rescue trucks
// for an aircraft in an emergency. Each drives from the nearest vehicle
// depot (the fire station's stand-in) along the vehicle roads to its
// standby point beside the landing runway and waits; once the aircraft is
// down it follows it, RescueGapMeters behind, down the runway and to where
// it stops; then it drives home and is removed. Driven by the Injector,
// like SimObjectFuelTruck.
type SimObjectRescue struct {
	client engine.Client
	inj    *Injector
	title  string
	reqID  uint32
	// Layout is the airport, for the roads from the depot and back; nil or
	// no road: it appears at its standby point and leaves from where it is.
	Layout *airport.Layout

	mu       sync.Mutex
	objectID uint32
	pose     GroundPose
	mover    *GroundMover // driving to standby or home; nil waiting or following
	homing   bool
	depot    airport.LatLon
	hasDepot bool
	kts      float64 // following: its speed now
	err      error
	done     bool
}

// Rescue vehicle figures: how far behind the aircraft it follows, its top
// speed following (on the runway after the touchdown), and how near it
// stops.
const (
	RescueGapMeters  = 80.0
	RescueFollowKts  = 50.0
	RescueStopMeters = 5.0
	rescueAccel      = 2.5  // m/s², up to speed after the aircraft
	rescueTurnDegS   = 30.0 // its quickest turn following
)

// NewSimObjectRescue creates a rescue vehicle of the given ground vehicle
// title; reqID is the SimConnect request ID of its creation.
func NewSimObjectRescue(client engine.Client, inj *Injector, title string, reqID uint32) *SimObjectRescue {
	return &SimObjectRescue{client: client, inj: inj, title: title, reqID: reqID}
}

// Standby creates it at the nearest depot and sends it to at, beside the
// runway, where it waits.
func (r *SimObjectRescue) Standby(at GroundPose) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pose = at
	if r.Layout != nil {
		if d, ok := nearestDepot(r.Layout, at.Position); ok {
			if route, err := r.Layout.VehicleRoute(d, at.Position); err == nil && len(route) >= 2 {
				if path, err := NewArcPath(append(route, at.Position), fuelRoadProfile(), 8); err == nil {
					r.mover = NewGroundMoverFrom(path, rescueRoadProfile(), localBearing(path.PointAt(0), path.PointAt(math.Min(5, path.Length()))), 0)
					r.pose, r.depot, r.hasDepot = r.mover.Pose(), d, true
				}
			}
		}
	}
	return r.client.AICreateSimulatedObject(r.title, types.SIMCONNECT_DATA_INITPOSITION{
		Latitude: r.pose.Position.Lat, Longitude: r.pose.Position.Lon, Heading: r.pose.Heading, OnGround: 1,
	}, r.reqID)
}

// Handle takes its creation's answer: the object is driven from then on.
func (r *SimObjectRescue) Handle(msg engine.Message) bool {
	if msg.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID {
		return false
	}
	a := msg.AsAssignedObjectID()
	if a == nil || uint32(a.DwRequestID) != r.reqID {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.objectID = uint32(a.DwObjectID)
	r.err = r.inj.Takeover(r.objectID)
	return true
}

// Update drives it on its way (to standby or home) by dt seconds; waiting
// it stays put.
func (r *SimObjectRescue) Update(dt float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.done || r.objectID == 0 {
		return nil
	}
	if err := r.err; err != nil {
		r.err = nil
		return err
	}
	if r.mover != nil {
		r.pose = r.mover.Step(dt)
		if r.pose.Arrived {
			r.mover = nil
			if r.homing {
				return r.finish()
			}
		}
	}
	return r.place()
}

// Follow drives it after the aircraft at pose, moving at kts:
// RescueGapMeters behind it, no faster than RescueFollowKts, stopping when
// it stops.
func (r *SimObjectRescue) Follow(pose GroundPose, kts, dt float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.done || r.objectID == 0 || r.homing {
		return nil
	}
	r.mover = nil // off its standby: following now
	target := offsetHeading(pose.Position, pose.Heading+180, RescueGapMeters)
	d := localDist(r.pose.Position, target)
	if d <= RescueStopMeters {
		r.kts = 0
		return r.place()
	}
	want := math.Min(RescueFollowKts, math.Max(kts+5, 10))
	// Slowing down toward the target: no faster than it stops in.
	stop := math.Sqrt(2*rescueAccel*math.Max(0, d-RescueStopMeters)) / knot
	want = math.Min(want, stop)
	if r.kts < want {
		r.kts = math.Min(want, r.kts+rescueAccel*dt/knot)
	} else {
		r.kts = want
	}
	step := math.Min(d, r.kts*knot*dt)
	// Turning toward it no faster than rescueTurnDegS: no snap of its
	// heading as the aircraft turns off.
	turn := math.Max(-rescueTurnDegS*dt, math.Min(rescueTurnDegS*dt, headingDiff(r.pose.Heading, localBearing(r.pose.Position, target))))
	hdg := math.Mod(r.pose.Heading+turn+360, 360)
	r.pose = GroundPose{Position: offsetHeading(r.pose.Position, hdg, step), Heading: hdg, GroundSpeedKts: r.kts}
	return r.place()
}

// Home sends it back to its depot (removed there), or removes it where it
// is without one.
func (r *SimObjectRescue) Home() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.done || r.homing {
		return nil
	}
	r.homing = true
	if r.objectID == 0 {
		r.done = true
		return nil
	}
	if r.Layout != nil && r.hasDepot {
		if route, err := r.Layout.VehicleRoute(r.pose.Position, r.depot); err == nil && len(route) >= 2 {
			if path, err := NewArcPath(append([]airport.LatLon{r.pose.Position}, route...), fuelRoadProfile(), 8); err == nil {
				r.mover = NewGroundMoverFrom(path, rescueRoadProfile(), r.pose.Heading, 0)
				return nil
			}
		}
	}
	return r.finish()
}

// Track is where it is and, driving to its standby or home, the way still
// ahead; false before it is in the simulator or once gone.
func (r *SimObjectRescue) Track() (pose GroundPose, route []airport.LatLon, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.objectID == 0 || r.done {
		return GroundPose{}, nil, false
	}
	if r.mover != nil {
		pts := r.mover.Path().Points()
		near, best := 0, math.Inf(1)
		for i, p := range pts {
			if d := localDist(p, r.pose.Position); d < best {
				near, best = i, d
			}
		}
		route = append([]airport.LatLon{r.pose.Position}, pts[min(near+1, len(pts)):]...)
	}
	return r.pose, route, true
}

// Done reports that it has gone.
func (r *SimObjectRescue) Done() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.done
}

// Remove takes it away at once.
func (r *SimObjectRescue) Remove() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.done || r.objectID == 0 {
		r.done = true
		return nil
	}
	return r.finish()
}

func (r *SimObjectRescue) place() error {
	err := r.inj.PlaceMoving(r.objectID, r.pose)
	if errors.Is(err, ErrGroundUnknown) {
		return nil
	}
	return err
}

func (r *SimObjectRescue) finish() error {
	r.done = true
	obj := r.objectID
	r.inj.Forget(obj)
	return r.client.AIRemoveObject(obj, r.reqID)
}

// rescueRoadProfile drives it on the roads, faster than a fuel truck: on
// its way to an emergency.
func rescueRoadProfile() MotionProfile {
	p := fuelRoadProfile()
	p.CruiseKts = 35
	return p
}

// RescueStandby are the standby points beside a landing runway end: where
// the fire trucks wait, n of them (1–3), RescueStandbyAsideMeters off the
// centreline on the side of the runway's exits (its taxiways), at a third,
// half and two thirds of its length, facing along the landing direction.
func RescueStandby(l *airport.Layout, runwayEnd string, n int) []GroundPose {
	rwy, end, ok := l.RunwayEnd(runwayEnd)
	if !ok || n <= 0 {
		return nil
	}
	side := 90.0 // right of the landing direction, unless its exits are left
	g, err := airport.BuildGraph(l)
	if err == nil {
		if exits, err := g.RunwayExits(runwayEnd); err == nil && len(exits) > 0 {
			left := 0
			for _, x := range exits {
				if x.Side == airport.ExitLeft {
					left++
				}
			}
			if left*2 > len(exits) {
				side = -90
			}
		}
	}
	var out []GroundPose
	for _, share := range []float64{1.0 / 3, 0.5, 2.0 / 3}[:min(n, 3)] {
		p := offsetHeading(end.Threshold, end.Heading, rwy.Length*share)
		p = offsetHeading(p, end.Heading+side, rwy.Width/2+RescueStandbyAsideMeters)
		out = append(out, GroundPose{Position: p, Heading: end.Heading})
	}
	return out
}

// RescueStandbyAsideMeters: standby points lie this far beside the runway's
// edge.
const RescueStandbyAsideMeters = 60.0
