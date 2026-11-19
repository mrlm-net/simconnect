package camera

import (
	"math"
	"sort"
	"sync"
	"time"
)

// TravelPath is a travel's flight on its own, for a host app that drives
// the camera itself: the same ease in, cruise and ease out (TravelRamp), the
// climb to MarginM over the higher end and the slow turn of the view
// (TravelTurnRate) as Spot.TravelTo, which flies one. Its ends are live:
// each At gives the eye and the look point now, and the drone lands
// exactly on the latest eye, smoothly however they moved. The way and the
// time are planned from the start, to the first eye.
type TravelPath struct {
	org    Point // the local origin: the first end's eye
	cosLat float64
	// The path in local metres from org: a Bézier curve on the ground, its
	// length s along steps; heights above org at each end and cruising.
	p      [4][2]float64
	steps  []float64 // length to sample i
	s      float64
	altA   float64
	altB   float64
	cruise float64
	speed  float64 // cruise, m/s
	ramp   float64 // s
	d      float64 // s
	fov0   float64
	turn   float64 // s: 0 no turn
	h0, p0 float64 // the view at the start, degrees
	dist0  float64

	mu       sync.Mutex
	lastHead float64 // the look's bearing unwrapped, for the turn
}

// NewTravelPath plans a travel from the camera now (from, its eye a World
// point, else the travel stands at toEye) to toEye, looking at toLook (World
// points; the look the subject's, a little above it as a spot aims), at o's
// cruise and margin as it plans (Observer.WithDefaults; the zero Observer
// flies at DefaultTravelSpeed and DefaultTravelMargin). The move is
// TravelAuto from a subject standing; Travel.Path picks another, or gives
// the subject's velocity.
func NewTravelPath(from Pose, toEye, toLook Point, o Observer) *TravelPath {
	o = o.WithDefaults()
	return Travel{From: from, SpeedMps: o.SpeedMps, MarginM: o.MarginM}.Path(toEye, toLook)
}

// Path plans t as a TravelPath from t.From to toEye, looking at toLook
// (World points): its Move, SpeedMps and MarginM as TravelTo flies them,
// TravelAuto and TravelDolly reading the subject's velocity from
// LookVelMps.
func (t Travel) Path(toEye, toLook Point) *TravelPath {
	x := &TravelPath{org: toEye, cosLat: math.Cos(toEye.Lat * math.Pi / 180)}
	v := t.LookVelMps
	return x.plan(t, x.local(toLook.Lat, toLook.Lon, toLook.AltM), vec3{v[0], v[1], v[2]}, true)
}

// plan plans t to org: look is where the view ends looking and vel the
// subject's velocity (local, at the start); have false, no subject known.
func (x *TravelPath) plan(t Travel, look, vel vec3, have bool) *TravelPath {
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

	l := math.Hypot(a.e, a.n)
	ue, un := 0.0, 0.0
	if l > 0 {
		ue, un = -a.e/l, -a.n/l // towards the end
	}
	move := t.Move
	if move == TravelAuto && have && l > 0 {
		move = autoTravel(a, look, vel, ue, un, l)
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
		side := (look.e-a.e)*-un + (look.n-a.n)*ue // > 0: the subject on the left
		bow = 0.2 * l
		if side > 0 {
			bow = -bow
		}
	case TravelPushIn:
		if w := math.Hypot(look.e, look.n); w > 0 {
			t1e, t1n = look.e/w, look.n/w
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
		h1, p1, _ := viewOf(a, look)
		if ang := angleBetween(x.h0, x.p0, h1, p1); ang > 0.5 {
			x.turn = max(travelTurnMin.Seconds(), ang*1.875/TravelTurnRate)
			x.lastHead = x.h0 + wrap180(h1-x.h0)
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

// Length is how long the travel takes.
func (x *TravelPath) Length() time.Duration { return time.Duration(x.d * float64(time.Second)) }

// Done tells whether the travel is over at elapsed: the drone on its eye.
func (x *TravelPath) Done(elapsed time.Duration) bool { return elapsed >= x.Length() }

// At is the camera elapsed into the travel, the ends now at toEye and
// toLook (World points; they may move between calls): the eye on its way,
// landing on toEye at Length, and the view turning to toLook, then on it.
// Its lens is the start's (Lens blends it). The view's turn is unwrapped
// call to call, so call it in time order.
func (x *TravelPath) At(elapsed time.Duration, toEye, toLook Point) Pose {
	sec := max(0, min(x.d, elapsed.Seconds()))
	eye := x.eyeAt(sec, x.local(toEye.Lat, toEye.Lon, toEye.AltM))
	tg, _ := x.look(sec, eye, x.local(toLook.Lat, toLook.Lon, toLook.AltM))
	return Pose{Eye: x.world(eye), Target: x.world(tg), FovDeg: x.fov0}
}

// Lens is the lens elapsed into the travel, blending from the start's to
// fov (degrees) as a travel to a spot does to its framing.
func (x *TravelPath) Lens(elapsed time.Duration, fov float64) float64 {
	f := 1.0
	if x.d > 0 {
		f = max(0, min(x.d, elapsed.Seconds())) / x.d
	}
	return x.fov0 + (fov-x.fov0)*Smooth(f)
}

func (x *TravelPath) local(lat, lon, alt float64) vec3 {
	return vec3{(lon - x.org.Lon) * mPerDegLat * x.cosLat, (lat - x.org.Lat) * mPerDegLat, alt - x.org.AltM}
}

func (x *TravelPath) world(p vec3) Point {
	return At(x.org.Lat+p.n/mPerDegLat, x.org.Lon+p.e/(mPerDegLat*x.cosLat), x.org.AltM+p.u)
}

func (x *TravelPath) bezier(u float64) (e, n float64) {
	v := 1 - u
	b0, b1, b2, b3 := v*v*v, 3*v*v*u, 3*v*u*u, u*u*u
	p := x.p
	return b0*p[0][0] + b1*p[1][0] + b2*p[2][0] + b3*p[3][0], b0*p[0][1] + b1*p[1][1] + b2*p[2][1] + b3*p[3][1]
}

// way is how far along the path at t seconds.
func (x *TravelPath) way(t float64) float64 {
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

// eyeAt is the eye at t seconds, local, the end's eye now at end: the path
// planned to the first end, shifted by how far the end has moved as the
// share of the way flown, so it lands on end with no jump.
func (x *TravelPath) eyeAt(t float64, end vec3) vec3 {
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
	return vec3{e, n, alt}.add(end, q)
}

// look is where the view looks at t seconds from eye (local), the look
// point now at at: turning to it while the turn lasts (turning true), then
// on it.
func (x *TravelPath) look(t float64, eye, at vec3) (tg vec3, turning bool) {
	w := 1.0
	if x.turn > 0 {
		w = Smooth(t / x.turn)
	}
	if w >= 1 {
		return at, false
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	h1, p1, d1 := viewOf(eye, at)
	h1 = x.lastHead + wrap180(h1-x.lastHead) // unwrapped as it turns
	x.lastHead = h1
	return along(eye, x.h0+(h1-x.h0)*w, x.p0+(p1-x.p0)*w, x.dist0+(d1-x.dist0)*w), true
}
