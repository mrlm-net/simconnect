package world

import (
	"github.com/mrlm-net/simconnect/pkg/airport"
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Pacing en route arrivals (live, LKPR: CSA499 at cruise, en route, caught
// up TVS384 that approach had slowed to 210 kt and was handed over at VLM
// 1 NM behind it). An arrival still flown by MSFS AI on its plan keeps no
// gap to the one ahead in the landing sequence by itself: once it closes
// to within enroutePaceNM it flies no faster than its leader, and gets its
// own speed back once the gap opens past enrouteResumeNM.
const (
	enroutePaceNM   = 10.0
	enrouteResumeNM = 14.0
	enroutePaceKts  = 10.0 // a speed change smaller than this is not made
	enroutePaceMin  = 210.0
	enroutePaceEach = 10 * time.Second
)

// paceEnroute matches each en route arrival's speed to its leader's.
func (s *scheduler) paceEnroute(now time.Time) {
	if s.cc.sequencesAt == nil || now.Sub(s.pacedAt) < enroutePaceEach {
		return
	}
	s.pacedAt = now
	air := map[string]traffic.TrackedAircraft{}
	for _, a := range s.cc.world.Aircraft() {
		air[a.Tail] = a
	}
	s.mu.Lock()
	var list []*enrouteAC
	for _, e := range s.enroute {
		if e.arrive != nil && !e.handing && e.corridor == "" && len(e.route) > 1 {
			list = append(list, e)
		}
	}
	s.mu.Unlock()
	for _, e := range list {
		seqs := s.cc.sequencesAt(e.f.Airport)
		leader, gap := "", math.Inf(1)
		for _, q := range seqs {
			for i, en := range q {
				if en.Callsign == e.f.Callsign && i > 0 {
					leader, gap = q[i-1].Callsign, en.DistanceToGoNM-q[i-1].DistanceToGoNM
				}
			}
		}
		me, ok := air[e.f.Callsign]
		if !ok {
			continue
		}
		want := 0.0 // its own speed
		if l, ok := air[leader]; ok && gap < enroutePaceNM && l.GroundKts > 0 && l.GroundKts < me.GroundKts {
			want = math.Max(enroutePaceMin, l.GroundKts)
		} else if e.pacedKts == 0 || gap < enrouteResumeNM {
			continue // as it is
		}
		if math.Abs(want-e.pacedKts) < enroutePaceKts {
			continue
		}
		if err := s.setEnrouteSpeed(e, me, want); err != nil {
			s.cc.log.printf("%-6s en route: pace behind %s: %v", e.f.Callsign, leader, err)
			continue
		}
		e.pacedKts = want
		if want > 0 {
			s.cc.log.printf("%-6s en route: %.0f kt behind %s (%.1f NM ahead)", e.f.Callsign, want, leader, gap)
		} else {
			s.cc.log.printf("%-6s en route: own speed again (%.1f NM behind %s)", e.f.Callsign, gap, leader)
		}
	}
}

// setEnrouteSpeed flies e's route ahead from where it is at kts at most
// (0: the speeds planned).
func (s *scheduler) setEnrouteSpeed(e *enrouteAC, me traffic.TrackedAircraft, kts float64) error {
	last := e.route[len(e.route)-1].Position
	left := calc.HaversineNM(me.Position.Lat, me.Position.Lon, last.Lat, last.Lon)
	route := []traffic.RoutePoint{{Position: me.Position, AltFt: me.AltFt, Kts: me.GroundKts}}
	for _, p := range e.route[1:] {
		if calc.HaversineNM(p.Position.Lat, p.Position.Lon, last.Lat, last.Lon) >= left {
			continue // behind it
		}
		p.Kts = traffic.EnrouteSpeedKts(p.AltFt, e.cruiseKts) // as planned
		if p.Position == last {
			p.Kts = entryKts(p.AltFt, e.cruiseKts) // the STAR entry: an arrival's speed
		}
		if kts > 0 {
			p.Kts = math.Min(p.Kts, kts)
		}
		route = append(route, p)
	}
	if len(route) < 2 {
		return nil
	}
	_, wps, err := traffic.EnrouteStart(route)
	if err != nil {
		return err
	}
	if err := s.cc.do(func() error { return s.cc.sim.SetRoute(e.objectID, wps) }); err != nil {
		return err
	}
	s.mu.Lock()
	e.route = route
	s.mu.Unlock()
	return nil
}

// enrouteRoutes are the routes ours flown by MSFS AI still fly, by object:
// for the map, which showed only where they came from and go (an
// arrival's way to its STAR looked like it flew wrongly).
func (s *scheduler) enrouteRoutes() map[uint32][]airport.LatLon {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[uint32][]airport.LatLon{}
	for _, e := range s.enroute {
		if e.objectID == 0 || len(e.route) == 0 {
			continue
		}
		r := make([]airport.LatLon, 0, len(e.route)+len(arriveRoute(e)))
		for _, p := range e.route {
			r = append(r, p.Position)
		}
		r = append(r, arriveRoute(e)...)
		out[e.objectID] = r
	}
	return out
}

// arriveRoute is an arrival's STAR and approach after its entry, nil for
// an overflight.
func arriveRoute(e *enrouteAC) []airport.LatLon {
	if e.arrive == nil {
		return nil
	}
	var r []airport.LatLon
	for _, n := range e.arrive.route {
		r = append(r, n.Position)
	}
	return r
}
