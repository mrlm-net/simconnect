package nav

import (
	"math"
	"math/rand/v2"
	"slices"
	"sort"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/dict"
)

// Route structure per airspace (#1484): where traffic crossing an area
// flies direct (free route airspace) and where it follows the airways.
// Free route is not the rule everywhere: many areas are airway-based, or
// free route only above a level, and there traffic follows the airways and
// named entry and exit points. A table per airspace (nav.routeStructure,
// replaceable at runtime like the others, pkg/dict): an area is a
// latitude/longitude box (a FIR's, roughly), free route at and above a
// level with airways below, or airways only, with optional named entry
// and exit points for its free route. An area with no entry follows the
// airways (the generic default). The shipped entries are the project's
// estimates, not from a source.

// RouteStructure is an airspace's route structure (nav.routeStructure).
type RouteStructure struct {
	Area string `json:"area"` // id: a FIR's ICAO code or a name
	Name string `json:"name,omitempty"`
	// The box, degrees (West > East: across the antimeridian).
	North float64 `json:"north"`
	South float64 `json:"south"`
	West  float64 `json:"west"`
	East  float64 `json:"east"`
	// FreeRouteFt: free route at and above this altitude (feet, FL245:
	// 24500), the airways below; 0 free route at every level.
	FreeRouteFt float64 `json:"freeRouteFt,omitempty"`
	// AirwaysOnly: no free route.
	AirwaysOnly bool `json:"airwaysOnly,omitempty"`
	// Points are the named entry and exit points of its free route (none:
	// anywhere on the boundary, the great circle).
	Points []EntryPoint `json:"points,omitempty"`
}

// EntryPoint is a named entry or exit point of a free route area.
type EntryPoint struct {
	Ident    string         `json:"ident"`
	Position airport.LatLon `json:"position"`
}

// Contains reports whether p is in the structure's box.
func (r RouteStructure) Contains(p airport.LatLon) bool {
	if p.Lat > r.North || p.Lat < r.South {
		return false
	}
	if r.West <= r.East {
		return p.Lon >= r.West && p.Lon <= r.East
	}
	return p.Lon >= r.West || p.Lon <= r.East
}

// Direct reports whether traffic at altFt crosses the area direct (free
// route), not along the airways.
func (r RouteStructure) Direct(altFt float64) bool {
	return !r.AirwaysOnly && altFt >= r.FreeRouteFt
}

var shippedRouteStructures = []RouteStructure{
	// Free route in the upper airspace over Czechia; the airways below.
	{Area: "LKAA", Name: "Praha FIR", North: 51.2, South: 48.4, West: 11.9, East: 19.0, FreeRouteFt: 24500},
	// London: the airways.
	{Area: "EGTT-LON", Name: "London", North: 52.6, South: 50.4, West: -2.2, East: 1.8, AirwaysOnly: true},
}

var routeStructuresNow dict.Value[[]RouteStructure]

func init() {
	dict.Register(dict.Keyed("nav.routeStructure", "area", "", "",
		func() []RouteStructure { return slices.Clone(shippedRouteStructures) },
		func(r RouteStructure) string { return strings.ToUpper(r.Area) },
		func(items []RouteStructure) { routeStructuresNow.Store(items) }))
	_ = dict.Reset("nav.routeStructure")
}

// RouteStructureAt is the route structure of the airspace holding p (the
// smallest box when several do); none: the generic default, the airways
// (AirwaysOnly), and ok false.
func RouteStructureAt(p airport.LatLon) (RouteStructure, bool) {
	var best RouteStructure
	bestSize, found := math.Inf(1), false
	for _, r := range routeStructuresNow.Load() {
		if !r.Contains(p) {
			continue
		}
		w := r.East - r.West
		if w < 0 {
			w += 360
		}
		if size := (r.North - r.South) * w; size < bestSize {
			best, bestSize, found = r, size, true
		}
	}
	if !found {
		return RouteStructure{Area: "", AirwaysOnly: true}, false
	}
	return best, true
}

// EntryNear is one of the structure's named points near p (within
// withinNM): drawn by rng among the n nearest, the nearest more often; ok
// false: none near.
func (r RouteStructure) EntryNear(p airport.LatLon, withinNM float64, n int, rng *rand.Rand) (EntryPoint, bool) {
	type cand struct {
		e  EntryPoint
		nm float64
	}
	var cs []cand
	for _, e := range r.Points {
		if d := calc.HaversineNM(p.Lat, p.Lon, e.Position.Lat, e.Position.Lon); d <= withinNM {
			cs = append(cs, cand{e, d})
		}
	}
	if len(cs) == 0 {
		return EntryPoint{}, false
	}
	sort.Slice(cs, func(i, j int) bool {
		return cs[i].nm < cs[j].nm || cs[i].nm == cs[j].nm && cs[i].e.Ident < cs[j].e.Ident
	})
	if n < 1 {
		n = 1
	}
	cs = cs[:min(n, len(cs))]
	if rng == nil || len(cs) == 1 {
		return cs[0].e, true
	}
	// The nearest twice as often as the next.
	total := 0.0
	for i := range cs {
		total += 1 / float64(i+1)
	}
	x := rng.Float64() * total
	for i := range cs {
		if x -= 1 / float64(i+1); x < 0 {
			return cs[i].e, true
		}
	}
	return cs[len(cs)-1].e, true
}

// StrategicOffset is route's points moved offsetNM to the right of each
// leg (a strategic lateral offset, SLOP): each point by the leg it begins,
// the last by the leg it ends.
func StrategicOffset(points []airport.LatLon, offsetNM float64) []airport.LatLon {
	if offsetNM == 0 || len(points) < 2 {
		return points
	}
	out := make([]airport.LatLon, len(points))
	for i, p := range points {
		a, b := p, airport.LatLon{}
		if i+1 < len(points) {
			b = points[i+1]
		} else {
			a, b = points[i-1], p
		}
		brg := calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon)
		lat, lon := calc.DisplaceByHeading(p.Lat, p.Lon, brg+90, offsetNM*1852)
		out[i] = airport.LatLon{Lat: lat, Lon: lon}
	}
	return out
}
