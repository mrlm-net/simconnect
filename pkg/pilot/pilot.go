//go:build windows

// Package pilot is a pilot flying for the user aircraft (#965): logic only.
// Each tick it takes the aircraft's systems (pkg/systems), how it flies (a
// pkg/flight sample), the flight plan and the ATC clearance the app gives,
// and returns what to do: generic systems actions for the app's Controls,
// what the pilot flying says, and requests to the pilot monitoring (the
// player): "Flaps one", "Gear down". The app voices them and routes them
// to the player; a request is done when the aircraft shows it, or by the
// copilot after a timeout when the app lets it (Config.CopilotActs).
//
// Iteration A: the autopilot only. The player flies the take-off and the
// landing; the engine takes over at the autopilot's engage height, flies
// the climb, cruise, descent and approach on the autopilot (the speed
// schedule, the flaps and gear by speed and height, the approach armed
// when cleared), and gives the controls back at minimums.
package pilot

import (
	"fmt"
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/flight"
	"github.com/mrlm-net/simconnect/pkg/systems"
)

// Phase is where the flight is for the engine.
type Phase int

const (
	PhaseTakeoff  Phase = iota // the player flies; the engine waits for the engage height
	PhaseClimb                 // on the autopilot, climbing to the cleared level
	PhaseCruise                // level at the cruise or cleared level
	PhaseDescent               // descending
	PhaseApproach              // slowing down, flaps and gear, the approach
	PhaseHandback              // at minimums the controls went back to the player
	PhaseRoll                  // hand flying: the take-off roll and the climb to the engage height
	PhaseLanding               // hand flying: from minimums to the rollout
	PhaseGoAround              // hand flying: the go-around, to the autopilot
)

func (p Phase) String() string {
	return [...]string{"takeoff", "climb", "cruise", "descent", "approach", "handback", "roll", "landing", "go-around"}[p]
}

// Plan is what the engine needs of the flight plan.
type Plan struct {
	CruiseFt       float64 // the cruise altitude, feet
	DestElevFt     float64 // the destination's elevation, feet
	DistanceToGoNM float64 // along the plan to the destination; 0 unknown
}

// Clearance is what ATC gave the flight, as the app hears it.
type Clearance struct {
	AltitudeFt float64 // cleared altitude or level, feet; 0 none (the plan's cruise)
	HeadingDeg float64 // an assigned heading (magnetic); 0 own navigation
	SpeedKts   float64 // an assigned speed; 0 own schedule
	Approach   bool    // cleared for the approach
	Takeoff    bool    // cleared for take-off (hand flying: the roll starts)
	Land       bool    // cleared to land (hand flying: landed, not gone around)
	GoAround   bool    // told to go around (hand flying: flown, then the autopilot)
}

// Input is one tick's view of the flight.
type Input struct {
	Now   time.Time
	State systems.State // the autopilot, flaps, gear, speeds, minimums
	Air   flight.Sample // altitude, ground, speeds, on the ground
	Plan  Plan
	ATC   Clearance
	// Runway is the runway end of the take-off or the landing (hand flying).
	Runway Runway
}

// AGLFt is the wheels' height above the ground.
func (in Input) AGLFt() float64 { return in.Air.AltFt - in.Air.GroundFt - in.Air.CGFt }

// Action is a systems action for the app's Controls: Set(Name, *On) when
// On is set, else SetValue(Name, *Value); Press for neither.
type Action struct {
	Name  string
	On    *bool
	Value *float64
}

func set(name string, on bool) Action        { return Action{Name: name, On: &on} }
func setValue(name string, v float64) Action { return Action{Name: name, Value: &v} }

func (a Action) String() string {
	switch {
	case a.On != nil:
		return fmt.Sprintf("%s=%v", a.Name, *a.On)
	case a.Value != nil:
		return fmt.Sprintf("%s=%g", a.Name, *a.Value)
	}
	return a.Name
}

// Request is the pilot flying asking the pilot monitoring (the player).
type Request struct {
	ID   int
	Kind string // "flaps", "gear", "descent"
	Say  string // "Flaps one", "Gear down"
	// Do is what the copilot does when the player does not in time
	// (Config.PMTimeout, Config.CopilotActs); none: it only asks.
	Do []Action
	// Asked is when it was said.
	Asked time.Time
}

// Output is what to do this tick.
type Output struct {
	Phase    Phase
	Actions  []Action  // send now
	Say      []string  // the pilot flying says now
	Requests []Request // new requests to the pilot monitoring
	// Done are requests the aircraft shows done (by the player) or the
	// copilot did after the timeout (TimedOut).
	Done, TimedOut []int
	// Handback: the controls went back to the player this tick.
	Handback bool
}

// Config tunes the engine; zero values take the defaults. Speeds knots,
// heights feet.
type Config struct {
	EngageAGLFt     float64 // the autopilot engaged above this, climbing (1000)
	AccelAGLFt      float64 // flaps retracted above this (1500)
	LowSpeedKts     float64 // the speed below LowSpeedBelowFt (250)
	LowSpeedBelowFt float64 // (10000)
	ClimbKts        float64 // above it (290)
	DescentKts      float64 // above it (280)
	// ApproachKts is the final approach speed; 0: 1.3 × VS0 + 5 (the
	// aircraft's design stall speed landing), else 140.
	ApproachKts float64
	// FlapStepKts: each flap detent is this much slower (15).
	FlapStepKts float64
	// FlapDetents: the flap lever's detents (the profile's, else 5).
	FlapDetents     int
	GearDownAGLFt   float64       // the gear down by this height on approach (2000)
	FinalFlapsAGLFt float64       // the landing flaps by this height (1500)
	MinimumsAGLFt   float64       // controls back here without minimums set (200)
	PMTimeout       time.Duration // a request waits this long for the player (8 s)
	// CopilotActs: past PMTimeout the copilot does what it asked.
	CopilotActs bool
	// Resend: an action not seen done is sent again after this (3 s).
	Resend time.Duration
	// HandFly (iteration B): the copilot flies the take-off and the landing
	// by hand too (hand.go), not only the autopilot part.
	HandFly bool
	// Learned is how the type is flown (flight.Learn): rotation, pitches,
	// flare height; zero values take hand.go's defaults.
	Learned flight.Learned
	// TakeoffThrust: the throttle for the take-off, percent (90).
	TakeoffThrust float64
}

func (c Config) withDefaults(s systems.State) Config {
	def := func(v *float64, d float64) {
		if *v == 0 {
			*v = d
		}
	}
	def(&c.EngageAGLFt, 1000)
	def(&c.AccelAGLFt, 1500)
	def(&c.LowSpeedKts, 250)
	def(&c.LowSpeedBelowFt, 10000)
	def(&c.ClimbKts, 290)
	def(&c.DescentKts, 280)
	def(&c.FlapStepKts, 15)
	def(&c.GearDownAGLFt, 2000)
	def(&c.FinalFlapsAGLFt, 1500)
	def(&c.MinimumsAGLFt, 200)
	def(&c.TakeoffThrust, 90)
	if c.ApproachKts == 0 {
		if vs0 := s.Values[systems.DesignVS0]; vs0 > 0 {
			c.ApproachKts = math.Round(1.3*vs0 + 5)
		} else {
			c.ApproachKts = 140
		}
	}
	if c.FlapDetents == 0 {
		c.FlapDetents = 5
	}
	if c.PMTimeout == 0 {
		c.PMTimeout = 8 * time.Second
	}
	if c.Resend == 0 {
		c.Resend = 3 * time.Second
	}
	return c
}

// flapLimit is the highest speed flap detent k (1 … n−1) is selected at:
// the landing flaps at the approach speed + 20, each detent before one
// step faster.
func (c Config) flapLimit(k int) float64 {
	return c.ApproachKts + 20 + c.FlapStepKts*float64(c.FlapDetents-1-k)
}

// Engine is the pilot flying. Update it each tick (a second or more often).
type Engine struct {
	cfg     Config
	detents []string // as said ("zero" … "full"); from the profile

	phase   Phase
	nextID  int
	pending map[string]*Request // by kind
	sent    map[string]sentAction
	said    map[string]bool          // once-per-flight callouts
	dones   map[int]func(Input) bool // when each request is done
	// taking: TakeControl asked; the next Update takes over in the air.
	taking bool
	// hand: the hand flying's state (hand.go).
	hand hand
}

type sentAction struct {
	at    time.Time
	value string
}

// New returns an engine for an aircraft whose flap lever says detents
// (systems.Profile.FlapDetents; nil: "zero" … "four").
func New(cfg Config, detents []string) *Engine {
	if cfg.FlapDetents == 0 && len(detents) > 0 {
		cfg.FlapDetents = len(detents)
	}
	return &Engine{cfg: cfg, detents: detents, pending: map[string]*Request{}, sent: map[string]sentAction{}, said: map[string]bool{}, dones: map[int]func(Input) bool{}, taking: true}
}

// Phase is where the engine is.
func (e *Engine) Phase() Phase { return e.phase }

// flapsSaid is detent k as the crew says it.
func (e *Engine) flapsSaid(k int) string {
	if k >= 0 && k < len(e.detents) {
		return "Flaps " + e.detents[k]
	}
	if k == 0 {
		return "Flaps up"
	}
	return fmt.Sprintf("Flaps %d", k)
}

// Update takes a tick and says what to do.
func (e *Engine) Update(in Input) Output {
	c := e.cfg.withDefaults(in.State)
	out := Output{}
	e.settle(in, c, &out)
	agl := in.AGLFt()
	ap := in.State.AP
	// Hand flying: the take-off roll once cleared, then the climb to the
	// engage height by hand; the landing from minimums.
	if e.phase == PhaseTakeoff && c.HandFly && in.Air.OnGround && in.ATC.Takeoff && in.Runway.valid() {
		e.phase, e.hand = PhaseRoll, hand{}
	}
	switch e.phase {
	case PhaseRoll:
		e.takeoff(in, c, &out)
		if !in.Air.OnGround && agl >= c.EngageAGLFt {
			out.Say = append(out.Say, "Autopilot on")
			e.act(in, &out, set(systems.APMaster, true), true)
			if !ap.ATHR {
				e.act(in, &out, set(systems.ATHR, true), true)
			}
			e.phase, e.taking = PhaseClimb, false
		}
		out.Phase = e.phase
		return out
	case PhaseGoAround:
		if e.goAround(in, c, &out) {
			e.phase = PhaseClimb
		}
		out.Phase = e.phase
		return out
	case PhaseLanding:
		if in.ATC.GoAround && !in.Air.OnGround {
			e.phase, e.hand = PhaseGoAround, hand{}
			e.goAround(in, c, &out)
			out.Phase = e.phase
			return out
		}
		if e.landing(in, c, &out) {
			out.Say = append(out.Say, "Your controls")
			out.Handback = true
			e.phase = PhaseHandback
		}
		out.Phase = e.phase
		return out
	case PhaseTakeoff:
		// Climbing through the engage height after take-off, or taking over
		// anywhere above it (a handover in cruise): from where the flight is.
		if !in.Air.OnGround && agl >= c.EngageAGLFt && (in.Air.VS > 0 || e.taking) {
			if ap.Master {
				out.Say = append(out.Say, "I have control")
			} else {
				out.Say = append(out.Say, "Autopilot on", "I have control")
				e.act(in, &out, set(systems.APMaster, true), true)
			}
			if !ap.ATHR {
				e.act(in, &out, set(systems.ATHR, true), true)
			}
			e.phase, e.taking = e.phaseNow(in, c), false
		}
	case PhaseHandback:
		out.Phase = e.phase
		return out
	}
	if e.phase == PhaseTakeoff {
		out.Phase = e.phase
		return out
	}

	// Minimums: the controls back to the player.
	// Told to go around on the approach: flown by hand from here.
	if e.phase == PhaseApproach && c.HandFly && in.ATC.GoAround && in.Runway.valid() {
		e.phase, e.hand = PhaseGoAround, hand{}
		e.goAround(in, c, &out)
		out.Phase = e.phase
		return out
	}
	if e.phase == PhaseApproach && e.atMinimums(in, c) && c.HandFly && in.Runway.valid() {
		e.act(in, &out, set(systems.APMaster, false), true)
		if in.ATC.Land {
			// On by hand to the runway: the thrust too.
			if ap.ATHR {
				e.act(in, &out, set(systems.ATHR, false), true)
			}
			out.Say = append(out.Say, "Autopilot off")
			e.phase, e.hand = PhaseLanding, hand{}
			out.Phase = e.phase
			return out
		}
		// Not cleared to land: the go-around, flown.
		e.phase, e.hand = PhaseGoAround, hand{}
		e.goAround(in, c, &out)
		out.Phase = e.phase
		return out
	}
	if e.phase == PhaseApproach && e.atMinimums(in, c) {
		e.act(in, &out, set(systems.APMaster, false), true)
		out.Say = append(out.Say, "Autopilot off", "Your controls")
		out.Handback = true
		e.phase = PhaseHandback
		out.Phase = e.phase
		return out
	}

	target := in.ATC.AltitudeFt
	if target == 0 {
		target = in.Plan.CruiseFt
	}
	alt := in.Air.AltFt
	e.phaseFrom(in, c, target, &out)

	// Lateral: an assigned heading, else the flight plan.
	if in.ATC.HeadingDeg > 0 {
		e.value(in, &out, systems.APHeadingSel, ap.HeadingSel, in.ATC.HeadingDeg, 1)
		if !ap.HeadingHold {
			e.act(in, &out, set(systems.APHeadingHold, true), false)
		}
	} else if !ap.Nav && !ap.Approach && !ap.ApproachArmed {
		e.act(in, &out, set(systems.APNav, true), false)
	}

	// Vertical: the cleared altitude, by level change.
	if target > 0 {
		e.value(in, &out, systems.APAltitudeSel, ap.AltitudeSel, target, 50)
		if math.Abs(target-alt) > 300 && !ap.FLC && !ap.Glideslope && !ap.Approach {
			e.act(in, &out, set(systems.APFLC, true), false)
		}
	}

	// Speed: an assigned one, else the schedule.
	e.value(in, &out, systems.APSpeedSel, ap.SpeedSel, e.speed(in, c), 2)

	switch e.phase {
	case PhaseClimb:
		e.retract(in, c, agl, &out)
		if in.Air.GearHandle && agl > 400 {
			e.ask(in, c, &out, "gear", "Gear up", func(in Input) bool { return !in.Air.GearHandle }, set(systems.GearDown, false))
		}
	case PhaseDescent, PhaseApproach:
		if e.phase == PhaseDescent && target > alt-500 && !e.said["descent"] && in.Plan.DistanceToGoNM > 0 {
			e.said["descent"] = true
			e.ask(in, c, &out, "descent", "Ask for descent, please", nil)
		}
		if e.phase == PhaseApproach {
			e.approach(in, c, agl, &out)
		}
	}
	out.Phase = e.phase
	return out
}

// phaseFrom moves the phase on from where the flight is.
func (e *Engine) phaseFrom(in Input, c Config, target float64, out *Output) {
	alt, agl := in.Air.AltFt, in.AGLFt()
	toDescend := in.Plan.DistanceToGoNM > 0 && in.Plan.DistanceToGoNM <= 3*(alt-in.Plan.DestElevFt)/1000+10
	switch e.phase {
	case PhaseClimb:
		if math.Abs(alt-target) < 200 && math.Abs(in.Air.VS) < 500 {
			e.phase = PhaseCruise
		}
		if toDescend && alt > target+300 {
			e.phase = PhaseDescent
		}
	case PhaseCruise:
		if toDescend {
			e.phase = PhaseDescent
			out.Say = append(out.Say, "Top of descent")
		}
	case PhaseDescent:
		if agl < 5000 || (in.Plan.DistanceToGoNM > 0 && in.Plan.DistanceToGoNM < 20) || in.ATC.Approach {
			e.phase = PhaseApproach
		}
	}
}

// speed is the speed to fly now.
func (e *Engine) speed(in Input, c Config) float64 {
	if in.ATC.SpeedKts > 0 && e.phase != PhaseApproach {
		return in.ATC.SpeedKts
	}
	alt := in.Air.AltFt
	switch e.phase {
	case PhaseClimb, PhaseCruise:
		if alt < c.LowSpeedBelowFt-500 {
			return c.LowSpeedKts
		}
		return c.ClimbKts
	case PhaseDescent:
		if alt < c.LowSpeedBelowFt+1500 {
			return c.LowSpeedKts
		}
		return c.DescentKts
	case PhaseApproach:
		k := flapsIndex(in)
		// Each configuration flown 10 kt under the next detent's limit, so
		// the next is due; the landing flaps at the approach speed.
		v := c.ApproachKts
		if k < c.FlapDetents-1 {
			v = c.flapLimit(k+1) - 10
		}
		if in.ATC.SpeedKts > 0 && in.ATC.SpeedKts > v {
			v = in.ATC.SpeedKts // ATC's speed until it is no longer flyable
		}
		return math.Min(v, c.LowSpeedKts)
	}
	return c.LowSpeedKts
}

func flapsIndex(in Input) int {
	return int(math.Round(in.State.Values[systems.FlapsIndex]))
}

// retract asks for the flaps up step by step, accelerating.
func (e *Engine) retract(in Input, c Config, agl float64, out *Output) {
	k := flapsIndex(in)
	if k == 0 || agl < c.AccelAGLFt {
		return
	}
	if in.Air.IAS >= c.flapLimit(k)-c.FlapStepKts {
		want := k - 1
		e.ask(in, c, out, "flaps", e.flapsSaid(want), func(in Input) bool { return flapsIndex(in) <= want }, Action{Name: systems.FlapsUp})
	}
}

// approach configures the aircraft: flaps by speed, distance and height,
// the gear, the approach armed once cleared.
func (e *Engine) approach(in Input, c Config, agl float64, out *Output) {
	ap := in.State.AP
	if in.ATC.Approach && !ap.Approach && !ap.ApproachArmed && !e.said["appr"] {
		e.said["appr"] = true
		e.act(in, out, set(systems.APApproach, true), false)
		out.Say = append(out.Say, "Approach armed")
	}
	k := flapsIndex(in)
	n := c.FlapDetents
	gear := in.Air.GearHandle
	// The gear: by height, or once on the glideslope.
	if !gear && (agl <= c.GearDownAGLFt || ap.Glideslope) {
		e.ask(in, c, out, "gear", "Gear down", func(in Input) bool { return in.Air.GearHandle }, set(systems.GearDown, true))
	}
	if k >= n-1 {
		return
	}
	next := k + 1
	// The landing flaps by FinalFlapsAGLFt after the gear; the others as the
	// speed allows, the one before the landing flaps only with the gear.
	due := in.Air.IAS <= c.flapLimit(next)-5
	if next == n-1 {
		due = gear && (agl <= c.FinalFlapsAGLFt || ap.Glideslope)
	} else if next == n-2 {
		due = due && (gear || agl <= c.GearDownAGLFt+500)
	}
	if due {
		e.ask(in, c, out, "flaps", e.flapsSaid(next), func(in Input) bool { return flapsIndex(in) >= next }, Action{Name: systems.FlapsDown})
	}
}

// atMinimums: at the decision height or altitude, else MinimumsAGLFt.
func (e *Engine) atMinimums(in Input, c Config) bool {
	s := in.State
	switch {
	case s.DHFt > 0:
		return in.AGLFt() <= s.DHFt
	case s.DAFt > 0:
		return in.Air.AltFt <= s.DAFt
	}
	return in.AGLFt() <= c.MinimumsAGLFt
}

// ask makes a request to the pilot monitoring of kind, one at a time per
// kind: said once, done when done says so, or by the copilot after the
// timeout (do).
func (e *Engine) ask(in Input, c Config, out *Output, kind, say string, done func(Input) bool, do ...Action) {
	if r, ok := e.pending[kind]; ok && r.Say == say {
		return
	}
	e.nextID++
	r := &Request{ID: e.nextID, Kind: kind, Say: say, Do: do, Asked: in.Now}
	if done != nil {
		e.dones[r.ID] = done
	}
	e.pending[kind] = r
	out.Requests = append(out.Requests, *r)
	out.Say = append(out.Say, say)
}

// settle ends the requests done, and does those timed out.
func (e *Engine) settle(in Input, c Config, out *Output) {
	for kind, r := range e.pending {
		if done := e.dones[r.ID]; done != nil && done(in) {
			out.Done = append(out.Done, r.ID)
			delete(e.pending, kind)
			delete(e.dones, r.ID)
			continue
		}
		if in.Now.Sub(r.Asked) < c.PMTimeout {
			continue
		}
		if len(r.Do) == 0 {
			delete(e.pending, kind) // only asked
			delete(e.dones, r.ID)
			continue
		}
		if !c.CopilotActs {
			continue // waits for the player
		}
		out.TimedOut = append(out.TimedOut, r.ID)
		out.Actions = append(out.Actions, r.Do...)
		r.Asked = in.Now // and again after another timeout if still not done
	}
}

// act sends a, not again within Resend of the same (always when force).
func (e *Engine) act(in Input, out *Output, a Action, force bool) {
	key, v := a.Name, a.String()
	if s, ok := e.sent[key]; ok && !force && s.value == v && in.Now.Sub(s.at) < e.cfg.withDefaults(in.State).Resend {
		return
	}
	e.sent[key] = sentAction{at: in.Now, value: v}
	out.Actions = append(out.Actions, a)
}

// value sets selected value name to want when now is further than tol off.
func (e *Engine) value(in Input, out *Output, name string, now, want, tol float64) {
	if math.Abs(now-want) <= tol {
		return
	}
	e.act(in, out, setValue(name, want), false)
}

// WithLearned takes what a type's recorded flights show (flight.Learn,
// #966) where c leaves a value to its default: the acceleration and gear
// heights, the approach speed, each flap detent's speeds (FlapStepKts from
// the detents' spacing on the approach).
func (c Config) WithLearned(l flight.Learned) Config {
	use := func(v *float64, learned float64) {
		if *v == 0 && learned > 0 {
			*v = learned
		}
	}
	use(&c.AccelAGLFt, l.AccelAGLFt)
	use(&c.GearDownAGLFt, l.GearDownAGLFt)
	use(&c.ApproachKts, l.ApproachKts)
	if c.FlapStepKts == 0 && len(l.FlapsDownKts) >= 2 {
		var ks []int
		for k := range l.FlapsDownKts {
			ks = append(ks, k)
		}
		lo, hi := ks[0], ks[0]
		for _, k := range ks {
			lo, hi = min(lo, k), max(hi, k)
		}
		if hi > lo {
			if step := (l.FlapsDownKts[lo] - l.FlapsDownKts[hi]) / float64(hi-lo); step > 0 {
				c.FlapStepKts = math.Round(step)
			}
		}
	}
	last := -1
	for k := range l.FlapsDownAGLFt {
		last = max(last, k)
	}
	if last >= 0 {
		use(&c.FinalFlapsAGLFt, l.FlapsDownAGLFt[last]) // the landing flaps
	}
	return c
}

// phaseNow is the phase of a flight taken over in the air: the approach
// low and close in, the descent going down, the cruise level at the
// target, else the climb.
func (e *Engine) phaseNow(in Input, c Config) Phase {
	target := in.ATC.AltitudeFt
	if target == 0 {
		target = in.Plan.CruiseFt
	}
	alt := in.Air.AltFt
	switch {
	case in.AGLFt() < 5000 && (in.ATC.Approach || (in.Plan.DistanceToGoNM > 0 && in.Plan.DistanceToGoNM < 30)):
		return PhaseApproach
	case in.Air.VS < -300 || (target > 0 && target < alt-300):
		return PhaseDescent
	case target > 0 && math.Abs(alt-target) < 300:
		return PhaseCruise
	}
	return PhaseClimb
}

// HandBack gives the controls to the player mid-flight (the player asked
// for them): the engine stops flying, the autopilot left as it is. It
// returns what the pilot flying says.
func (e *Engine) HandBack() []string {
	was := e.phase
	e.phase, e.taking = PhaseHandback, false
	if was == PhaseHandback || was == PhaseTakeoff {
		return nil // it was not flying
	}
	return []string{"Your controls"}
}

// TakeControl has the engine fly again: at the next Update it takes over
// from where the flight is (in the air above the engage height; on the
// ground, after the take-off as at the start), saying "I have control".
func (e *Engine) TakeControl() {
	e.phase, e.taking = PhaseTakeoff, true
	clear(e.pending)
	clear(e.dones)
}
