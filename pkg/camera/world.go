package camera

import (
	"math"
	"time"
)

// ObjectPose is an aircraft's position and attitude, for placing a shot's
// object-relative points in the world (a recorded track's, at a time).
type ObjectPose struct {
	Lat, Lon, AltM float64
	// Heading (true), Pitch (nose up positive) and Bank (right wing down
	// positive), degrees.
	Heading, Pitch, Bank float64
}

// mPerDegLat is metres per degree of latitude.
const mPerDegLat = 111_320.0

// Placed is p in the world: a World point as it is, an Object one from its
// object's pose (poseOf; false unknown), rotated by its heading, pitch and
// bank. An Eyepoint point cannot be placed (ok false).
func Placed(p Point, poseOf func(object uint32) (ObjectPose, bool)) (lat, lon, altM float64, ok bool) {
	switch p.Frame {
	case World:
		return p.Lat, p.Lon, p.AltM, true
	case Object:
		o, ok := poseOf(p.Object)
		if !ok {
			return 0, 0, 0, false
		}
		e, n, u := bodyToENU(o, p.Offset)
		return o.Lat + n/mPerDegLat, o.Lon + e/(mPerDegLat*math.Cos(o.Lat*math.Pi/180)), o.AltM + u, true
	}
	return 0, 0, 0, false
}

// bodyToENU turns an offset to the aircraft's right, above it and ahead of
// it into metres east, north and up, by its heading, pitch and bank.
func bodyToENU(o ObjectPose, off Offset) (e, n, u float64) {
	rad := math.Pi / 180
	sh, ch := math.Sin(o.Heading*rad), math.Cos(o.Heading*rad)
	sp, cp := math.Sin(o.Pitch*rad), math.Cos(o.Pitch*rad)
	sb, cb := math.Sin(o.Bank*rad), math.Cos(o.Bank*rad)
	type v3 [3]float64 // east, north, up
	f0, r0, u0 := v3{sh, ch, 0}, v3{ch, -sh, 0}, v3{0, 0, 1}
	add := func(a, b v3, x, y float64) v3 { return v3{a[0]*x + b[0]*y, a[1]*x + b[1]*y, a[2]*x + b[2]*y} }
	// Pitch about the right axis, then bank about the forward axis.
	fwd, upL := add(f0, u0, cp, sp), add(f0, u0, -sp, cp)
	right, up := add(r0, upL, cb, -sb), add(r0, upL, sb, cb)
	w := add(add(right, up, off.Right, off.Up), fwd, 1, off.Forward)
	return w[0], w[1], w[2]
}

// WorldPose is a shot's camera in the world at T into it: the eye and the
// point looked at, and the field of view.
type WorldPose struct {
	T                       time.Duration `json:"t"`
	EyeLat, EyeLon, EyeAltM float64
	TgtLat, TgtLon, TgtAltM float64
	FovDeg                  float64 `json:"fovDeg"`
}

// WorldPoses samples shot every step into the world, its objects placed by
// poseAt (the object's pose at t into the shot; false unknown, the sample
// left out): the camera's path and view for a map preview that matches
// what the simulator shows.
func WorldPoses(shot Shot, step time.Duration, poseAt func(t time.Duration, object uint32) (ObjectPose, bool)) []WorldPose {
	if step <= 0 {
		step = 100 * time.Millisecond
	}
	var out []WorldPose
	for t := time.Duration(0); ; t += step {
		if t > shot.Length() {
			t = shot.Length()
		}
		p := shot.PoseAt(t)
		of := func(o uint32) (ObjectPose, bool) { return poseAt(t, o) }
		elat, elon, ealt, ok1 := Placed(p.Eye, of)
		tlat, tlon, talt, ok2 := Placed(p.Target, of)
		if ok1 && ok2 {
			fov := p.FovDeg
			if fov <= 0 {
				fov = DefaultFovDeg
			}
			out = append(out, WorldPose{T: t, EyeLat: elat, EyeLon: elon, EyeAltM: ealt, TgtLat: tlat, TgtLon: tlon, TgtAltM: talt, FovDeg: fov})
		}
		if t >= shot.Length() {
			return out
		}
	}
}
