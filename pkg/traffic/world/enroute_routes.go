package world

import (
	"hash/fnv"
	"math/rand/v2"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Overflight routes by the airspace's route structure (#1484), with
// OverflightTraffic.OverflightRoutes: an overflight crosses free route
// airspace direct (the great circle, or between the area's named entry and
// exit points when it lists them) and airway-based airspace, or free route
// airspace below its level, along the airways (its plan, the route
// variety of #1213 kept); with no table entry, along the airways
// (nav.RouteStructureAt). Each overflight flies a strategic lateral offset
// of 0, 1 or 2 NM right of its route, by its call sign, so a stream is
// not one line. Without the setting every overflight crosses on its great
// circle as before.

// overflightOffsetsNM are the strategic offsets an overflight may fly.
var overflightOffsetsNM = []float64{0, 1, 2}

// entryPointNM: a named entry or exit point is taken within this of where
// the great circle crosses the boundary.
const entryPointNM = 40.0

// routesOn reports whether the overflights follow the route structure.
func (s *scheduler) routesOn() bool { return s.overflightSettings().routes }

// structureHere is the route structure of the area the overflights cross.
func (s *scheduler) structureHere() (nav.RouteStructure, bool) {
	c, ok := s.cc.world.Centre()
	if !ok {
		return nav.RouteStructure{AirwaysOnly: true}, false
	}
	return nav.RouteStructureAt(c)
}

// flightSeed is a call sign's own random numbers (its route's choices).
func flightSeed(callsign string) *rand.Rand {
	h := fnv.New64a()
	h.Write([]byte(callsign))
	return rand.New(rand.NewPCG(h.Sum64(), 0x1484))
}

// freeRouteCrossing is an overflight's way across the area when it flies
// it direct: the great circle (greatCircleCrossing), through the area's
// named entry and exit points when it lists them; nil when it follows the
// airways there (the route structure, with the routes on).
func (s *scheduler) freeRouteCrossing(fp *nav.FlightPlan, f traffic.ManagedFlight, kts float64) []traffic.RoutePoint {
	gc := greatCircleCrossing(fp, s.cc, kts)
	if !s.routesOn() || len(gc) < 2 {
		return gc
	}
	rs, _ := s.structureHere()
	if !rs.Direct(gc[0].AltFt) {
		return nil // along the airways
	}
	c, _ := s.cc.world.Centre()
	if len(rs.Points) == 0 {
		return gc
	}
	// Where it leaves the area: the last point within it.
	last := 0
	for i, p := range gc {
		if calc.HaversineNM(c.Lat, c.Lon, p.Position.Lat, p.Position.Lon) <= overflightRadiusNM {
			last = i
		}
	}
	rng := flightSeed(f.Callsign)
	in, okIn := rs.EntryNear(gc[0].Position, entryPointNM, 2, rng)
	out, okOut := rs.EntryNear(gc[last].Position, entryPointNM, 2, rng)
	if !okIn || !okOut || in.Ident == out.Ident {
		return gc
	}
	pts := []airport.LatLon{gc[0].Position, in.Position, out.Position, gc[len(gc)-1].Position}
	route := make([]traffic.RoutePoint, 0, len(pts))
	alt := gc[0].AltFt
	for i, p := range pts {
		q := p
		if i+1 < len(pts) {
			q = pts[i+1]
		} else {
			p = pts[i-1]
		}
		level := cruiseLevelFt(alt, calc.BearingDegrees(p.Lat, p.Lon, q.Lat, q.Lon))
		route = append(route, traffic.RoutePoint{Position: pts[i], AltFt: level, Kts: traffic.EnrouteSpeedKts(level, kts)})
	}
	return route
}

// lateralSpread is an overflight's route at its strategic offset (with the
// routes on), the levels and speeds as they were.
func (s *scheduler) lateralSpread(f traffic.ManagedFlight, e *enrouteAC, route []traffic.RoutePoint) []traffic.RoutePoint {
	if !f.Overflight() || e.corridor != "" || !s.routesOn() || len(route) < 2 {
		return route
	}
	off := overflightOffsetsNM[flightSeed(f.Callsign+"/offset").IntN(len(overflightOffsetsNM))]
	if off == 0 {
		return route
	}
	pts := make([]airport.LatLon, len(route))
	for i, p := range route {
		pts[i] = p.Position
	}
	moved := nav.StrategicOffset(pts, off)
	out := append([]traffic.RoutePoint(nil), route...)
	for i := range out {
		out[i].Position = moved[i]
	}
	return out
}
