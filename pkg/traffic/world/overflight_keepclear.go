package world

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/airspace"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Crossing traffic keeps clear of the terminal airspace (#1486), with
// OverflightTraffic.CrossingKeepClear: an overflight or transit is planned
// above each layer of the focus airports' terminal airspace inside its
// outline (with terminalMarginFt to spare), and of the other airports' in
// the area too, its own ends' excepted; a level that would cut into one is
// not given (clearLevel, clearTransitLevel), and a transit whose every top
// does is taken round it (aroundTerminals). The terminal airspace is the
// control zone and the terminal areas stacked on it from the real
// airspaces (Options.Airspaces, airspace.Store.TerminalStack: LKPR to
// 16,500 ft), else a generic one round each focus airport
// (genericTerminalNM, genericTerminalTopFt). Separation from the focus
// airports' own traffic stays the up-front plan's: their departures' climb
// clearances and their arrivals' levels are planned against everyone's
// planned ways, the crossing traffic's among them.

const (
	// terminalMarginFt: crossing traffic keeps this far above a layer's
	// ceiling (and below its floor) inside its outline.
	terminalMarginFt = 1000.0
	// terminalTopFt: no layer above this is terminal airspace (en route).
	terminalTopFt = 24500.0
	// genericTerminalNM and genericTerminalTopFt: the terminal airspace of
	// a focus airport when the real airspaces are not given: a cylinder.
	genericTerminalNM    = 30.0
	genericTerminalTopFt = 12500.0
	// terminalAirportsNM: the other airports' terminal airspace counts
	// within this of the centre.
	terminalAirportsNM = overflightRadiusNM + 20
	// keepClearStepNM: a way is looked at every this many NM.
	keepClearStepNM = 2.0
	// aroundNM: a way taken round a terminal airspace passes this far
	// outside its widest point.
	aroundNM = 3.0
)

// terminalArea is an airport's terminal airspace.
type terminalArea struct {
	icao    string
	pos     airport.LatLon
	groundM float64
	stack   []*airspace.Shape
	// reachNM: its widest point from the airport (an outline's farthest
	// vertex).
	reachNM float64
	generic bool
}

// terminalsCache keeps the terminal areas by the area and airports.
type terminalsCache struct {
	mu    sync.Mutex
	key   string
	areas []terminalArea
}

var terminals terminalsCache

// keepClearOn reports whether crossing traffic keeps clear of the
// terminal airspace.
func (s *scheduler) keepClearOn() bool { return s.overflightSettings().keepClear }

// terminalAreas are the terminal airspaces crossing traffic keeps clear
// of: the focus airports', and with the real airspaces the other
// airports' within terminalAirportsNM of the centre.
func (s *scheduler) terminalAreas() []terminalArea {
	c, ok := s.cc.world.Centre()
	if !ok {
		return nil
	}
	s.mu.Lock()
	focus := append([]string(nil), s.focus...)
	s.mu.Unlock()
	store := s.cc.core.hooks.Airspaces
	key := fmt.Sprintf("%.3f,%.3f|%s|%p", c.Lat, c.Lon, strings.Join(focus, ","), store)
	terminals.mu.Lock()
	defer terminals.mu.Unlock()
	if terminals.key == key {
		return terminals.areas
	}
	var out []terminalArea
	seen := map[string]bool{}
	add := func(icao string, pos airport.LatLon, groundM float64, isFocus bool) {
		if seen[icao] {
			return
		}
		seen[icao] = true
		t := terminalArea{icao: icao, pos: pos, groundM: groundM}
		if store != nil {
			if st, err := store.TerminalStack(pos.Lat, pos.Lon, groundM, terminalTopFt*0.3048); err == nil {
				t.stack = st
			}
		}
		if len(t.stack) == 0 {
			if !isFocus {
				return
			}
			a := &airspace.Airspace{Name: icao + " terminal (generic)", Type: airspace.ClassC, MinAltType: airspace.AltMSL, MaxAltType: airspace.AltMSL,
				MinAlt: 0, MaxAlt: groundM + genericTerminalTopFt*0.3048,
				Vectors: []airspace.Vector{{Type: airspace.VectorOrigin, Lat: pos.Lat, Lon: pos.Lon}, {Type: airspace.VectorCircle, Radius: genericTerminalNM * 1852}}}
			sh, err := a.Shape()
			if err != nil {
				return
			}
			t.stack, t.generic = []*airspace.Shape{sh}, true
		}
		for _, sh := range t.stack {
			for _, r := range sh.Rings {
				for _, p := range r {
					t.reachNM = math.Max(t.reachNM, calc.HaversineNM(pos.Lat, pos.Lon, p.Lat, p.Lon))
				}
			}
		}
		out = append(out, t)
	}
	for _, icao := range focus {
		if g, err := s.cc.graph(icao); err == nil {
			l := g.Layout
			add(icao, airport.LatLon{Lat: l.Latitude, Lon: l.Longitude}, l.Altitude, true)
		}
	}
	if store != nil {
		var near []traffic.ScheduleAirport
		for _, a := range s.config().Airports {
			if calc.HaversineNM(c.Lat, c.Lon, a.Position.Lat, a.Position.Lon) <= terminalAirportsNM {
				near = append(near, a)
			}
		}
		sort.Slice(near, func(i, j int) bool { return near[i].ICAO < near[j].ICAO })
		for _, a := range near {
			add(a.ICAO, a.Position, 0, false)
		}
	}
	terminals.key, terminals.areas = key, out
	return out
}

// crossingEnds are a crossing flight's own airports (their terminal
// airspace is its to fly through); ok false: not crossing traffic of
// ours (an arrival, a departure: the focus airports' own).
func (s *scheduler) crossingEnds(callsign string) ([2]string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.over.crossing[callsign]
	return e, ok
}

// keepsClear reports whether route keeps clear of the terminal airspace
// (the flight's own ends' excepted); always for traffic not crossing, or
// with the setting off. The terminal area cut into, when not.
func (s *scheduler) keepsClear(callsign string, route []traffic.RoutePoint) (string, bool) {
	if !s.keepClearOn() || len(route) < 2 {
		return "", true
	}
	ends, ok := s.crossingEnds(callsign)
	if !ok {
		return "", true
	}
	for _, t := range s.terminalAreas() {
		if t.icao == ends[0] || t.icao == ends[1] {
			continue
		}
		if !wayInStack(route, t) {
			continue
		}
		return t.icao, false
	}
	return "", true
}

// wayInStack reports whether route, flown leg by leg (each point's
// altitude reached at it, in a straight line), comes into t.
func wayInStack(route []traffic.RoutePoint, t terminalArea) bool {
	margin := terminalMarginFt * 0.3048
	for i := 1; i < len(route); i++ {
		a, b := route[i-1], route[i]
		leg := calc.HaversineNM(a.Position.Lat, a.Position.Lon, b.Position.Lat, b.Position.Lon)
		// Far from it all along the leg: nothing to look at.
		if calc.HaversineNM(t.pos.Lat, t.pos.Lon, a.Position.Lat, a.Position.Lon) > t.reachNM+leg+1 {
			continue
		}
		n := max(1, int(math.Ceil(leg/keepClearStepNM)))
		for k := 0; k <= n; k++ {
			f := float64(k) / float64(n)
			lat, lon := calc.IntermediatePoint(a.Position.Lat, a.Position.Lon, b.Position.Lat, b.Position.Lon, f)
			alt := (a.AltFt + f*(b.AltFt-a.AltFt)) * 0.3048
			if airspace.InStack(t.stack, lat, lon, alt, t.groundM, margin) {
				return true
			}
		}
	}
	return false
}

// aroundTerminals is route taken round the terminal areas it cuts into:
// its points within an area's reach moved square off its overall track,
// away from the airport, to aroundNM outside the reach; ok false when it
// still cuts into one.
func (s *scheduler) aroundTerminals(callsign string, route []traffic.RoutePoint) ([]traffic.RoutePoint, bool) {
	if len(route) < 2 {
		return route, false
	}
	ends, _ := s.crossingEnds(callsign)
	out := append([]traffic.RoutePoint(nil), route...)
	a, b := route[0].Position, route[len(route)-1].Position
	trk := calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon)
	for _, t := range s.terminalAreas() {
		if t.icao == ends[0] || t.icao == ends[1] || !wayInStack(out, t) {
			continue
		}
		// Away from the airport: to the side of the track it is not on.
		side := -1.0
		if calc.CrossTrackMeters(a.Lat, a.Lon, b.Lat, b.Lon, t.pos.Lat, t.pos.Lon) < 0 {
			side = 1 // the airport left of the track: round it on the right
		}
		need := t.reachNM + aroundNM
		for i := range out {
			p := out[i].Position
			if calc.HaversineNM(t.pos.Lat, t.pos.Lon, p.Lat, p.Lon) >= need {
				continue
			}
			for k := 1.0; k <= 2*need; k++ {
				lat, lon := calc.DisplaceByHeading(p.Lat, p.Lon, trk+90*side, k*1852)
				if calc.HaversineNM(t.pos.Lat, t.pos.Lon, lat, lon) >= need {
					out[i].Position = airport.LatLon{Lat: lat, Lon: lon}
					break
				}
			}
		}
	}
	if _, ok := s.keepsClear(callsign, out); !ok {
		return route, false
	}
	return out, true
}
