package camera

import (
	"math"
	"time"
)

// A travel is the drone flying from where the camera is to a spot, slowly
// and smoothly, keeping its subject in frame: a shot in itself, so the
// camera moves as one observer would instead of cutting far away. It eases
// in and out of a steady cruise, climbs over the higher of its two ends with
// a margin (there is no ground data: the spots' heights are what is known),
// turns its view to the subject no faster than TravelTurnRate and ends
// exactly at the spot's eye with the spot's lens, so the spot shot after it
// (Spot.Follow, joined, on the same Aim) is no cut.

// TravelMove is the shape of a travel.
type TravelMove string

// The travels.
const (
	// TravelAuto picks one from the subject: a dolly alongside it moving
	// the way the drone goes, a slide past it standing near the way, a push
	// in when it lies beyond the spot, else a glide.
	TravelAuto TravelMove = ""
	// TravelGlide flies straight.
	TravelGlide TravelMove = "glide"
	// TravelDolly flies alongside a moving aircraft, on its heading.
	TravelDolly TravelMove = "dolly"
	// TravelSlide bows out round a parked aircraft, sliding past it.
	TravelSlide TravelMove = "slide"
	// TravelRise climbs higher on the way, over an apron.
	TravelRise TravelMove = "rise"
	// TravelPushIn ends moving towards the subject.
	TravelPushIn TravelMove = "pushIn"
)

// Travel is how the drone flies to a spot (Spot.TravelTo).
type Travel struct {
	// Name names the shot (default "travel <spot>").
	Name string
	// From is the camera now. Its eye should be a World point (else the
	// travel stands at the spot's eye); a World target turns to the
	// subject slowly, any other is let go of at once.
	From Pose
	Move TravelMove
	// SpeedMps is the cruise (0: DefaultTravelSpeed).
	SpeedMps float64
	// MarginM is how far above the higher end it cruises (0:
	// DefaultTravelMargin).
	MarginM float64
	// LookVelMps is the subject's velocity, metres a second east, north and
	// up, for TravelAuto and TravelDolly on a Travel.Path (TravelTo reads
	// its Aim's).
	LookVelMps [3]float64
}

// Defaults and limits of a Travel.
const (
	DefaultTravelSpeed  = 15.0
	DefaultTravelMargin = 15.0
	// TravelRamp: the ease in to the cruise, and out of it.
	TravelRamp = 8 * time.Second
	// TravelTurnRate is the fastest the view turns to the subject at the
	// start, degrees a second.
	TravelTurnRate = 6.0
	// travelTurnMin: the turn takes at least this long.
	travelTurnMin = 3 * time.Second
	// travelClimb: the share of the way spent climbing to the cruise at
	// each end.
	travelClimb = 0.35
	// travelRiseMax: TravelRise climbs up to this much higher.
	travelRiseMax = 40.0
	// travelSteps: the samples of the path's length.
	travelSteps = 512
)

// travelShot flies a TravelPath to the spot's eye, the look on the aim's
// subject: the one travel there is, a host app's and the World's alike.
type travelShot struct {
	spot Spot
	aim  *Aim
	size float64
	name string
	path *TravelPath // local metres from the spot's eye
}

// TravelTo flies the drone from t.From to s's eye, aimed at aim's subject
// of size (its largest dimension, metres) as s aims (LockAim too, once
// turned to it), the lens blending from t.From's to s's framing. It is as
// long as the way takes at the cruise, with TravelRamp at each end, and
// longer when the view has far to turn (at most MaxStretch). Follow the
// same Aim with s joined after it.
func (s Spot) TravelTo(aim *Aim, size float64, t Travel) Shot {
	s = s.withDefaults()
	x := &travelShot{spot: s, aim: aim, size: size, name: t.Name}
	if x.name == "" {
		x.name = "travel " + s.Name
	}
	x.path = &TravelPath{org: s.Eye, cosLat: math.Cos(s.Eye.Lat * math.Pi / 180)}

	// The subject now, for the shape and the turn: looked at a little above.
	aim.mu.Lock()
	now, have := aim.update()
	var look, vel vec3
	if have {
		look = x.path.local(aim.at(now))
		look.u += spotLookUp
		vel = aim.vel
	}
	aim.mu.Unlock()
	x.path.plan(t, look, vel, have)
	return x
}

// autoTravel is TravelAuto's pick, from the start a, the subject and its
// velocity, the way's direction and length (local metres from the spot).
func autoTravel(a, subj, vel vec3, ue, un, l float64) TravelMove {
	if v := math.Hypot(vel.e, vel.n); v > 3 && math.Abs(vel.e*ue+vel.n*un)/v > math.Cos(40*math.Pi/180) {
		return TravelDolly
	}
	along := (subj.e-a.e)*ue + (subj.n-a.n)*un
	cross := math.Abs((subj.e-a.e)*-un + (subj.n-a.n)*ue)
	if math.Hypot(vel.e, vel.n) < 1 && along > 0 && along < l && cross < max(150, 0.3*l) {
		return TravelSlide
	}
	if w := math.Hypot(subj.e, subj.n); w > 0 && (subj.e*ue+subj.n*un)/w > math.Cos(50*math.Pi/180) {
		return TravelPushIn
	}
	return TravelGlide
}

// viewOf is the heading and pitch (degrees) and distance from a to b.
func viewOf(a, b vec3) (h, p, d float64) {
	e, n, u := b.e-a.e, b.n-a.n, b.u-a.u
	g := math.Hypot(e, n)
	return math.Mod(math.Atan2(e, n)*180/math.Pi+360, 360), math.Atan2(u, g) * 180 / math.Pi, math.Hypot(g, u)
}

// angleBetween is the angle between two views, degrees.
func angleBetween(h0, p0, h1, p1 float64) float64 {
	r := math.Pi / 180
	c := math.Sin(p0*r)*math.Sin(p1*r) + math.Cos(p0*r)*math.Cos(p1*r)*math.Cos((h1-h0)*r)
	return math.Acos(max(-1, min(1, c))) / r
}

// along is the point d along a view from a.
func along(a vec3, h, p, d float64) vec3 {
	r := math.Pi / 180
	g := d * math.Cos(p*r)
	return vec3{a.e + g*math.Sin(h*r), a.n + g*math.Cos(h*r), a.u + d*math.Sin(p*r)}
}

// rampIn is the way covered over the ease in at f (0..1) of it, as a share
// of the cruise speed times the ramp: the speed rises as Smooth does.
func rampIn(f float64) float64 {
	f = max(0, min(1, f))
	return f * f * f * f * (f*(f-3) + 2.5)
}

func (x *travelShot) Length() time.Duration { return x.path.Length() }
func (x *travelShot) Name() string          { return x.name }

// PoseAt is the drone t into the travel; its aim is live, on the aim's
// clock.
func (x *travelShot) PoseAt(t time.Duration) Pose {
	path := x.path
	sec := max(0, min(path.d, t.Seconds()))
	eye := path.eyeAt(sec, vec3{})
	a := x.aim
	a.mu.Lock()
	defer a.mu.Unlock()
	now, ok := a.update()
	p := Pose{Eye: path.world(eye), Target: On(a.object, 0, spotLookUp, 0), FovDeg: path.fov0}
	if !ok {
		return p
	}
	aim := path.local(a.at(now))
	aim.u += spotLookUp
	p.FovDeg = path.Lens(t, x.spot.Framing(x.size, aim.sub(eye).len()))
	tg, turning := path.look(sec, eye, aim)
	switch {
	case turning:
		p.Target = path.world(tg)
	case x.spot.LockAim:
		p.Target = On(a.object, 0, spotLookUp, 0)
	default:
		p.Target = path.world(aim)
	}
	a.show(p.Eye, p.FovDeg)
	return p
}

// Pan turns a camera standing at eye from looking at from to looking at to
// (World points), the lens from fromFov to toFov, over d (at most
// MaxStretch), eased (Smooth): a slow look round. Its length for a turn is
// PanLength.
func Pan(name string, eye, from, to Point, fromFov, toFov float64, d time.Duration) Shot {
	if d <= 0 || d > MaxStretch {
		d = MaxStretch
	}
	return pan{name, eye, from, to, fromFov, toFov, d}
}

// PanRate is the fastest a Pan of PanLength turns, degrees a second.
const PanRate = 4.0

// PanLength is how long a Pan at eye from looking at from to looking at to
// takes at most PanRate, at least min.
func PanLength(eye, from, to Point, least time.Duration) time.Duration {
	o := vec3{}
	cos := math.Cos(eye.Lat * math.Pi / 180)
	loc := func(p Point) vec3 {
		return vec3{(p.Lon - eye.Lon) * mPerDegLat * cos, (p.Lat - eye.Lat) * mPerDegLat, p.AltM - eye.AltM}
	}
	h0, p0, _ := viewOf(o, loc(from))
	h1, p1, _ := viewOf(o, loc(to))
	sec := angleBetween(h0, p0, h1, p1) * 1.875 / PanRate
	return max(least, time.Duration(sec*float64(time.Second)))
}

type pan struct {
	name           string
	eye, from, to  Point
	fromFov, toFov float64
	d              time.Duration
}

func (p pan) Length() time.Duration { return p.d }
func (p pan) Name() string          { return p.name }
func (p pan) PoseAt(t time.Duration) Pose {
	cos := math.Cos(p.eye.Lat * math.Pi / 180)
	loc := func(q Point) vec3 {
		return vec3{(q.Lon - p.eye.Lon) * mPerDegLat * cos, (q.Lat - p.eye.Lat) * mPerDegLat, q.AltM - p.eye.AltM}
	}
	f := progress(t, p.d, Smooth)
	h0, p0, d0 := viewOf(vec3{}, loc(p.from))
	h1, p1, d1 := viewOf(vec3{}, loc(p.to))
	v := along(vec3{}, h0+wrap180(h1-h0)*f, p0+(p1-p0)*f, d0+(d1-d0)*f)
	fa, fb := fovOf(Pose{FovDeg: p.fromFov}), fovOf(Pose{FovDeg: p.toFov})
	return Pose{
		Eye:    p.eye,
		Target: At(p.eye.Lat+v.n/mPerDegLat, p.eye.Lon+v.e/(mPerDegLat*cos), p.eye.AltM+v.u),
		FovDeg: fa + (fb-fa)*f,
	}
}
