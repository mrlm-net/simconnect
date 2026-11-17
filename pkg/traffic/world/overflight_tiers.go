package world

import (
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Distance tiers for the overflights (#1482). Each overflight is otherwise
// a simulator object for its whole crossing of the area, and every object
// costs the simulator's main thread whether anyone sees it or not. With
// the tiers on, an overflight far from the viewer (the user aircraft, and
// the aircraft our camera is on) is a virtual track: flown by the World
// alone along its planned waypoint chain (traffic.VirtualFlight, the way
// SimlessAI flies one), with no object and no request. Once it is within
// the materialise range of a viewer it is created as a NonATC aircraft at
// exactly the virtual track's pose and given the waypoints it has left;
// farther than the dematerialise range it is removed and flown virtually
// again from where it was. The gap between the two ranges keeps it from
// flipping at the edge.
//
// A virtual track still counts: on the map and in the snapshot (Traffic
// with Virtual), in the planning of the others (plannedTraffic: their
// levels and releases keep clear of it) and in the traffic information
// told to the user aircraft (PlayerTraffic).
//
// Off by default: with no materialise range every overflight is a
// simulator object as before, and MaxOverflights counts them all. With
// the tiers on, MaxOverflights counts the overflights in the simulator,
// and MaxVirtual all of them at once, virtual or not.

// OverflightTiers set the overflights' distance tiers (Options.Overflights,
// POST /api/schedule overflightMaterialiseNM, overflightDematerialiseNM,
// maxVirtualOverflights); the zero value is off.
type OverflightTiers struct {
	// MaterialiseNM: a virtual overflight this near a viewer (NM) is
	// created in the simulator; 0: off, every overflight in the
	// simulator for its whole crossing. About the visibility from the
	// flight deck (40 NM) is enough.
	MaterialiseNM float64
	// DematerialiseNM: an overflight in the simulator farther than this
	// from every viewer is taken out and flown virtually (0, or not past
	// MaterialiseNM: MaterialiseNM + 10).
	DematerialiseNM float64
	// MaxVirtual: the overflights at once, virtual and in the simulator
	// (0: 200).
	MaxVirtual int
}

const (
	// overflightHysteresisNM: the dematerialise range's default, past the
	// materialise one.
	overflightHysteresisNM = 10.0
	// defaultMaxVirtualOverflights: OverflightTiers.MaxVirtual's default.
	defaultMaxVirtualOverflights = 200
	// virtualIDBase: virtual tracks are given IDs from here (Traffic.ObjectID
	// on the map), clear of the simulator's object IDs.
	virtualIDBase uint32 = 0xFF00_0000
)

// On reports whether the tiers are on.
func (t OverflightTiers) On() bool { return t.MaterialiseNM > 0 }

// Dematerialise is the dematerialise range in effect.
func (t OverflightTiers) Dematerialise() float64 {
	if t.DematerialiseNM > t.MaterialiseNM {
		return t.DematerialiseNM
	}
	return t.MaterialiseNM + overflightHysteresisNM
}

// Virtual is the cap on the overflights at once in effect.
func (t OverflightTiers) Virtual() int {
	if t.MaxVirtual > 0 {
		return t.MaxVirtual
	}
	return defaultMaxVirtualOverflights
}

// maxOverflightsOr is the overflights' cap as set (MaxOverflights), or
// the manager's default. s.mu held.
func (s *scheduler) maxOverflightsOr() int {
	if s.maxOver > 0 {
		return s.maxOver
	}
	return traffic.DefaultMaxOverflights
}

// applyOverflightCaps gives the manager its cap on overflights at once:
// MaxOverflights with the tiers off, as before; with them on, the virtual
// cap, the tiers keeping those in the simulator to MaxOverflights.
func (s *scheduler) applyOverflightCaps() {
	s.mu.Lock()
	t, n := s.tiers, s.maxOverflightsOr()
	s.mu.Unlock()
	if t.On() {
		n = t.Virtual()
	}
	s.mgr.SetMaxOverflights(n)
}

// tiersTick moves the virtual overflights on, and creates or removes each
// one by its distance from the viewers (scheduler goroutine, each second).
func (s *scheduler) tiersTick(now time.Time) {
	s.mu.Lock()
	t, maxOver := s.tiers, s.maxOverflightsOr()
	var virtual []*enrouteAC
	for _, e := range s.enroute {
		if e.vt != nil && !e.materialising {
			virtual = append(virtual, e)
		}
	}
	s.mu.Unlock()
	if !t.On() {
		// Off (switched off while some were virtual): each one into the
		// simulator, as every overflight is then.
		for _, e := range virtual {
			s.mu.Lock()
			s.advance(e, now)
			s.mu.Unlock()
			s.materialise(e, -1)
		}
		return
	}
	air := s.cc.world.Aircraft()
	views := s.viewers(air)
	at := make(map[uint32]traffic.TrackedAircraft, len(air))
	for _, a := range air {
		at[a.ObjectID] = a
	}
	type cand struct {
		e  *enrouteAC
		nm float64
	}
	type leaving struct {
		e *enrouteAC
		a traffic.TrackedAircraft
	}
	var near []cand
	var far []leaving
	var done []string
	in := 0 // overflights in the simulator, or being created there
	s.mu.Lock()
	for _, e := range s.enroute {
		if !e.f.Overflight() || e.corridor != "" {
			continue
		}
		switch {
		case e.vt == nil: // in the simulator
			in++
			if a, ok := at[e.objectID]; ok && len(views) > 0 && nearestNM(views, a.Position) > t.Dematerialise() {
				far = append(far, leaving{e, a})
			}
		case e.materialising:
			in++
		default:
			s.advance(e, now)
			if e.vt.Done() {
				done = append(done, e.f.Callsign)
				continue
			}
			if len(views) == 0 {
				continue
			}
			if nm := nearestNM(views, e.vt.Pose().Position); nm <= t.MaterialiseNM {
				near = append(near, cand{e, nm})
			}
		}
	}
	for _, e := range s.pending {
		if e.vt == nil && e.f.Overflight() && e.corridor == "" {
			in++ // created as before the tiers were on
		}
	}
	s.mu.Unlock()
	for _, cs := range done {
		s.mgr.Remove(cs, now) // through the area: done (Remove takes its track)
	}
	for _, l := range far {
		if s.dematerialise(l.e, l.a, now) {
			in--
		}
	}
	// The nearest first, while there is room in the simulator.
	sort.Slice(near, func(i, j int) bool {
		return near[i].nm < near[j].nm || near[i].nm == near[j].nm && near[i].e.f.Callsign < near[j].e.f.Callsign
	})
	for _, c := range near {
		if in >= maxOver {
			break
		}
		if s.materialise(c.e, c.nm) {
			in++
		}
	}
}

// advance flies a virtual overflight on to now. s.mu held.
func (s *scheduler) advance(e *enrouteAC, now time.Time) {
	if e.vtAt.IsZero() {
		e.vtAt = now // flown from here on (back from a creation not answered)
		return
	}
	if dt := now.Sub(e.vtAt).Seconds(); dt > 0 {
		e.vt.Step(dt)
		e.vtAt = now
	}
}

// viewers are where the traffic is seen from: the user aircraft, and the
// aircraft our camera is on (air: the picture now).
func (s *scheduler) viewers(air []traffic.TrackedAircraft) []airport.LatLon {
	var out []airport.LatLon
	if u := s.st.core.userAt(); u != (airport.LatLon{}) {
		out = append(out, u)
	}
	if tail := s.cameraSubject(); tail != "" {
		for _, a := range air {
			if a.Tail == tail && !a.User {
				out = append(out, a.Position)
				break
			}
		}
	}
	return out
}

// cameraSubject is the aircraft our camera is on, "" none (off, or no
// camera on this connection).
func (s *scheduler) cameraSubject() string {
	s.st.mu.Lock()
	m := s.st.camera
	s.st.mu.Unlock()
	if m == nil {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.mode == "" || m.mode == "off" {
		return ""
	}
	return m.subject
}

// nearestNM is how far p is from the nearest of views.
func nearestNM(views []airport.LatLon, p airport.LatLon) float64 {
	best := math.Inf(1)
	for _, v := range views {
		best = math.Min(best, calc.HaversineNM(v.Lat, v.Lon, p.Lat, p.Lon))
	}
	return best
}

// virtualTrack makes e a virtual overflight appearing at spawn and flying
// wps (EnrouteStart's, planned once with its route, levels and speeds), no
// object created: the tiers create it once it is near a viewer. s.mu not
// held.
func (s *scheduler) virtualTrack(f traffic.ManagedFlight, e *enrouteAC, spawn types.SIMCONNECT_DATA_INITPOSITION, wps []types.SIMCONNECT_DATA_WAYPOINT) {
	now := s.cc.clock.Now()
	s.mu.Lock()
	e.vt, e.vtAt = traffic.NewVirtualFlight(spawn, wps), now
	e.virtualID = s.newVirtualID()
	e.pendingAt = traffic.Now() // its first point taken for justCreatedFor, as a creation's
	s.enroute[f.Callsign] = e
	s.mu.Unlock()
	s.mgr.Describe(f.Callsign, e.model, "", "")
	s.mgr.Update(f.Callsign, traffic.FlightEnroute, now)
}

// newVirtualID is the next ID for a virtual track. s.mu held.
func (s *scheduler) newVirtualID() uint32 {
	s.nextVirtual = (s.nextVirtual + 1) & 0x00FF_FFFF
	return virtualIDBase + s.nextVirtual
}

// materialise creates the aircraft of a virtual overflight at its pose,
// to fly the waypoints it has left; nm is how near the viewer it is (for
// the log, negative: the tiers switched off). The track stands still
// until the simulator answers (handle). Reports whether it was asked for.
func (s *scheduler) materialise(e *enrouteAC, nm float64) bool {
	s.mu.Lock()
	if e.vt == nil || e.materialising || e.gone || s.enroute[e.f.Callsign] != e || len(e.vt.Ahead()) == 0 {
		s.mu.Unlock()
		return false
	}
	e.materialising = true
	e.reqID, e.pendingAt = s.freeEnrouteReq(), traffic.Now()
	s.pending[e.reqID] = e
	req := e.reqID
	s.mu.Unlock()
	title, livery, _ := strings.Cut(e.model, liverySep)
	var pose traffic.VirtualPose
	err := s.cc.do(func() error {
		// Where it is as the simulator creates it, not as the tick saw it:
		// the call may wait a frame or more (offline: 50 m at 480 kt with
		// a few created at once).
		s.mu.Lock()
		s.advance(e, s.cc.clock.Now())
		pose = e.vt.Pose()
		e.waypoints = e.vt.Ahead()
		s.mu.Unlock()
		if len(e.waypoints) == 0 {
			return errors.New("through its waypoints")
		}
		return s.cc.sim.SpawnEnroute(traffic.NonATCOpts{Model: title, Livery: livery, Tail: e.f.Callsign, Position: pose.InitPosition()}, req)
	})
	if err != nil {
		s.mu.Lock()
		delete(s.pending, req)
		e.materialising, e.vtAt = false, time.Time{}
		s.mu.Unlock()
		s.cc.log.printf("%-6s overflight: into the simulator: %v", e.f.Callsign, err)
		return false
	}
	if nm >= 0 {
		s.cc.log.printf("%-6s overflight: %.0f NM from the viewer: into the simulator at %s", e.f.Callsign, nm, flLog(pose.AltFt))
	} else {
		s.cc.log.printf("%-6s overflight: tiers off: into the simulator at %s", e.f.Callsign, flLog(pose.AltFt))
	}
	return true
}

// dematerialise takes an overflight's aircraft (a, as the picture last saw
// it) out of the simulator and flies it on virtually from there, along its
// route ahead (as resolutions left it). Reports whether it was taken out.
func (s *scheduler) dematerialise(e *enrouteAC, a traffic.TrackedAircraft, now time.Time) bool {
	s.mu.Lock()
	route := e.route
	s.mu.Unlock()
	kts := a.GroundKts
	if kts <= 0 {
		kts = e.cruiseKts
	}
	way, ok := wayOnFrom(a.Position, a.Heading, a.AltFt, kts, route)
	start, wps, err := traffic.EnrouteStart(way)
	if !ok || err != nil {
		// Past the end of its way: its crossing is over.
		s.mgr.Remove(e.f.Callsign, now)
		return true
	}
	start.Heading = a.Heading // as it flies now, mid-turn too
	vt := traffic.NewVirtualFlight(start, wps)
	// Where it is by now: the picture saw it a moment ago.
	if dt := now.Sub(a.Updated).Seconds(); dt > 0 && dt < 5 {
		vt.Step(dt)
	}
	s.mu.Lock()
	if s.enroute[e.f.Callsign] != e || e.vt != nil || e.objectID != a.ObjectID {
		s.mu.Unlock()
		return false // gone, or changed meanwhile
	}
	id := e.objectID
	e.objectID, e.vt, e.vtAt = 0, vt, now
	if e.virtualID == 0 {
		e.virtualID = s.newVirtualID()
	}
	s.mu.Unlock()
	s.cc.do(func() error { return s.cc.sim.RemoveObject(id, reqRemoveEnroute) })
	s.cc.dropOwn(id)
	s.cc.world.ForgetOwn(id)
	s.mgr.Attach(e.f.Callsign, 0) // not followed in the picture while virtual
	s.cc.log.printf("%-6s overflight: farther than %.0f NM from the viewer: out of the simulator, flown on virtually", e.f.Callsign, s.tiersNow().Dematerialise())
	return true
}

// tiersNow are the tiers in effect.
func (s *scheduler) tiersNow() OverflightTiers {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tiers
}

// wayOnFrom is the way on from pos (heading hdg, at altFt and kts) along
// route: pos, then the points of route still to fly; false when it is past
// the last one (behind it), its way flown.
func wayOnFrom(pos airport.LatLon, hdg, altFt, kts float64, route []traffic.RoutePoint) ([]traffic.RoutePoint, bool) {
	ahead := traffic.ProfileAhead(pos, route)
	if len(ahead) == 0 {
		return nil, false
	}
	if len(ahead) == 1 {
		p := ahead[0].Position
		if math.Abs(math.Remainder(calc.BearingDegrees(pos.Lat, pos.Lon, p.Lat, p.Lon)-hdg, 360)) > 90 {
			return nil, false
		}
	}
	return append([]traffic.RoutePoint{{Position: pos, AltFt: altFt, Kts: kts}}, ahead...), true
}

// virtualView is a virtual overflight as it is now.
type virtualView struct {
	e     *enrouteAC
	pose  traffic.VirtualPose
	route []traffic.RoutePoint
}

// virtuals are the virtual overflights, each where it is at now (flown on
// from the last tick, not moved). s.mu held.
func (s *scheduler) virtuals(now time.Time) []virtualView {
	var out []virtualView
	for _, e := range s.enroute {
		if e.vt == nil {
			continue
		}
		p := e.vt.Pose()
		if !e.materialising && !e.vtAt.IsZero() {
			if dt := now.Sub(e.vtAt).Seconds(); dt > 0 {
				p = e.vt.PoseAfter(dt)
			}
		}
		out = append(out, virtualView{e: e, pose: p, route: e.route})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].e.f.Callsign < out[j].e.f.Callsign })
	return out
}

// virtualCount is how many overflights are virtual now.
func (s *scheduler) virtualCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, e := range s.enroute {
		if e.vt != nil {
			n++
		}
	}
	return n
}

// virtualTraffic are the virtual overflights as the map shows traffic
// (Traffic with Virtual; ObjectID a World-given one, no object).
func (s *scheduler) virtualTraffic(now time.Time) []Traffic {
	s.mu.Lock()
	vs := s.virtuals(now)
	s.mu.Unlock()
	out := make([]Traffic, 0, len(vs))
	for _, v := range vs {
		title, _, _ := strings.Cut(v.e.model, liverySep)
		way := make([]airport.LatLon, 0, len(v.route))
		for _, p := range v.route {
			way = append(way, p.Position)
		}
		out = append(out, Traffic{ObjectID: v.e.virtualID, Route: pointsAhead(v.pose.Position, way), Title: title, Tail: v.e.f.Callsign,
			Latitude: v.pose.Position.Lat, Longitude: v.pose.Position.Lon, AGL: v.pose.AltFt, GroundKts: v.pose.Kts, Heading: v.pose.Heading,
			VerticalFpm: v.pose.VSFpm, Ours: true, Alt: v.pose.AltFt, Type: typeOfTitle(title), Virtual: true})
	}
	return out
}

// virtualAircraft are the virtual overflights as the picture has aircraft
// (en route, ours), with their planned ways ahead.
func (s *scheduler) virtualAircraft(now time.Time) ([]traffic.TrackedAircraft, map[string][]traffic.RoutePoint) {
	s.mu.Lock()
	vs := s.virtuals(now)
	s.mu.Unlock()
	out := make([]traffic.TrackedAircraft, 0, len(vs))
	plans := make(map[string][]traffic.RoutePoint, len(vs))
	for _, v := range vs {
		title, _, _ := strings.Cut(v.e.model, liverySep)
		out = append(out, traffic.TrackedAircraft{Observation: traffic.Observation{Tail: v.e.f.Callsign, Title: title, Position: v.pose.Position,
			AltFt: v.pose.AltFt, AGLFt: v.pose.AltFt, GroundKts: v.pose.Kts, Heading: v.pose.Heading, VSFpm: v.pose.VSFpm},
			Phase: traffic.PhaseEnroute, Ours: true, Updated: now})
		if len(v.route) > 1 {
			plans[v.e.f.Callsign] = traffic.ProfileAhead(v.pose.Position, v.route)
		}
	}
	return out, plans
}
