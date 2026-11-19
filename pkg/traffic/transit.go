package traffic

import (
	"math"
	"math/rand/v2"
	"sort"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/nav"
)

// Transit traffic (#1485): the departures and arrivals of the airports in
// and near the area (not the focus airports) whose way crosses it, on
// their climb or descent. Every overflight crosses at its cruise level,
// and airports in the area are no overflight's end, so without them the
// sky over the airport has only level traffic in the upper levels.
//
// Each neighbour's movements are its share of the schedule's (PeakPerHour
// 24 at a hub, half at a major, a fifth at a regional, on its own waves);
// a movement goes to or comes from an airport of the config, drawn by
// size, and is a transit when its great circle crosses the core of the
// area. A transit is never flown to or from the ground: it appears and
// goes at FloorFt on its climb or descent, out of view (at least ViewNM
// from the centre), so no runway, stand or taxi is needed at its airport.
// Its cruise level is its flight plan's (nav.CruiseLevelRule: short hops
// lower, by distance and the type), and the World flies its profile.

// TransitOptions steer Transits.
type TransitOptions struct {
	// Centre and RadiusNM are the area.
	Centre   airport.LatLon
	RadiusNM float64
	// Density scales the volume (0: 1); Scale is the share of the
	// neighbours' movements drawn (0: 1).
	Density, Scale float64
	Seed           uint64
	// Exclude are the focus airports: no transit starts or ends there.
	Exclude []string
	// NeighbourNM: airports within this of the centre are the neighbours
	// (0: RadiusNM + 100).
	NeighbourNM float64
	// CoreNM: a transit's great circle passes within this of the centre
	// (0: 0.6 RadiusNM).
	CoreNM float64
	// FloorFt: a transit appears and goes at this altitude at least (0:
	// 10,000 ft); ViewNM: never nearer than this to the centre there (0:
	// 45).
	FloorFt, ViewNM float64
}

// TransitFloorFt and TransitViewNM are the defaults of
// TransitOptions.FloorFt and ViewNM.
const (
	TransitFloorFt = 10000.0
	TransitViewNM  = 45.0
)

func (o *TransitOptions) defaults() {
	if o.Density <= 0 {
		o.Density = 1
	}
	if o.Scale <= 0 {
		o.Scale = 1
	}
	if o.NeighbourNM <= 0 {
		o.NeighbourNM = o.RadiusNM + 100
	}
	if o.CoreNM <= 0 {
		o.CoreNM = 0.6 * o.RadiusNM
	}
	if o.FloorFt <= 0 {
		o.FloorFt = TransitFloorFt
	}
	if o.ViewNM <= 0 {
		o.ViewNM = TransitViewNM
	}
}

// transitMinNM: a transit's ends are at least this far apart.
const transitMinNM = 100.0

// sizeShare is an airport's movements against a hub's (Schedule's sizes).
func sizeShare(size int) float64 {
	switch {
	case size >= 3:
		return 1
	case size == 2:
		return 0.5
	}
	return 0.2
}

// FloorAlongNM is how far from its airport a flight of typ is at floorFt on
// its climb (departure) or its descent, by its type's mean rates
// (nav.Performance), from an airport at sea level.
func FloorAlongNM(typ string, floorFt float64, departure bool) float64 {
	p := nav.PerformanceFor(typ)
	fpm, kts := p.ClimbFPM, p.ClimbTASKts
	if !departure {
		fpm, kts = p.DescentFPM, p.DescentTASKts
	}
	if fpm <= 0 || kts <= 0 {
		return floorFt / 300 // about 3° (318 ft a mile)
	}
	return floorFt / fpm / 60 * kts
}

// Transits generates the transits entering the area between from and to,
// with Enter and Exit as the overflights have them: when each comes into
// view (its floor out of view, or the area's edge) and leaves it.
// Deterministic for a seed.
func Transits(cfg ScheduleConfig, o TransitOptions, from, to time.Time) []Flight {
	o.defaults()
	if o.RadiusNM <= 0 {
		return nil
	}
	rng := rand.New(rand.NewPCG(o.Seed, 0x1485^uint64(from.Truncate(time.Hour).Unix())))
	g := scheduler{cfg: cfg, rng: rng, used: map[string]bool{}, airports: map[string]ScheduleAirport{}}
	var neighbours, ends []ScheduleAirport
	var endW []float64
	for _, a := range cfg.Airports {
		g.airports[a.ICAO] = a
		if contains(o.Exclude, a.ICAO) {
			continue
		}
		ends, endW = append(ends, a), append(endW, float64(a.Size*a.Size))
		if calc.HaversineNM(o.Centre.Lat, o.Centre.Lon, a.Position.Lat, a.Position.Lon) <= o.NeighbourNM {
			neighbours = append(neighbours, a)
		}
	}
	sort.Slice(neighbours, func(i, j int) bool { return neighbours[i].ICAO < neighbours[j].ICAO })
	var out []Flight
	for h := from.Truncate(time.Hour); h.Before(to); h = h.Add(time.Hour) {
		for _, n := range neighbours {
			mean := 24 * sizeShare(n.Size) * cfg.Waves[LocalHour(n.ICAO, n.Position, h)] * o.Density * o.Scale
			for range g.count(mean) {
				at := h.Add(time.Duration(rng.Float64() * float64(time.Hour))).Truncate(time.Minute)
				departure := rng.IntN(2) == 0
				x := ends[max(0, pick(rng, endW))]
				f, ok := g.transit(n, x, departure, o, at)
				if !ok || f.Enter.Before(from) || !f.Enter.Before(to) {
					continue
				}
				out = append(out, f)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Enter.Before(out[j].Enter) })
	return out
}

// transit is neighbour n's movement to (departure) or from x as a transit,
// at its time at n (STD or STA) at; false when it is none: too short, its
// great circle misses the core, or its floor would be in view.
func (g *scheduler) transit(n, x ScheduleAirport, departure bool, o TransitOptions, at time.Time) (Flight, bool) {
	if n.ICAO == x.ICAO {
		return Flight{}, false
	}
	a, b := x, n // origin, destination
	if departure {
		a, b = n, x
	}
	dist := calc.HaversineNM(a.Position.Lat, a.Position.Lon, b.Position.Lat, b.Position.Lon)
	if dist < transitMinNM {
		return Flight{}, false
	}
	if xt := math.Abs(calc.CrossTrackMeters(a.Position.Lat, a.Position.Lon, b.Position.Lat, b.Position.Lon, o.Centre.Lat, o.Centre.Lon)) / 1852; xt > o.CoreNM {
		return Flight{}, false
	}
	in, outAt, ok := crossing(a.Position, b.Position, dist, o.Centre, o.RadiusNM)
	if !ok || outAt-in < o.RadiusNM/2 {
		return Flight{}, false
	}
	// An airline serving both ends, a type for the distance.
	var ws []float64
	for _, al := range g.cfg.Airlines {
		w := al.Weight
		serves := func(p ScheduleAirport) bool {
			return contains(al.Bases, p.ICAO) || contains(al.Regions, "*") || contains(al.Regions, p.ICAO[:2])
		}
		switch {
		case !serves(a) || !serves(b):
			w = 0
		case contains(al.Bases, n.ICAO):
			w *= 4
		}
		ws = append(ws, w)
	}
	ai := pick(g.rng, ws)
	if ai < 0 {
		return Flight{}, false
	}
	al := g.cfg.Airlines[ai]
	typ, ok := g.fleetType(al, dist, math.Min(a.RunwayM, b.RunwayM))
	if !ok {
		return Flight{}, false
	}
	// Where it appears (a departure) or goes (an arrival) at its floor:
	// out of view, and its way in view from where it comes into the area
	// to where it leaves it.
	floor := FloorAlongNM(typ, o.FloorFt, departure)
	start, end := in, outAt
	if departure {
		start = math.Max(in, floor)
	} else {
		end = math.Min(outAt, dist-floor)
	}
	if end-start < o.RadiusNM/2 {
		return Flight{}, false
	}
	// Over the core in view: its nearest point to the centre between the
	// two.
	if along := calc.AlongTrackMeters(a.Position.Lat, a.Position.Lon, b.Position.Lat, b.Position.Lon, o.Centre.Lat, o.Centre.Lon) / 1852; along < start || along > end {
		return Flight{}, false
	}
	pointAt := func(d float64) airport.LatLon {
		lat, lon := calc.IntermediatePoint(a.Position.Lat, a.Position.Lon, b.Position.Lat, b.Position.Lon, d/dist)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	for _, d := range []float64{start, end} {
		if p := pointAt(d); calc.HaversineNM(o.Centre.Lat, o.Centre.Lon, p.Lat, p.Lon) < o.ViewNM {
			return Flight{}, false
		}
	}
	kts := cruiseKts(typ)
	f := Flight{Callsign: g.callsign(al.ICAO), Airline: al.ICAO, Type: typ, Origin: a.ICAO, Destination: b.ICAO, DistanceNM: math.Round(dist)}
	if departure {
		f.STD = at.Truncate(5 * time.Minute)
		f.STA = f.STD.Add(blockTime(typ, dist))
		climb := 10 * time.Minute
		f.Enter = f.STD.Add(climb + time.Duration(start/kts*float64(time.Hour))).Truncate(time.Minute)
		f.Exit = f.STD.Add(climb + time.Duration(end/kts*float64(time.Hour)))
	} else {
		f.STA = at.Truncate(5 * time.Minute)
		f.STD = f.STA.Add(-blockTime(typ, dist))
		descent := 10 * time.Minute
		f.Enter = f.STA.Add(-descent - time.Duration((dist-start)/kts*float64(time.Hour))).Truncate(time.Minute)
		f.Exit = f.STA.Add(-descent - time.Duration((dist-end)/kts*float64(time.Hour)))
	}
	return f, true
}
