//go:build windows
// +build windows

package main

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
	enrouteReqBase      uint32 = 7100
	enrouteReqCount     uint32 = 800
	reqRemoveEnroute    uint32 = 2007
	reqReleaseEnroute   uint32 = 2008
	enrouteDefWaypoints uint32 = 2009
)

// Handover: at this distance from the STAR entry, or this long after the
// time the arrival should be there.
const (
	handoverNM    = 8.0
	handoverAfter = 2 * time.Minute
)

// enrouteAC is an aircraft of ours flown by MSFS AI on its flight plan.
type enrouteAC struct {
	f        traffic.ManagedFlight
	reqID    uint32
	objectID uint32
	model    string
	arrive   *planned // arrival: the STAR and approach from the entry
	// waypoints: the rest of the flight, flown by MSFS AI once released.
	waypoints []types.SIMCONNECT_DATA_WAYPOINT
	handing   bool
}

// spawnEnroute creates an enroute arrival or an overflight where its
// flight is now.
func (s *scheduler) spawnEnroute(f traffic.ManagedFlight) error {
	cc, st := s.cc, s.st
	arrRwy := ""
	if f.Arrival() {
		g, err := st.cache.Graph(f.Airport)
		if err != nil {
			return err
		}
		arrRwy = cc.activeRunway(g, true)
	}
	a := s.airlines[f.Airline]
	models := traffic.ModelsFor(cc.modelList(), f.Airline, a.Name, f.Type, 6)
	if len(models) == 0 {
		return fmt.Errorf("no model of a %s", f.Type)
	}
	model := models[(f.Attempts-1)%len(models)]
	fp, err := planBetween(context.Background(), st, f.Origin, f.Destination, "", arrRwy, f.Type)
	if err != nil {
		return fmt.Errorf("flight plan %s → %s: %w", f.Origin, f.Destination, err)
	}
	kts := fp.Performance.CruiseTASKts
	if kts < 200 {
		kts = 420
	}
	var dist float64
	e := &enrouteAC{f: f, model: model}
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
		dist = entry - time.Until(due).Hours()*kts
		if dist > entry-15 {
			return fmt.Errorf("only %.0f NM before its STAR entry: it appears there", entry-dist)
		}
	} else {
		dist = time.Since(f.STD.Add(10*time.Minute)).Hours() * kts
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
				route = append(route, traffic.RoutePoint{Position: w.Position, AltFt: w.AltFt, Kts: traffic.EnrouteSpeedKts(w.AltFt, kts)})
			}
			break
		}
		route = append(route, traffic.RoutePoint{Position: w.Position, AltFt: w.AltFt, Kts: traffic.EnrouteSpeedKts(w.AltFt, kts)})
	}
	spawn, wps, err := traffic.EnrouteStart(route)
	if err != nil {
		return err
	}
	e.waypoints = wps
	title, livery, _ := strings.Cut(model, liverySep)
	s.mu.Lock()
	s.nextReq = (s.nextReq + 1) % enrouteReqCount
	e.reqID = enrouteReqBase + s.nextReq
	s.pending[e.reqID] = e
	s.mu.Unlock()
	err = cc.do(func() error {
		return cc.fleet.RequestNonATC(traffic.NonATCOpts{Model: title, Livery: livery, Tail: f.Callsign, Position: spawn}, e.reqID)
	})
	if err != nil {
		s.mu.Lock()
		delete(s.pending, e.reqID)
		s.mu.Unlock()
		return err
	}
	tlog.printf("%-6s schedule: %s %s → %s en route, %s, FL%03d, %.0f NM along %s, %d waypoints", f.Callsign, f.Kind, f.Origin, f.Destination, f.Type,
		int(math.Round(altFt/100)), dist, fp.Route, len(wps))
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
	cc.fleet.Acknowledge(e.reqID, e.objectID)
	s.defOnce.Do(func() {
		if err := cc.client.AddToDataDefinition(enrouteDefWaypoints, "AI Waypoint List", "number", types.SIMCONNECT_DATATYPE_WAYPOINT, 0, 0); err != nil {
			tlog.printf("schedule: waypoint definition: %v", err)
		}
	})
	if err := cc.fleet.ReleaseControl(e.objectID, reqReleaseEnroute); err != nil {
		tlog.printf("%-6s schedule: release: %v", e.f.Callsign, err)
	}
	if err := cc.fleet.SetWaypoints(e.objectID, enrouteDefWaypoints, e.waypoints); err != nil {
		tlog.printf("%-6s schedule: waypoints: %v", e.f.Callsign, err)
	}
	cc.addOwn(e.objectID)
	cc.world.SetOwn(e.objectID, traffic.PhaseEnroute, "")
	s.mgr.Attach(e.f.Callsign, e.objectID)
	s.mgr.Describe(e.f.Callsign, e.model, "", "")
	s.mgr.Update(e.f.Callsign, traffic.FlightEnroute, time.Now())
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
		if seen && calc.HaversineNM(p.Lat, p.Lon, entry.Lat, entry.Lon) < handoverNM || now.After(e.f.STA.Add(-lead).Add(handoverAfter)) {
			e.handing = true
			due = append(due, e)
		}
	}
	s.mu.Unlock()
	for _, e := range due {
		go func(e *enrouteAC) {
			s.dropEnroute(e)
			f := e.f
			f.Stage = ""
			tlog.printf("%-6s schedule: at %s, handed to the arrival controller", f.Callsign, e.arrive.route[0].Ident)
			if err := s.spawnWith(f, e.arrive, e.model); err != nil {
				tlog.printf("%-6s schedule: handover failed: %v", f.Callsign, err)
				s.mgr.Failed(f.Callsign, err, time.Now())
			}
		}(e)
	}
}

// dropEnroute takes an enroute aircraft out of the simulator.
func (s *scheduler) dropEnroute(e *enrouteAC) {
	s.mu.Lock()
	delete(s.enroute, e.f.Callsign)
	s.mu.Unlock()
	s.cc.do(func() error { return s.cc.client.AIRemoveObject(e.objectID, reqRemoveEnroute) })
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
	ignore := s.mgr.Options().Others == traffic.OtherIgnore
	for _, a := range s.cc.world.Aircraft() {
		if a.OnGround || ignore && !a.Ours {
			continue
		}
		if calc.HaversineNM(a.Position.Lat, a.Position.Lon, p.Lat, p.Lon) < entryClearNM && (altFt == 0 || math.Abs(a.AltFt-altFt) < entryClearFt) {
			if a.Tail != "" {
				return a.Tail
			}
			return a.Title
		}
	}
	return ""
}
