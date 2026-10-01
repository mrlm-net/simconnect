//go:build windows
// +build windows

package main

import (
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/camera"
)

// The tower looking round the airfield: from the tower's eye, the camera
// swings slowly from one side of the airfield to the other and back, one
// long shot (no cut, no settle between the swings).
const (
	towerPanPeriod = 70 * time.Second // there and back
	towerPanSpan   = 110.0            // degrees, side to side
	towerPanFov    = 50.0
	towerPanLength = 30 * time.Minute // replayed when it ends
)

// towerPan is the swing: a fixed eye, the target on the ground at dist
// meters along a bearing that eases between centre ± span/2.
type towerPan struct {
	eye    camera.Point
	from   airport.LatLon // the eye's ground position
	ground float64        // the airfield's altitude, meters MSL
	centre float64        // bearing to the middle of the airfield
	dist   float64        // meters to the target
}

func (p towerPan) Length() time.Duration { return towerPanLength }
func (p towerPan) Name() string          { return "tower: looking round" }

func (p towerPan) PoseAt(t time.Duration) camera.Pose {
	// A sine: slow at the ends of a swing, as a head turns.
	phase := 2 * math.Pi * t.Seconds() / towerPanPeriod.Seconds()
	brg := p.centre + math.Sin(phase)*towerPanSpan/2
	lat, lon := calc.DisplaceByHeading(p.from.Lat, p.from.Lon, brg, p.dist)
	return camera.Pose{Eye: p.eye, Target: camera.At(lat, lon, p.ground), FovDeg: towerPanFov}
}

// lookAround is the tower at g's airport looking round its airfield: the
// swing centred on the runways' middle.
func (m *cameraMan) lookAround(g *airport.Graph) camera.Shot {
	eye := m.towerEye(g)
	l := g.Layout
	from := airport.LatLon{Lat: eye.Lat, Lon: eye.Lon}
	// The middle of the runways, else the airport reference point.
	mid := airport.LatLon{Lat: l.Latitude, Lon: l.Longitude}
	if n := len(l.Runways); n > 0 {
		mid = airport.LatLon{}
		for _, r := range l.Runways {
			mid.Lat += r.Center.Lat / float64(n)
			mid.Lon += r.Center.Lon / float64(n)
		}
	}
	centre := calc.BearingDegrees(from.Lat, from.Lon, mid.Lat, mid.Lon)
	dist := math.Min(math.Max(calc.HaversineMeters(from.Lat, from.Lon, mid.Lat, mid.Lon), 600), 2500)
	return towerPan{eye: eye, from: from, ground: l.Altitude, centre: centre, dist: dist}
}
