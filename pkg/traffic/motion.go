//go:build windows
// +build windows

package traffic

import (
	"math"
	"sort"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// MotionProfile describes how an aircraft moves on the ground when this
// package drives it by position injection instead of MSFS AI (#309).
type MotionProfile struct {
	// WheelbaseMeters is the distance from the nose gear to the main gear.
	// The nose gear follows the path; the main gear trails it and cuts
	// inside turns, so the heading eases into and out of every turn.
	WheelbaseMeters float64
	// RefAheadMeters is how far the sim's reference point (the position a
	// SimConnect write sets) lies ahead of the main gear.
	RefAheadMeters float64
	// CruiseKts is the speed on straight taxiway sections.
	CruiseKts float64
	// MinTurnKts is the lowest speed a tight turn slows down to.
	MinTurnKts float64
	// LateralAccel (m/s²) sets turn speeds: v = √(LateralAccel · radius).
	LateralAccel float64
	// Accel and Decel (m/s²) are the largest planned speed changes; Jerk
	// (m/s³) is how fast the acceleration itself may change, so every speed
	// change starts and ends softly.
	Accel, Decel, Jerk float64
	// SpanMeters and TailMeters (main gear to the tail end) outline the
	// airframe for clearance checks, such as the pushback swing past
	// neighbouring stands; zero uses the A320 figures.
	SpanMeters, TailMeters float64
}

// DefaultMotionProfile is tuned for an A320 family aircraft from live runs
// at LKPR (#309).
func DefaultMotionProfile() MotionProfile {
	return MotionProfile{
		WheelbaseMeters: 12.6,
		RefAheadMeters:  1.0,
		CruiseKts:       TaxiSpeedKts,
		MinTurnKts:      3,
		LateralAccel:    0.6,
		Accel:           0.45, // live: 0.35 pulled away a little slowly
		Decel:           0.5,
		Jerk:            0.2,
		SpanMeters:      35.8,
		TailMeters:      20.5,
	}
}

// GroundPath is a route for injected ground movement: the route points with
// their corners rounded, and the allowed speed along it.
type GroundPath struct {
	pts     []airport.LatLon
	cum     []float64             // metres from the start
	limit   []float64             // m/s
	decelAt func(float64) float64 // planned braking (m/s²) at a distance
}

// NewGroundPath rounds the corners of points (GroundPathSmoothingPasses
// Chaikin passes) and plans the allowed speed along it for profile p: the
// cruise speed, slower in turns, with braking planned ahead of each turn.
// Stops are not part of the plan; a GroundMover brakes onto the end of the
// path or a hold point by itself.
func NewGroundPath(points []airport.LatLon, p MotionProfile) (*GroundPath, error) {
	return newGroundPath(points, p, firmZone{})
}

// firmZone is the start of a path (a runway) where braking and cornering are
// planned firmer than taxiing.
type firmZone struct{ meters, decel, lateral float64 }

func newGroundPath(points []airport.LatLon, p MotionProfile, firm firmZone) (*GroundPath, error) {
	decelAt := func(d float64) float64 {
		if d < firm.meters {
			return math.Max(firm.decel, p.Decel)
		}
		return p.Decel
	}
	lateralAt := func(d float64) float64 {
		if d < firm.meters {
			return math.Max(firm.lateral, p.LateralAccel)
		}
		return p.LateralAccel
	}
	pts := make([]airport.LatLon, 0, len(points))
	for _, q := range points {
		if len(pts) == 0 || localDist(pts[len(pts)-1], q) > 0.01 {
			pts = append(pts, q)
		}
	}
	if len(pts) < 2 {
		return nil, ErrPathTooShort
	}
	// Merge route points a few meters apart first (taxiway nodes near
	// junctions often are): corner rounding reaches at most half a segment, so
	// short segments turn corners into pivots on the spot.
	pts = chaikin(cornerZones(mergeClose(pts, MergeMeters), CornerMeters), GroundPathSmoothingPasses)
	g := &GroundPath{pts: pts, cum: make([]float64, len(pts))}
	for i := 1; i < len(pts); i++ {
		g.cum[i] = g.cum[i-1] + localDist(pts[i-1], pts[i])
	}
	g.decelAt = decelAt
	g.limit = speedLimits(g.pts, g.cum, p, decelAt, lateralAt)
	return g, nil
}

// LimitEnd caps the speed over the last meters of the path at kts (a stand
// entry), with braking planned down to it.
func (g *GroundPath) LimitEnd(meters, kts float64) {
	g.limitRange(g.Length()-meters, g.Length(), kts, g.decelAt)
}

// LimitRange caps the speed between distances from and to at kts, with
// braking at decel (m/s²) planned before it.
func (g *GroundPath) LimitRange(from, to, kts, decel float64) {
	g.limitRange(from, to, kts, func(float64) float64 { return decel })
}

func (g *GroundPath) limitRange(from, to, kts float64, decelAt func(float64) float64) {
	vmax := kts * ktsToMS
	for i, d := range g.cum {
		if d >= from && d <= to {
			g.limit[i] = math.Min(g.limit[i], vmax)
		}
	}
	for i := len(g.limit) - 2; i >= 0; i-- {
		g.limit[i] = math.Min(g.limit[i], math.Sqrt(g.limit[i+1]*g.limit[i+1]+2*decelAt(g.cum[i])*(g.cum[i+1]-g.cum[i])))
	}
}

// Length is the path length in metres.
func (g *GroundPath) Length() float64 { return g.cum[len(g.cum)-1] }

// Points returns the smoothed path.
func (g *GroundPath) Points() []airport.LatLon { return g.pts }

// segment returns the index i of the segment [i-1, i] containing distance s
// and the fraction along it.
func (g *GroundPath) segment(s float64) (int, float64) {
	i := sort.SearchFloat64s(g.cum, s)
	switch {
	case i <= 0:
		return 1, 0
	case i >= len(g.cum):
		return len(g.cum) - 1, 1
	}
	return i, (s - g.cum[i-1]) / math.Max(g.cum[i]-g.cum[i-1], 1e-9)
}

// PointAt returns the point at distance s along the path (clamped).
func (g *GroundPath) PointAt(s float64) airport.LatLon {
	i, f := g.segment(s)
	a, b := g.pts[i-1], g.pts[i]
	return airport.LatLon{Lat: a.Lat + (b.Lat-a.Lat)*f, Lon: a.Lon + (b.Lon-a.Lon)*f}
}

// SpeedLimitKts returns the planned speed at distance s.
func (g *GroundPath) SpeedLimitKts(s float64) float64 { return g.limitAt(s) / ktsToMS }

func (g *GroundPath) limitAt(s float64) float64 {
	i, f := g.segment(s)
	return g.limit[i-1] + (g.limit[i]-g.limit[i-1])*f
}

// GroundPose is where a GroundMover puts the aircraft.
type GroundPose struct {
	// Position is the sim reference point; Heading is true degrees.
	Position airport.LatLon
	Heading  float64
	// GroundSpeedKts is the current speed.
	GroundSpeedKts float64
	// Distance is how far the nose gear is along the path, in metres.
	Distance float64
	// Stopped is set while the aircraft stands still; Arrived once it has
	// stopped at the end of the path.
	Stopped, Arrived bool
}

// GroundMover moves an aircraft along a GroundPath: it accelerates to the
// planned speed, slows into turns and brakes onto the end of the path or a
// hold point, with limited jerk. The nose gear follows the path and the main
// gear trails it at the wheelbase, which gives realistic turns. It is pure
// computation; an Injector puts the poses into the sim.
type GroundMover struct {
	path       *GroundPath
	p          MotionProfile
	s, v, a    float64
	gear       airport.LatLon // the trailing gear: main gear, or nose gear in reverse
	reverse    bool           // pushed back: the main gear follows the path
	hold       float64        // stop point on the path; path length when none
	shortStart bool           // started from a standstill within StopApproachMeters of the stop
	slowAt     float64        // SlowAt point and speed; slowKts 0 when none
	slowKts    float64
	pose       GroundPose

	placed   bool
	placedAt float64 // nose distance of the current pose
}

// NewGroundMover starts at rest with the main gear at the start of path and
// the nose gear one wheelbase along it (or at the end of a shorter path).
func NewGroundMover(path *GroundPath, p MotionProfile) *GroundMover {
	m := &GroundMover{path: path, p: p, gear: path.PointAt(0), hold: path.Length()}
	m.s = math.Min(p.WheelbaseMeters, path.Length())
	m.place()
	return m
}

// NewPushbackMover pushes an aircraft back: path runs from its main gear
// backwards (tail first) and the main gear follows it, with the fuselage
// along the path as the tug swings the nose (see placeReverse). heading is
// the stand heading at the start. Speeds come from p (use a pushback
// CruiseKts).
func NewPushbackMover(path *GroundPath, p MotionProfile, heading float64) *GroundMover {
	m := &GroundMover{path: path, p: p, hold: path.Length(), reverse: true}
	gear := path.PointAt(0)
	m.gear = offsetHeading(gear, heading, p.WheelbaseMeters) // the nose, trailing
	m.place()
	return m
}

// NewGroundMoverFrom takes over an aircraft that is already moving: the
// nose gear at the start of path, the main gear one wheelbase behind it
// along heading (true degrees), at speedKts.
func NewGroundMoverFrom(path *GroundPath, p MotionProfile, heading, speedKts float64) *GroundMover {
	m := &GroundMover{path: path, p: p, hold: path.Length(), v: math.Max(0, speedKts) * ktsToMS}
	nose := path.PointAt(0)
	kx := metersPerDegree * math.Cos(nose.Lat*math.Pi/180)
	h := heading * math.Pi / 180
	m.gear = airport.LatLon{
		Lat: nose.Lat - math.Cos(h)*p.WheelbaseMeters/metersPerDegree,
		Lon: nose.Lon - math.Sin(h)*p.WheelbaseMeters/kx,
	}
	m.place()
	return m
}

// NoseGear returns where the nose gear of an aircraft is whose sim
// reference point is at ref, heading true degrees: profile p's wheelbase
// minus RefAheadMeters ahead.
func NoseGear(ref airport.LatLon, heading float64, p MotionProfile) airport.LatLon {
	d := p.WheelbaseMeters - p.RefAheadMeters
	kx := metersPerDegree * math.Cos(ref.Lat*math.Pi/180)
	h := heading * math.Pi / 180
	return airport.LatLon{Lat: ref.Lat + math.Cos(h)*d/metersPerDegree, Lon: ref.Lon + math.Sin(h)*d/kx}
}

// Path returns the path being followed.
func (m *GroundMover) Path() *GroundPath { return m.path }

// Pose returns the current pose without moving.
func (m *GroundMover) Pose() GroundPose { return m.pose }

// HoldAt makes the aircraft stop with its nose gear at distance d along the
// path (a hold-short line, traffic ahead, a stop bar). It brakes as hard as
// Decel·1.5 allows; a hold behind the aircraft stops it where it is.
func (m *GroundMover) HoldAt(d float64) { m.hold = math.Max(m.s, math.Min(d, m.path.Length())) }

// ClearHold lets the aircraft continue to the end of the path.
func (m *GroundMover) ClearHold() { m.hold = m.path.Length() }

// SlowAt makes the aircraft slow down to kts with its nose gear at distance
// d along the path and carry on without stopping (a rolling clearance).
func (m *GroundMover) SlowAt(d, kts float64) { m.slowAt, m.slowKts = d, kts }

// Step advances the motion by dt seconds and returns the new pose. Long
// steps are split so a stalled caller does not jump.
func (m *GroundMover) Step(dt float64) GroundPose {
	for dt > 0 {
		h := math.Min(dt, 0.1)
		m.step(h)
		dt -= h
	}
	m.place()
	return m.pose
}

func (m *GroundMover) step(dt float64) {
	p := m.p
	// Chase the planned speed here and a little ahead (the nose must already
	// be slow entering a turn) and the braking curve to the stop point,
	// reaching it in about SpeedResponseSeconds.
	rem := m.hold - m.s
	// Look ahead by what the response lag covers (at least
	// TurnLookaheadMeters): chasing the plan at the aircraft's own position
	// runs about SpeedResponseSeconds late on every slow-down.
	ahead := math.Max(TurnLookaheadMeters, m.v*SpeedResponseSeconds)
	target := math.Min(m.path.limitAt(m.s), m.path.limitAt(m.s+ahead))
	target = math.Min(target, math.Sqrt(2*p.Decel*math.Max(0, rem)))
	if m.slowKts > 0 && m.s < m.slowAt {
		v0 := m.slowKts * ktsToMS
		target = math.Min(target, math.Sqrt(v0*v0+2*p.Decel*(m.slowAt-m.s)))
	}
	want := (target - m.v) / SpeedResponseSeconds
	// Brake exactly onto the stop point. A move that starts from a
	// standstill already this close to it first pulls away until it meets
	// the braking curve: the cap of -v²/2r is 0 at a standstill and the
	// aircraft would never start (short hops, a tug backing off).
	if rem > 0.05 && rem < StopApproachMeters {
		if m.v == 0 {
			m.shortStart = true
		}
		if m.shortStart && m.v*m.v >= 0.8*2*p.Decel*rem {
			m.shortStart = false
		}
		if !m.shortStart {
			want = math.Min(want, -m.v*m.v/(2*rem)) // brake exactly onto the stop point
		}
	}
	if r := m.slowAt - m.s; m.slowKts > 0 && r > 0.05 && r < StopApproachMeters {
		v0 := m.slowKts * ktsToMS
		if m.v > v0 {
			want = math.Min(want, (v0*v0-m.v*m.v)/(2*r)) // brake exactly onto the slow point
		}
	}
	want = math.Max(-1.5*p.Decel, math.Min(p.Accel, want))
	if want > m.a {
		m.a = math.Min(want, m.a+p.Jerk*dt)
	} else {
		m.a = math.Max(want, m.a-p.Jerk*dt)
	}
	m.v = math.Max(0, m.v+m.a*dt)
	// Around a SlowAt point the aircraft keeps rolling at its slow speed
	// (braking momentum would otherwise stop it) unless it must hold.
	if v0 := m.slowKts * ktsToMS; v0 > 0 && m.v < v0 && math.Abs(m.slowAt-m.s) < 10 && m.hold-m.s > 1 {
		m.v, m.a = v0, math.Max(m.a, 0)
	}
	step := m.v * dt
	// Braking stops a hair short of the point: cover the last centimetres at
	// a creep instead of snapping onto it (the speed stays as braked).
	if rem := m.hold - m.s; rem > 0 && rem < 0.3 && m.v < finalCreep {
		step = math.Max(step, finalCreep*dt)
	}
	m.s = math.Min(m.s+step, m.hold)
	if m.s >= m.hold {
		m.s, m.v, m.a = m.hold, 0, 0
	}
}

// place moves the main gear after the nose gear like a towed trailer and
// derives the pose. The geometry is done in local metres around the nose:
// repeated bearing/displacement round trips are not exact and drift the gear
// sideways every frame (seen live as sliding).
func (m *GroundMover) place() {
	if m.placed && m.s == m.placedAt {
		m.pose.GroundSpeedKts, m.pose.Stopped = m.v/ktsToMS, m.v == 0
		m.pose.Arrived = m.v == 0 && m.s >= m.path.Length()-0.05
		return // not moved: recomputing would only add rounding noise
	}
	m.placed, m.placedAt = true, m.s
	if m.reverse {
		m.placeReverse()
		return
	}
	nose := m.path.PointAt(m.s)
	kx := metersPerDegree * math.Cos(nose.Lat*math.Pi/180)
	dx, dy := (m.gear.Lon-nose.Lon)*kx, (m.gear.Lat-nose.Lat)*metersPerDegree // nose → gear
	wb := m.p.WheelbaseMeters
	if d := math.Hypot(dx, dy); d > 1e-6 && wb > 0 {
		dx, dy = dx/d*wb, dy/d*wb
		m.gear = airport.LatLon{Lat: nose.Lat + dy/metersPerDegree, Lon: nose.Lon + dx/kx}
	}
	// Forward the path point is the nose gear and the main gear trails; the
	// reference point lies RefAheadMeters ahead of the main gear. Pushed back
	// (reverse) the path point is the main gear and the nose trails, steered
	// by the tug.
	f := 1.0
	if wb > 0 {
		f = 1 - m.p.RefAheadMeters/wb
		if m.reverse {
			f = m.p.RefAheadMeters / wb
		}
	}
	hdg := m.pose.Heading
	if dx != 0 || dy != 0 {
		if m.reverse {
			hdg = math.Mod(math.Atan2(dx, dy)*180/math.Pi+360, 360)
		} else {
			hdg = math.Mod(math.Atan2(-dx, -dy)*180/math.Pi+360, 360)
		}
	}
	m.pose = GroundPose{
		Position:       airport.LatLon{Lat: nose.Lat + dy*f/metersPerDegree, Lon: nose.Lon + dx*f/kx},
		Heading:        hdg,
		GroundSpeedKts: m.v / ktsToMS,
		Distance:       m.s,
		Stopped:        m.v == 0,
		Arrived:        m.v == 0 && m.s >= m.path.Length()-0.05,
	}
}

const (
	metersPerDegree = 111319.49
	ktsToMS         = 0.514444
	// finalCreep (m/s) covers the last centimetres onto a stop point.
	finalCreep = 0.05
)

// localDist is the flat-earth distance in metres; exact enough on an airport.
func localDist(a, b airport.LatLon) float64 {
	kx := metersPerDegree * math.Cos((a.Lat+b.Lat)/2*math.Pi/180)
	return math.Hypot((b.Lon-a.Lon)*kx, (b.Lat-a.Lat)*metersPerDegree)
}

// cornerZones adds a point d metres (at most half the segment) either side
// of every interior point, so corner rounding stays within d of the corner
// instead of scaling with the segment length (a long straight would
// otherwise turn a taxiway corner into a 100 m arc across the grass).
func cornerZones(p []airport.LatLon, d float64) []airport.LatLon {
	if len(p) < 3 {
		return p
	}
	lerp := func(a, b airport.LatLon, f float64) airport.LatLon {
		return airport.LatLon{Lat: a.Lat + (b.Lat-a.Lat)*f, Lon: a.Lon + (b.Lon-a.Lon)*f}
	}
	out := []airport.LatLon{p[0]}
	for i := 0; i+1 < len(p); i++ {
		a, b := p[i], p[i+1]
		l := localDist(a, b)
		c := math.Min(d, l/2) / l
		if i > 0 && c < 0.5 {
			out = append(out, lerp(a, b, c))
		}
		if i+2 < len(p) && c < 0.5 {
			out = append(out, lerp(a, b, 1-c))
		}
		out = append(out, b)
	}
	return out
}

// chaikin rounds the corners of a polyline: each pass replaces every segment
// by its ¼ and ¾ points, keeping the end points.
func chaikin(p []airport.LatLon, passes int) []airport.LatLon {
	for ; passes > 0 && len(p) > 2; passes-- {
		out := make([]airport.LatLon, 0, 2*len(p))
		out = append(out, p[0])
		for i := 0; i+1 < len(p); i++ {
			a, b := p[i], p[i+1]
			out = append(out,
				airport.LatLon{Lat: a.Lat*0.75 + b.Lat*0.25, Lon: a.Lon*0.75 + b.Lon*0.25},
				airport.LatLon{Lat: a.Lat*0.25 + b.Lat*0.75, Lon: a.Lon*0.25 + b.Lon*0.75})
		}
		p = append(out, p[len(p)-1])
	}
	return p
}

// speedLimits plans the allowed speed (m/s) at each point: cruise, slower
// where the path curves, with Decel braking planned ahead of each turn.
func speedLimits(pts []airport.LatLon, cum []float64, p MotionProfile, decelAt, lateralAt func(float64) float64) []float64 {
	n := len(pts)
	cruise, minTurn := p.CruiseKts*ktsToMS, math.Min(p.MinTurnKts, p.CruiseKts)*ktsToMS
	v := make([]float64, n)
	for i := range v {
		v[i] = cruise
	}
	// Turn radius from the heading change over ±TurnWindowMeters.
	j0, j1 := 0, 0
	for i := 0; i < n; i++ {
		for j0 < i && cum[i]-cum[j0] > TurnWindowMeters {
			j0++
		}
		for j1 < n-1 && cum[j1]-cum[i] < TurnWindowMeters {
			j1++
		}
		if j0 == i || j1 == i {
			continue
		}
		d := math.Abs(headingDiff(localBearing(pts[j0], pts[i]), localBearing(pts[i], pts[j1]))) * math.Pi / 180
		if d > 0.01 {
			r := (cum[j1] - cum[j0]) / d
			v[i] = math.Min(v[i], math.Max(minTurn, math.Sqrt(lateralAt(cum[i])*r)))
		}
	}
	for i := n - 2; i >= 0; i-- {
		v[i] = math.Min(v[i], math.Sqrt(v[i+1]*v[i+1]+2*decelAt(cum[i])*(cum[i+1]-cum[i])))
	}
	return v
}

func localBearing(a, b airport.LatLon) float64 {
	kx := metersPerDegree * math.Cos((a.Lat+b.Lat)/2*math.Pi/180)
	return math.Mod(math.Atan2((b.Lon-a.Lon)*kx, (b.Lat-a.Lat)*metersPerDegree)*180/math.Pi+360, 360)
}

// headingDiff is the signed heading change from a to b, -180–180°.
func headingDiff(a, b float64) float64 { return math.Mod(b-a+540, 360) - 180 }

// SetProfile changes the speed behaviour (accelerations, jerk) from now on,
// e.g. from runway braking to taxiing once clear of the runway. The
// geometry (wheelbase, reference point) should stay the same.
func (m *GroundMover) SetProfile(p MotionProfile) { m.p = p }

// DistanceTo projects p onto the path and returns the distance along the
// path to the nearest point and how far p lies from it, in meters.
func (g *GroundPath) DistanceTo(p airport.LatLon) (along, off float64) {
	off = math.Inf(1)
	kx := metersPerDegree * math.Cos(p.Lat*math.Pi/180)
	for i := 1; i < len(g.pts); i++ {
		a, b := g.pts[i-1], g.pts[i]
		ax, ay := (a.Lon-p.Lon)*kx, (a.Lat-p.Lat)*metersPerDegree
		bx, by := (b.Lon-p.Lon)*kx, (b.Lat-p.Lat)*metersPerDegree
		dx, dy := bx-ax, by-ay
		f := 0.0
		if l2 := dx*dx + dy*dy; l2 > 0 {
			f = math.Max(0, math.Min(1, -(ax*dx+ay*dy)/l2))
		}
		if d := math.Hypot(ax+dx*f, ay+dy*f); d < off {
			off, along = d, g.cum[i-1]+f*(g.cum[i]-g.cum[i-1])
		}
	}
	return along, off
}

// mergeClose drops points closer than d to the last kept point, keeping the
// first and the last point.
func mergeClose(p []airport.LatLon, d float64) []airport.LatLon {
	if len(p) < 3 {
		return p
	}
	out := []airport.LatLon{p[0]}
	for _, q := range p[1 : len(p)-1] {
		if localDist(out[len(out)-1], q) >= d {
			out = append(out, q)
		}
	}
	last := p[len(p)-1]
	if len(out) > 1 && localDist(out[len(out)-1], last) < d {
		out = out[:len(out)-1] // keep the end exactly, drop the close one before it
	}
	return append(out, last)
}

// placeReverse places a pushed-back aircraft. The main gear cannot slide
// sideways: it rolls along the fuselage axis while the tug swings the nose,
// so the fuselage lies along the main gear's path and the aircraft points
// against the direction of travel. The main gear traces the path's arcs and
// the nose swings wide the other way (#304, GSX-style pushbacks).
func (m *GroundMover) placeReverse() {
	const d = 1.0 // meters either side for the path direction
	a := m.path.PointAt(math.Max(0, m.s-d))
	b := m.path.PointAt(math.Min(m.path.Length(), m.s+d))
	gear := m.path.PointAt(m.s)
	hdg := m.pose.Heading
	if localDist(a, b) > 1e-6 {
		hdg = localBearing(b, a) // the nose points back along the path
	}
	m.pose = GroundPose{
		Position:       offsetHeading(gear, hdg, m.p.RefAheadMeters),
		Heading:        hdg,
		GroundSpeedKts: m.v / ktsToMS,
		Distance:       m.s,
		Stopped:        m.v == 0,
		Arrived:        m.v == 0 && m.s >= m.path.Length()-0.05,
	}
}

// NewArcPath is a GroundPath whose corners are circular arcs of radius
// (smaller where the segments are too short): for pushbacks, where the
// fuselage follows the path directly and a tug swings the tail through a
// steady arc.
func NewArcPath(points []airport.LatLon, p MotionProfile, radius float64) (*GroundPath, error) {
	pts := mergeClose(points, 1)
	if len(pts) < 2 {
		return nil, ErrPathTooShort
	}
	return newPlainPath(fillet(pts, radius), p), nil
}

// NewSmoothPath is a GroundPath along points that are already smooth (e.g.
// sampled arcs from NewArcPath or a Dubins path): no merging or rounding,
// which on metre-spaced samples would leave zero-length kinks.
func NewSmoothPath(points []airport.LatLon, p MotionProfile) (*GroundPath, error) {
	pts := mergeClose(points, 0.05)
	if len(pts) < 2 {
		return nil, ErrPathTooShort
	}
	return newPlainPath(pts, p), nil
}

func newPlainPath(pts []airport.LatLon, p MotionProfile) *GroundPath {
	g := &GroundPath{pts: pts}
	g.cum = make([]float64, len(g.pts))
	for i := 1; i < len(g.pts); i++ {
		g.cum[i] = g.cum[i-1] + localDist(g.pts[i-1], g.pts[i])
	}
	decelAt := func(float64) float64 { return p.Decel }
	g.decelAt = decelAt
	g.limit = speedLimits(g.pts, g.cum, p, decelAt, func(float64) float64 { return p.LateralAccel })
	return g
}

// fillet replaces each corner of a polyline by a circular arc of radius r,
// tangent to both segments (the tangent length is limited to half of each
// segment), sampled every 1 m.
func fillet(p []airport.LatLon, r float64) []airport.LatLon {
	if len(p) < 3 {
		return p
	}
	o := p[0]
	kx := metersPerDegree * math.Cos(o.Lat*math.Pi/180)
	xy := func(q airport.LatLon) (float64, float64) {
		return (q.Lon - o.Lon) * kx, (q.Lat - o.Lat) * metersPerDegree
	}
	ll := func(x, y float64) airport.LatLon {
		return airport.LatLon{Lat: o.Lat + y/metersPerDegree, Lon: o.Lon + x/kx}
	}
	out := []airport.LatLon{p[0]}
	for i := 1; i+1 < len(p); i++ {
		ax, ay := xy(p[i-1])
		vx, vy := xy(p[i])
		bx, by := xy(p[i+1])
		l1, l2 := math.Hypot(vx-ax, vy-ay), math.Hypot(bx-vx, by-vy)
		ux, uy := (vx-ax)/l1, (vy-ay)/l1             // in
		wx, wy := (bx-vx)/l2, (by-vy)/l2             // out
		turn := math.Atan2(ux*wy-uy*wx, ux*wx+uy*wy) // signed, left positive
		if math.Abs(turn) < 0.5*math.Pi/180 {
			out = append(out, p[i])
			continue
		}
		t := math.Min(r*math.Tan(math.Abs(turn)/2), math.Min(l1, l2)/2)
		rr := t / math.Tan(math.Abs(turn)/2)
		sx, sy := vx-ux*t, vy-uy*t // arc start
		side := math.Copysign(1, turn)
		cx, cy := sx-uy*rr*side, sy+ux*rr*side // centre, left of travel for a left turn
		a0 := math.Atan2(sy-cy, sx-cx)
		n := max(2, int(rr*math.Abs(turn)))
		for k := 0; k <= n; k++ {
			a := a0 + turn*float64(k)/float64(n)
			out = append(out, ll(cx+rr*math.Cos(a), cy+rr*math.Sin(a)))
		}
	}
	return append(out, p[len(p)-1])
}
