package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TakeoffProfile describes an injected take-off (#320).
type TakeoffProfile struct {
	// RollAccel (m/s²) is the acceleration at take-off thrust; it falls
	// off towards RotateKts as drag builds up. SpoolSeconds is how long the
	// engines take from idle to take-off thrust at the start of the roll:
	// the acceleration builds from SpoolStartFactor of it (0: 7 s).
	RollAccel    float64
	SpoolSeconds float64
	// RotateKts is Vr; the nose comes up at RotateRate (°/s) to ClimbPitch,
	// and the main wheels leave the ground at LiftoffPitch.
	RotateKts, RotateRate, LiftoffPitch, ClimbPitch float64
	// ClimbKts is the initial climb speed (V2 + 10–15); ClimbFpm the climb
	// rate reached ClimbRampSeconds after lift-off.
	ClimbKts, ClimbFpm, ClimbRampSeconds float64
	// AccelFt is the acceleration altitude (ft above the runway; 0:
	// TakeoffAccelFt): the climb speed is held to it, then the aircraft
	// accelerates to CleanKts (0: ClimbKts + TakeoffCleanAddKts) at
	// TakeoffAccelKtsPerSecond, climbing at TakeoffAccelClimbFactor of
	// ClimbFpm meanwhile; the flaps retract on the way (speed, not height).
	AccelFt, CleanKts float64
	// TailstrikePitch is the pitch (°) at which the tail touches the runway
	// with the main gear on it; 0 means 11.5 (A320). On the runway the pitch
	// stays TailstrikeMarginDeg below it, lift-off included; after lift-off
	// it is held until a positive climb (PositiveClimbFt), then rises no
	// faster than the tail clears the runway (TailClearFtPerDeg).
	TailstrikePitch float64
	// SettlePitch is the pitch (°) the climb settles to once the gear is up
	// (GearUp): lift-off at LiftoffPitch (5–7°), ClimbPitch (12–15°) for the
	// first climb, gear up, then this (0: ClimbPitch − SettleBelowClimbDeg,
	// about 10°), as asked 2026-10-03.
	SettlePitch float64
}

// SettleBelowClimbDeg and SettlePitchRate: the climb pitch settles this
// far below ClimbPitch after gear-up (TakeoffProfile.SettlePitch unset), at
// this rate (°/s).
const (
	SettleBelowClimbDeg = 3.0
	SettlePitchRate     = 1.0
)

// AirbornePullDeg: fully airborne (AirbornePullFt), the nose comes up this
// much past ClimbPitch for the first climb: "getting off the runway is
// perfect but usually there is a bit of pulling more once airborne fully"
// (user, 2026-10-07).
const (
	AirbornePullDeg = 2.5
	AirbornePullFt  = 50.0
)

// settlePitch is the profile's pitch after gear-up, the default where unset.
func (p TakeoffProfile) settlePitch() float64 {
	if p.SettlePitch > 0 {
		return p.SettlePitch
	}
	return math.Max(p.LiftoffPitch, p.ClimbPitch-SettleBelowClimbDeg)
}

// DefaultTakeoffProfile is an A320 family take-off.
func DefaultTakeoffProfile() TakeoffProfile {
	return TakeoffProfile{
		RollAccel: 2.4, // live: 2.0 lifted off after ~1875 m, long for an A320
		RotateKts: 138, RotateRate: 3, LiftoffPitch: 6.5, ClimbPitch: 15,
		ClimbKts: 160, ClimbFpm: 2200, ClimbRampSeconds: 2.5, TailstrikePitch: 11.5,
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
	// shown, shownRate: the pitch as drawn, following pitch smoothly
	// (PitchFollowRate): no corners where a rotation or settle starts or ends.
	shown, shownRate float64
	phase       TakeoffPhase
	airborneFor float64
	rollFor     float64 // s since the thrust was set
	liftoffX    float64
	rejected    bool    // braking to a stop (Reject)
	accel       bool    // past the acceleration altitude
	accelFor    float64 // s since
	settle      bool    // gear up: the pitch settles to SettlePitch
}

// GearUp tells the mover the gear is up: the pitch eases from the climb
// pitch down to the profile's SettlePitch.
func (m *TakeoffMover) GearUp() { m.settle = true }

// Acceleration after take-off: held at the climb speed (V2 + 10) to the
// acceleration altitude, then towards the clean speed, the flaps coming up
// as the speed passes their schedule (NADP 2: 800–1500 ft).
const (
	TakeoffAccelFt           = 1000.0
	TakeoffCleanAddKts       = 50.0
	TakeoffAccelKtsPerSecond = 1.5
	TakeoffAccelClimbFactor  = 0.6
	TakeoffAccelEaseSeconds  = 5.0
)

// accelFt and cleanKts are the profile's acceleration altitude and clean
// speed, the defaults where unset.
func (p TakeoffProfile) accelFt() float64 {
	if p.AccelFt > 0 {
		return p.AccelFt
	}
	return TakeoffAccelFt
}

func (p TakeoffProfile) cleanKts() float64 {
	if p.CleanKts > 0 {
		return p.CleanKts
	}
	return p.ClimbKts + TakeoffCleanAddKts
}

// FlapsShare is how much of the take-off flap setting is still out at
// groundKts: all of it up to the climb speed, none from the clean speed on,
// in between in proportion (the flaps retract on the speed schedule).
func (p TakeoffProfile) FlapsShare(groundKts float64) float64 {
	lo, hi := p.ClimbKts, p.cleanKts()
	if hi <= lo {
		return 0
	}
	return math.Max(0, math.Min(1, (hi-groundKts)/(hi-lo)))
}

// SpoolStartFactor is the thrust's share of take-off thrust as the roll
// starts (thrust set from idle); DefaultSpoolSeconds the spool-up to full.
const (
	SpoolStartFactor    = 0.15
	DefaultSpoolSeconds = 7.0
)

// spool is the share of take-off thrust rollFor into the roll: from
// SpoolStartFactor to 1, eased at both ends (no jerk).
func (m *TakeoffMover) spool() float64 {
	sec := m.p.SpoolSeconds
	if sec <= 0 {
		sec = DefaultSpoolSeconds
	}
	k := math.Min(1, m.rollFor/sec)
	k = k * k * (3 - 2*k)
	return SpoolStartFactor + (1-SpoolStartFactor)*k
}

// RejectDecel (m/s²) is the braking of a rejected take-off (maximum
// autobrake); V1MarginKts puts V1 that far below Vr: a take-off is
// rejected only before V1.
var (
	RejectDecel = 3.0
	V1MarginKts = 5.0
)

// Reject aborts the take-off: before V1 on the roll the aircraft brakes to
// a stop on the runway (Rejected, then Stopped). It reports false past V1
// or once rotating, when the take-off has to continue.
func (m *TakeoffMover) Reject() bool {
	if m.phase != TakeoffRoll || m.v >= (m.p.RotateKts-V1MarginKts)*ktsToMS {
		return false
	}
	m.rejected = true
	return true
}

// Rejected reports a rejected take-off; Stopped once it has come to a stop.
func (m *TakeoffMover) Rejected() bool { return m.rejected }
func (m *TakeoffMover) Stopped() bool  { return m.rejected && m.v == 0 }

// NewTakeoffMover starts the roll at start (the reference point on the
// runway centreline) along heading, at speedKts (0 from a standing start).
func NewTakeoffMover(start airport.LatLon, heading, speedKts float64, p TakeoffProfile) *TakeoffMover {
	return &TakeoffMover{p: p, start: start, heading: heading, v: speedKts * ktsToMS}
}

// Pose returns the current pose.
func (m *TakeoffMover) Pose() TakeoffPose {
	return TakeoffPose{
		Position: offsetHeading(m.start, m.heading, m.x), Heading: m.heading,
		HeightFt: m.h, PitchDeg: m.shown, GroundSpeedKts: m.v / ktsToMS, VerticalFpm: m.vs,
		Distance: m.x, Phase: m.phase, LiftoffDistance: m.liftoffX, AirborneSeconds: m.airborneFor,
	}
}

// follow moves the drawn pitch towards pitch as an aircraft pitches: easing
// in and out, without overshoot (critically damped, PitchFollowRate), and
// never past the tail-strike limit on the runway.
func (m *TakeoffMover) follow(dt float64) {
	w := PitchFollowRate
	m.shownRate += (w*w*(m.pitch-m.shown) - 2*w*m.shownRate) * dt
	m.shown += m.shownRate * dt
	if m.phase != TakeoffAirborne && m.shown > m.groundPitchLimit() {
		m.shown, m.shownRate = m.groundPitchLimit(), 0
	}
}

// PitchFollowRate (rad/s): how quickly the drawn take-off pitch follows the
// planned one; about a quarter of a second behind it, rounding the start and
// end of each change ("pitch more smooth as it is in reality", 2026-10-07).
const PitchFollowRate = 4.0

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
		m.follow(h)
		dt -= h
	}
	return m.Pose()
}

func (m *TakeoffMover) step(dt float64) {
	p := m.p
	vr := p.RotateKts * ktsToMS
	if m.rejected {
		m.v = math.Max(0, m.v-RejectDecel*dt)
		m.pitch = 0
		m.x += m.v * dt
		return
	}
	switch m.phase {
	case TakeoffRoll, TakeoffRotate:
		// The engines spool up to take-off thrust, then the acceleration falls
		// off by a third towards Vr as drag builds up.
		m.rollFor += dt
		m.v += p.RollAccel * m.spool() * (1 - 0.33*math.Min(1, m.v/vr)) * dt
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
		if m.settle {
			// Gear up: the nose eases down to the settled climb.
			m.pitch = math.Max(math.Min(m.pitch, p.settlePitch()), m.pitch-SettlePitchRate*dt)
		} else if m.h >= PositiveClimbFt {
			limit := m.groundPitchLimit() + m.h/TailClearFtPerDeg
			target := p.ClimbPitch
			if m.h >= AirbornePullFt {
				target += AirbornePullDeg
			}
			m.pitch = math.Max(m.pitch, math.Min(math.Min(target, limit), m.pitch+p.RotateRate*dt))
		}
		f := math.Min(1, m.airborneFor/math.Max(p.ClimbRampSeconds, 0.01))
		m.vs = p.ClimbFpm * f * f * (3 - 2*f) // eases into the climb
		if m.h >= p.accelFt() {
			m.accel = true
		}
		if m.accel {
			// Past the acceleration altitude: nose down a little (over
			// TakeoffAccelEaseSeconds), speed up to the clean speed.
			m.accelFor += dt
			e := math.Min(1, m.accelFor/TakeoffAccelEaseSeconds)
			m.vs *= 1 - (1-TakeoffAccelClimbFactor)*e*e*(3-2*e)
			m.v = math.Min(p.cleanKts()*ktsToMS, m.v+TakeoffAccelKtsPerSecond*ktsToMS*dt)
		} else if vc := p.ClimbKts * ktsToMS; m.v < vc {
			// Speed builds towards the climb speed.
			m.v = math.Min(vc, m.v+0.8*dt)
		}
		m.h += m.vs / 60 * dt
	}
	m.x += m.v * dt
}

// TakeoffConditions are what lengthen a take-off beyond the profile's
// sea-level ISA run: the airport elevation and the temperature above ISA
// (from the weather; 0 when unknown).
type TakeoffConditions struct {
	ElevationFt   float64
	ISADeviationC float64
}

// RequiredTakeoffRun is the runway an aircraft needs ahead of it to take off
// (e.g. from an intersection): the distance to 35 ft flown by the
// TakeoffMover with this profile, lengthened for the conditions (about 10%
// per 1000 ft of elevation and 1% per °C above ISA) and by the certification
// margin TakeoffRunMargin. Computed, so it follows the aircraft's figures.
func RequiredTakeoffRun(p TakeoffProfile, c TakeoffConditions) float64 {
	m := NewTakeoffMover(airport.LatLon{}, 0, 0, p)
	pose := m.Pose()
	for i := 0; i < 20*300 && pose.HeightFt < 35; i++ {
		pose = m.Step(0.05)
	}
	factor := 1 + 0.10*math.Max(0, c.ElevationFt)/1000 + 0.01*math.Max(0, c.ISADeviationC)
	return pose.Distance * factor * TakeoffRunMargin
}
