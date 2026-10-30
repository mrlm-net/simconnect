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
func (a vec3) len() float64 { return math.Sqrt(a.e*a.e + a.n*a.n + a.u*a.u) }

type spotShot struct {
	spot   Spot
	object uint32
	size   float64
	d      time.Duration
	feed   func() (SpotSample, bool)
	clock  func() time.Time
	name   string

	mu sync.Mutex
	// Local metres from the Eye.
	cosLat float64
	// The filter: pos and vel at sampleAt, the correction still to fade.
	have     bool
	sampleAt time.Time
	pos, vel vec3
	fade     vec3
	haveVel  bool
	last     time.Time
	// The rail: how far along it, how fast; the lens.
	rail, railV float64
	railLen     float64
	railDir     vec3
	fov         float64
}

// Track plays the spot on object for d (at most MaxStretch): size is the
// aircraft's largest dimension in metres, feed its latest sample (it must not
// block: it runs on the camera's goroutine, under the Director's lock), clock
// the time the samples are stamped in (nil: time.Now). The shot runs on that
// clock and only forward; it is live, a pose asked again for an earlier t is
// the current one.
func (s Spot) Track(object uint32, size float64, d time.Duration, feed func() (SpotSample, bool), clock func() time.Time) Shot {
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
	if clock == nil {
		clock = time.Now
	}
	if d <= 0 || d > MaxStretch {
		d = MaxStretch
	}
	x := &spotShot{spot: s, object: object, size: size, d: d, feed: feed, clock: clock, name: s.Name,
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

// sample takes a new sample into the filter, the aim carrying on from where
// it was shown.
func (x *spotShot) sample(s SpotSample, now time.Time) {
	z := x.local(s.Pose.Lat, s.Pose.Lon, s.Pose.AltM)
	reported := vec3{s.MpsEast, s.MpsNorth, s.MpsUp}
	if !x.have {
		x.have, x.sampleAt, x.pos, x.vel = true, s.At, z, reported
		x.haveVel = reported != vec3{}
		return
	}
	dt := s.At.Sub(x.sampleAt).Seconds()
	if dt <= 0 {
		return
	}
	shown := x.aimed(s.At)
	pred := x.pos.add(x.vel, dt)
	if !x.haveVel {
		// The second sample gives the velocity.
		x.vel, x.haveVel = vec3{(z.e - x.pos.e) / dt, (z.n - x.pos.n) / dt, (z.u - x.pos.u) / dt}, true
		x.pos = z
	} else {
		r := vec3{z.e - pred.e, z.n - pred.n, z.u - pred.u}
		x.pos = pred.add(r, spotAlpha)
		x.vel = x.vel.add(r, spotBeta/dt)
	}
	x.sampleAt = s.At
	x.fade = vec3{shown.e - x.pos.e, shown.n - x.pos.n, shown.u - x.pos.u}
}

// aimed is the aim at now: the filter run on from its sample, no further than
// spotCoast, with what is left of the last correction.
func (x *spotShot) aimed(now time.Time) vec3 {
	dt := now.Sub(x.sampleAt)
	if dt < 0 {
		dt = 0
	}
	f := math.Exp(-dt.Seconds() / spotCorrection.Seconds())
	return x.pos.add(x.vel, min(dt, spotCoast).Seconds()).add(x.fade, f)
}

// railAt moves the camera along its rail towards abeam p, starting and
// stopping gently. The first call puts it there.
func (x *spotShot) railAt(p vec3, dt float64, first bool) {
	if x.railLen == 0 {
		return
	}
	want := max(0, min(x.railLen, p.e*x.railDir.e+p.n*x.railDir.n+p.u*x.railDir.u))
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

// PoseAt: see Track.
func (x *spotShot) PoseAt(time.Duration) Pose {
	x.mu.Lock()
	defer x.mu.Unlock()
	now := x.clock()
	first := !x.have
	if s, ok := x.feed(); ok && (first || s.At.After(x.sampleAt)) {
		x.sample(s, now)
	}
	if !x.have {
		return Pose{Eye: x.spot.Eye, Target: On(x.object, 0, spotLookUp, 0), FovDeg: x.spot.FovMax}
	}
	dt := 0.0
	if !x.last.IsZero() {
		dt = max(0, now.Sub(x.last).Seconds())
	}
	x.last = now
	aim := x.aimed(now)
	x.railAt(aim, dt, first)
	eye := vec3{x.railDir.e * x.rail, x.railDir.n * x.rail, x.railDir.u * x.rail}
	dist := vec3{aim.e - eye.e, aim.n - eye.n, aim.u - eye.u}.len()
	want := x.spot.FovMax
	if dist > 0 && x.size > 0 {
		want = 2 * math.Atan(x.size/x.spot.Share/(2*dist)) * 180 / math.Pi
	}
	want = max(x.spot.FovMin, min(x.spot.FovMax, want))
	if first {
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
		p.Target = On(x.object, 0, spotLookUp, 0)
	}
	return p
}
