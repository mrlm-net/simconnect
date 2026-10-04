package camera

import "time"

// Key is one keyframe of a Path: the pose at At (0..1 through the shot).
type Key struct {
	At   float64
	Pose Pose
}

type path struct {
	name string
	keys []Key
	d    time.Duration
	ease Ease
}

// Path flies the camera through keys over d (at most MaxShot) on a smooth
// curve (Catmull-Rom through the eye, target and field of view of each
// key), eased overall (nil: Smooth): the combined moves of a drone — rising
// while circling while pushing in. Keys should share their frames; they are
// sorted by At, the first at 0 and the last at 1.
func Path(name string, d time.Duration, ease Ease, keys ...Key) Shot {
	return path{name, keys, clamp(d), ease}
}

func (p path) Length() time.Duration { return p.d }
func (p path) Name() string          { return p.name }

func (p path) PoseAt(t time.Duration) Pose {
	n := len(p.keys)
	switch n {
	case 0:
		return Pose{}
	case 1:
		return p.keys[0].Pose
	}
	f := progress(t, p.d, p.ease)
	i := 0
	for i < n-2 && f > p.keys[i+1].At {
		i++
	}
	a, b := p.keys[i], p.keys[i+1]
	u := 0.0
	if b.At > a.At {
		u = (f - a.At) / (b.At - a.At)
	}
	u = max(0, min(1, u))
	p0, p3 := a.Pose, b.Pose
	if i > 0 {
		p0 = p.keys[i-1].Pose
	}
	if i+2 < n {
		p3 = p.keys[i+2].Pose
	}
	return Pose{
		Eye:    splinePoint(p0.Eye, a.Pose.Eye, b.Pose.Eye, p3.Eye, u),
		Target: splinePoint(p0.Target, a.Pose.Target, b.Pose.Target, p3.Target, u),
		FovDeg: catmull(fovOf(p0), fovOf(a.Pose), fovOf(b.Pose), fovOf(p3), u),
	}
}

func fovOf(p Pose) float64 {
	if p.FovDeg <= 0 {
		return DefaultFovDeg
	}
	return p.FovDeg
}

// catmull is the Catmull-Rom curve through b (u=0) and c (u=1), shaped by a
// and d.
func catmull(a, b, c, d, u float64) float64 {
	u2, u3 := u*u, u*u*u
	return 0.5 * (2*b + (c-a)*u + (2*a-5*b+4*c-d)*u2 + (3*b-a-3*c+d)*u3)
}

// splinePoint is the curve between b and c; points in other frames than
// b's are taken as b (or c) so the curve stays in one frame.
func splinePoint(a, b, c, d Point, u float64) Point {
	if b.Frame != c.Frame || b.Frame != World && b.Object != c.Object {
		return lerpPoint(b, c, u)
	}
	same := func(p, q Point) Point {
		if p.Frame != b.Frame || p.Frame != World && p.Object != b.Object {
			return q
		}
		return p
	}
	a, d = same(a, b), same(d, c)
	k := func(x, y, z, w float64) float64 { return catmull(x, y, z, w, u) }
	return Point{Frame: b.Frame, Object: b.Object,
		Lat:  k(a.Lat, b.Lat, c.Lat, d.Lat),
		Lon:  k(a.Lon, b.Lon, c.Lon, d.Lon),
		AltM: k(a.AltM, b.AltM, c.AltM, d.AltM),
		Offset: Offset{
			Right:   k(a.Offset.Right, b.Offset.Right, c.Offset.Right, d.Offset.Right),
			Up:      k(a.Offset.Up, b.Offset.Up, c.Offset.Up, d.Offset.Up),
			Forward: k(a.Offset.Forward, b.Offset.Forward, c.Offset.Forward, d.Offset.Forward),
		}}
}
