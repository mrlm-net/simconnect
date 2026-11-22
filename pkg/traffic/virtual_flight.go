package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// VirtualFlight is an aircraft flown along a waypoint chain with no
// simulator object behind it (#1482): the way SimlessAI flies one — at each
// point's speed, reaching each point's altitude by it, turning at the rate
// MSFS AI does — so an overflight far from the viewer costs the simulator
// nothing, and once it is near, the aircraft created for it at its Pose
// flies on from there along Ahead.
//
// Its chain is planned once, when the flight is (EnrouteStart); each Step
// only moves the position. Not safe for concurrent use.
type VirtualFlight struct {
	o simlessObject
}

// VirtualPose is where a VirtualFlight is: its position, altitude (feet
// MSL), true heading, speed (knots), vertical speed (ft/min) and bank
// (degrees, right wing down positive).
type VirtualPose struct {
	Position airport.LatLon `json:"position"`
	AltFt    float64        `json:"altFt"`
	Heading  float64        `json:"heading"`
	Kts      float64        `json:"kts"`
	VSFpm    float64        `json:"vsFpm"`
	Bank     float64        `json:"bank"`
}

// InitPosition is the pose as a SimConnect initial position (airborne,
// SimConnect's signs: pitch and bank positive nose down and left wing
// down): where an aircraft created for the flight appears.
func (p VirtualPose) InitPosition() types.SIMCONNECT_DATA_INITPOSITION {
	return types.SIMCONNECT_DATA_INITPOSITION{
		Latitude: p.Position.Lat, Longitude: p.Position.Lon, Altitude: p.AltFt,
		Pitch:    -simlessPitch(p.VSFpm),
		Bank:     -p.Bank,
		Heading:  p.Heading,
		Airspeed: types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(math.Round(p.Kts)),
	}
}

// virtualMaxStep: a Step longer than this is flown in steps of it, so a
// long one still turns as the aircraft would.
const virtualMaxStep = 1.0

// NewVirtualFlight starts a flight at at (its position, altitude, heading
// and airspeed; no speed: SimlessAI's start speed) flying wps, the chain
// EnrouteStart gives with it. The chain is kept, not copied: it must not
// change afterwards.
func NewVirtualFlight(at types.SIMCONNECT_DATA_INITPOSITION, wps []types.SIMCONNECT_DATA_WAYPOINT) *VirtualFlight {
	v := &VirtualFlight{o: simlessObject{wps: wps, released: true, ai: true, posed: true,
		pos: airport.LatLon{Lat: at.Latitude, Lon: at.Longitude}, altFt: at.Altitude, hdg: at.Heading, kts: float64(at.Airspeed), bank: -at.Bank}}
	if v.o.kts <= 0 {
		v.o.kts = simlessStartKts
	}
	return v
}

// Step flies the flight dt seconds on (nothing for dt ≤ 0); past its last
// waypoint it flies on along the last leg (Done).
func (v *VirtualFlight) Step(dt float64) {
	if len(v.o.wps) == 0 {
		return
	}
	for dt > 0 {
		s := math.Min(dt, virtualMaxStep)
		v.o.fly(s)
		dt -= s
	}
}

// Pose is where the flight is now.
func (v *VirtualFlight) Pose() VirtualPose {
	o := &v.o
	return VirtualPose{Position: o.pos, AltFt: o.altFt, Heading: o.hdg, Kts: o.kts, VSFpm: o.vs, Bank: o.bank}
}

// PoseAfter is where the flight will be dt seconds from now, without
// moving it (a reader between Steps: the map, a snapshot).
func (v *VirtualFlight) PoseAfter(dt float64) VirtualPose {
	c := *v
	c.Step(dt)
	return c.Pose()
}

// Ahead is the waypoints not passed yet, the one it flies to first: the
// chain an aircraft created at Pose flies on.
func (v *VirtualFlight) Ahead() []types.SIMCONNECT_DATA_WAYPOINT {
	if v.o.next >= len(v.o.wps) {
		return nil
	}
	return v.o.wps[v.o.next:]
}

// Done reports whether the flight passed its last waypoint.
func (v *VirtualFlight) Done() bool { return v.o.next >= len(v.o.wps) }
