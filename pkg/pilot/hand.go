//go:build windows

package pilot

import (
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/flight"
	"github.com/mrlm-net/simconnect/pkg/systems"
)

// Hand flying (#965 iteration B): the copilot flies the take-off up to the
// autopilot's engage height and the landing from minimums to the rollout
// through the flight controls: the elevator flies a pitch, the ailerons a
// bank, the rudder the centreline on the ground, the throttles a speed.
// The loops are simple and damped by the rates; a profile learned from
// the player's flights (Config.Learned) gives the type's rotation, pitches
// and flare. Update it every sim frame or so while hand flying.

// Runway is the runway end flown from or to.
type Runway struct {
	Threshold airport.LatLon
	Heading   float64 // true
	ElevFt    float64
	LengthM   float64
}

func (r Runway) valid() bool { return r.Threshold.Lat != 0 || r.Threshold.Lon != 0 }

// track is p's place to the centreline: metres right of it (negative
// left) and along it from the threshold (negative before it).
func (r Runway) track(p airport.LatLon) (right, along float64) {
	lat, lon := calc.DisplaceByHeading(r.Threshold.Lat, r.Threshold.Lon, r.Heading, 10000)
	far := airport.LatLon{Lat: lat, Lon: lon}
	right = calc.CrossTrackMeters(r.Threshold.Lat, r.Threshold.Lon, far.Lat, far.Lon, p.Lat, p.Lon)
	along = calc.AlongTrackMeters(r.Threshold.Lat, r.Threshold.Lon, far.Lat, far.Lon, p.Lat, p.Lon)
	return right, along
}

// Hand flying defaults.
const (
	defRotatePitch  = 12.5 // the pitch the rotation aims at
	defClimbPitch   = 15.0
	defFlareAGLFt   = 30.0
	flarePitchUp    = 4.0             // the flare's pitch over the approach's
	rotateRateDeg   = 3.0             // the rotation, degrees a second
	retardAGLFt     = 20.0            // the thrust to idle
	decrabAGLFt     = 10.0            // the nose straightened with the rudder below this
	glideTan        = 0.0524          // tan 3°
	tchFt           = 50.0            // the glide path over the threshold
	handbackKts     = 40.0            // the rollout handed back below this
	reverseThrust   = 70.0            // the throttles in reverse, percent
	reverseIdleKts  = 70.0            // idle reverse below this
	reverseStowKts  = 60.0            // the reversers stowed by this
	rolloutDecelKts = 4.0             // the deceleration braked for, knots a second (a medium autobrake)
	brakeAfter      = 2 * time.Second // the brakes from this after touchdown
)

// hand is the hand flying's state between ticks.
type hand struct {
	prev      flight.Sample
	prevAt    time.Time
	have      bool
	pitchCmd  float64 // the pitch flown to, degrees
	appPitch  float64 // the approach pitch at minimums
	thrI      float64 // the speed loop's integral
	flaring   bool
	retarded  bool
	started   time.Time
	gearAsked bool
	// track: the ground track from the positions (true), flown instead of
	// the heading so a crosswind gives the crab; decel: the speed lost a
	// second.
	track     float64
	haveTrack bool
	decel     float64
	// The rollout: touched down at, reversers out and stowed, brakes.
	touchAt          time.Time
	reversed, stowed bool
	brake            float64
	flapsAsked       bool // the go-around's flap step
}

// flownTrack is the ground track, the heading until one is known.
func (h *hand) flownTrack(in Input) float64 {
	if h.haveTrack {
		return h.track
	}
	return in.Air.Heading
}

// rates are the pitch and bank rates (degrees a second) and dt since the
// last tick; zero on the first.
func (h *hand) rates(in Input) (pitchRate, bankRate, dt float64) {
	if h.have {
		dt = in.Now.Sub(h.prevAt).Seconds()
		if dt > 0 && dt < 1 {
			pitchRate = (in.Air.Pitch - h.prev.Pitch) / dt
			bankRate = (in.Air.Bank - h.prev.Bank) / dt
			h.decel = (h.prev.IAS - in.Air.IAS) / dt
			if calc.HaversineMeters(h.prev.Lat, h.prev.Lon, in.Air.Lat, in.Air.Lon) > 0.5 {
				h.track, h.haveTrack = calc.BearingDegrees(h.prev.Lat, h.prev.Lon, in.Air.Lat, in.Air.Lon), true
			}
		}
	}
	h.prev, h.prevAt, h.have = in.Air, in.Now, true
	return pitchRate, bankRate, math.Min(dt, 1)
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// angleDiff is b − a, −180…180.
func angleDiff(a, b float64) float64 { return math.Mod(b-a+540, 360) - 180 }

// elevator flies pitch target (nose up positive percent).
func elevator(target, pitch, rate float64) float64 {
	return clamp(8*(target-pitch)-4*rate, -60, 60)
}

// aileron flies bank target.
func aileron(target, bank, rate float64) float64 {
	return clamp(3*(target-bank)-1.5*rate, -60, 60)
}

// centreline is the rudder holding the centreline on the ground: the
// heading to the runway's, turned toward the line by the offset.
func centreline(r Runway, in Input) float64 {
	right, _ := r.track(airport.LatLon{Lat: in.Air.Lat, Lon: in.Air.Lon})
	want := r.Heading + clamp(-right*2, -10, 10)
	return clamp(4*angleDiff(in.Air.Heading, want), -80, 80)
}

// speedThrottle holds speed kts: percent.
func (h *hand) speedThrottle(kts, ias, dt, base float64) float64 {
	err := kts - ias
	h.thrI = clamp(h.thrI+0.5*err*dt, -30, 30)
	return clamp(base+2*err+h.thrI, 0, 100)
}

func (c Config) learned(v, def float64) float64 {
	if v > 0 {
		return v
	}
	return def
}

// rotateKts and climbKts: VR (the aircraft's, else learned, else V2 − 5)
// and the initial climb speed (V2 + 10).
func (c Config) rotateKts(s systems.State) float64 {
	switch {
	case s.VRKt > 0:
		return s.VRKt
	case c.Learned.RotateKts > 0:
		return c.Learned.RotateKts
	case s.V2Kt > 0:
		return s.V2Kt - 5
	}
	return 140
}

func (c Config) climbKts(s systems.State) float64 {
	if s.V2Kt > 0 {
		return s.V2Kt + 10
	}
	return c.rotateKts(s) + 20
}

// takeoff flies the roll, the rotation and the climb to the engage height.
func (e *Engine) takeoff(in Input, c Config, out *Output) {
	h := &e.hand
	pr, br, dt := h.rates(in)
	if h.started.IsZero() {
		h.started, h.pitchCmd = in.Now, in.Air.Pitch
		out.Say = append(out.Say, "Takeoff")
	}
	r := in.Runway
	thr := c.TakeoffThrust
	if in.Now.Sub(h.started) < 2*time.Second {
		thr = 50 // spooled up evenly first
	}
	act := func(name string, v float64) { out.Actions = append(out.Actions, setValue(name, v)) }
	act(systems.Throttle, thr)
	vr := c.rotateKts(in.State)
	rotate, climb := c.learned(c.Learned.RotatePitch, defRotatePitch), c.learned(c.Learned.ClimbPitch, defClimbPitch)
	switch {
	case in.Air.OnGround && in.Air.IAS < vr:
		h.pitchCmd = in.Air.Pitch // held on the runway
	case in.Air.OnGround:
		h.pitchCmd = math.Min(rotate, h.pitchCmd+rotateRateDeg*dt) // rotating
	default:
		// In the air: the climb pitch, adjusted for V2 + 10.
		want := climb + clamp((in.Air.IAS-c.climbKts(in.State))*0.4, -5, 5)
		h.pitchCmd += clamp(want-h.pitchCmd, -rotateRateDeg*dt, rotateRateDeg*dt)
	}
	act(systems.Elevator, elevator(h.pitchCmd, in.Air.Pitch, pr))
	if in.Air.OnGround {
		act(systems.Rudder, centreline(r, in))
		act(systems.Aileron, aileron(0, in.Air.Bank, br))
	} else {
		act(systems.Rudder, 0)
		hdg := clamp(2*angleDiff(in.Air.Heading, r.Heading), -10, 10)
		act(systems.Aileron, aileron(hdg, in.Air.Bank, br))
		if !h.gearAsked && in.Air.VS > 300 && in.AGLFt() > 35 {
			h.gearAsked = true
			e.ask(in, c, out, "gear", "Positive climb, gear up", func(in Input) bool { return !in.Air.GearHandle }, set(systems.GearDown, false))
		}
	}
}

// landing flies from minimums to the rollout: the glide path, the flare,
// the touchdown and the rollout on the centreline; true once slow enough
// to hand back.
func (e *Engine) landing(in Input, c Config, out *Output) bool {
	h := &e.hand
	pr, br, dt := h.rates(in)
	r := in.Runway
	agl := in.AGLFt()
	if h.started.IsZero() {
		h.started, h.pitchCmd, h.appPitch = in.Now, in.Air.Pitch, in.Air.Pitch
	}
	appPitch := h.appPitch
	act := func(name string, v float64) { out.Actions = append(out.Actions, setValue(name, v)) }
	right, along := r.track(airport.LatLon{Lat: in.Air.Lat, Lon: in.Air.Lon})
	flare := c.learned(c.Learned.FlareAGLFt, defFlareAGLFt)
	switch {
	case in.Air.OnGround:
		// Touchdown and rollout: the nose down gently, the centreline; the
		// reversers out with the nose down, reverse thrust to 70 kt, idle
		// reverse, stowed by 60 kt; the brakes for a steady deceleration.
		if h.touchAt.IsZero() {
			h.touchAt = in.Now
		}
		h.pitchCmd = math.Max(0, h.pitchCmd-1.5*dt)
		act(systems.Elevator, elevator(h.pitchCmd, in.Air.Pitch, pr))
		act(systems.Aileron, 0)
		act(systems.Rudder, centreline(r, in))
		thr := 0.0
		switch {
		case !h.reversed && in.Air.Pitch < 1 && in.Air.IAS > reverseStowKts:
			h.reversed = true
			out.Actions = append(out.Actions, set(systems.Reversers, true))
		case h.reversed && !h.stowed && in.Air.IAS > reverseIdleKts:
			thr = reverseThrust
		case h.reversed && !h.stowed && in.Air.IAS <= reverseStowKts:
			h.stowed = true
			out.Actions = append(out.Actions, set(systems.Reversers, false))
		}
		act(systems.Throttle, thr)
		if in.Now.Sub(h.touchAt) >= brakeAfter {
			h.brake = clamp(h.brake+(rolloutDecelKts-h.decel)*20*dt, 0, 80)
		}
		done := in.Air.IAS < handbackKts
		if done {
			h.brake = 0 // the player taxies on
		}
		act(systems.BrakeLeft, h.brake)
		act(systems.BrakeRight, h.brake)
		return done
	case agl <= math.Max(flare, -in.Air.VS/60*3) || h.flaring: // or 3 s from the ground
		if !h.flaring {
			h.flaring = true
			h.thrI = 0
		}
		// The sink eased with the height: 100 ft/min + 8 a foot (340 at
		// 30 ft, 180 at 10), the pitch flown up to it, at most the
		// approach's + flarePitchUp + 2.
		vsWant := -(100 + 8*math.Max(agl, 0))
		h.pitchCmd += clamp((vsWant-in.Air.VS)*0.006, -1, 2.5) * dt
		h.pitchCmd = math.Min(h.pitchCmd, appPitch+flarePitchUp+2)
		if agl <= retardAGLFt && !h.retarded {
			h.retarded = true
			out.Say = append(out.Say, "Retard")
		}
		if h.retarded {
			act(systems.Throttle, 0)
		} else {
			act(systems.Throttle, h.speedThrottle(c.ApproachKts, in.Air.IAS, dt, 45))
		}
		if agl > decrabAGLFt {
			// Still the centreline by bank, as on the final.
			trackWant := r.Heading + clamp(-right*0.06, -15, 15)
			act(systems.Aileron, aileron(clamp(3*angleDiff(h.flownTrack(in), trackWant), -10, 10), in.Air.Bank, br))
			act(systems.Rudder, 0)
		} else {
			// De-crab: the nose to the runway with the rudder, the wings
			// near level, a little into the drift.
			act(systems.Rudder, clamp(5*angleDiff(in.Air.Heading, r.Heading), -80, 80))
			// The wing down into the wind holds the track on the line (a
			// crosswind landing: the nose straight, the drift banked out).
			want := r.Heading + clamp(-right*0.06, -5, 5)
			act(systems.Aileron, aileron(clamp(4*angleDiff(h.flownTrack(in), want), -8, 8), in.Air.Bank, br))
		}
	default:
		// The glide path: 3° through 50 ft over the threshold.
		before := math.Max(0, -along) * 3.28084
		path := tchFt + before*glideTan
		vsWant := -in.Air.GS*101.27*glideTan + clamp((path-agl)*3, -400, 400)
		h.pitchCmd = clamp(h.pitchCmd+(vsWant-in.Air.VS)*0.002*dt, -2, 8)
		act(systems.Throttle, h.speedThrottle(c.ApproachKts, in.Air.IAS, dt, 55))
		trackWant := r.Heading + clamp(-right*0.06, -15, 15) // slow next to the turn loop: no overshoot
		limit := 15.0
		if agl < 500 {
			limit = 10
		}
		act(systems.Aileron, aileron(clamp(3*angleDiff(h.flownTrack(in), trackWant), -limit, limit), in.Air.Bank, br))
		act(systems.Rudder, 0)
	}
	act(systems.Elevator, elevator(h.pitchCmd, in.Air.Pitch, pr))
	return false
}

// goAround flies the go-around: the autopilot off, TOGA, "Go around,
// flaps" (one flap step asked of the pilot monitoring), the climb pitch
// for the approach speed + 10 on the runway's track, "Positive climb, gear
// up"; at the engage height the autopilot takes it to the cleared
// altitude. True once the autopilot has it.
func (e *Engine) goAround(in Input, c Config, out *Output) bool {
	h := &e.hand
	pr, br, dt := h.rates(in)
	act := func(name string, v float64) { out.Actions = append(out.Actions, setValue(name, v)) }
	if h.started.IsZero() {
		h.started, h.pitchCmd = in.Now, in.Air.Pitch
		out.Say = append(out.Say, "Go around, flaps")
		if in.State.AP.Master {
			e.act(in, out, set(systems.APMaster, false), true)
		}
		if k := flapsIndex(in); k > 0 && !h.flapsAsked {
			h.flapsAsked = true
			want := k - 1
			e.ask(in, c, out, "flaps", e.flapsSaid(want), func(in Input) bool { return flapsIndex(in) <= want }, Action{Name: systems.FlapsUp})
			out.Say = out.Say[:len(out.Say)-1] // said as "Go around, flaps"
		}
	}
	act(systems.Throttle, 100)
	climb := c.learned(c.Learned.ClimbPitch, defClimbPitch)
	want := climb + clamp((in.Air.IAS-(c.ApproachKts+10))*0.4, -5, 5)
	h.pitchCmd += clamp(want-h.pitchCmd, -2.5*dt, 2.5*dt)
	act(systems.Elevator, elevator(h.pitchCmd, in.Air.Pitch, pr))
	act(systems.Rudder, 0)
	track := in.Runway.Heading
	if !in.Runway.valid() {
		track = in.Air.Heading
	}
	act(systems.Aileron, aileron(clamp(3*angleDiff(h.flownTrack(in), track), -15, 15), in.Air.Bank, br))
	if !h.gearAsked && in.Air.VS > 300 && in.Air.GearHandle {
		h.gearAsked = true
		e.ask(in, c, out, "gear", "Positive climb, gear up", func(in Input) bool { return !in.Air.GearHandle }, set(systems.GearDown, false))
	}
	if in.AGLFt() >= c.EngageAGLFt && in.Air.VS > 0 {
		out.Say = append(out.Say, "Autopilot on")
		e.act(in, out, set(systems.APMaster, true), true)
		e.act(in, out, set(systems.ATHR, true), true)
		return true
	}
	return false
}
