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
		case systems.Throttle:
			p.thr = *ac.Value
		}
	}
	for _, r := range out.Requests { // the player does what is asked
		for _, d := range r.Do {
			if d.Name == systems.GearDown && d.On != nil {
				a.GearHandle = *d.On
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
		a.Heading += p.rud * 0.01 * dt // the rudder yaws it
		a.Heading += 1091 * math.Tan(a.Bank*math.Pi/180) / math.Max(a.IAS, 60) * dt
		fpa = a.Pitch - aoa
	}
	a.Heading = math.Mod(a.Heading+360, 360)
	acc := 0.1*p.thr - 0.0002*a.IAS*a.IAS - 0.25*fpa
	if a.OnGround && p.thr < 5 {
		acc -= 2.5 // idle, braking
	}
	a.IAS = math.Max(0, a.IAS+acc*dt)
	a.GS = a.IAS
	a.VS = a.IAS * 101.27 * math.Sin(fpa*math.Pi/180)
	a.AltFt += a.VS / 60 * dt
	if !a.OnGround && a.AltFt <= a.GroundFt && a.IAS > 30 {
		p.touched, p.touchVS, p.touchAlong, p.touchRight = true, a.VS, along, right
		a.OnGround, a.AltFt, a.VS, a.Bank = true, a.GroundFt, 0, 0
	}
	if a.OnGround {
		a.AltFt = a.GroundFt
	}
	d := a.GS * 0.5144 * dt
	trk := a.Heading * math.Pi / 180
	p.x += d * math.Sin(trk)
	p.y += d * math.Cos(trk)
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
	p := newPlane()
	along := -(250 - 50) / 3.28084 / 0.0524 // 250 ft on the glide path
	p.at(along, 15)                         // 15 m right of the centreline
	a := &p.in.Air
	a.OnGround, a.AltFt, a.IAS, a.GS, a.Pitch, a.Heading = false, 300+250, 148, 148, 2.5, 266
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
	_, right := 0.0, 0.0
	h := rwy.Heading * math.Pi / 180
	right = p.x*math.Cos(h) - p.y*math.Sin(h)
	if math.Abs(right) > 8 {
		t.Errorf("stopped %.1f m off the centreline", right)
	}
	for _, s := range []string{"Autopilot off", "Retard", "Your controls"} {
		if !slices.Contains(p.says, s) {
			t.Errorf("did not say %q: %v", s, p.says)
		}
	}
	t.Logf("touchdown %.0f ft/min %.0f m in, %.1f m right; said %v", p.touchVS, p.touchAlong, p.touchRight, p.says)
}
