package traffic

import (
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// ApproachProfile describes how an aircraft flies the final approach,
// flare and touchdown when injected (#318). MSFS AI flies finals at a fixed
// ~165 kt with no pitch or flare, and its touchdowns ranged from -54 to
// -1214 fpm.
type ApproachProfile struct {
	// GlideSlopeDeg and ThresholdHeightFt set the glide path: the main
	// wheels cross the threshold ThresholdHeightFt high.
	GlideSlopeDeg     float64
	ThresholdHeightFt float64
	// StartKts is the speed where the approach starts, ApproachKts the speed
	// from ApproachSpeedNm out, TouchdownKts the speed at touchdown; the
	// speed follows the schedule with SpeedTimeSeconds lag.
	StartKts, ApproachKts, TouchdownKts float64
	ApproachSpeedNm                     float64
	SpeedTimeSeconds                    float64
	// FlareFt is the wheel height where the flare starts; the sink rate
	// eases to TouchdownFpm (negative) at touchdown.
	FlareFt      float64
	TouchdownFpm float64
	// ApproachPitchDeg and FlarePitchDeg are nose-up attitudes; the pitch
	// rises from one to the other through the flare.
	ApproachPitchDeg, FlarePitchDeg float64
	// DerotateSeconds is how long the nose takes to come down after
	// touchdown; the speed falls at DerotateDecel meanwhile.
	DerotateSeconds float64
	DerotateDecel   float64
}

// DefaultApproachProfile is an A320 family approach, tried live at LKPR
// (#318).
func DefaultApproachProfile() ApproachProfile {
	return ApproachProfile{
		GlideSlopeDeg: 3, ThresholdHeightFt: 50,
		StartKts: 150, ApproachKts: 135, TouchdownKts: 130, ApproachSpeedNm: 1, SpeedTimeSeconds: 3,
		FlareFt: 30, TouchdownFpm: -120,
		ApproachPitchDeg: 2.5, FlarePitchDeg: 5.5,
		DerotateSeconds: 4, DerotateDecel: 0.5,
	}
}

// ApproachPhase is the phase of an injected approach.
type ApproachPhase uint8

const (
	ApproachFinal    ApproachPhase = iota // on the glide path
	ApproachFlare                         // below FlareFt
	ApproachDerotate                      // on the main wheels, nose coming down
	ApproachDone                          // all wheels on the runway: hand over to a GroundMover
)

var approachPhaseNames = [...]string{"final", "flare", "derotate", "done"}

func (p ApproachPhase) String() string {
	if int(p) < len(approachPhaseNames) {
		return approachPhaseNames[p]
	}
	return "unknown"
}

// ApproachPose is where an ApproachMover puts the aircraft.
type ApproachPose struct {
	// Position is on the extended runway centreline; Heading is the runway
	// heading (true degrees), crabbed into a crosswind on final.
	Position airport.LatLon
	Heading  float64
	// HeightFt is the main wheels' height above the runway; PitchDeg is
	// nose-up attitude.
	HeightFt float64
	PitchDeg float64
	// BankDeg is the wing low into a crosswind in the flare, positive right
	// wing down (SetCrosswind).
	BankDeg float64
	// GroundSpeedKts and VerticalFpm are the current speeds.
	GroundSpeedKts float64
	VerticalFpm    float64
	// Distance is meters past the threshold (negative before it).
	Distance float64
	Phase    ApproachPhase
	// OnGround is set from touchdown on.
	OnGround bool
	// RunwayFt is the runway's elevation (feet MSL) HeightFt is above;
	// 0 when unknown: the height is then taken above the ground under the
	// aircraft, which follows the terrain (Injector.PlaceAir).
	RunwayFt float64
	// Touchdown and TouchdownFpm describe the touchdown once it happened.
	Touchdown    float64
	TouchdownFpm float64
}

// ApproachMover flies an aircraft down the glide path to a touchdown with a
// flare and lowers the nose. It is pure computation; an Injector puts the
// poses into the sim.
type ApproachMover struct {
	p           ApproachProfile
	thr         airport.LatLon
	heading     float64
	x, h, v, vs float64 // m past the threshold, ft, m/s, fpm
	pitch       float64
	phase       ApproachPhase
	derotateT   float64
	touchX      float64
	touchFpm    float64
	startNm     float64
	// cross is the crosswind in knots, positive from the right of the
	// runway heading (SetCrosswind).
	cross float64
	// aim shifts the whole path along the runway, meters (SetAimShift): the
	// touchdown point varies from landing to landing.
	aim float64
	// side shifts it across, meters right of the centreline (SetSideShift),
	// faded in on short final.
	side float64
}

// SetAimShift moves the aiming point, and so the touchdown, by meters
// along the runway (negative: earlier).
func (m *ApproachMover) SetAimShift(meters float64) { m.aim = meters }

// TouchdownSpreadMeters: an injected landing touches down up to this much
// before or past its type's usual point.
const TouchdownSpreadMeters = 10.0

// SetSideShift moves the touchdown meters right of the centreline
// (negative: left), faded in over the last sideFadeMeters of the final:
// no two landings on the same line (live: every one the same bit off).
func (m *ApproachMover) SetSideShift(meters float64) { m.side = meters }

// TouchdownSideMeters: an injected landing touches down up to this much
// either side of the centreline.
const TouchdownSideMeters = 3.0

// The side shift fades in from sideFadeFromMeters before the threshold to
// full sideFadeMeters later.
const (
	sideFadeFromMeters = 3000.0
	sideFadeMeters     = 2500.0
)

// bankDeg is the wing low into a crosswind: none on final (crabbed), into
// the wind as the crab comes out in the flare (the upwind main gear first),
// level again as the nose comes down. Positive: right wing down.
func (m *ApproachMover) bankDeg() float64 {
	if m.cross == 0 {
		return 0
	}
	full := math.Copysign(math.Min(maxWingLowDeg, math.Abs(m.cross)*wingLowPerKt), m.cross)
	switch m.phase {
	case ApproachFlare:
		if m.p.FlareFt <= 0 {
			return 0
		}
		return full * (1 - math.Max(0, m.h/m.p.FlareFt))
	case ApproachDerotate:
		return full * math.Max(0, 1-m.derotateT/wingLevelSeconds)
	}
	return 0
}

// The wing low: wingLowPerKt degrees per knot of crosswind, at most
// maxWingLowDeg, levelled in wingLevelSeconds after touchdown.
const (
	wingLowPerKt     = 0.25
	maxWingLowDeg    = 4.0
	wingLevelSeconds = 1.5
)

// SetCrosswind sets the crosswind the approach is flown in, knots,
// positive from the right of the runway: on final the aircraft crabs into
// it, nose into the wind by the drift angle, and straightens through the
// flare to touch down along the centreline.
func (m *ApproachMover) SetCrosswind(kts float64) { m.cross = kts }

// crabDeg is the heading off the runway's now: the drift angle on final
// (into the wind), taken out through the flare, none on the ground.
func (m *ApproachMover) crabDeg() float64 {
	if m.cross == 0 || m.phase >= ApproachDerotate || m.v <= 0 {
		return 0
	}
	ratio := math.Max(-maxCrabSin, math.Min(maxCrabSin, m.cross/(m.v/ktsToMS)))
	crab := math.Asin(ratio) * 180 / math.Pi
	if m.phase == ApproachFlare && m.p.FlareFt > 0 {
		crab *= math.Max(0, m.h/m.p.FlareFt) // de-crab: aligned at touchdown
	}
	return crab
}

// maxCrabSin bounds the drift: about 17° at most.
const maxCrabSin = 0.3

// NewApproachMover starts startMeters before the threshold of a runway end
// (threshold position, true heading), on the glide path at StartKts.
func NewApproachMover(threshold airport.LatLon, heading, startMeters float64, p ApproachProfile) *ApproachMover {
	m := &ApproachMover{p: p, thr: threshold, heading: heading, x: -startMeters, v: p.StartKts * ktsToMS, pitch: p.ApproachPitchDeg, startNm: startMeters / 1852}
	m.h = p.ThresholdHeightFt + startMeters*math.Tan(p.GlideSlopeDeg*math.Pi/180)/0.3048
	return m
}

// Pose returns the current pose.
func (m *ApproachMover) Pose() ApproachPose {
	pos := offsetHeading(m.thr, m.heading, m.x+m.aim)
	if m.side != 0 {
		fade := math.Max(0, math.Min(1, (m.x+m.aim+sideFadeFromMeters)/sideFadeMeters))
		pos = offsetHeading(pos, m.heading+90, m.side*fade)
	}
	return ApproachPose{
		Position: pos, Heading: math.Mod(m.heading+m.crabDeg()+360, 360),
		HeightFt: m.h, PitchDeg: m.pitch, BankDeg: m.bankDeg(), GroundSpeedKts: m.v / ktsToMS, VerticalFpm: m.vs,
		Distance: m.x + m.aim, Phase: m.phase, OnGround: m.phase >= ApproachDerotate,
		Touchdown: m.touchX + m.aim, TouchdownFpm: m.touchFpm,
	}
}

// Step advances by dt seconds.
func (m *ApproachMover) Step(dt float64) ApproachPose {
	for dt > 0 {
		h := math.Min(dt, 0.05)
		m.step(h)
		dt -= h
	}
	return m.Pose()
}

// slowNow has the approach fly its approach speed from here on, not the
// faster StartKts it slows from ("reduce to final approach speed"); it
// returns the time that adds to the threshold: spacing gained behind a
// slower leader.
func (m *ApproachMover) slowNow() time.Duration {
	before := m.timeToGo()
	m.p.StartKts = m.p.ApproachKts
	return m.timeToGo() - before
}

// timeToGo is how long the approach takes to the threshold on its schedule.
func (m *ApproachMover) timeToGo() time.Duration {
	const step = 50.0 // meters
	t := 0.0
	for x := m.x; x < 0; x += step {
		v := math.Max(m.speedAt(x+step/2), 1)
		t += math.Min(step, -x) / v
	}
	return time.Duration(t * float64(time.Second))
}

// speedAt is the scheduled speed (m/s) at distance x: StartKts at the start,
// falling linearly to ApproachKts at ApproachSpeedNm, TouchdownKts past the
// threshold.
func (m *ApproachMover) speedAt(x float64) float64 {
	p := m.p
	d := -x / 1852
	switch {
	case x >= 0:
		return p.TouchdownKts * ktsToMS
	case d <= p.ApproachSpeedNm || m.startNm <= p.ApproachSpeedNm:
		return p.ApproachKts * ktsToMS
	}
	f := math.Min(1, (d-p.ApproachSpeedNm)/(m.startNm-p.ApproachSpeedNm))
	return (p.ApproachKts + (p.StartKts-p.ApproachKts)*f) * ktsToMS
}

func (m *ApproachMover) step(dt float64) {
	p := m.p
	switch m.phase {
	case ApproachFinal, ApproachFlare:
		target := m.speedAt(m.x)
		m.v += (target - m.v) * math.Min(1, dt/math.Max(p.SpeedTimeSeconds, 0.01))
		onGS := -m.v * math.Tan(p.GlideSlopeDeg*math.Pi/180) / 0.3048 * 60
		m.vs, m.pitch = onGS, p.ApproachPitchDeg
		if m.h <= p.FlareFt {
			m.phase = ApproachFlare
			f := math.Max(0, m.h/p.FlareFt)
			m.vs = p.TouchdownFpm + (onGS-p.TouchdownFpm)*f
			m.pitch = p.FlarePitchDeg + (p.ApproachPitchDeg-p.FlarePitchDeg)*f
		}
		m.h += m.vs / 60 * dt
		m.x += m.v * dt
		if m.h <= 0 {
			m.h, m.phase, m.touchX, m.touchFpm = 0, ApproachDerotate, m.x, m.vs
		}
	case ApproachDerotate:
		m.derotateT += dt
		f := math.Min(1, m.derotateT/math.Max(p.DerotateSeconds, 0.01))
		m.pitch = p.FlarePitchDeg * (1 - f*f*(3-2*f)) // smoothstep to 0
		m.vs = 0
		m.v = math.Max(0, m.v-p.DerotateDecel*dt)
		m.x += m.v * dt
		if f >= 1 {
			m.phase = ApproachDone
		}
	case ApproachDone:
		m.x += m.v * dt // keeps rolling until the caller hands over
	}
}
