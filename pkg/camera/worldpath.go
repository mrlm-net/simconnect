package camera

import (
	"errors"
	"math"
	"time"
)

// WorldKey is one key of a designed shot: the eye in the world at replay
// time T (from the recording's first sample, the replay's clock).
type WorldKey struct {
	T time.Duration
	// Lat, Lon and AltM (meters above sea level) are the eye.
	Lat, Lon, AltM float64
	// HeadingDeg (true) and PitchDeg (up positive) aim the camera; used
	// only with LookFree.
	HeadingDeg, PitchDeg float64
	// FovDeg is the horizontal field of view (0: DefaultFovDeg); see
	// FovFromFocalMM.
	FovDeg float64
}

// LookMode is what a WorldPath looks at.
type LookMode uint8

const (
	// LookAircraft: an aircraft at replay time, its live object (LookAt
	// Object) or, without one, its recorded track (LookAt.Track).
	LookAircraft LookMode = iota
	// LookPoint: a fixed point in the world (LookAt.Lat, Lon, AltM).
	LookPoint
	// LookFree: where each key's heading and pitch aim.
	LookFree
)

// LookAt is a WorldPath's target.
type LookAt struct {
	Mode LookMode
	// LookAircraft: the replayed object (the simulator places it, in step
	// with what it shows), and the point on it (Offset)...
	Object uint32
	Offset Offset
	// ...or, Object 0, the aircraft at replay time t from its track
	// (flight.Track.At); false unknown.
	Track func(t time.Duration) (lat, lon, altM float64, ok bool)
	// LookPoint.
	Lat, Lon, AltM float64
}

// WorldPath is a designed shot in replay time: the eye through Keys (world
// coordinates, at their replay times) on a Catmull-Rom curve, eased over
// the whole shot (nil: Smooth), looking at Look. Cued at its first key's
// time (Cue) on a Timeline the Director follows on the replay's clock
// (Director.Follow), it stays in step through pause, seek and rate. Its
// length is not capped at MaxShot (Long says when it is over).
type WorldPath struct {
	// Label names the shot (Name, for logs).
	Label string
	Keys  []WorldKey
	Look  LookAt
	Ease  Ease
}

// Errors from WorldPath.Validate and Long.
var (
	ErrFewKeys     = errors.New("camera: a path needs two keys or more")
	ErrKeyOrder    = errors.New("camera: path keys must have increasing times")
	ErrLookTarget  = errors.New("camera: path look mode without its target")
	ErrShotTooLong = errors.New("camera: shot longer than MaxShot")
)

// freeTargetMeter: with LookFree, the point looked at is this far ahead.
const freeTargetMeter = 100.0

// Validate checks p's structure: two keys or more, their times increasing,
// a look mode with its target. Not its length (Long).
func (p WorldPath) Validate() error {
	if len(p.Keys) < 2 {
		return ErrFewKeys
	}
	for i := 1; i < len(p.Keys); i++ {
		if p.Keys[i].T <= p.Keys[i-1].T {
			return ErrKeyOrder
		}
	}
	switch p.Look.Mode {
	case LookAircraft:
		if p.Look.Object == 0 && p.Look.Track == nil {
			return ErrLookTarget
		}
	case LookPoint, LookFree:
	default:
		return ErrLookTarget
	}
	return nil
}

// Long is ErrShotTooLong when p runs longer than MaxShot (a warning: the
// filming rule, not a limit), nil else.
func (p WorldPath) Long() error {
	if p.Length() > MaxShot {
		return ErrShotTooLong
	}
	return nil
}

// Cue places p on a Timeline at its first key's replay time.
func (p WorldPath) Cue() Cue {
	var at time.Duration
	if len(p.Keys) > 0 {
		at = p.Keys[0].T
	}
	return Cue{At: at, Shot: p}
}

// Name is p's label.
func (p WorldPath) Name() string { return p.Label }

// Length is from the first key to the last.
func (p WorldPath) Length() time.Duration {
	if len(p.Keys) < 2 {
		return 0
	}
	return p.Keys[len(p.Keys)-1].T - p.Keys[0].T
}

// PoseAt is the camera t into the shot (replay time Keys[0].T + t).
func (p WorldPath) PoseAt(t time.Duration) Pose {
	n := len(p.Keys)
	if n == 0 {
		return Pose{}
	}
	k := p.Keys[0]
	if n > 1 {
		k = p.keyAt(p.Keys[0].T + time.Duration(progress(t, p.Length(), p.Ease)*float64(p.Length())))
	}
	eye := Point{Frame: World, Lat: k.Lat, Lon: k.Lon, AltM: k.AltM}
	var target Point
	switch p.Look.Mode {
	case LookAircraft:
		if p.Look.Object != 0 {
			target = Point{Frame: Object, Object: p.Look.Object, Offset: p.Look.Offset}
		} else if lat, lon, alt, ok := p.Look.Track(p.Keys[0].T + t); ok {
			target = Point{Frame: World, Lat: lat, Lon: lon, AltM: alt}
		} else {
			target = aimed(k)
		}
	case LookPoint:
		target = Point{Frame: World, Lat: p.Look.Lat, Lon: p.Look.Lon, AltM: p.Look.AltM}
	default:
		target = aimed(k)
	}
	return Pose{Eye: eye, Target: target, FovDeg: k.FovDeg}
}

// keyAt is the key at replay time rt between the keys (Catmull-Rom).
func (p WorldPath) keyAt(rt time.Duration) WorldKey {
	n := len(p.Keys)
	i := 0
	for i < n-2 && rt > p.Keys[i+1].T {
		i++
	}
	a, b := p.Keys[i], p.Keys[i+1]
	u := max(0, min(1, float64(rt-a.T)/float64(b.T-a.T)))
	k0, k3 := a, b
	if i > 0 {
		k0 = p.Keys[i-1]
	}
	if i+2 < n {
		k3 = p.Keys[i+2]
	}
	c := func(f func(WorldKey) float64) float64 { return catmull(f(k0), f(a), f(b), f(k3), u) }
	// Headings unwrapped about a's, so 350° to 10° turns 20°, not 340°.
	hdg := func(k WorldKey) float64 { return a.HeadingDeg + wrap180(k.HeadingDeg-a.HeadingDeg) }
	return WorldKey{
		T:          rt,
		Lat:        c(func(k WorldKey) float64 { return k.Lat }),
		Lon:        c(func(k WorldKey) float64 { return k.Lon }),
		AltM:       c(func(k WorldKey) float64 { return k.AltM }),
		HeadingDeg: math.Mod(c(hdg)+360, 360),
		PitchDeg:   c(func(k WorldKey) float64 { return k.PitchDeg }),
		FovDeg:     c(func(k WorldKey) float64 { return fovOf(Pose{FovDeg: k.FovDeg}) }),
	}
}

// aimed is the point freeTargetMeter ahead of k's eye along its heading
// and pitch.
func aimed(k WorldKey) Point {
	rad := math.Pi / 180
	h, p := k.HeadingDeg*rad, k.PitchDeg*rad
	n, e, u := freeTargetMeter*math.Cos(p)*math.Cos(h), freeTargetMeter*math.Cos(p)*math.Sin(h), freeTargetMeter*math.Sin(p)
	return Point{Frame: World, Lat: k.Lat + n/mPerDegLat, Lon: k.Lon + e/(mPerDegLat*math.Cos(k.Lat*rad)), AltM: k.AltM + u}
}

func wrap180(d float64) float64 {
	d = math.Mod(d+180, 360)
	if d < 0 {
		d += 360
	}
	return d - 180
}

// KeysOf is shot (a preset move, SideDolly, TopOrbit, ...) played from
// replay time at, as world keys every step for the designer to edit: its
// objects placed by poseAt (the object's pose at replay time; false
// unknown, that key left out), each key aimed (heading, pitch) where the
// shot looks, so LookFree plays it back as it was.
func KeysOf(shot Shot, at, step time.Duration, poseAt func(t time.Duration, object uint32) (ObjectPose, bool)) []WorldKey {
	if step <= 0 {
		step = time.Second
	}
	poses := WorldPoses(shot, step, func(t time.Duration, o uint32) (ObjectPose, bool) { return poseAt(at+t, o) })
	keys := make([]WorldKey, 0, len(poses))
	for _, w := range poses {
		n := (w.TgtLat - w.EyeLat) * mPerDegLat
		e := (w.TgtLon - w.EyeLon) * mPerDegLat * math.Cos(w.EyeLat*math.Pi/180)
		hdg := math.Mod(math.Atan2(e, n)*180/math.Pi+360, 360)
		pitch := math.Atan2(w.TgtAltM-w.EyeAltM, math.Hypot(n, e)) * 180 / math.Pi
		keys = append(keys, WorldKey{T: at + w.T, Lat: w.EyeLat, Lon: w.EyeLon, AltM: w.EyeAltM, HeadingDeg: hdg, PitchDeg: pitch, FovDeg: w.FovDeg})
	}
	return keys
}

// fullFrameWidthMM is a 35 mm camera's sensor width, the focal lengths a
// photographer knows.
const fullFrameWidthMM = 36.0

// FovFromFocalMM is the horizontal field of view of a lens of focal mm on a
// full-frame camera (50 mm: 39.6°; 24 mm: 73.7°).
func FovFromFocalMM(mm float64) float64 {
	if mm <= 0 {
		return DefaultFovDeg
	}
	return 2 * math.Atan(fullFrameWidthMM/(2*mm)) * 180 / math.Pi
}

// FocalMMFromFov is the full-frame focal length giving horizontal field of
// view deg.
func FocalMMFromFov(deg float64) float64 {
	if deg <= 0 || deg >= 180 {
		deg = DefaultFovDeg
	}
	return fullFrameWidthMM / (2 * math.Tan(deg/2*math.Pi/180))
}
