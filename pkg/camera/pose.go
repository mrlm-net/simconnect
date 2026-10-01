//go:build windows
// +build windows

// Package camera drives the MSFS 2024 add-on camera: poses relative to the
// world, an aircraft or the pilot's eyepoint, shots that hold or move
// between them over time with eased motion, and a Director that plays a
// shot list on the camera at a steady rate.
//
// Conventions, measured live (SDK 1.7.3; the SDK documentation leaves them
// out): in the world referential x is the latitude (degrees), y the
// longitude (degrees) and z the altitude (meters above sea level); relative
// to an aircraft x points to its left, y up and z forward, in meters. The
// rotation SimConnect returns does not follow the header's pitch, bank,
// heading order, so poses aim with a point to look at (targeted) rather than
// with angles.
package camera

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// Frame is what a point is given in.
type Frame uint8

const (
	// World: latitude, longitude and altitude (meters above sea level).
	World Frame = iota
	// Object: an offset from a SimObject (Point.Object; 0 is the user
	// aircraft).
	Object
	// Eyepoint: an offset from the user aircraft's pilot eyepoint.
	Eyepoint
)

// Offset is a position relative to an aircraft, in meters: to its right,
// above it, ahead of it.
type Offset struct {
	Right, Up, Forward float64
}

// Point is a place for the camera or for it to look at.
type Point struct {
	Frame Frame
	// World.
	Lat, Lon, AltM float64
	// Object and Eyepoint.
	Object uint32
	Offset Offset
}

// At is a point in the world.
func At(lat, lon, altM float64) Point { return Point{Frame: World, Lat: lat, Lon: lon, AltM: altM} }

// On is a point relative to object (0: the user aircraft).
func On(object uint32, right, up, forward float64) Point {
	return Point{Frame: Object, Object: object, Offset: Offset{right, up, forward}}
}

// referential is the SimConnect referential and the x, y, z of p.
func (p Point) referential() (types.SIMCONNECT_POSITION_REFERENTIAL, types.DWORD, types.SIMCONNECT_DATA_XYZ) {
	switch p.Frame {
	case Object:
		return types.SIMCONNECT_POSITION_REFERENTIAL_SIMOBJECT, types.DWORD(p.Object), types.SIMCONNECT_DATA_XYZ{X: -p.Offset.Right, Y: p.Offset.Up, Z: p.Offset.Forward}
	case Eyepoint:
		return types.SIMCONNECT_POSITION_REFERENTIAL_EYEPOINT, 0, types.SIMCONNECT_DATA_XYZ{X: -p.Offset.Right, Y: p.Offset.Up, Z: p.Offset.Forward}
	}
	return types.SIMCONNECT_POSITION_REFERENTIAL_WORLD, 0, types.SIMCONNECT_DATA_XYZ{X: p.Lat, Y: p.Lon, Z: p.AltM}
}

// Pose is where the camera is, what it looks at and its field of view.
type Pose struct {
	Eye    Point
	Target Point
	// FovDeg is the horizontal field of view in degrees (0: DefaultFovDeg).
	FovDeg float64
}

// DefaultFovDeg is the field of view of a pose without one.
const DefaultFovDeg = 55.0

// Data is the pose as SimConnect places the camera: position, the point
// looked at, the field of view.
func (p Pose) Data() (types.SIMCONNECT_DATA_CAMERA, types.SIMCONNECT_CAMERA_DATA_MASK) {
	fov := p.FovDeg
	if fov <= 0 {
		fov = DefaultFovDeg
	}
	pr, pid, pos := p.Eye.referential()
	tr, tid, tgt := p.Target.referential()
	return types.SIMCONNECT_DATA_CAMERA{
		Position: pos, PositionReferential: pr, PositionReferentialObjectID: pid,
		TargetedPos: tgt, RotationReferential: tr, RotationReferentialObjectID: tid,
		Fov: fov * math.Pi / 180,
	}, types.SIMCONNECT_CAMERA_DATA_MASK_ALL_TARGETED
}

// lerpPoint is a between b and c at f (0..1) when both are in the same
// frame (and on the same object); otherwise c from f ≥ 0.5.
func lerpPoint(b, c Point, f float64) Point {
	if b.Frame != c.Frame || b.Frame != World && b.Object != c.Object {
		if f < 0.5 {
			return b
		}
		return c
	}
	l := func(x, y float64) float64 { return x + (y-x)*f }
	return Point{Frame: b.Frame, Object: b.Object,
		Lat: l(b.Lat, c.Lat), Lon: l(b.Lon, c.Lon), AltM: l(b.AltM, c.AltM),
		Offset: Offset{l(b.Offset.Right, c.Offset.Right), l(b.Offset.Up, c.Offset.Up), l(b.Offset.Forward, c.Offset.Forward)}}
}

// Lerp is the pose between a and b at f (0..1).
func Lerp(a, b Pose, f float64) Pose {
	fa, fb := a.FovDeg, b.FovDeg
	if fa <= 0 {
		fa = DefaultFovDeg
	}
	if fb <= 0 {
		fb = DefaultFovDeg
	}
	return Pose{Eye: lerpPoint(a.Eye, b.Eye, f), Target: lerpPoint(a.Target, b.Target, f), FovDeg: fa + (fb-fa)*f}
}
