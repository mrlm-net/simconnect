package traffic

import (
	"math"
	"slices"
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
		LateralAccel:    1.3,
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
	plan    []speedKnot           // the speed along it: finer than pts (speedLimits)
	decelAt func(float64) float64 // planned braking (m/s²) at a distance
	jerk    float64               // the profile's, for easing off onto a stand entry
}

// speedKnot is the planned speed v (m/s) at metres along a path.
type speedKnot struct{ at, v float64 }

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
		return plannedDecel(p)
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
	g.decelAt, g.jerk = decelAt, p.Jerk
	g.plan = speedLimits(g.pts, g.cum, p, lateralAt)
	g.capTurnRateAligned(p.WheelbaseMeters)
	g.brakeAhead(decelAt)
	return g, nil
}

// LimitEnd caps the speed over the last meters of the path at kts (a stand
// entry), with braking planned down to it.
func (g *GroundPath) LimitEnd(meters, kts float64) {
	from := g.Length() - meters
	g.limitRange(from, g.Length(), kts, g.decelAt)
	g.easeInto(from, kts*ktsToMS)
}

// easeInto has the braking down to v (m/s) at s eased off at the path's
// jerk before it, as a crew lets off the brakes, so the speed is down to
// v by s (#1651). Braked at full decel right up to it, the mover, which
// eases off at that jerk, still met the stand entry over its speed.
func (g *GroundPath) easeInto(s, v float64) {
	if g.jerk <= 0 || v <= 0 {
		return
	}
	full := g.decelAt(s) / g.jerk
	for i := g.knotFrom(s) - 1; i >= 0; i-- {
		// The time to it: covered at v plus what the eased-off braking adds
		// (x = v·t + jerk·t³/6), a few Newton steps from x/v.
		x := s - g.plan[i].at
		t := x / v
		for range 4 {
			t -= (v*t + g.jerk*t*t*t/6 - x) / (v + g.jerk*t*t/2)
		}
		if t >= full {
			break
		}
		g.plan[i].v = math.Min(g.plan[i].v, v+g.jerk*t*t/2)
	}
	g.brakeAhead(g.decelAt)
}

// LimitRange caps the speed between distances from and to at kts, with
// braking at decel (m/s²) planned before it.
func (g *GroundPath) LimitRange(from, to, kts, decel float64) {
	g.limitRange(from, to, kts, func(float64) float64 { return decel })
}

func (g *GroundPath) limitRange(from, to, kts float64, decelAt func(float64) float64) {
	vmax := kts * ktsToMS
	// Knots at both ends of the range: capped only at the knots inside it,
	// the speed between the last one before it and the first in it was
	// still over the cap where the range begins (#1651).
	g.knotAt(from)
	g.knotAt(to)
	for i, k := range g.plan {
		if k.at >= from && k.at <= to {
			g.plan[i].v = math.Min(k.v, vmax)
		}
	}
	g.brakeAhead(decelAt)
}

// limitTurnOff caps the speed at kts, with the path's own braking planned
// before it, wherever between from and to the path heads SharpTurnAngleDeg
// or more off heading (#1586): a turn off the runway through a right angle
// is taken at the exit speed whatever its radius. A high-speed exit that
// sweeps round onto a cross taxiway is otherwise held to the lateral limit
// only, which on a wide curve is the taxi speed, so the speed through the
// turn followed the aircraft's taxi speed draw (a B77W at 15.3 kt).
func (g *GroundPath) limitTurnOff(heading, from, to, kts float64) {
	vmax := kts * ktsToMS
	for i := 1; i < len(g.pts); i++ {
		if g.cum[i] < from || g.cum[i-1] > to {
			continue
		}
		if math.Abs(headingDiff(heading, localBearing(g.pts[i-1], g.pts[i]))) >= SharpTurnAngleDeg {
			for k := g.knotFrom(g.cum[i-1]); k < len(g.plan) && g.plan[k].at <= g.cum[i]; k++ {
				g.plan[k].v = math.Min(g.plan[k].v, vmax)
			}
		}
	}
	g.brakeAhead(g.decelAt)
}

// brakeAhead plans braking at decelAt before every lower limit.
func (g *GroundPath) brakeAhead(decelAt func(float64) float64) {
	for i := len(g.plan) - 2; i >= 0; i-- {
		k, next := g.plan[i], g.plan[i+1]
		g.plan[i].v = math.Min(k.v, math.Sqrt(next.v*next.v+2*decelAt(k.at)*(next.at-k.at)))
	}
}

// knotAt puts a knot at s, on the path, where the plan has none, at the
// speed planned there.
func (g *GroundPath) knotAt(s float64) {
	s = math.Max(0, math.Min(s, g.Length()))
	i := g.knotFrom(s)
	if i < len(g.plan) && g.plan[i].at-s < 1e-6 || i > 0 && s-g.plan[i-1].at < 1e-6 {
		return
	}
	g.plan = slices.Insert(g.plan, i, speedKnot{s, g.limitAt(s)})
}

// shortStartCurve: within StopApproachMeters of its stop, a mover slower
// than this share of the braking curve (in v²) pulls away again first.
const shortStartCurve = 0.1

// knotFrom is the first knot of the plan at s metres or on.
func (g *GroundPath) knotFrom(s float64) int {
	return sort.Search(len(g.plan), func(i int) bool { return g.plan[i].at >= s })
}

// planSeconds is how long the path takes at its planned speeds.
func (g *GroundPath) planSeconds() float64 {
	t := 0.0
	for i := 1; i < len(g.plan); i++ {
		a, b := g.plan[i-1], g.plan[i]
		t += (b.at - a.at) / math.Max((a.v+b.v)/2, finalCreep)
	}
	return t
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
	i := g.knotFrom(s)
	switch {
	case i == 0:
		return g.plan[0].v
	case i == len(g.plan):
		return g.plan[i-1].v
	}
	a, b := g.plan[i-1], g.plan[i]
	return a.v + (b.v-a.v)*(s-a.at)/math.Max(b.at-a.at, 1e-9)
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
	shortStart bool           // started from a standstill (or near one) within StopApproachMeters of the stop
	trafficAt  float64        // stop behind traffic ahead (SetTrafficStop), when hasTraffic
	hasTraffic bool
	giveWay    bool    // the traffic stop gives way at a crossing: slow down early (SetGiveWayStop)
	slowAt     float64 // SlowAt point and speed; slowKts 0 when none
	slowKts    float64
	refSpeed   float64 // the reference point's speed over the nose gear's (place)
	pose       GroundPose

	// runOnTo: how far along the path it may run on braking for a traffic
	// stop too near to brake onto at trafficStopDecel (the body ahead, as
	// groundDrive found it); 0 none, it stops on the point.
	runOnTo float64

	placed   bool
	placedAt float64 // nose distance of the current pose
	// sincePlace: seconds stepped since the pose last moved (turnCapped).
	sincePlace float64
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
	// The plan took it lined up with the path: set off across it (a tug
	// turning away from the nose) it swings round first.
	if path.capTurnRate(p.WheelbaseMeters, m.gear, swingWheelbases*p.WheelbaseMeters) {
		path.brakeAhead(path.decelAt)
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

// SetTrafficStop makes the aircraft stop with its nose gear at distance d
// along the path behind traffic ahead, besides any HoldAt; it follows as d
// moves on. ClearTrafficStop lifts it.
func (m *GroundMover) SetTrafficStop(d float64) {
	m.trafficAt, m.hasTraffic, m.giveWay = d, true, false
	m.runOnTo = 0
}

// SetGiveWayStop is SetTrafficStop short of a crossing or merging route
// whose traffic goes first: known well ahead, the aircraft slows down on a
// gentler curve (GiveWayDecelFactor) and early, as a crew letting another
// pass does, rather than taxi on at speed and brake at the last moment
// (live, DLH1233 behind CSA999 at LKPR looked at the edge).
func (m *GroundMover) SetGiveWayStop(d float64) {
	m.trafficAt, m.hasTraffic, m.giveWay = d, true, true
	m.runOnTo = 0
}

// ClearTrafficStop lifts the traffic stop.
func (m *GroundMover) ClearTrafficStop() { m.hasTraffic = false }

// stop is where the aircraft must stop: the hold, or traffic before it.
func (m *GroundMover) stop() float64 {
	if m.hasTraffic && m.trafficAt < m.hold {
		return math.Max(m.s, m.trafficAt)
	}
	return m.hold
}

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
	stop := m.stop()
	rem := stop - m.s
	// Traffic stopping it (not a hold): it may brake firmly and, where the
	// stop turned up inside the braking distance, run on into the gap kept
	// behind the traffic rather than halt on the spot (it looked like a
	// glitch).
	traffic := m.hasTraffic && m.trafficAt < m.hold
	limit := stop
	if traffic {
		// Never behind it: a stop set behind where it is (a give-way
		// junction already passed) stops it where it is. Taken back to it,
		// the aircraft jumped back along its path and swung round its main
		// gear (live, DLH1707 62° and 3.5 m, again at each update of the
		// push it gave way to; CSA1682 94° and 23 m).
		limit = math.Max(m.s, math.Min(m.hold, m.trafficAt+TrafficOverrunMeters))
	}
	// Found too near to stop by there: it brakes as hard as a taxiing crew
	// would and rolls on as far as that takes, short of the body ahead,
	// rather than stand from 10 kt in one frame (#1640, TFFJ's backtrack).
	if traffic && m.runOnTo > 0 {
		limit = math.Max(m.s, math.Min(limit, m.runOnTo))
	}
	short := false
	if traffic && m.runOnTo > 0 && m.v > 0 {
		if to := math.Min(math.Min(m.hold, m.runOnTo), m.s+m.v*m.v/(2*trafficStopDecel)); to > limit {
			limit, short = to, true
		}
	}
	// Look ahead by what the response lag covers (at least
	// TurnLookaheadMeters): chasing the plan at the aircraft's own position
	// runs about SpeedResponseSeconds late on every slow-down.
	ahead := math.Max(TurnLookaheadMeters, m.v*SpeedResponseSeconds)
	target := math.Min(m.path.limitAt(m.s), m.path.limitAt(m.s+ahead))
	// Its turns as it will be placed, braked for ahead (#1664): the plan's
	// radius can miss a corner its trailing gear turns tighter on.
	target = math.Min(target, m.turnSpeedAhead(ahead+m.v*m.v/(2*p.Decel)))
	if traffic {
		// Traffic: its own firm braking below catches what this misses.
		target = math.Min(target, math.Sqrt(2*p.Decel*math.Max(0, rem)))
		if m.giveWay {
			target = math.Min(target, math.Sqrt(2*GiveWayDecelFactor*p.Decel*math.Max(0, rem)))
		}
	} else {
		// A hold: the stop as reachable from here. Still accelerating, the
		// aircraft first covers what it rolls while that winds down (at the
		// jerk) and the lag before it answers, then brakes (stopSpeed).
		windDown := 0.0
		if p.Jerk > 0 { // a profile without one winds down at once (#104: 0/0, NaN)
			windDown = math.Max(0, m.a) / p.Jerk
		}
		remEff := rem - m.v*(windDown+SpeedResponseSeconds/2)
		target = math.Min(target, stopSpeed(p.Decel, p.Jerk, remEff))
	}
	if m.slowKts > 0 && m.s < m.slowAt {
		v0 := m.slowKts * ktsToMS
		target = math.Min(target, math.Sqrt(v0*v0+2*p.Decel*(m.slowAt-m.s)))
	}
	want := (target - m.v) / SpeedResponseSeconds
	// Brake exactly onto the stop point. A move that starts from a
	// standstill already this close to it first pulls away until it meets
	// the braking curve: the cap of -v²/2r is 0 at a standstill and the
	// aircraft would never start (short hops, a tug backing off). So does
	// one come into it far slower than that curve, held down by a tight turn
	// just before (#1651: a tug 25 m short of the nose crept on at 0.02 kt
	// for ever, braking "onto" a stop it would never reach). Not one held
	// for traffic, nor pushed back: those stop where they are.
	if rem > 0.05 && rem < StopApproachMeters {
		if m.v == 0 || !traffic && !m.reverse && m.v < target && m.v*m.v < shortStartCurve*2*p.Decel*rem {
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
	firm, jerk := 1.5*p.Decel, p.Jerk
	if traffic && m.v*m.v > 2*p.Decel*math.Max(rem, 0.01) {
		firm, jerk = TrafficBrakeFactor*p.Decel, TrafficJerkFactor*p.Jerk // closer than a normal stop
		if m.runOnTo > 0 {
			firm = math.Min(firm, trafficStopDecel) // more only as needed, below
		}
		// Last resort: brake exactly onto the end of the gap, hard but
		// smoothly, rather than meet it at speed.
		if r := limit - m.s; r > 0.05 {
			need := m.v * m.v / (2 * r)
			want = math.Min(want, -need)
			firm = math.Max(firm, need)
		}
		if short {
			jerk = math.Max(jerk, trafficStopJerk)
		}
	}
	want = math.Max(-firm, math.Min(p.Accel, want))
	if want > m.a {
		m.a = math.Min(want, m.a+p.Jerk*dt)
	} else {
		m.a = math.Max(want, m.a-jerk*dt)
	}
	m.v = math.Max(0, m.v+m.a*dt)
	// Braking towards a stop it is on: standing, not creeping at a
	// thousandth of a knot for ever (live, HBGOS read 0.00004 kt, never
	// "stopped", so it neither arrived nor said why).
	if want <= 0 && m.v < minRollMS && stop-m.s < 0.3 {
		m.v, m.a = 0, 0
	}
	// Around a SlowAt point the aircraft keeps rolling at its slow speed
	// (braking momentum would otherwise stop it) unless it must hold.
	if v0 := m.slowKts * ktsToMS; v0 > 0 && m.v < v0 && math.Abs(m.slowAt-m.s) < 10 && stop-m.s > 1 {
		m.v, m.a = v0, math.Max(m.a, 0)
	}
	step := m.v * dt
	step = m.turnCapped(step, dt)
	// Braking stops a hair short of the point: cover the last centimetres at
	// a creep instead of snapping onto it (the speed stays as braked).
	if rem := stop - m.s; rem > 0 && rem < 0.3 && m.v < finalCreep {
		step = math.Max(step, finalCreep*dt)
	}
	m.s = math.Min(m.s+step, limit)
	if m.s >= limit || traffic && m.v == 0 {
		m.v, m.a = 0, 0
		if m.s >= limit {
			m.s = limit
		}
	}
}

// place moves the main gear after the nose gear like a towed trailer and
// derives the pose. The geometry is done in local metres around the nose:
// repeated bearing/displacement round trips are not exact and drift the gear
// sideways every frame (seen live as sliding).
// turnAheadMaxMeters: how far ahead turnSpeedAhead looks at most.
const turnAheadMaxMeters = 60.0

// turnSpeedAhead is the speed (m/s) from which the mover brakes, at its
// Decel, to every point within span metres ahead slow enough to turn
// there, as placed, at no more than plannedTurnDegPerSec: its trailing gear
// followed along the path from where it is. Forward only.
func (m *GroundMover) turnSpeedAhead(span float64) float64 {
	wb := m.p.WheelbaseMeters
	if m.reverse || !m.placed || wb <= 0 || m.p.Decel <= 0 {
		return math.Inf(1)
	}
	span = math.Min(span, turnAheadMaxMeters)
	most := plannedTurnDegPerSec * math.Pi / 180
	o := m.path.PointAt(m.s)
	kx := metersPerDegree * math.Cos(o.Lat*math.Pi/180)
	xy := func(q airport.LatLon) (float64, float64) {
		return (q.Lon - o.Lon) * kx, (q.Lat - o.Lat) * metersPerDegree
	}
	gx, gy := xy(m.gear)
	nx, ny := xy(o)
	ux, uy := nx-gx, ny-gy // gear to nose
	if d := math.Hypot(ux, uy); d > 1e-6 {
		ux, uy = ux/d, uy/d
	} else {
		return math.Inf(1)
	}
	best := math.Inf(1)
	for d := trailStepMeters; d <= span && m.s+d <= m.path.Length(); d += trailStepMeters {
		x, y := xy(m.path.PointAt(m.s + d))
		vx, vy := x-gx, y-gy
		l := math.Hypot(vx, vy)
		if l < 1e-6 {
			continue
		}
		vx, vy = vx/l, vy/l
		gx, gy = x-vx*wb, y-vy*wb
		turn := math.Abs(math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy))
		ux, uy = vx, vy
		perM := math.Min(turn/trailStepMeters, 1/wb)
		if perM <= 0 {
			continue
		}
		v := most / perM
		best = math.Min(best, math.Sqrt(v*v+2*m.p.Decel*d))
	}
	return best
}

// trailHeading is the heading place gives with the nose at s, its
// trailing gear as placed last (forward).
func (m *GroundMover) trailHeading(s float64) (float64, bool) {
	nose := m.path.PointAt(s)
	kx := metersPerDegree * math.Cos(nose.Lat*math.Pi/180)
	dx, dy := (m.gear.Lon-nose.Lon)*kx, (m.gear.Lat-nose.Lat)*metersPerDegree
	if math.Hypot(dx, dy) < 1e-6 {
		return 0, false
	}
	return math.Mod(math.Atan2(-dx, -dy)*180/math.Pi+360, 360), true
}

// turnCapped is step (metres the nose moves in dt) shortened where the
// heading it would be placed at turns quicker than maxGroundTurnDegPerSec
// since the pose last moved, the speed with it: the speed plan's turns
// judged as placed, its true radius (#1664). Forward only; a turn already
// past it is not held up.
func (m *GroundMover) turnCapped(step, dt float64) float64 {
	m.sincePlace += dt
	if m.reverse || !m.placed || step <= 0 || m.p.WheelbaseMeters <= 0 {
		return step
	}
	room := maxGroundTurnDegPerSec * m.sincePlace
	turned := func(d float64) float64 {
		h, ok := m.trailHeading(math.Min(m.s+d, m.path.Length()))
		if !ok {
			return 0
		}
		return math.Abs(headingDiff(m.pose.Heading, h))
	}
	if turned(step) <= room || turned(0) > room {
		return step
	}
	lo, hi := 0.0, step
	for range 12 {
		if mid := (lo + hi) / 2; turned(mid) > room {
			hi = mid
		} else {
			lo = mid
		}
	}
	m.v, m.a = math.Min(m.v, lo/dt), math.Min(m.a, 0)
	return lo
}

func (m *GroundMover) place() {
	if m.placed && m.s == m.placedAt {
		m.pose.GroundSpeedKts, m.pose.Stopped = m.v*m.refSpeed/ktsToMS, m.v == 0
		m.pose.Arrived = m.v == 0 && m.s >= m.path.Length()-0.05
		return // not moved: recomputing would only add rounding noise
	}
	m.placed, m.placedAt, m.sincePlace = true, m.s, 0
	if m.reverse {
		m.refSpeed = 1
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
	// The speed given is the reference point's, not the nose gear's: with
	// the nose gear steered a, the main gear rolls at cos a of its speed
	// and the reference point, RefAheadMeters ahead of it, swings sideways
	// at sin a of it times RefAheadMeters over the wheelbase (#1628: 5 kt
	// given, 0.8 m/s moved in a tight line-up turn).
	m.refSpeed = 1
	if a, b := m.path.PointAt(math.Max(0, m.s-0.5)), m.path.PointAt(math.Min(m.path.Length(), m.s+0.5)); localDist(a, b) > 1e-6 && wb > 0 {
		steer := headingDiff(hdg, localBearing(a, b)) * math.Pi / 180
		m.refSpeed = math.Hypot(math.Max(0, math.Cos(steer)), math.Sin(steer)*m.p.RefAheadMeters/wb)
	}
	m.pose = GroundPose{
		Position:       airport.LatLon{Lat: nose.Lat + dy*f/metersPerDegree, Lon: nose.Lon + dx*f/kx},
		Heading:        hdg,
		GroundSpeedKts: m.v * m.refSpeed / ktsToMS,
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
	// minRollMS (m/s): slower than this, braking onto its stop, it stands.
	minRollMS = 0.003
	// trafficStopDecel, trafficStopJerk (m/s², m/s³): the brakes for traffic
	// found too near to stop short of (GroundMover.runOnTo).
	trafficStopDecel = 1.5
	trafficStopJerk  = 6.0
)

// localDist is the flat-earth distance in metres; exact enough on an airport.
func localDist(a, b airport.LatLon) float64 {
	kx := metersPerDegree * math.Cos((a.Lat+b.Lat)/2*math.Pi/180)
	dx, dy := (b.Lon-a.Lon)*kx, (b.Lat-a.Lat)*metersPerDegree
	return math.Sqrt(dx*dx + dy*dy) // not Hypot: no overflow at these sizes, and cheaper
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

const (
	// speedKnotMeters: the speed plan has a knot at least this often, so a
	// turn's speed stays in the turn however long the straights beside it.
	speedKnotMeters = TurnWindowMeters / 2
	// minBearingMeters: a shorter segment has no direction of its own (two
	// arcs meeting leave a point twice).
	minBearingMeters = 0.05
	// straightRad: turning less over the window is a straight (a bend of a
	// degree in a runway's centreline does not slow a rollout).
	straightRad = 0.02
	// plannedTurnDegPerSec: no turn is planned quicker, the MinTurnKts floor
	// or not: the quickest of ICAO Doc 9157 Part 2's 180-degree turns (Table
	// 1-6, code A: 14.0 km/h on an 11.5 m radius). The jump trace allows
	// 45°/s: the rest is room for the speed control's overshoot and a vehicle
	// easing into its lane.
	plannedTurnDegPerSec = 19.4
	// maxGroundTurnDegPerSec: no ground mover turns quicker as placed,
	// whatever the plan missed (#1664: a tug turned 80°/s at 5 kt off a
	// corner tighter than its plan took); the jump trace allows 45°/s.
	maxGroundTurnDegPerSec = 30.0
	// trailStepMeters: the trailing gear is followed along a path this often.
	trailStepMeters = 0.5
	// swingWheelbases: a vehicle set off across its path has lined up with
	// it this many wheelbases on.
	swingWheelbases = 8
)

// capTurnRateAligned is capTurnRate over the whole path, set off along it.
func (g *GroundPath) capTurnRateAligned(wheelbase float64) {
	p0 := g.pts[0]
	along := localBearing(p0, g.PointAt(math.Min(trailStepMeters, g.Length())))
	g.capTurnRate(wheelbase, offsetHeading(p0, along+180, wheelbase), g.Length())
}

// capTurnRate lowers the plan, as far as upTo metres, where the turn is too
// tight for its speed (#1651): the body, its gear trailing the path from
// gear as place moves it, turns no quicker than plannedTurnDegPerSec. That
// is never less than one round the trailing gear (wheelbase), so it never
// plans a standstill. It reports whether it lowered any.
func (g *GroundPath) capTurnRate(wheelbase float64, gear airport.LatLon, upTo float64) bool {
	if wheelbase <= 0 || len(g.pts) < 2 {
		return false
	}
	most := plannedTurnDegPerSec * math.Pi / 180
	o := g.pts[0]
	kx := metersPerDegree * math.Cos(o.Lat*math.Pi/180)
	xy := func(q airport.LatLon) (float64, float64) {
		return (q.Lon - o.Lon) * kx, (q.Lat - o.Lat) * metersPerDegree
	}
	gx, gy := xy(gear)
	d := math.Hypot(gx, gy)
	if d < 1e-6 {
		return false
	}
	ux, uy := gx/d, gy/d // nose to gear
	lowered, lo := false, 0
	for i := 1; i < len(g.pts) && g.cum[i-1] < upTo; i++ {
		l := g.cum[i] - g.cum[i-1]
		if l <= 0 {
			continue
		}
		ax, ay := xy(g.pts[i-1])
		bx, by := xy(g.pts[i])
		n := math.Ceil(l / trailStepMeters)
		for k := 1.0; k <= n; k++ {
			x, y := ax+(bx-ax)*k/n, ay+(by-ay)*k/n
			nx, ny := gx-x, gy-y
			d := math.Hypot(nx, ny)
			if d < 1e-6 {
				continue
			}
			nx, ny = nx/d, ny/d
			gx, gy = x+nx*wheelbase, y+ny*wheelbase
			turn := math.Abs(math.Atan2(ux*ny-uy*nx, ux*nx+uy*ny))
			ux, uy = nx, ny
			perM := math.Min(turn/(l/n), 1/wheelbase)
			if perM == 0 {
				continue
			}
			// The knots either side, so the speed between them is under it too.
			v, at := most/perM, g.cum[i-1]+l*k/n
			for lo < len(g.plan) && g.plan[lo].at < at-speedKnotMeters {
				lo++
			}
			for j := lo; j < len(g.plan) && g.plan[j].at <= at+speedKnotMeters; j++ {
				if g.plan[j].v > v {
					g.plan[j].v, lowered = v, true
				}
			}
		}
	}
	return lowered
}

// plannedDecel is the braking to plan ahead of a turn: p's Decel, but no
// more than its jerk builds up while the mover answers
// (SpeedResponseSeconds). A follow-me car's 1.5 m/s² took 7.5 s to come
// and it met its turn still fast; a tug planned at 0.8 m/s² (twice the
// answer) still met its turns 2.7 m/s over the plan and was cut down by a
// metre a second in a frame (#1664).
func plannedDecel(p MotionProfile) float64 {
	if p.Jerk <= 0 {
		return p.Decel
	}
	return math.Min(p.Decel, SpeedResponseSeconds*p.Jerk)
}

// speedLimits plans the speed along the path before braking: cruise, and
// in a turn what LateralAccel allows on the radius truly turned on (#1651).
// Before, that was measured between chords to the path's own points: twice
// the radius on an arc, and no turn at all where the next point was the far
// end of a straight.
func speedLimits(pts []airport.LatLon, cum []float64, p MotionProfile, lateralAt func(float64) float64) []speedKnot {
	cruise, minTurn := p.CruiseKts*ktsToMS, math.Min(p.MinTurnKts, p.CruiseKts)*ktsToMS
	turns := turningAlong(pts, cum)
	end := cum[len(cum)-1]
	plan := make([]speedKnot, 0, len(cum)+int(end/speedKnotMeters))
	knot := func(s float64) {
		v := cruise
		if r := turns.radius(s, end); !math.IsInf(r, 1) {
			v = math.Min(v, math.Max(minTurn, math.Sqrt(lateralAt(s)*r)))
		}
		plan = append(plan, speedKnot{s, v})
	}
	knot(0)
	for i := 1; i < len(cum); i++ {
		from, l := cum[i-1], cum[i]-cum[i-1]
		n := math.Ceil(l / speedKnotMeters)
		for k := 1.0; k < n; k++ {
			knot(from + l*k/n)
		}
		knot(cum[i])
	}
	return plan
}

// pathTurning is how far a path has turned (radians, signed) by each
// distance along it, linear between its knots.
type pathTurning struct{ at, turned []float64 }

// turningAlong spreads the turn at each point of the path over half the
// shorter segment either side, speedKnotMeters at most: exact on a sampled
// arc, and a corner between long straights stays at the corner.
func turningAlong(pts []airport.LatLon, cum []float64) pathTurning {
	t := pathTurning{at: make([]float64, 1, 2*len(pts)), turned: make([]float64, 1, 2*len(pts))}
	total, heading, last := 0.0, 0.0, 0.0
	for i := 1; i < len(pts); i++ {
		l := cum[i] - cum[i-1]
		if l < minBearingMeters {
			continue
		}
		h := localBearing(pts[i-1], pts[i])
		if last > 0 {
			w := min(last/2, l/2, speedKnotMeters)
			t.at = append(t.at, cum[i-1]-w, cum[i-1]+w)
			t.turned = append(t.turned, total, total+headingDiff(heading, h)*math.Pi/180)
			total = t.turned[len(t.turned)-1]
		}
		heading, last = h, l
	}
	return t
}

// by is how far the path has turned by s.
func (t pathTurning) by(s float64) float64 {
	i := sort.SearchFloat64s(t.at, s)
	switch {
	case i == 0:
		return t.turned[0]
	case i == len(t.at):
		return t.turned[i-1]
	}
	return t.turned[i-1] + (t.turned[i]-t.turned[i-1])*(s-t.at[i-1])/math.Max(t.at[i]-t.at[i-1], 1e-9)
}

// radius is the radius turned on at s of a path end metres long: over
// ±TurnWindowMeters, and over half of that where the turn is tighter than
// that window can show (an 8 m arc of 90° is shorter than it). A jog is no
// turn (jogged).
func (t pathTurning) radius(s, end float64) float64 {
	if t.jogged(s, end) {
		return math.Inf(1)
	}
	over := func(w float64) float64 {
		from, to := math.Max(0, s-w), math.Min(end, s+w)
		d := math.Abs(t.by(to) - t.by(from))
		if d <= straightRad {
			return math.Inf(1)
		}
		return (to - from) / d
	}
	r := over(TurnWindowMeters)
	if tight := over(TurnWindowMeters / 2); tight < 2*TurnWindowMeters {
		r = math.Min(r, tight)
	}
	return r
}

// jogged reports whether s lies in a jog: within ±jogWindowMeters the path
// turns back to the heading it had (jogNetRad: the nose set down a little
// off the centreline's heading), never more than jogRad off it, a step of
// a metre or two sideways. A route's runway node off the
// centreline (LKPR 30 at R, 1–2 m) planned a rollout at 28 kt through it
// and a P28A judged too fast for the exit beyond (#1651).
func (t pathTurning) jogged(s, end float64) bool {
	from, to := math.Max(0, s-jogWindowMeters), math.Min(end, s+jogWindowMeters)
	start := t.by(from)
	if math.Abs(t.by(to)-start) > jogNetRad {
		return false
	}
	for x := from; x <= to; x += speedKnotMeters / 4 {
		if math.Abs(t.by(x)-start) > jogRad {
			return false
		}
	}
	return true
}

const (
	// jogWindowMeters, jogRad, jogNetRad: see jogged.
	jogWindowMeters = 3 * TurnWindowMeters
	jogRad          = 0.15
	jogNetRad       = 0.035
)

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
	decel := plannedDecel(p)
	g.decelAt, g.jerk = func(float64) float64 { return decel }, p.Jerk
	g.plan = speedLimits(g.pts, g.cum, p, func(float64) float64 { return p.LateralAccel })
	g.capTurnRateAligned(p.WheelbaseMeters)
	g.brakeAhead(g.decelAt)
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

// stopSpeed is the highest speed (m/s) from which braking that builds up
// at jerk (m/s³) to decel (m/s²) stops within rem metres: it covers
// v·decel/(2·jerk) while the braking builds, then v²/(2·decel). Planning
// with v²/(2·decel) alone, the aircraft met the stop point still rolling
// and halted in one frame (live, TVS206 at 7 kt at its holding point; from
// rest 40 m before a hold, 7 kt to 0 at once).
func stopSpeed(decel, jerk, rem float64) float64 {
	if rem <= 0 || decel <= 0 {
		return 0
	}
	if jerk <= 0 {
		return math.Sqrt(2 * decel * rem)
	}
	k := decel / (2 * jerk)
	return decel * (-k + math.Sqrt(k*k+2*rem/decel))
}
