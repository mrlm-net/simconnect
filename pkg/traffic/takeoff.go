//go:build windows
// +build windows

package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TakeoffProfile describes an injected take-off (#320).
type TakeoffProfile struct {
	// RollAccel (m/s²) is the acceleration at the start of the take-off
	// roll; it falls off towards RotateKts as drag builds up.
	RollAccel float64
	// RotateKts is Vr; the nose comes up at RotateRate (°/s) to ClimbPitch,
	// and the main wheels leave the ground at LiftoffPitch.
	RotateKts, RotateRate, LiftoffPitch, ClimbPitch float64
	// ClimbKts is the initial climb speed (V2 + 10–15); ClimbFpm the climb
	// rate reached ClimbRampSeconds after lift-off.
	ClimbKts, ClimbFpm, ClimbRampSeconds float64
	// TailstrikePitch is the pitch (°) at which the tail touches the runway
	// with the main gear on it; 0 means 11.5 (A320). On the runway the pitch
	// stays TailstrikeMarginDeg below it, lift-off included; after lift-off
	// it is held until a positive climb (PositiveClimbFt), then rises no
	// faster than the tail clears the runway (TailClearFtPerDeg).
	TailstrikePitch float64
}

// DefaultTakeoffProfile is an A320 family take-off.
func DefaultTakeoffProfile() TakeoffProfile {
	return TakeoffProfile{
		RollAccel: 2.4, // live: 2.0 lifted off after ~1875 m, long for an A320
		RotateKts: 138, RotateRate: 3, LiftoffPitch: 8, ClimbPitch: 15,
		ClimbKts: 160, ClimbFpm: 2200, ClimbRampSeconds: 5, TailstrikePitch: 11.5,
	}
}

// TakeoffPhase is the phase of an injected take-off.
type TakeoffPhase uint8

const (
	TakeoffRoll     TakeoffPhase = iota // accelerating on the runway
	TakeoffRotate                       // nose coming up, main wheels on the runway
	TakeoffAirborne                     // climbing out
)

// TakeoffPose is where a TakeoffMover puts the aircraft.
type TakeoffPose struct {
	Position       airport.LatLon // on the runway centreline (extended)
	Heading        float64
	HeightFt       float64 // main wheels above the runway
	PitchDeg       float64 // nose up
	GroundSpeedKts float64
	VerticalFpm    float64
	Distance       float64 // meters from the start of the roll
	Phase          TakeoffPhase
	// AirborneSeconds is the time since lift-off.
	AirborneSeconds float64
	// LiftoffDistance is where the wheels left the runway, once airborne.
	LiftoffDistance float64
}

// ApproachPose returns the pose in the form Injector.PlaceAir takes.
func (p TakeoffPose) ApproachPose() ApproachPose {
	return ApproachPose{
		Position: p.Position, Heading: p.Heading, HeightFt: p.HeightFt, PitchDeg: p.PitchDeg,
		GroundSpeedKts: p.GroundSpeedKts, VerticalFpm: p.VerticalFpm, Distance: p.Distance,
		OnGround: p.Phase != TakeoffAirborne,
	}
}

// TakeoffMover flies an injected take-off along a runway heading: roll,
// rotation and initial climb. It is pure computation.
type TakeoffMover struct {
	p           TakeoffProfile
	start       airport.LatLon
	heading     float64
	x, v        float64 // m, m/s
	h, vs       float64 // ft, fpm
	pitch       float64
	phase       TakeoffPhase
	airborneFor float64
	liftoffX    float64
}

// NewTakeoffMover starts the roll at start (the reference point on the
// runway centreline) along heading, at speedKts (0 from a standing start).
func NewTakeoffMover(start airport.LatLon, heading, speedKts float64, p TakeoffProfile) *TakeoffMover {
	return &TakeoffMover{p: p, start: start, heading: heading, v: speedKts * ktsToMS}
}

// Pose returns the current pose.
func (m *TakeoffMover) Pose() TakeoffPose {
	return TakeoffPose{
		Position: offsetHeading(m.start, m.heading, m.x), Heading: m.heading,
		HeightFt: m.h, PitchDeg: m.pitch, GroundSpeedKts: m.v / ktsToMS, VerticalFpm: m.vs,
		Distance: m.x, Phase: m.phase, LiftoffDistance: m.liftoffX, AirborneSeconds: m.airborneFor,
	}
}

// groundPitchLimit is the highest pitch with the main gear on the runway.
func (m *TakeoffMover) groundPitchLimit() float64 {
	tail := m.p.TailstrikePitch
	if tail <= 0 {
		tail = 11.5
	}
	return tail - TailstrikeMarginDeg
}

// Step advances by dt seconds.
func (m *TakeoffMover) Step(dt float64) TakeoffPose {
	for dt > 0 {
		h := math.Min(dt, 0.05)
		m.step(h)
		dt -= h
	}
	return m.Pose()
}

func (m *TakeoffMover) step(dt float64) {
	p := m.p
	vr := p.RotateKts * ktsToMS
	switch m.phase {
	case TakeoffRoll, TakeoffRotate:
		// Acceleration falls off by a third towards Vr as drag builds up.
		m.v += p.RollAccel * (1 - 0.33*math.Min(1, m.v/vr)) * dt
		if m.phase == TakeoffRoll && m.v >= vr {
			m.phase = TakeoffRotate
		}
		if m.phase == TakeoffRotate {
			// A small pull: the lift-off pitch, well clear of a tail strike.
			lift := math.Min(p.LiftoffPitch, m.groundPitchLimit())
			m.pitch = math.Min(lift, m.pitch+p.RotateRate*dt)
			if m.pitch >= lift-1e-9 {
				m.phase, m.liftoffX = TakeoffAirborne, m.x
			}
		}
	case TakeoffAirborne:
		m.airborneFor += dt
		// Held until a positive climb, then up to the climb pitch no faster
		// than the rising tail allows.
		if m.h >= PositiveClimbFt {
			limit := m.groundPitchLimit() + m.h/TailClearFtPerDeg
			m.pitch = math.Max(m.pitch, math.Min(math.Min(p.ClimbPitch, limit), m.pitch+p.RotateRate*dt))
		}
		f := math.Min(1, m.airborneFor/math.Max(p.ClimbRampSeconds, 0.01))
		m.vs = p.ClimbFpm * f * f * (3 - 2*f) // eases into the climb
		m.h += m.vs / 60 * dt
		// Speed builds towards the climb speed.
		if vc := p.ClimbKts * ktsToMS; m.v < vc {
			m.v = math.Min(vc, m.v+0.8*dt)
		}
	}
	m.x += m.v * dt
}
