//go:build windows
// +build windows

package camera

import (
	"math"
	"time"
)

// A Shot is the camera over a stretch of time: its pose at each moment.
type Shot interface {
	// Length is how long the shot runs.
	Length() time.Duration
	// PoseAt is the pose at t into the shot (0 ≤ t ≤ Length).
	PoseAt(t time.Duration) Pose
	// Name says what the shot is, for logs.
	Name() string
}

// MaxShot is the longest a shot runs: a still or a move held longer loses
// the viewer (and the Director cuts there).
const MaxShot = 10 * time.Second

// Ease shapes a move's progress: f(0) = 0, f(1) = 1.
type Ease func(f float64) float64

// Linear moves at a constant pace.
func Linear(f float64) float64 { return f }

// Smooth starts and stops gently (smootherstep): no jerk at either end, the
// steady camera a viewer expects.
func Smooth(f float64) float64 {
	f = math.Max(0, math.Min(1, f))
	return f * f * f * (f*(f*6-15) + 10)
}

// progress is t through length, eased.
func progress(t, length time.Duration, ease Ease) float64 {
	if length <= 0 {
		return 1
	}
	f := math.Max(0, math.Min(1, float64(t)/float64(length)))
	if ease == nil {
		ease = Smooth
	}
	return ease(f)
}

// clamp keeps a shot's length within MaxShot.
func clamp(d time.Duration) time.Duration {
	if d <= 0 || d > MaxShot {
		return MaxShot
	}
	return d
}

type hold struct {
	name string
	p    Pose
	d    time.Duration
}

// Hold keeps pose p for d (at most MaxShot).
func Hold(name string, p Pose, d time.Duration) Shot { return hold{name, p, clamp(d)} }

func (h hold) Length() time.Duration     { return h.d }
func (h hold) PoseAt(time.Duration) Pose { return h.p }
func (h hold) Name() string              { return h.name }

type move struct {
	name     string
	from, to Pose
	d        time.Duration
	ease     Ease
}

// Move goes from pose from to pose to over d (at most MaxShot), eased (nil:
// Smooth): a dolly, a crane, a push-in by the field of view. Both ends
// should be in the same frames for a continuous move.
func Move(name string, from, to Pose, d time.Duration, ease Ease) Shot {
	return move{name, from, to, clamp(d), ease}
}

func (m move) Length() time.Duration { return m.d }
func (m move) PoseAt(t time.Duration) Pose {
	return Lerp(m.from, m.to, progress(t, m.d, m.ease))
}
func (m move) Name() string { return m.name }

type orbit struct {
	name                   string
	object                 uint32
	radius, height, lookUp float64
	fromDeg, toDeg, fovDeg float64
	d                      time.Duration
	ease                   Ease
}

// Orbit circles object at radius meters and height meters above it, from
// fromDeg to toDeg around it (0 dead ahead, 90 its right, 180 behind),
// looking at it lookUp meters above its reference point.
func Orbit(name string, object uint32, radius, height, lookUp, fromDeg, toDeg, fovDeg float64, d time.Duration, ease Ease) Shot {
	return orbit{name, object, radius, height, lookUp, fromDeg, toDeg, fovDeg, clamp(d), ease}
}

func (o orbit) Length() time.Duration { return o.d }
func (o orbit) PoseAt(t time.Duration) Pose {
	a := (o.fromDeg + (o.toDeg-o.fromDeg)*progress(t, o.d, o.ease)) * math.Pi / 180
	return Pose{
		Eye:    On(o.object, o.radius*math.Sin(a), o.height, o.radius*math.Cos(a)),
		Target: On(o.object, 0, o.lookUp, 0),
		FovDeg: o.fovDeg,
	}
}
func (o orbit) Name() string { return o.name }

// Track is a camera fixed in the world at eye, following object (looking
// lookUp meters above it): a tower or runway-side camera. The simulator
// keeps it on the aircraft from frame to frame.
func Track(name string, eye Point, object uint32, lookUp, fovDeg float64, d time.Duration) Shot {
	return Hold(name, Pose{Eye: eye, Target: On(object, 0, lookUp, 0), FovDeg: fovDeg}, d)
}

// Chase rides with object at offset from, moving to offset to over d,
// looking lookAhead meters ahead of it: behind it on the climb-out, beside
// it on the taxiway.
func Chase(name string, object uint32, from, to Offset, lookAhead, fovDeg float64, d time.Duration) Shot {
	look := On(object, 0, 2, lookAhead)
	return Move(name,
		Pose{Eye: On(object, from.Right, from.Up, from.Forward), Target: look, FovDeg: fovDeg},
		Pose{Eye: On(object, to.Right, to.Up, to.Forward), Target: look, FovDeg: fovDeg}, d, nil)
}
