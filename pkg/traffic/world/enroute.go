package world

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Enroute traffic (#369): arrivals appear en route on their flight plan and
// overflights cross the area: created airborne where the flight is now and
// released to MSFS AI with the rest of the plan as a waypoint chain
// (traffic.EnrouteStart). An arrival is handed to the arrival controller at
// its STAR entry: the MSFS AI aircraft makes way for the controlled one
// there.

// Request IDs of enroute creations, and of their removal.
const (
	enrouteReqBase      uint32 = 41000 // clear of the library's 7300–7999 and the controllers' blocks
	enrouteReqCount     uint32 = 1000
	reqRemoveEnroute    uint32 = 2007
	reqReleaseEnroute   uint32 = 2008
	enrouteDefWaypoints uint32 = 2009
)

// Handover: at this distance from the STAR entry, or this long after the
// time the arrival should be there.
const (
	handoverNM    = 8.0
	handoverAfter = 2 * time.Minute
	// handoverLateNM: a late one is handed over by time only this near.
	handoverLateNM = 40.0
	// enrouteSlowNM: an arrival keeps its cruise speed down to this far
	// before its STAR entry.
	enrouteSlowNM = 30.0
)

// slowDownBefore is the point enrouteSlowNM before entry on the leg from
// prev, at cruise speed, when the leg is longer than that; none otherwise.
// enrouteEntryKts: an arrival reaches its STAR entry no faster than this
// (live: TVS1878 handed over at 422 kt and FL320, swung 8 NM wide of the
// STAR's first turn, and was sent direct to the final).
const enrouteEntryKts = 280.0

// entryKts is the speed at a STAR entry at altFt for a cruise of kts.
func entryKts(altFt, kts float64) float64 {
	return math.Min(traffic.EnrouteSpeedKts(altFt, kts), enrouteEntryKts)
}

func slowDownBefore(prev traffic.RoutePoint, entry airport.LatLon, entryAltFt, kts float64) []traffic.RoutePoint {
	if calc.HaversineNM(prev.Position.Lat, prev.Position.Lon, entry.Lat, entry.Lon) <= enrouteSlowNM+10 {
		return nil
	}
	brg := calc.BearingDegrees(entry.Lat, entry.Lon, prev.Position.Lat, prev.Position.Lon)
	lat, lon := calc.DisplaceByHeading(entry.Lat, entry.Lon, brg, enrouteSlowNM*1852)
	alt := math.Max(entryAltFt, math.Min(prev.AltFt, entryAltFt+enrouteSlowNM*300))
	return []traffic.RoutePoint{{Position: airport.LatLon{Lat: lat, Lon: lon}, AltFt: alt, Kts: traffic.EnrouteSpeedKts(math.Max(alt, 10000), kts)}}
}

// enrouteAC is an aircraft of ours flown by MSFS AI on its flight plan.
type enrouteAC struct {
	f        traffic.ManagedFlight
	reqID    uint32
	objectID uint32
	model    string
	arrive   *planned // arrival: the STAR and approach from the entry
	// waypoints: the rest of the flight, flown by MSFS AI once released;
	// route: the same as planned, which conflict resolutions change (#395).
	waypoints []types.SIMCONNECT_DATA_WAYPOINT
	route     []traffic.RoutePoint
	fixes     []airFix // the named fixes of route, for shortcuts (conflicts)
	handing   bool
	// corridor: one of the traffic along the user's route (#740), not the
	// schedule's.
	corridor   traffic.CorridorKind
	corridorNM float64 // its distance from the user at the last look
	// cruiseKts: the speed it was planned at; pacedKts the leader's speed it
	// flies at most now (paceEnroute), 0 its own.
	cruiseKts, pacedKts float64
	// pendingAt: when its creation was asked (pending until answered).
	pendingAt time.Time
}

// spawnEnroute creates an enroute arrival or an overflight where its
// flight is now.
// overflightEntry is how far along fp its route first comes within
// overflightRadiusNM of the traffic picture's centre, every 5 NM.
func overflightEntry(fp *nav.FlightPlan, cc *controlCenter) (float64, bool) {
	c, ok := cc.world.Centre()
	if !ok {
		return 0, false
	}
	for d := 0.0; d <= fp.DistanceNM; d += 5 {
		if p, _, _ := fp.PositionAt(d); calc.HaversineNM(c.Lat, c.Lon, p.Lat, p.Lon) <= overflightRadiusNM {
			return d, true
		}
	}
	return 0, false
}

func (s *scheduler) spawnEnroute(f traffic.ManagedFlight) error {
	cc, st := s.cc, s.st
	arrRwy := ""
	if f.Arrival() {
		g, err := st.cache.Graph(f.Airport)
		if err != nil {
			return err
		}
		arrRwy = cc.pickRunway(g, true, -1)
	}
	a := s.airlines[f.Airline]
	models := traffic.ModelsForFlight(cc.modelList(), f.Airline, a.Name, f.Type, f.Callsign, 6)
	if len(models) == 0 {
		return fmt.Errorf("no model of a %s", f.Type)
	}
	model := models[(f.Attempts-1)%len(models)]
	var fp *nav.FlightPlan
	var err error
	if f.Arrival() {
		fp, err = planBetween(context.Background(), st, f.Origin, f.Destination, "", arrRwy, f.Type)
	} else { // an overflight: its ends where they are, not loaded
		fp, err = planOverflight(context.Background(), st, f.Origin, f.Destination, f.Type)
	}
	if err != nil {
		return fmt.Errorf("flight plan %s → %s: %w", f.Origin, f.Destination, err)
	}
	kts := fp.Performance.CruiseTASKts
	if kts < 200 {
		kts = 420
	}
	var dist float64
	e := &enrouteAC{f: f, model: model, cruiseKts: kts}
	if f.Arrival() {
		if e.arrive, err = plannedFrom(fp, "arrival"); err != nil {
			return err
		}
		entry := fp.DistanceNM
		for _, w := range fp.Waypoints {
			if w.Phase == nav.PhaseSTAR || w.Phase == nav.PhaseApproach {
				entry = w.DistanceNM
				break
			}
		}
		// Where it must be now to reach the entry when the arrival is due there.
		due := f.STA.Add(-s.mgr.Options().ArrivalLead)
		dist = entry - due.Sub(cc.clock.Now()).Hours()*kts
		if dist > entry-15 {
			return fmt.Errorf("only %.0f NM before its STAR entry: it appears there", entry-dist)
		}
	} else {
		// Where its plan enters the area, and on from there by the time since
		// it was due to (#469: placed by its STD along the plan, whose route
		// and speed differ from the schedule's, RYR1850 appeared 230 NM out).
		in, ok := overflightEntry(fp, cc)
		if !ok {
			// Its airways pass the area by, though the flight was picked for
			// crossing it: across on the great circle instead (#783).
			gc := greatCircleCrossing(fp, cc, kts)
			if len(gc) < 2 {
				return fmt.Errorf("%w: its plan %s → %s never enters the area", traffic.ErrSpawnImpossible, f.Origin, f.Destination)
			}
			return s.spawnEnrouteOn(f, e, model, gc, "great circle")
		}
		dist = in
		if !f.Enter.IsZero() {
			dist += cc.clock.Now().Sub(f.Enter).Hours() * kts
		}
	}
	dist = math.Max(10, math.Min(dist, fp.DistanceNM-20))
	pos, altFt, _ := fp.PositionAt(dist)
	// Nobody appears on top of other traffic.
	if who := s.nearPoint(pos, altFt); who != "" {
		return fmt.Errorf("%w: %s near its position", traffic.ErrSpawnBlocked, who)
	}
	// The rest of the flight: from here along the plan at its levels, an
	// arrival to its STAR entry, an overflight up to its descent.
	route := []traffic.RoutePoint{{Position: pos, AltFt: altFt, Kts: traffic.EnrouteSpeedKts(altFt, kts)}}
	for _, w := range fp.Waypoints {
		if w.DistanceNM <= dist+2 || w.Kind == nav.PointRunway || w.Kind == nav.PointAirport {
			continue
		}
		if w.Phase == nav.PhaseSTAR || w.Phase == nav.PhaseApproach {
			if f.Arrival() {
				// MSFS AI flies a leg at the speed of the point it heads for:
				// the entry's 250 kt below FL100 slowed the whole DCT (live:
				// QTR1636 and OKOEX at 250 kt GS, late, handed over 100 to
				// 400 NM out). At cruise down to enrouteSlowNM before it.
				route = append(route, slowDownBefore(route[len(route)-1], w.Position, w.AltFt, kts)...)
				route = append(route, traffic.RoutePoint{Position: w.Position, AltFt: w.AltFt, Kts: entryKts(w.AltFt, kts)})
			}
			break
		}
		route = append(route, traffic.RoutePoint{Position: w.Position, AltFt: w.AltFt, Kts: traffic.EnrouteSpeedKts(w.AltFt, kts)})
		if w.Ident != "" && w.Kind != "" {
			e.fixes = append(e.fixes, airFix{Ident: w.Ident, LatLon: w.Position})
		}
	}
	along := fp.Route
	if along == "" {
		along = "direct"
	}
	if len(route) < 2 && !f.Arrival() {
		// Its plan enters the area only where it starts down its
		// destination's STAR: across on the great circle instead (live:
		// DLH1163 KJFK → LHBP failed "an enroute flight needs two points").
		if gc := greatCircleCrossing(fp, cc, kts); len(gc) >= 2 {
			return s.spawnEnrouteOn(f, e, model, gc, "great circle")
		}
	}
	return s.spawnEnrouteOn(f, e, model, route, along)
}

// spawnEnrouteOn creates e flying route (its first point where it appears).
func (s *scheduler) spawnEnrouteOn(f traffic.ManagedFlight, e *enrouteAC, model string, route []traffic.RoutePoint, along string) error {
	cc := s.cc
	if who := s.nearPoint(route[0].Position, route[0].AltFt); who != "" {
		return fmt.Errorf("%w: %s near its position", traffic.ErrSpawnBlocked, who)
	}
	spawn, wps, err := traffic.EnrouteStart(route)
	if err != nil {
		return err
	}
	e.waypoints, e.route = wps, route
	title, livery, _ := strings.Cut(model, liverySep)
	s.mu.Lock()
	e.reqID, e.pendingAt = s.freeEnrouteReq(), time.Now()
	s.pending[e.reqID] = e
	s.mu.Unlock()
	err = cc.do(func() error {
		return cc.sim.SpawnEnroute(traffic.NonATCOpts{Model: title, Livery: livery, Tail: f.Callsign, Position: spawn}, e.reqID)
	})
	if err != nil {
		s.mu.Lock()
		delete(s.pending, e.reqID)
		s.mu.Unlock()
		return err
	}
	s.cc.log.printf("%-6s schedule: %s %s → %s en route, %s, FL%03d along %s, %d waypoints", f.Callsign, f.Kind, orUnknown(f.Origin), orUnknown(f.Destination), f.Type,
		int(math.Round(route[0].AltFt/100)), along, len(wps))
	return nil
}

// handle takes the simulator's answer to an enroute creation (connection
// goroutine): the aircraft is ours; released to MSFS AI it flies its
// waypoints.
func (s *scheduler) handle(msg engine.Message) bool {
	if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID {
		return false
	}
	m := msg.AsAssignedObjectID()
	s.mu.Lock()
	e := s.pending[uint32(m.DwRequestID)]
	delete(s.pending, uint32(m.DwRequestID))
	if e != nil {
		e.objectID = uint32(m.DwObjectID)
		s.enroute[e.f.Callsign] = e
	}
	s.mu.Unlock()
	if e == nil {
		return false
	}
	cc := s.cc
	if err := cc.sim.FlyEnroute(e.reqID, e.objectID, e.waypoints); err != nil {
		s.cc.log.printf("%-6s schedule: en route: %v", e.f.Callsign, err)
	}
	cc.addOwn(e.objectID)
	cc.world.SetOwn(e.objectID, traffic.PhaseEnroute, "")
	if e.corridor != "" {
		return true // not the schedule's
	}
	s.mgr.Attach(e.f.Callsign, e.objectID)
	s.mgr.Describe(e.f.Callsign, e.model, "", "")
	s.mgr.Update(e.f.Callsign, traffic.FlightEnroute, s.cc.clock.Now())
	return true
}

// handovers passes enroute arrivals to the arrival controller at their
// STAR entry (scheduler goroutine).
func (s *scheduler) handovers(now time.Time) {
	lead := s.mgr.Options().ArrivalLead
	pos := map[uint32]airport.LatLon{}
	for _, a := range s.cc.world.Aircraft() {
		pos[a.ObjectID] = a.Position
	}
	s.mu.Lock()
	var due []*enrouteAC
	for _, e := range s.enroute {
		if e.arrive == nil || e.handing {
			continue
		}
		entry := e.arrive.route[0].Position
		p, seen := pos[e.objectID]
		far := math.Inf(1)
		if seen {
			far = calc.HaversineNM(p.Lat, p.Lon, entry.Lat, entry.Lon)
		}
		// Late at the entry: handed over where it is only near it (or unseen,
		// or past its STA), never from hundreds of miles out.
		lateAt := e.f.STA.Add(-lead).Add(handoverAfter)
		if e.f.Observed != nil {
			lateAt = e.f.STA.Add(handoverAfter) // a real aircraft: STA is its time at the entry (#841)
		}
		late := now.After(lateAt) && (!seen || far < handoverLateNM || now.After(e.f.STA))
		if far < handoverNM || late {
			// Not onto other traffic at the entry: the handover waits.
			if who := s.cc.nearAirborne(entry, 0, e.f.Callsign, now); who != "" {
				continue
			}
			e.handing = true
			due = append(due, e)
		}
	}
	s.mu.Unlock()
	for _, e := range due {
		go func(e *enrouteAC) {
			// The aircraft flies on: the arrival controller adopts it, no
			// new one at the entry (#643: a jump of 15 km and 11,000 ft).
			s.handEnroute(e)
			f := e.f
			f.Stage = ""
			s.cc.log.printf("%-6s schedule: at %s, handed to the arrival controller", f.Callsign, e.arrive.route[0].Ident)
			arrive := *e.arrive
			// The runway in use changed since it was planned (#776: EZY1205
			// on 24 after the change to 06): its STAR and approach again,
			// from where it is, for the runway in use.
			if g, err := s.st.cache.Graph(f.Airport); err == nil && arrive.plan != nil {
				if rwy := s.cc.pickRunway(g, true, -1); rwy != "" && rwy != arrive.plan.Request.ArrivalRunway {
					if fp, err := planBetween(context.Background(), s.st, f.Origin, f.Destination, "", rwy, f.Type); err == nil {
						if p, err := plannedFrom(fp, "arrival"); err == nil {
							s.cc.log.printf("%-6s schedule: runway %s in use: arrival planned again from %s", f.Callsign, rwy, arrive.plan.Request.ArrivalRunway)
							arrive = *p
						}
					}
				}
			}
			arrive.adopt = e.objectID
			if err := s.spawnWith(f, &arrive, e.model); err != nil {
				s.cc.do(func() error { return s.cc.sim.RemoveObject(e.objectID, reqRemoveEnroute) })
				s.cc.log.printf("%-6s schedule: handover failed: %v", f.Callsign, err)
				s.mgr.Failed(f.Callsign, err, s.cc.clock.Now())
			}
		}(e)
	}
}

// dropEnroute takes an enroute aircraft out of the simulator.
func (s *scheduler) dropEnroute(e *enrouteAC) {
	s.mu.Lock()
	delete(s.enroute, e.f.Callsign)
	s.mu.Unlock()
	s.cc.do(func() error { return s.cc.sim.RemoveObject(e.objectID, reqRemoveEnroute) })
	s.cc.dropOwn(e.objectID)
	s.cc.world.ForgetOwn(e.objectID)
}

// removeEnroute removes a flight's enroute aircraft; false when it has none.
func (s *scheduler) removeEnroute(callsign string) bool {
	s.mu.Lock()
	e := s.enroute[callsign]
	s.mu.Unlock()
	if e == nil {
		return false
	}
	s.dropEnroute(e)
	return true
}

// nearPoint names an airborne aircraft near a point, "" when clear (other
// traffic only when respected).
func (s *scheduler) nearPoint(p airport.LatLon, altFt float64) string {
	if s.mgr.Options().Others == traffic.OtherIgnore {
		// Ignoring other traffic: only ours keeps its distance.
		for _, a := range s.cc.world.Aircraft() {
			if a.Ours && !a.OnGround && calc.HaversineNM(a.Position.Lat, a.Position.Lon, p.Lat, p.Lon) < entryClearNM &&
				(altFt == 0 || math.Abs(a.AltFt-altFt) < entryClearFt) {
				return a.Tail
			}
		}
		return ""
	}
	return s.cc.nearAirborne(p, altFt, "", s.cc.clock.Now())
}

// handEnroute takes an enroute aircraft off the scheduler's list without
// removing it from the simulator: the arrival controller adopts it.
func (s *scheduler) handEnroute(e *enrouteAC) {
	s.mu.Lock()
	delete(s.enroute, e.f.Callsign)
	s.mu.Unlock()
	s.cc.dropOwn(e.objectID)
	s.cc.world.ForgetOwn(e.objectID)
}

// greatCircleCrossing is an overflight's way across the area on the great
// circle from its origin to its destination at its cruise level: from where
// it enters overflightRadiusNM of the picture's centre to 60 NM past where
// it leaves (#783); nil when it does not cross.
func greatCircleCrossing(fp *nav.FlightPlan, cc *controlCenter, kts float64) []traffic.RoutePoint {
	c, ok := cc.world.Centre()
	if !ok || len(fp.Waypoints) < 2 {
		return nil
	}
	from, to := fp.Waypoints[0].Position, fp.Waypoints[len(fp.Waypoints)-1].Position
	alt := float64(fp.CruiseFL) * 100
	if alt <= 0 {
		alt = 35000
	}
	var out []traffic.RoutePoint
	inside, past := false, 0.0
	p := from
	for n := 0; n < 4000; n++ {
		d := calc.HaversineNM(p.Lat, p.Lon, to.Lat, to.Lon)
		if d < 5 {
			break
		}
		in := calc.HaversineNM(c.Lat, c.Lon, p.Lat, p.Lon) <= overflightRadiusNM
		if in || inside {
			if !inside || n%4 == 0 { // its entry, then every 20 NM
				out = append(out, traffic.RoutePoint{Position: p, AltFt: alt, Kts: traffic.EnrouteSpeedKts(alt, kts)})
			}
			inside = true
			if !in {
				if past += 5; past > 60 {
					break
				}
			}
		}
		lat, lon := calc.DisplaceByHeading(p.Lat, p.Lon, calc.BearingDegrees(p.Lat, p.Lon, to.Lat, to.Lon), 5*1852)
		p = airport.LatLon{Lat: lat, Lon: lon}
	}
	return out
}

// enroutePendingFor: a creation not answered by then is dropped (#63: a
// refused one stayed pending, and its kind was never spawned again).
const enroutePendingFor = 30 * time.Second

// freeEnrouteReq is the next request ID not in use by a pending creation
// (#63: the counter wrapped onto one still waiting). s.mu held.
func (s *scheduler) freeEnrouteReq() uint32 {
	base := s.st.core.libIDs().enrouteReq
	for range enrouteReqCount {
		s.nextReq = (s.nextReq + 1) % enrouteReqCount
		if s.pending[base+s.nextReq] == nil {
			break
		}
	}
	return base + s.nextReq
}

// expirePending drops the creations the simulator never answered.
func (s *scheduler) expirePending() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, e := range s.pending {
		if !e.pendingAt.IsZero() && time.Since(e.pendingAt) > enroutePendingFor {
			delete(s.pending, id)
			s.cc.log.printf("%-6s en route: not created by the simulator: dropped", e.f.Callsign)
		}
	}
}
