package camera

import (
	"math"
	"sort"
	"sync"
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

type travelShot struct {
	spot Spot
	aim  *Aim
	size float64
	name string

	cosLat float64
	// The path in local metres from the spot's eye: a Bézier curve on the
	// ground, its length S along steps; heights above the eye at each end
	// and cruising.
	p        [4][2]float64
	steps    []float64 // length to sample i
	s        float64
	altA     float64
	altB     float64
	cruise   float64
	speed    float64 // cruise, m/s
	ramp     float64 // s
	d        float64 // s
	fov0     float64
	turn     float64 // s: 0 no turn
	h0, p0   float64 // the view at the start, degrees
	dist0    float64
	mu       sync.Mutex
	lastHead float64 // the subject's bearing unwrapped, for the turn
	haveHead bool
}

// TravelTo flies the drone from t.From to s's eye, aimed at aim's subject
// of size (its largest dimension, metres) as s aims (LockAim too, once
// turned to it), the lens blending from t.From's to s's framing. It is as
// long as the way takes at the cruise, with TravelRamp at each end, and
// longer when the view has far to turn (at most MaxStretch). Follow the
// same Aim with s joined after it.
func (s Spot) TravelTo(aim *Aim, size float64, t Travel) Shot {
	s = s.withDefaults()
	x := &travelShot{spot: s, aim: aim, size: size, name: t.Name, cosLat: math.Cos(s.Eye.Lat * math.Pi / 180)}
	if x.name == "" {
		x.name = "travel " + s.Name
	}
	speed := t.SpeedMps
	if speed <= 0 {
		speed = DefaultTravelSpeed
	}
	margin := t.MarginM
	if margin <= 0 {
		margin = DefaultTravelMargin
	}
	var a vec3 // the start, local
	if t.From.Eye.Frame == World {
		a = x.local(t.From.Eye.Lat, t.From.Eye.Lon, t.From.Eye.AltM)
	}
	x.fov0 = fovOf(t.From)

	// The subject now, for the shape and the turn.
	aim.mu.Lock()
	now, have := aim.update()
	var subj, vel vec3
	if have {
		subj = x.local(aim.at(now))
		vel = aim.vel
	}
	aim.mu.Unlock()

	l := math.Hypot(a.e, a.n)
	ue, un := 0.0, 0.0
	if l > 0 {
		ue, un = -a.e/l, -a.n/l // towards the spot
	}
	move := t.Move
	if move == TravelAuto && have && l > 0 {
		move = autoTravel(a, subj, vel, ue, un, l)
	}
	// Control points: tangents of a third of the way at each end.
	t0e, t0n, t1e, t1n := ue, un, ue, un
	bow := 0.0
	switch move {
	case TravelDolly:
		if v := math.Hypot(vel.e, vel.n); v > 0 {
			ve, vn := vel.e/v, vel.n/v
			if ve*ue+vn*un < 0 {
				ve, vn = -ve, -vn
			}
			t0e, t0n, t1e, t1n = ve, vn, ve, vn
		}
	case TravelSlide:
		// Away from the subject's side of the way, a fifth of it.
		side := (subj.e-a.e)*-un + (subj.n-a.n)*ue // > 0: the subject on the left
		bow = 0.2 * l
		if side > 0 {
			bow = -bow
		}
	case TravelPushIn:
		if w := math.Hypot(subj.e, subj.n); w > 0 {
			t1e, t1n = subj.e/w, subj.n/w
		}
	}
	k := l / 3
	ne, nn := -un, ue // the left of the way
	x.p = [4][2]float64{
		{a.e, a.n},
		{a.e + t0e*k + ne*bow, a.n + t0n*k + nn*bow},
		{-t1e*k + ne*bow, -t1n*k + nn*bow},
		{0, 0},
	}
	x.steps = make([]float64, travelSteps+1)
	pe, pn := x.bezier(0)
	for i := 1; i <= travelSteps; i++ {
		e, n := x.bezier(float64(i) / travelSteps)
		x.steps[i] = x.steps[i-1] + math.Hypot(e-pe, n-pn)
		pe, pn = e, n
	}
	x.s = x.steps[travelSteps]
	x.altA, x.altB = a.u, 0
	x.cruise = max(x.altA, x.altB) + margin
	if move == TravelRise {
		x.cruise += min(travelRiseMax, max(10, 0.06*l))
	}

	// The climb at its steepest counts in the speed: the cruise is the
	// drone's, not its way's over the ground.
	if x.s > 0 {
		k := max(x.cruise-x.altA, x.cruise-x.altB) * 1.875 / (travelClimb * x.s)
		speed /= math.Sqrt(1 + k*k)
	}

	// The view's turn to the subject.
	if t.From.Target.Frame == World && have {
		tg := x.local(t.From.Target.Lat, t.From.Target.Lon, t.From.Target.AltM)
		x.h0, x.p0, x.dist0 = viewOf(a, tg)
		h1, p1, _ := viewOf(a, vec3{subj.e, subj.n, subj.u + spotLookUp})
		if ang := angleBetween(x.h0, x.p0, h1, p1); ang > 0.5 {
			x.turn = max(travelTurnMin.Seconds(), ang*1.875/TravelTurnRate)
			x.lastHead, x.haveHead = x.h0+wrap180(h1-x.h0), true
		}
	}

	// The time: the way at the cruise with a ramp at each end, or longer
	// for the turn, slower.
	ramp := TravelRamp.Seconds()
	if x.s > 0 {
		ramp = min(ramp, x.s/speed)
		x.d = x.s/speed + ramp
	}
	if need := x.turn + 2; x.turn > 0 && x.d < need {
		x.d = need
	}
	x.d = min(x.d, MaxStretch.Seconds())
	if x.s > 0 {
		ramp = min(TravelRamp.Seconds(), x.d/2)
		speed = x.s / (x.d - ramp)
	}
	x.speed, x.ramp = speed, ramp
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

func (x *travelShot) local(lat, lon, alt float64) vec3 {
	return vec3{(lon - x.spot.Eye.Lon) * mPerDegLat * x.cosLat, (lat - x.spot.Eye.Lat) * mPerDegLat, alt - x.spot.Eye.AltM}
}

func (x *travelShot) world(p vec3) Point {
	return At(x.spot.Eye.Lat+p.n/mPerDegLat, x.spot.Eye.Lon+p.e/(mPerDegLat*x.cosLat), x.spot.Eye.AltM+p.u)
}

func (x *travelShot) bezier(u float64) (e, n float64) {
	v := 1 - u
	b0, b1, b2, b3 := v*v*v, 3*v*v*u, 3*v*u*u, u*u*u
	p := x.p
	return b0*p[0][0] + b1*p[1][0] + b2*p[2][0] + b3*p[3][0], b0*p[0][1] + b1*p[1][1] + b2*p[2][1] + b3*p[3][1]
}

// rampIn is the way covered over the ease in at f (0..1) of it, as a share
// of the cruise speed times the ramp: the speed rises as Smooth does.
func rampIn(f float64) float64 {
	f = max(0, min(1, f))
	return f * f * f * f * (f*(f-3) + 2.5)
}

// way is how far along the path at t seconds.
func (x *travelShot) way(t float64) float64 {
	switch {
	case x.s == 0:
		return 0
	case t <= x.ramp:
		return x.speed * x.ramp * rampIn(t/x.ramp)
	case t >= x.d-x.ramp:
		return x.s - x.speed*x.ramp*rampIn((x.d-t)/x.ramp)
	}
	return x.speed*x.ramp/2 + x.speed*(t-x.ramp)
}

// eyeAt is the eye at t seconds, local.
func (x *travelShot) eyeAt(t float64) vec3 {
	w := max(0, min(x.s, x.way(t)))
	i := sort.SearchFloat64s(x.steps, w)
	u := 1.0
	if i <= 0 {
		u = 0
	} else if i <= travelSteps {
		a, b := x.steps[i-1], x.steps[i]
		f := 0.0
		if b > a {
			f = (w - a) / (b - a)
		}
		u = (float64(i-1) + f) / travelSteps
	}
	e, n := x.bezier(u)
	q := 1.0
	if x.s > 0 {
		q = w / x.s
	}
	up := Smooth(q / travelClimb)
	down := Smooth((1 - q) / travelClimb)
	alt := x.cruise - (x.cruise-x.altA)*(1-up) - (x.cruise-x.altB)*(1-down)
	if x.s == 0 {
		alt = x.altB
	}
	return vec3{e, n, alt}
}

func (x *travelShot) Length() time.Duration { return time.Duration(x.d * float64(time.Second)) }
func (x *travelShot) Name() string          { return x.name }

// PoseAt is the drone t into the travel; its aim is live, on the aim's
// clock.
func (x *travelShot) PoseAt(t time.Duration) Pose {
	sec := max(0, min(x.d, t.Seconds()))
	eye := x.eyeAt(sec)
	x.mu.Lock()
	defer x.mu.Unlock()
	a := x.aim
	a.mu.Lock()
	defer a.mu.Unlock()
	now, ok := a.update()
	p := Pose{Eye: x.world(eye), Target: On(a.object, 0, spotLookUp, 0), FovDeg: x.fov0}
	if !ok {
		return p
	}
	aim := x.local(a.at(now))
	aim.u += spotLookUp
	f := 1.0
	if x.d > 0 {
		f = sec / x.d
	}
	want := x.spot.Framing(x.size, aim.sub(eye).len())
	p.FovDeg = x.fov0 + (want-x.fov0)*Smooth(f)
	w := 1.0
	if x.turn > 0 {
		w = Smooth(sec / x.turn)
	}
	switch {
	case w < 1:
		h1, p1, d1 := viewOf(eye, aim)
		h1 = x.lastHead + wrap180(h1-x.lastHead) // unwrapped as it turns
		x.lastHead = h1
		p.Target = x.world(along(eye, x.h0+(h1-x.h0)*w, x.p0+(p1-x.p0)*w, x.dist0+(d1-x.dist0)*w))
	case x.spot.LockAim:
		p.Target = On(a.object, 0, spotLookUp, 0)
	default:
		p.Target = x.world(aim)
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
