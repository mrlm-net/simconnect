//go:build windows
// +build windows

package camera

import (
	"math"
	"time"
)

// Drone moves on an aircraft, scaled to its size: wide shots that show the
// whole aircraft, moving in more than one way at once (a rise while it
// circles, a push-in while it slides), and detail shots close to a part of
// it, to cut between. Side is +1 for its right, -1 for its left.

// Size is an aircraft's span and length, meters.
type Size struct {
	Span, Length float64
}

// sized gives a usable size: an A320's when unknown.
func (s Size) sized() (w, l float64) {
	w, l = s.Span/2, s.Length
	if w <= 0 {
		w = 17.9
	}
	if l <= 0 {
		l = 2 * w * 1.05
	}
	return w, l
}

// Kind tells a wide shot from a detail one.
type Kind uint8

const (
	Wide Kind = iota
	Detail
)

// Move is a named drone move on an object of a size.
type MoveFunc func(object uint32, s Size, side float64, d time.Duration) Shot

// key is a keyframe relative to object.
func key(object uint32, at float64, eye, target Offset, fov float64) Key {
	return Key{At: at, Pose: Pose{Eye: On(object, eye.Right, eye.Up, eye.Forward), Target: On(object, target.Right, target.Up, target.Forward), FovDeg: fov}}
}

// RevealRise: low behind the aircraft, rising over it as it comes into view.
func RevealRise(o uint32, s Size, side float64, d time.Duration) Shot {
	w, l := s.sized()
	return Path("reveal rise", d, nil,
		key(o, 0, Offset{side * w * 0.8, 1.5, -l * 1.4}, Offset{0, 1, l * 0.4}, 40),
		key(o, 0.5, Offset{side * w * 1.4, l * 0.45, -l * 1.2}, Offset{0, 2, 0}, 48),
		key(o, 1, Offset{side * w * 1.1, l * 0.9, -l * 0.5}, Offset{0, 0, l * 0.25}, 55))
}

// Flyover: the drone passes over the aircraft from nose to tail, looking
// down at it.
func Flyover(o uint32, s Size, side float64, d time.Duration) Shot {
	w, l := s.sized()
	return Path("flyover", d, Linear,
		key(o, 0, Offset{side * w * 0.3, l * 0.8, l * 1.5}, Offset{0, 0, l * 0.5}, 55),
		key(o, 0.5, Offset{side * w * 0.2, l * 0.7, l * 0.1}, Offset{0, 0, 0}, 58),
		key(o, 1, Offset{side * w * 0.1, l * 0.8, -l * 1.3}, Offset{0, 0, -l * 0.2}, 55))
}

// SpiralDescend: circling a third of the way round while dropping from
// high and wide to low and close.
func SpiralDescend(o uint32, s Size, side float64, d time.Duration) Shot {
	_, l := s.sized()
	var keys []Key
	for i := 0; i <= 4; i++ {
		f := float64(i) / 4
		a := (150 - 120*f) * side * math.Pi / 180
		r := l * (2.6 - 1.3*f)
		keys = append(keys, key(o, f, Offset{r * math.Sin(a), l*(1.1-0.95*f) + 1, r * math.Cos(a)}, Offset{0, 1.5, 0}, 55-13*f))
	}
	return Path("spiral descend", d, nil, keys...)
}

// LeadChase: ahead of the aircraft flying backwards, sliding to its side
// as it closes on the nose.
func LeadChase(o uint32, s Size, side float64, d time.Duration) Shot {
	w, l := s.sized()
	return Path("lead chase", d, nil,
		key(o, 0, Offset{side * w * 0.2, 3, l * 2.2}, Offset{0, 1.5, 0}, 45),
		key(o, 0.5, Offset{side * w * 0.6, 2, l * 1.5}, Offset{0, 1.5, 0}, 41),
		key(o, 1, Offset{side * w * 1.1, 4, l * 1.0}, Offset{0, 1.5, -l * 0.1}, 37))
}

// ParallaxTrack: alongside at a distance, overtaking it slowly: the
// background slides behind it.
func ParallaxTrack(o uint32, s Size, side float64, d time.Duration) Shot {
	_, l := s.sized()
	return Path("parallax track", d, Linear,
		key(o, 0, Offset{side * l * 1.4, 2.5, -l * 1.1}, Offset{0, 1.5, -l * 0.2}, 40),
		key(o, 1, Offset{side * l * 1.4, 3.5, l * 1.1}, Offset{0, 1.5, l * 0.2}, 40))
}

// HeroLowPushIn: low at the front quarter, pushing in and narrowing: the
// aircraft looms.
func HeroLowPushIn(o uint32, s Size, side float64, d time.Duration) Shot {
	w, l := s.sized()
	return Path("hero push-in", d, nil,
		key(o, 0, Offset{side * w * 1.3, 0.8, l * 1.4}, Offset{0, 2, 0}, 40),
		key(o, 1, Offset{side * w * 0.8, 0.6, l * 0.85}, Offset{0, 2.5, -l * 0.1}, 30))
}

// PullBackReveal: from close on the engine back and up to the whole
// aircraft.
func PullBackReveal(o uint32, s Size, side float64, d time.Duration) Shot {
	w, l := s.sized()
	return Path("pull-back reveal", d, nil,
		key(o, 0, Offset{side * w * 0.55, 0.2, l * 0.3}, Offset{side * w * 0.33, -0.6, l * 0.05}, 35),
		key(o, 0.4, Offset{side * w * 1.2, 2, l * 0.7}, Offset{side * w * 0.2, 0, 0}, 45),
		key(o, 1, Offset{side * w * 2.4, l * 0.6, l * 1.4}, Offset{0, 1, 0}, 55))
}

// TopDown: high above, looking almost straight down, drifting forward.
func TopDown(o uint32, s Size, side float64, d time.Duration) Shot {
	_, l := s.sized()
	return Path("top down", d, Linear,
		key(o, 0, Offset{side * 1, l * 2.2, -l * 0.45}, Offset{0, 0, 0}, 55),
		key(o, 1, Offset{side * 1, l * 1.7, l * 0.1}, Offset{0, 0, l * 0.3}, 55))
}

// EngineDetail: close on the engine on side, drifting along it.
func EngineDetail(o uint32, s Size, side float64, d time.Duration) Shot {
	w, l := s.sized()
	return Path("engine detail", d, nil,
		key(o, 0, Offset{side * w * 0.6, 0.1, l * 0.45}, Offset{side * w * 0.33, -0.7, l * 0.05}, 32),
		key(o, 1, Offset{side * w * 0.5, 0.4, l * 0.32}, Offset{side * w * 0.33, -0.7, 0}, 28))
}

// NoseGearDetail: low by the nose gear.
func NoseGearDetail(o uint32, s Size, side float64, d time.Duration) Shot {
	_, l := s.sized()
	return Path("nose gear detail", d, nil,
		key(o, 0, Offset{side * 4, -0.8, l * 0.6}, Offset{0, -1.2, l * 0.36}, 32),
		key(o, 1, Offset{side * 3, -0.6, l * 0.52}, Offset{0, -1.2, l * 0.36}, 28))
}

// CockpitDetail: beside the cockpit windows, easing round them.
func CockpitDetail(o uint32, s Size, side float64, d time.Duration) Shot {
	_, l := s.sized()
	return Path("cockpit detail", d, nil,
		key(o, 0, Offset{side * 5, 2.4, l * 0.62}, Offset{0, 1.8, l * 0.42}, 30),
		key(o, 1, Offset{side * 3, 2.0, l * 0.68}, Offset{0, 1.8, l * 0.43}, 26))
}

// TailDetail: up by the fin and the livery.
func TailDetail(o uint32, s Size, side float64, d time.Duration) Shot {
	w, l := s.sized()
	return Path("tail detail", d, nil,
		key(o, 0, Offset{side * w * 0.6, l * 0.22, -l * 0.8}, Offset{0, l * 0.17, -l * 0.42}, 36),
		key(o, 1, Offset{side * w * 0.4, l * 0.28, -l * 0.7}, Offset{0, l * 0.17, -l * 0.44}, 32))
}

// WingtipAlong: by the wingtip, looking along the wing to the fuselage.
func WingtipAlong(o uint32, s Size, side float64, d time.Duration) Shot {
	w, _ := s.sized()
	return Path("wingtip", d, nil,
		key(o, 0, Offset{side * (w + 3), 1.5, -3}, Offset{0, 0.5, 0}, 45),
		key(o, 1, Offset{side * (w + 2), 2.2, 2}, Offset{0, 0.5, 0}, 42))
}

// ChaseRise: behind the aircraft climbing out, rising and falling back.
func ChaseRise(o uint32, s Size, side float64, d time.Duration) Shot {
	w, l := s.sized()
	return Path("chase rise", d, nil,
		key(o, 0, Offset{side * w * 0.4, 3, -l * 1.8}, Offset{0, 2, l * 1.5}, 48),
		key(o, 1, Offset{side * w * 1.2, l * 0.45, -l * 3.2}, Offset{0, 0, l * 1.5}, 52))
}

// TrackZoom: a camera fixed at eye following the aircraft, zooming in as it
// passes: beside the runway, under the approach.
func TrackZoom(name string, eye Point, object uint32, fromFov, toFov float64, d time.Duration) Shot {
	t := On(object, 0, 1.5, 0)
	return Move(name, Pose{Eye: eye, Target: t, FovDeg: fromFov}, Pose{Eye: eye, Target: t, FovDeg: toFov}, d, nil)
}

// TopOrbit: high above, looking down, turning slowly round: the tug
// coming in under the nose, the push starting.
func TopOrbit(o uint32, s Size, side float64, d time.Duration) Shot {
	_, l := s.sized()
	var keys []Key
	for i := 0; i <= 4; i++ {
		f := float64(i) / 4
		a := (200 + 100*f*side) * math.Pi / 180
		r := l * 0.35
		keys = append(keys, key(o, f, Offset{r * math.Sin(a), l * (1.9 - 0.3*f), r * math.Cos(a)}, Offset{0, 0, l * 0.15}, 52))
	}
	return Path("top orbit", d, Linear, keys...)
}

// MainGearDetail: low by the main gear, the wheels rolling.
func MainGearDetail(o uint32, s Size, side float64, d time.Duration) Shot {
	w, l := s.sized()
	return Path("main gear detail", d, nil,
		key(o, 0, Offset{side * w * 0.55, -1.2, l * 0.12}, Offset{side * 2.5, -1.8, -l * 0.04}, 32),
		key(o, 1, Offset{side * w * 0.45, -1.0, l * 0.02}, Offset{side * 2.5, -1.8, -l * 0.05}, 28))
}

// SideDolly: alongside at the aircraft's own speed, low, edging forward
// along it: the take-off roll, the landing.
func SideDolly(o uint32, s Size, side float64, d time.Duration) Shot {
	w, l := s.sized()
	return Path("side dolly", d, nil,
		key(o, 0, Offset{side * (w + l*0.9), 3, -l * 0.35}, Offset{0, 1.5, -l * 0.1}, 42),
		key(o, 1, Offset{side * (w + l*0.8), 4, l * 0.35}, Offset{0, 1.5, l * 0.05}, 38))
}

// HeadOnPass: a drone flying at the aircraft head-on, low, and up over it
// at the last moment.
func HeadOnPass(o uint32, s Size, side float64, d time.Duration) Shot {
	w, l := s.sized()
	return Path("head-on pass", d, Linear,
		key(o, 0, Offset{side * w * 0.2, 4, l * 3.5}, Offset{0, 2, 0}, 40),
		key(o, 0.65, Offset{side * w * 0.15, 6, l * 1.1}, Offset{0, 1.5, 0}, 48),
		key(o, 1, Offset{side * w * 0.1, l * 0.5, l * 0.1}, Offset{0, 0, -l * 0.6}, 56))
}

// LightsDetail: low ahead of the nose, the landing and taxi lights coming
// on towards the camera.
func LightsDetail(o uint32, s Size, side float64, d time.Duration) Shot {
	w, l := s.sized()
	return Path("lights", d, nil,
		key(o, 0, Offset{side * w * 0.5, 0.5, l * 1.2}, Offset{0, 0, l * 0.3}, 34),
		key(o, 1, Offset{side * w * 0.35, 0.3, l * 0.95}, Offset{0, 0, l * 0.3}, 30))
}
