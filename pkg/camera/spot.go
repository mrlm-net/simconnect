package camera

import (
	"math"
	"sync"
	"time"
)

// Spot is a camera standing in the world, as a spotter stands at the fence:
// fixed, or on a rail following its subject, the lens chosen to keep the
// aircraft at a set share of the frame.
type Spot struct {
	Name string
	// Eye is where it stands (a World point).
	Eye Point
	// RailEnd, when set, is the other end of a straight rail from Eye: the
	// camera goes abeam its subject, at most RailSpeed (m/s, 0:
	// DefaultRailSpeed).
	RailEnd   *Point
	RailSpeed float64
	// FovMin and FovMax are the lens, horizontal degrees (0:
	// DefaultSpotFovMin, DefaultSpotFovMax).
	FovMin, FovMax float64
	// Share is how much of the frame's width the aircraft fills, 0..1 (0:
	// DefaultSpotShare).
	Share float64
	// LockAim aims at the object itself, the simulator keeping it dead
	// centre from frame to frame, instead of the smoothed aim.
	LockAim bool
}

// Defaults of a Spot.
const (
	DefaultSpotFovMin = 6.0
	DefaultSpotFovMax = 50.0
	DefaultSpotShare  = 0.35
	DefaultRailSpeed  = 4.0
)

// SpotSample is where the subject was at At. MpsEast, MpsNorth and MpsUp are
// its velocity if reported (all 0: found from the samples).
type SpotSample struct {
	Pose                     ObjectPose
	At                       time.Time
	MpsEast, MpsNorth, MpsUp float64
}

// The aim: an alpha-beta filter on the samples, which may come only once a
// second, says where the subject is and how it moves; between them the aim
// runs on with that velocity. What a sample corrects is taken out slowly
// (spotCorrection), so a long lens never shows the step, and there is no lag
// behind a subject flying straight, as smoothing the position itself would
// have.
const (
	spotAlpha = 0.5
	spotBeta  = 0.2
	// spotCorrection: a sample's correction fades with this time constant.
	spotCorrection = 800 * time.Millisecond
	// spotCoast: no sample for this long (the simulator paused, the subject
	// gone), the aim stops where it got to.
	spotCoast = 2500 * time.Millisecond
	// spotZoom: the lens follows the distance with this time constant.
	spotZoom = 2 * time.Second
	// spotLookUp: the aim is this far above the aircraft's reference point.
	spotLookUp = 2.0
)

type vec3 struct{ e, n, u float64 }

func (a vec3) add(b vec3, f float64) vec3 {
	return vec3{a.e + b.e*f, a.n + b.n*f, a.u + b.u*f}
}
func (a vec3) sub(b vec3) vec3 { return vec3{a.e - b.e, a.n - b.n, a.u - b.u} }
func (a vec3) len() float64    { return math.Sqrt(a.e*a.e + a.n*a.n + a.u*a.u) }

// Aim is where a subject is, from its samples (the filter above): one per
// subject, shared by the shots that film it one after another (a travel and
// the spot it ends at, a spot taken again), so the aim and the lens run on
// across them with no step. Safe for concurrent use.
type Aim struct {
	object uint32
	feed   func() (SpotSample, bool)
	clock  func() time.Time

	mu sync.Mutex
	// Local metres from the first sample (origin).
	lat0, lon0, alt0 float64
	cosLat           float64
	// The filter: pos and vel at sampleAt, the correction still to fade.
	have     bool
	sampleAt time.Time
	pos, vel vec3
	fade     vec3
	haveVel  bool
	// What the last shot on it showed: its eye (a World point) and lens.
	shown    bool
	shownEye Point
	shownFov float64
}

// NewAim follows object (0: the user aircraft): feed is its latest sample
// (it must not block: it runs on the camera's goroutine, under the
// Director's lock), clock the time the samples are stamped in (nil:
// time.Now).
func NewAim(object uint32, feed func() (SpotSample, bool), clock func() time.Time) *Aim {
	if clock == nil {
		clock = time.Now
	}
	return &Aim{object: object, feed: feed, clock: clock}
}

// Object is the object the aim follows.
func (a *Aim) Object() uint32 { return a.object }

// update takes the latest sample in: the time now, and whether there is
// an aim. a.mu held.
func (a *Aim) update() (time.Time, bool) {
	now := a.clock()
	if a.feed != nil {
		if s, ok := a.feed(); ok && (!a.have || s.At.After(a.sampleAt)) {
			a.sample(s)
		}
	}
	return now, a.have
}

func (a *Aim) local(lat, lon, alt float64) vec3 {
	return vec3{(lon - a.lon0) * mPerDegLat * a.cosLat, (lat - a.lat0) * mPerDegLat, alt - a.alt0}
}

// sample takes a new sample into the filter, the aim carrying on from where
// it was shown.
func (a *Aim) sample(s SpotSample) {
	if !a.have {
		a.lat0, a.lon0, a.alt0 = s.Pose.Lat, s.Pose.Lon, s.Pose.AltM
		a.cosLat = math.Cos(s.Pose.Lat * math.Pi / 180)
	}
	z := a.local(s.Pose.Lat, s.Pose.Lon, s.Pose.AltM)
	reported := vec3{s.MpsEast, s.MpsNorth, s.MpsUp}
	if !a.have {
		a.have, a.sampleAt, a.pos, a.vel = true, s.At, z, reported
		a.haveVel = reported != vec3{}
		return
	}
	dt := s.At.Sub(a.sampleAt).Seconds()
	if dt <= 0 {
		return
	}
	shown := a.aimed(s.At)
	pred := a.pos.add(a.vel, dt)
	if !a.haveVel {
		// The second sample gives the velocity.
		a.vel, a.haveVel = vec3{(z.e - a.pos.e) / dt, (z.n - a.pos.n) / dt, (z.u - a.pos.u) / dt}, true
		a.pos = z
	} else {
		r := z.sub(pred)
		a.pos = pred.add(r, spotAlpha)
		a.vel = a.vel.add(r, spotBeta/dt)
	}
	a.sampleAt = s.At
	a.fade = shown.sub(a.pos)
}

// aimed is the aim at now in local metres: the filter run on from its
// sample, no further than spotCoast, with what is left of the last
// correction.
func (a *Aim) aimed(now time.Time) vec3 {
	dt := now.Sub(a.sampleAt)
	if dt < 0 {
		dt = 0
	}
	f := math.Exp(-dt.Seconds() / spotCorrection.Seconds())
	return a.pos.add(a.vel, min(dt, spotCoast).Seconds()).add(a.fade, f)
}

// at is the aim at now in the world. a.mu held, a.have.
func (a *Aim) at(now time.Time) (lat, lon, alt float64) {
	p := a.aimed(now)
	return a.lat0 + p.n/mPerDegLat, a.lon0 + p.e/(mPerDegLat*a.cosLat), a.alt0 + p.u
}

// show notes what a shot on it showed. a.mu held.
func (a *Aim) show(eye Point, fov float64) {
	a.shown, a.shownEye, a.shownFov = true, eye, fov
}

type spotShot struct {
	spot Spot
	aim  *Aim
	size float64
	d    time.Duration
	name string
	// join: the shot carries on from what the last shot on its aim showed.
	join bool

	mu sync.Mutex
	// Local metres from the Eye.
	cosLat  float64
	started bool
	last    time.Time
	// The rail: how far along it, how fast; the lens.
	rail, railV float64
	railLen     float64
	railDir     vec3
	fov         float64
}

// withDefaults fills s's zero values.
func (s Spot) withDefaults() Spot {
	if s.FovMin <= 0 {
		s.FovMin = DefaultSpotFovMin
	}
	if s.FovMax <= 0 {
		s.FovMax = DefaultSpotFovMax
	}
	if s.Share <= 0 {
		s.Share = DefaultSpotShare
	}
	if s.RailSpeed <= 0 {
		s.RailSpeed = DefaultRailSpeed
	}
	return s
}

// Framing is the lens keeping an aircraft of size (its largest
// dimension) at s's share of the frame at dist metres, within its lens.
func (s Spot) Framing(size, dist float64) float64 {
	s = s.withDefaults()
	want := s.FovMax
	if dist > 0 && size > 0 {
		want = 2 * math.Atan(size/s.Share/(2*dist)) * 180 / math.Pi
	}
	return max(s.FovMin, min(s.FovMax, want))
}

// Track plays the spot on object for d (at most MaxStretch): size is the
// aircraft's largest dimension in metres, feed its latest sample (it must not
// block: it runs on the camera's goroutine, under the Director's lock), clock
// the time the samples are stamped in (nil: time.Now). The shot runs on that
// clock and only forward; it is live, a pose asked again for an earlier t is
// the current one.
func (s Spot) Track(object uint32, size float64, d time.Duration, feed func() (SpotSample, bool), clock func() time.Time) Shot {
	return s.Follow(NewAim(object, feed, clock), size, d, false)
}

// Follow plays the spot on aim's subject for d (at most MaxStretch), as
// Track. join carries on from the last shot on aim: its rail starts at the
// point nearest the eye shown last and moves on gently, the lens from the
// last one shown; after a TravelTo ending at Eye, or for the spot taken
// again, there is no cut.
func (s Spot) Follow(aim *Aim, size float64, d time.Duration, join bool) Shot {
	s = s.withDefaults()
	if d <= 0 || d > MaxStretch {
		d = MaxStretch
	}
	x := &spotShot{spot: s, aim: aim, size: size, d: d, name: s.Name, join: join,
		cosLat: math.Cos(s.Eye.Lat * math.Pi / 180), fov: s.FovMax}
	if s.RailEnd != nil {
		to := x.local(s.RailEnd.Lat, s.RailEnd.Lon, s.RailEnd.AltM)
		if l := to.len(); l > 0 {
			x.railLen, x.railDir = l, vec3{to.e / l, to.n / l, to.u / l}
		}
	}
	return x
}

func (x *spotShot) Length() time.Duration { return x.d }
func (x *spotShot) Name() string          { return x.name }

func (x *spotShot) local(lat, lon, alt float64) vec3 {
	return vec3{(lon - x.spot.Eye.Lon) * mPerDegLat * x.cosLat, (lat - x.spot.Eye.Lat) * mPerDegLat, alt - x.spot.Eye.AltM}
}

func (x *spotShot) world(p vec3) Point {
	return At(x.spot.Eye.Lat+p.n/mPerDegLat, x.spot.Eye.Lon+p.e/(mPerDegLat*x.cosLat), x.spot.Eye.AltM+p.u)
}

// railAt moves the camera along its rail towards abeam p, starting and
// stopping gently. The first call puts it there.
func (x *spotShot) railAt(p vec3, dt float64, first bool) {
	if x.railLen == 0 {
		return
	}
	want := x.onRail(p)
	if first {
		x.rail = want
		return
	}
	limit := x.spot.RailSpeed
	v := max(-limit, min(limit, (want-x.rail)/2))
	step := limit / 3 * dt // full speed in 3 s
	x.railV += max(-step, min(step, v-x.railV))
	x.rail = max(0, min(x.railLen, x.rail+x.railV*dt))
}

// onRail is how far along the rail is nearest p.
func (x *spotShot) onRail(p vec3) float64 {
	return max(0, min(x.railLen, p.e*x.railDir.e+p.n*x.railDir.n+p.u*x.railDir.u))
}

// PoseAt: see Track.
func (x *spotShot) PoseAt(time.Duration) Pose {
	x.mu.Lock()
	defer x.mu.Unlock()
	a := x.aim
	a.mu.Lock()
	defer a.mu.Unlock()
	now, ok := a.update()
	if !ok {
		return Pose{Eye: x.spot.Eye, Target: On(a.object, 0, spotLookUp, 0), FovDeg: x.spot.FovMax}
	}
	first := !x.started
	x.started = true
	dt := 0.0
	if !x.last.IsZero() {
		dt = max(0, now.Sub(x.last).Seconds())
	}
	x.last = now
	aim := x.local(a.at(now))
	joined := first && x.join && a.shown
	if joined {
		// Carry on from the last shot: no jump on the rail or in the lens.
		x.rail = x.onRail(x.local(a.shownEye.Lat, a.shownEye.Lon, a.shownEye.AltM))
		x.fov = a.shownFov
	}
	x.railAt(aim, dt, first && !joined)
	eye := vec3{x.railDir.e * x.rail, x.railDir.n * x.rail, x.railDir.u * x.rail}
	want := x.spot.Framing(x.size, aim.sub(eye).len())
	if first && !joined {
		x.fov = want
	} else {
		x.fov += (want - x.fov) * (1 - math.Exp(-dt/spotZoom.Seconds()))
	}
	aim.u += spotLookUp
	p := Pose{Eye: x.spot.Eye, Target: x.world(aim), FovDeg: x.fov}
	if x.railLen > 0 {
		p.Eye = x.world(eye)
	}
	if x.spot.LockAim {
		p.Target = On(a.object, 0, spotLookUp, 0)
	}
	a.show(p.Eye, p.FovDeg)
	return p
}
