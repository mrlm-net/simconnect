package pilot

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/systems"
)

// apGoAroundAGLFt: the go-around climbs to ATC's altitude, else this far
// above the field.
const apGoAroundAGLFt = 3000

// apGoAround flies ATC's go-around on the autopilot (not HandFly): TOGA
// thrust, "Go around, flaps" with one flap step asked, the approach mode
// off, the runway heading (or ATC's) held, the altitude selected (ATC's)
// by level change at the approach speed + 10; the gear up once climbing.
// Done above the acceleration height, climbing, with the autothrust on:
// the climb takes over and the next approach is armed anew.
func (e *Engine) apGoAround(in Input, c Config, out *Output) bool {
	h := &e.hand
	ap := in.State.AP
	if h.started.IsZero() {
		h.started = in.Now
		h.track = in.Air.Heading
		if in.Runway.valid() {
			h.track = in.Runway.Heading
		}
		out.Say = append(out.Say, "Go around, flaps")
		e.act(in, out, Action{Name: systems.TOGA}, true)
		if !ap.Master {
			e.act(in, out, set(systems.APMaster, true), true)
		}
		if k := flapsIndex(in); k > 0 && !h.flapsAsked {
			h.flapsAsked = true
			want := k - 1
			e.ask(in, c, out, "flaps", e.flapsSaid(want), func(in Input) bool { return flapsIndex(in) <= want }, Action{Name: systems.FlapsUp})
			out.Say = out.Say[:len(out.Say)-1] // said as "Go around, flaps"
		}
	}
	e.act(in, out, setValue(systems.Throttle, 100), false)
	if ap.Approach || ap.ApproachArmed {
		e.act(in, out, set(systems.APApproach, false), false)
	}
	hdg := h.track
	if in.ATC.HeadingDeg > 0 {
		hdg = in.ATC.HeadingDeg
	}
	e.value(in, out, systems.APHeadingSel, ap.HeadingSel, hdg, 1)
	if !ap.HeadingHold {
		e.act(in, out, set(systems.APHeadingHold, true), false)
	}
	alt := in.ATC.AltitudeFt
	if alt == 0 {
		alt = in.Air.AltFt - in.AGLFt() + apGoAroundAGLFt
	}
	e.value(in, out, systems.APAltitudeSel, ap.AltitudeSel, math.Round(alt/100)*100, 50)
	if !ap.FLC {
		e.act(in, out, set(systems.APFLC, true), false)
	}
	e.value(in, out, systems.APSpeedSel, ap.SpeedSel, c.ApproachKts+10, 2)
	if !h.gearAsked && in.Air.VS > 300 && in.Air.GearHandle {
		h.gearAsked = true
		e.ask(in, c, out, "gear", "Positive climb, gear up", func(in Input) bool { return !in.Air.GearHandle }, set(systems.GearDown, false))
	}
	if in.AGLFt() >= c.AccelAGLFt && in.Air.VS > 0 {
		if !ap.ATHR {
			e.act(in, out, set(systems.ATHR, true), true)
		}
		return true
	}
	return false
}
