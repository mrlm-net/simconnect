package world

import (
	"math"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/camera"
)

// The tower looking round the airfield, from a fixed eye at the tower: by
// itself, a slow swing from one side of the airfield to the other and back;
// or where the user turns it (Look: left and right, up and down, zoom). One
// long shot either way: no cut, no settle as it turns.
const (
	towerPanPeriod = 70 * time.Second // there and back
	towerPanSpan   = 110.0            // degrees, side to side
	towerPanFov    = 50.0
	towerPanLength = 30 * time.Minute // replayed when it ends
	// towerEase: how quickly a turn by the user is followed (seconds).
	towerEase = 0.35
)

// towerLook is the tower camera's aim, shared by the shot (which reads it
// every frame) and the user (who turns it).
type towerLook struct {
	mu   sync.Mutex
	auto bool    // swinging by itself
	yaw  float64 // degrees true, where the user wants it
	tilt float64 // degrees, negative down
	fov  float64
	// cur is where it looks now, eased towards the wanted aim.
	curYaw, curTilt, curFov float64
	last                    time.Time
	set                     bool // the aim has been initialised
}

// turn moves the wanted aim by the given steps (degrees) and turns the
// swing on or off (nil: as it is). From the swing to the user's hands, the
// aim starts where the swing is.
func (k *towerLook) turn(dyaw, dtilt, dfov float64, auto *bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if auto != nil {
		k.auto = *auto
	}
	if dyaw != 0 || dtilt != 0 || dfov != 0 {
		k.auto = false
	}
	k.yaw = math.Mod(k.yaw+dyaw, 360)
	if k.yaw < 0 {
		k.yaw += 360 // any turn, more than a whole one back too (#69)
	}
	k.tilt = math.Max(-60, math.Min(20, k.tilt+dtilt))
	k.fov = math.Max(8, math.Min(90, k.fov+dfov))
}

// towerPan is the shot: the eye at the tower, the aim from look.
type towerPan struct {
	eye    camera.Point
	from   airport.LatLon // the eye's ground position
	ground float64        // the airfield's altitude, meters MSL
	centre float64        // bearing to the middle of the airfield
	dist   float64        // meters to the middle of the airfield
	look   *towerLook
}

func (p towerPan) Length() time.Duration { return towerPanLength }
func (p towerPan) Name() string          { return "tower: looking round" }

func (p towerPan) PoseAt(t time.Duration) camera.Pose {
	k := p.look
	k.mu.Lock()
	defer k.mu.Unlock()
	if !k.set {
		// Level with the middle of the airfield, looking down at it.
		k.yaw, k.tilt, k.fov = p.centre, -math.Atan2(p.eye.AltM-p.ground, p.dist)*180/math.Pi, towerPanFov
		k.curYaw, k.curTilt, k.curFov, k.set = k.yaw, k.tilt, k.fov, true
	}
	if k.auto {
		// A sine: slow at the ends of a swing, as a head turns.
		phase := 2 * math.Pi * t.Seconds() / towerPanPeriod.Seconds()
		k.yaw = math.Mod(p.centre+math.Sin(phase)*towerPanSpan/2+360, 360)
	}
	now := time.Now()
	f := 1.0
	if !k.last.IsZero() {
		f = 1 - math.Exp(-now.Sub(k.last).Seconds()/towerEase)
	}
	k.last = now
	k.curYaw = math.Mod(k.curYaw+headingDiff(k.curYaw, k.yaw)*f+360, 360)
	k.curTilt += (k.tilt - k.curTilt) * f
	k.curFov += (k.fov - k.curFov) * f
	// The aim: a point 1 km along the bearing, at the tilt.
	const reach = 1000.0
	lat, lon := calc.DisplaceByHeading(p.from.Lat, p.from.Lon, k.curYaw, reach)
	alt := p.eye.AltM + reach*math.Tan(k.curTilt*math.Pi/180)
	return camera.Pose{Eye: p.eye, Target: camera.At(lat, lon, alt), FovDeg: k.curFov}
}

// lookAround is the tower at g's airport looking round its airfield,
// centred on the runways' middle, aimed by m.look.
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
	return towerPan{eye: eye, from: from, ground: l.Altitude, centre: centre, dist: dist, look: &m.look}
}
