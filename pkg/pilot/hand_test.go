//go:build windows

package pilot

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/flight"
	"github.com/mrlm-net/simconnect/pkg/systems"
)

// plane is a crude jet for the loops: the elevator turns the pitch, the
// ailerons the bank, the bank the heading; the throttle against drag and
// the climb gives the speed; the climb is the pitch over the angle of
// attack the speed needs. 20 steps a second. Runway 26 at 300 ft.
type plane struct {
	in                        Input
	x, y                      float64 // metres east, north of the threshold
	elev, ail, rud, thr       float64
	reversed, reverseSeen     bool
	windE, windN              float64 // knots the air moves east, north
	touchHdg, stopAlong       float64
	brake                     float64
	touchVS, touchAlong, maxX float64
	touchRight                float64
	touched                   bool
	says                      []string
}

var rwy = Runway{Threshold: airport.LatLon{Lat: 44.57, Lon: 26.08}, Heading: 264, ElevFt: 300, LengthM: 3500}

func newPlane() *plane {
	p := &plane{}
	p.in = Input{Now: time.Unix(0, 0), Runway: rwy,
		State: systems.State{Values: map[string]float64{systems.FlapsIndex: 2, systems.DesignVS0: 110}, VRKt: 140, V2Kt: 150, DHFt: 200}}
	p.in.Air = flight.Sample{GroundFt: 300, Heading: 264, OnGround: true, GearHandle: true}
	return p
}

// at puts the plane d metres along the centreline (negative before the
// threshold) and off metres right of it.
func (p *plane) at(along, right float64) {
	h := rwy.Heading * math.Pi / 180
	p.x = along*math.Sin(h) + right*math.Cos(h)
	p.y = along*math.Cos(h) - right*math.Sin(h)
}

func (p *plane) step(e *Engine) Output {
	a := &p.in.Air
	h := rwy.Heading * math.Pi / 180
	along, right := p.x*math.Sin(h)+p.y*math.Cos(h), p.x*math.Cos(h)-p.y*math.Sin(h)
	lat, lon := calc.DisplaceByHeading(rwy.Threshold.Lat, rwy.Threshold.Lon, rwy.Heading, along)
	lat, lon = calc.DisplaceByHeading(lat, lon, rwy.Heading+90, right)
	a.Lat, a.Lon = lat, lon
	out := e.Update(p.in)
	p.says = append(p.says, out.Say...)
	for _, ac := range out.Actions {
		if ac.Value == nil {
			if ac.Name == systems.Reversers && ac.On != nil {
				p.reversed = *ac.On
				p.reverseSeen = p.reverseSeen || *ac.On
			}
			if ac.Name == systems.GearDown && ac.On != nil {
				a.GearHandle = *ac.On
			}
			continue
		}
		switch ac.Name {
		case systems.Elevator:
			p.elev = *ac.Value
		case systems.Aileron:
			p.ail = *ac.Value
		case systems.Rudder:
			p.rud = *ac.Value
		case systems.BrakeLeft:
			p.brake = *ac.Value
		case systems.Throttle:
			p.thr = *ac.Value
		}
	}
	for _, r := range out.Requests { // the player does what is asked
		for _, d := range r.Do {
			if d.Name == systems.GearDown && d.On != nil {
				a.GearHandle = *d.On
			}
			if d.Name == systems.FlapsUp {
				p.in.State.Values[systems.FlapsIndex]--
			}
		}
	}
	const dt = 0.05
	p.in.Now = p.in.Now.Add(50 * time.Millisecond)
	aoa := 5 * math.Pow(140/math.Max(a.IAS, 60), 2)
	fpa := 0.0
	if a.OnGround {
		if a.IAS > 135 || p.elev < 0 {
			a.Pitch = math.Max(0, a.Pitch+0.06*p.elev*dt)
		}
		a.Heading += p.rud * 0.02 * math.Min(a.IAS/30, 1) * dt
		if a.IAS >= 143 && a.Pitch >= 7 {
			a.OnGround = false
			fpa = a.Pitch - aoa // climbing from this step
		}
	} else {
		a.Pitch += 0.06 * p.elev * dt
		a.Bank = math.Max(-30, math.Min(30, a.Bank+0.1*p.ail*dt))
		a.Heading += p.rud * 0.08 * dt // the rudder yaws it
		a.Heading += 1091 * math.Tan(a.Bank*math.Pi/180) / math.Max(a.IAS, 60) * dt
		fpa = a.Pitch - aoa
	}
	a.Heading = math.Mod(a.Heading+360, 360)
	acc := 0.1*p.thr - 0.0002*a.IAS*a.IAS - 0.25*fpa
	if p.reversed {
		acc = -0.05*p.thr - 0.0002*a.IAS*a.IAS // reverse thrust
	}
	if a.OnGround {
		acc -= 0.5 + p.brake*0.06 // rolling, the brakes
	}
	a.IAS = math.Max(0, a.IAS+acc*dt)
	a.GS = a.IAS
	a.VS = a.IAS * 101.27 * math.Sin(fpa*math.Pi/180)
	a.AltFt += a.VS / 60 * dt
	if !a.OnGround && a.AltFt <= a.GroundFt && a.IAS > 30 {
		p.touched, p.touchVS, p.touchAlong, p.touchRight, p.touchHdg = true, a.VS, along, right, a.Heading
		a.OnGround, a.AltFt, a.VS, a.Bank = true, a.GroundFt, 0, 0
	}
	if a.OnGround {
		a.AltFt = a.GroundFt
	}
	// The air moves it with the wind; on the ground it rolls where it points.
	hdg := a.Heading * math.Pi / 180
	vx, vy := a.IAS*math.Sin(hdg), a.IAS*math.Cos(hdg)
	if !a.OnGround {
		vx, vy = vx+p.windE, vy+p.windN
		if math.Abs(p.rud) > 5 { // crossed controls: it slips toward the low wing
			slip := a.IAS * math.Sin(a.Bank*math.Pi/180) * 1.2
			vx, vy = vx+slip*math.Cos(hdg), vy-slip*math.Sin(hdg)
		}
	}
	a.GS = math.Hypot(vx, vy)
	p.x += vx * 0.5144 * dt
	p.y += vy * 0.5144 * dt
	p.maxX = math.Max(p.maxX, math.Abs(right))
	return out
}

// TestHandTakeoff: cleared for take-off the copilot sets the thrust, holds
// the centreline, rotates at VR toward the rotation pitch, asks for the
// gear on a positive climb and engages the autopilot at 1000 ft, near
// V2 + 10.
func TestHandTakeoff(t *testing.T) {
	p := newPlane()
	p.at(0, 3) // lined up 3 m right of the centreline
	p.in.ATC = Clearance{Takeoff: true, AltitudeFt: 5000}
	e := New(Config{HandFly: true}, []string{"zero", "one", "two", "three", "full"})
	rotatedAt, maxPitch := 0.0, 0.0
	for i := 0; i < 20*120 && e.Phase() != PhaseClimb; i++ {
		p.step(e)
		if rotatedAt == 0 && p.in.Air.Pitch > 1 {
			rotatedAt = p.in.Air.IAS
		}
		maxPitch = math.Max(maxPitch, p.in.Air.Pitch)
	}
	if e.Phase() != PhaseClimb {
		t.Fatalf("not at the engage height: phase %v at %.0f ft AGL, %.0f kt, pitch %.1f", e.Phase(), p.in.AGLFt(), p.in.Air.IAS, p.in.Air.Pitch)
	}
	if rotatedAt < 138 || rotatedAt > 148 { // pitch over 1°: a moment after VR
		t.Errorf("rotated at %.0f kt, VR 140", rotatedAt)
	}
	if maxPitch > 20 || maxPitch < 10 {
		t.Errorf("pitch up to %.1f°", maxPitch)
	}
	if v := p.in.Air.IAS; v < 145 || v > 185 {
		t.Errorf("at the engage height %.0f kt, V2 + 10 = 160", v)
	}
	if p.maxX > 8 {
		t.Errorf("off the centreline up to %.1f m", p.maxX)
	}
	for _, s := range []string{"Takeoff", "Positive climb, gear up", "Autopilot on"} {
		if !slices.Contains(p.says, s) {
			t.Errorf("did not say %q: %v", s, p.says)
		}
	}
	if p.in.Air.GearHandle {
		t.Error("gear still down")
	}
}

// TestHandLanding: from minimums on a 3° final the copilot flies the glide
// path, flares, retards, lands in the touchdown zone on the centreline at
// a gentle sink rate, and hands back below 40 kt.
func TestHandLanding(t *testing.T) {
	for _, c := range []struct {
		name         string
		windE, windN float64
	}{
		{"calm", 0, 0},
		{"15 kt crosswind from the left", 0, 15}, // runway 264: wind from 174
	} {
		t.Run(c.name, func(t *testing.T) {
			p := newPlane()
			p.windE, p.windN = c.windE, c.windN
			along := -(205 - 50) / 3.28084 / 0.0524 // 205 ft on the glide path, at minimums
			p.at(along, 15)                         // 15 m right of the centreline
			a := &p.in.Air
			// Crabbed into the wind as the autopilot flew it (2° right of it).
			crab := -math.Asin((c.windE*math.Cos(rwy.Heading*math.Pi/180)-c.windN*math.Sin(rwy.Heading*math.Pi/180))/148) * 180 / math.Pi
			a.OnGround, a.AltFt, a.IAS, a.GS, a.Pitch, a.Heading = false, 300+205, 148, 148, 2.5, rwy.Heading+2+crab
			a.VS = -148 * 101.27 * 0.0524
			p.in.State.AP.Master = true
			p.in.State.Values[systems.FlapsIndex] = 4
			p.in.ATC = Clearance{Approach: true, Land: true}
			p.in.Plan.DistanceToGoNM = 1
			e := New(Config{HandFly: true}, []string{"zero", "one", "two", "three", "full"})
			e.phase, e.taking = PhaseApproach, false
			handback := false
			for i := 0; i < 20*180 && !handback; i++ {
				handback = p.step(e).Handback
			}
			if !handback {
				t.Fatalf("no handback: phase %v, %.0f kt, touched %v, %s", e.Phase(), a.IAS, p.touched, p.says)
			}
			if !p.touched || p.touchVS < -400 || p.touchVS > -30 {
				t.Errorf("touchdown at %.0f ft/min", p.touchVS)
			}
			if p.touchAlong < 100 || p.touchAlong > 800 {
				t.Errorf("touchdown %.0f m past the threshold", p.touchAlong)
			}
			if math.Abs(p.touchRight) > 8 {
				t.Errorf("touchdown %.1f m off the centreline", p.touchRight)
			}
			if d := math.Abs(angleDiff(p.touchHdg, rwy.Heading)); d > 4 {
				t.Errorf("touchdown %.1f° off the runway heading: not de-crabbed", d)
			}
			h := rwy.Heading * math.Pi / 180
			right := p.x*math.Cos(h) - p.y*math.Sin(h)
			stop := p.x*math.Sin(h) + p.y*math.Cos(h)
			if math.Abs(right) > 8 {
				t.Errorf("handed back %.1f m off the centreline", right)
			}
			if !p.reverseSeen || p.reversed {
				t.Errorf("reversers: used %v, still out %v", p.reverseSeen, p.reversed)
			}
			if stop-p.touchAlong > 1800 {
				t.Errorf("%.0f m from touchdown to 40 kt", stop-p.touchAlong)
			}
			if p.brake != 0 {
				t.Errorf("handed back braking %.0f %%", p.brake)
			}
			for _, s := range []string{"Autopilot off", "Retard", "Your controls"} {
				if !slices.Contains(p.says, s) {
					t.Errorf("did not say %q: %v", s, p.says)
				}
			}
			t.Logf("touchdown %.0f ft/min %.0f m in, %.1f m right, heading %.1f; 40 kt %.0f m later", p.touchVS, p.touchAlong, p.touchRight, p.touchHdg, stop-p.touchAlong)
		})
	}
}

// TestHandGoAround: at minimums, not cleared to land, the copilot goes
// around: "Go around, flaps", one flap step asked, TOGA, the nose up, never
// lower than 100 ft; "Positive climb, gear up"; the autopilot at 1000 ft,
// climbing to the cleared altitude.
func TestHandGoAround(t *testing.T) {
	p := newPlane()
	along := -(205 - 50) / 3.28084 / 0.0524
	p.at(along, 0)
	a := &p.in.Air
	a.OnGround, a.AltFt, a.IAS, a.GS, a.Pitch, a.Heading = false, 300+205, 148, 148, 2.5, rwy.Heading
	a.VS = -148 * 101.27 * 0.0524
	p.in.State.AP.Master = true
	p.in.State.Values[systems.FlapsIndex] = 4
	p.in.ATC = Clearance{Approach: true, AltitudeFt: 3000}
	p.in.Plan.DistanceToGoNM = 1
	e := New(Config{HandFly: true}, []string{"zero", "one", "two", "three", "full"})
	e.phase, e.taking = PhaseApproach, false
	lowest := math.Inf(1)
	for i := 0; i < 20*120 && e.Phase() != PhaseClimb; i++ {
		p.step(e)
		lowest = math.Min(lowest, p.in.AGLFt())
		if p.in.State.AP.Master && e.Phase() == PhaseGoAround {
			p.in.State.AP.Master = false // the disconnect took
		}
	}
	if e.Phase() != PhaseClimb {
		t.Fatalf("not climbing on the autopilot: phase %v at %.0f ft, %.0f kt; %v", e.Phase(), p.in.AGLFt(), a.IAS, p.says)
	}
	if lowest < 100 || p.touched {
		t.Errorf("down to %.0f ft (touched %v)", lowest, p.touched)
	}
	if f := p.in.State.Values[systems.FlapsIndex]; f != 3 {
		t.Errorf("flaps %v, want one step up (3)", f)
	}
	for _, s := range []string{"Go around, flaps", "Positive climb, gear up", "Autopilot on"} {
		if !slices.Contains(p.says, s) {
			t.Errorf("did not say %q: %v", s, p.says)
		}
	}
	if a.GearHandle {
		t.Error("gear still down")
	}
	t.Logf("lowest %.0f ft, %.0f kt at 1000 ft; said %v", lowest, a.IAS, p.says)
}

// TestApproachArmedAgain: the approach armed and the descent asked for are
// once an approach: after a go-around, and after the controls are taken
// again, the next approach is armed anew.
func TestApproachArmedAgain(t *testing.T) {
	armed := func(e *Engine, p *plane) bool {
		p.in.State.AP = systems.AutopilotState{Master: true}
		p.in.ATC = Clearance{Approach: true, AltitudeFt: 3000}
		out := e.Update(p.in)
		return slices.Contains(out.Say, "Approach armed")
	}
	p := newPlane()
	a := &p.in.Air
	a.OnGround, a.AltFt, a.IAS, a.Heading = false, 300+2500, 160, rwy.Heading
	p.in.Plan.DistanceToGoNM = 10
	e := New(Config{HandFly: true}, nil)
	e.phase, e.taking = PhaseApproach, false
	if !armed(e, p) {
		t.Fatal("first approach not armed")
	}
	if armed(e, p) {
		t.Error("armed twice on one approach")
	}
	p.in.ATC.GoAround = true
	e.Update(p.in) // the go-around
	if e.Phase() != PhaseGoAround {
		t.Fatalf("phase %v, want go-around", e.Phase())
	}
	e.phase = PhaseApproach // back round for the next one
	if !armed(e, p) {
		t.Error("not armed after the go-around")
	}
	e.HandBack()
	e.TakeControl()
	if e.said["appr"] || e.said["descent"] {
		t.Error("said kept over TakeControl")
	}
	if !armed(e, p) && !armed(e, p) { // takes over, then arms
		t.Error("not armed after the controls were taken again")
	}
}
