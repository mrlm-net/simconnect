package traffic

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/dict"
)

// Overflight streams and volume per area (#1483). Real traffic over an
// area runs in strong, place-specific streams (Western Europe to and from
// the Gulf and Asia, the North Atlantic's night eastbound and morning
// westbound waves, leisure flows to the Mediterranean) at very different
// volumes by region. With OverflightOptions.Flows the generator draws a
// stream first (by its weight and its hourly profile in UTC), then a city
// pair of the stream crossing the area and an airline serving both ends,
// so directions and times of day follow the stream; a stream's long
// distances get long-haul types by their range (TypeLimits).
//
// The streams come from a table per area (traffic.overflightFlows,
// replaceable at runtime like the others, pkg/dict): an area is a
// latitude/longitude box (a FIR's, roughly), with its peak volume per hour
// and its streams. An area with no entry gets the generic fallback: the
// volume from the airport list (sizes and the pairs whose great circle
// crosses the area), the pairs drawn by size as before, on the first focus
// airport's waves. The shipped entries are the project's estimates, not
// from a source.

// OverflightStream is a stream of overflights: from airports of one region
// to another's, regions as ICAO prefixes ("EG", "K"; "*" any).
type OverflightStream struct {
	Name string   `json:"name"`
	From []string `json:"from"`
	To   []string `json:"to"`
	// Weight is the stream's share of the area's volume against the
	// others'.
	Weight float64 `json:"weight"`
	// HoursUTC: the stream's volume by UTC hour (0–23) against its peak
	// (1); none: the same all day.
	HoursUTC []float64 `json:"hoursUTC,omitempty"`
	// MinNM: its pairs are at least this far apart (0: 200).
	MinNM float64 `json:"minNM,omitempty"`
}

// OverflightArea is an area's overflights (traffic.overflightFlows): the
// box it covers, the overflights an hour when every stream is at its peak
// (at density 1), and its streams.
type OverflightArea struct {
	Area string `json:"area"` // id: a FIR's ICAO code or a name
	Name string `json:"name,omitempty"`
	// The box, degrees (West > East: across the antimeridian).
	North       float64            `json:"north"`
	South       float64            `json:"south"`
	West        float64            `json:"west"`
	East        float64            `json:"east"`
	PeakPerHour float64            `json:"peakPerHour"`
	Streams     []OverflightStream `json:"streams"`
}

// Contains reports whether p is in the area's box.
func (a OverflightArea) Contains(p airport.LatLon) bool {
	if p.Lat > a.North || p.Lat < a.South {
		return false
	}
	if a.West <= a.East {
		return p.Lon >= a.West && p.Lon <= a.East
	}
	return p.Lon >= a.West || p.Lon <= a.East // across the antimeridian
}

// hourly is a profile of 24 values by UTC hour.
func hourly(v ...float64) []float64 { return v }

var (
	// Day: a morning and an evening wave, quiet at night (UTC, Europe).
	flowDay = hourly(0.1, 0.05, 0.05, 0.05, 0.2, 0.6, 0.9, 1, 0.9, 0.8, 0.8, 0.8, 0.8, 0.8, 0.8, 0.9, 1, 0.9, 0.8, 0.6, 0.4, 0.3, 0.2, 0.1)
	// Out to the Gulf and Asia: the long-haul departures from Western
	// Europe from late morning into the evening.
	flowEastLong = hourly(0.1, 0.05, 0.05, 0.05, 0.1, 0.2, 0.3, 0.4, 0.6, 0.8, 1, 0.9, 0.8, 0.9, 1, 1, 0.9, 0.9, 1, 0.9, 0.7, 0.5, 0.3, 0.2)
	// Back from the Gulf and Asia: the night departures arriving in the
	// European morning, and a second wave at midday.
	flowWestLong = hourly(0.4, 0.6, 0.8, 1, 1, 0.9, 0.7, 0.5, 0.4, 0.5, 0.8, 0.9, 0.8, 0.6, 0.4, 0.3, 0.3, 0.3, 0.2, 0.2, 0.2, 0.2, 0.3, 0.3)
	// Leisure: out in the morning, back in the afternoon and evening.
	flowLeisureOut  = hourly(0.05, 0.02, 0.02, 0.1, 0.4, 0.8, 1, 1, 0.9, 0.7, 0.5, 0.4, 0.4, 0.5, 0.6, 0.6, 0.5, 0.4, 0.3, 0.3, 0.2, 0.1, 0.05, 0.05)
	flowLeisureBack = hourly(0.1, 0.05, 0.02, 0.02, 0.05, 0.1, 0.2, 0.3, 0.4, 0.6, 0.8, 0.9, 1, 1, 0.9, 0.9, 1, 0.9, 0.8, 0.7, 0.5, 0.4, 0.3, 0.2)
	// The North Atlantic: westbound by day, eastbound arriving in the
	// European morning (through the night over the ocean).
	flowNATWest = hourly(0.05, 0.02, 0.02, 0.02, 0.05, 0.1, 0.3, 0.6, 0.9, 1, 1, 1, 0.9, 0.8, 0.7, 0.6, 0.5, 0.4, 0.3, 0.2, 0.1, 0.1, 0.05, 0.05)
	flowNATEast = hourly(0.3, 0.2, 0.3, 0.6, 0.9, 1, 1, 0.8, 0.5, 0.3, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2)
)

// Regions as ICAO prefixes.
var (
	regWestEurope = []string{"EG", "EI", "EH", "EB", "LF", "LE", "LP", "LS"}
	regGulfAsia   = []string{"OM", "OT", "OB", "OK", "OE", "OJ", "HE", "LT", "LC", "R", "V", "Z", "W"}
	regGermanyBNL = []string{"ED", "EH", "EB", "EL"}
	regSouthEast  = []string{"LH", "LR", "LB", "LY", "LW", "LQ", "LA", "LT", "LG", "LC", "U"}
	regNorth      = []string{"EK", "EN", "ES", "EF", "EV", "EY", "EE", "EP"}
	regMed        = []string{"LI", "LD", "LG", "LT", "LJ", "LM", "HE", "LC", "DT", "LY"}
	regPoland     = []string{"EP", "EY", "EV", "EE"}
	regWestSouth  = []string{"ED", "LF", "LS", "LE", "LP", "EG", "EH", "EB", "LI"}
	regNorthAm    = []string{"K", "C"}
	regEurope     = []string{"E", "L"}
	regIsles      = []string{"EI", "EG"}
	regContinent  = []string{"ED", "EH", "EB", "LF", "LS", "LO", "LK", "EP", "LI", "LE", "EK", "ES"}
)

var shippedOverflightAreas = []OverflightArea{
	{Area: "LKAA", Name: "Praha FIR", North: 51.2, South: 48.4, West: 11.9, East: 19.0, PeakPerHour: 36, Streams: []OverflightStream{
		{Name: "Western Europe to the Gulf and Asia", From: regWestEurope, To: regGulfAsia, Weight: 3, HoursUTC: flowEastLong},
		{Name: "The Gulf and Asia to Western Europe", From: regGulfAsia, To: regWestEurope, Weight: 3, HoursUTC: flowWestLong},
		{Name: "Germany and Benelux to south-east Europe", From: regGermanyBNL, To: regSouthEast, Weight: 4, HoursUTC: flowDay},
		{Name: "South-east Europe to Germany and Benelux", From: regSouthEast, To: regGermanyBNL, Weight: 4, HoursUTC: flowDay},
		{Name: "The north to the Mediterranean", From: regNorth, To: regMed, Weight: 3, HoursUTC: flowLeisureOut},
		{Name: "The Mediterranean to the north", From: regMed, To: regNorth, Weight: 3, HoursUTC: flowLeisureBack},
		{Name: "Poland and the Baltics to the west", From: regPoland, To: regWestSouth, Weight: 2, HoursUTC: flowDay},
		{Name: "The west to Poland and the Baltics", From: regWestSouth, To: regPoland, Weight: 2, HoursUTC: flowDay},
	}},
	{Area: "EGTT-LON", Name: "London", North: 52.6, South: 50.4, West: -2.2, East: 1.8, PeakPerHour: 24, Streams: []OverflightStream{
		{Name: "North Atlantic westbound", From: regEurope, To: regNorthAm, Weight: 3, HoursUTC: flowNATWest, MinNM: 2000},
		{Name: "North Atlantic eastbound", From: regNorthAm, To: regEurope, Weight: 3, HoursUTC: flowNATEast, MinNM: 2000},
		{Name: "Ireland and the North to the continent", From: regIsles, To: regContinent, Weight: 3, HoursUTC: flowDay},
		{Name: "The continent to Ireland and the North", From: regContinent, To: regIsles, Weight: 3, HoursUTC: flowDay},
		{Name: "Ireland and the North to the Mediterranean", From: regIsles, To: append(slices.Clone(regMed), "LE", "LP", "GC"), Weight: 2, HoursUTC: flowLeisureOut},
		{Name: "The Mediterranean to Ireland and the North", From: append(slices.Clone(regMed), "LE", "LP", "GC"), To: regIsles, Weight: 2, HoursUTC: flowLeisureBack},
	}},
}

var overflightAreasNow dict.Value[[]OverflightArea]

func init() {
	dict.Register(dict.Keyed("traffic.overflightFlows", "area", "", "",
		func() []OverflightArea { return slices.Clone(shippedOverflightAreas) },
		func(a OverflightArea) string { return strings.ToUpper(a.Area) },
		func(items []OverflightArea) { overflightAreasNow.Store(items) }))
	_ = dict.Reset("traffic.overflightFlows")
}

// OverflightAreaAt is the area of the flows table holding p, the smallest
// box when several do; ok false: none (the generic fallback).
func OverflightAreaAt(p airport.LatLon) (OverflightArea, bool) {
	var best OverflightArea
	bestSize, found := math.Inf(1), false
	for _, a := range overflightAreasNow.Load() {
		if len(a.Streams) == 0 || !a.Contains(p) {
			continue
		}
		w := a.East - a.West
		if w < 0 {
			w += 360
		}
		if size := (a.North - a.South) * w; size < bestSize {
			best, bestSize, found = a, size, true
		}
	}
	return best, found
}

// streamShare is stream s's volume at UTC hour h against its peak.
func (s OverflightStream) share(h int) float64 {
	if len(s.HoursUTC) == 0 {
		return 1
	}
	return s.HoursUTC[h%len(s.HoursUTC)]
}

// OverflightRate is the mean overflights entering the area in the hour
// from at (density 1 unless o.Density): with o.Flows the area's streams
// (OverflightAreaAt the centre) or the generic fallback, else o.PerHour (0:
// 6) on the waves, as Overflights draws them.
func OverflightRate(cfg ScheduleConfig, o OverflightOptions, at time.Time) float64 {
	if o.Density <= 0 {
		o.Density = 1
	}
	clock := ""
	if len(o.Exclude) > 0 {
		clock = o.Exclude[0]
	}
	wave := cfg.Waves[LocalHour(clock, o.Centre, at)]
	if !o.Flows {
		if o.PerHour <= 0 {
			o.PerHour = 6
		}
		return o.PerHour * o.Density * wave
	}
	if area, ok := OverflightAreaAt(o.Centre); ok {
		peak := area.PeakPerHour
		if o.PerHour > 0 {
			peak = o.PerHour
		}
		total, w := 0.0, 0.0
		for _, s := range area.Streams {
			total += s.Weight * s.share(at.UTC().Hour())
			w += s.Weight
		}
		if w <= 0 {
			return 0
		}
		return peak * o.Density * total / w
	}
	peak := o.PerHour
	if peak <= 0 {
		peak = FallbackOverflightsPerHour(cfg, o.Centre, o.RadiusNM, o.Exclude)
	}
	return peak * o.Density * wave
}

// fallbackPairsPerOverflight: the generic fallback's volume is the sum
// over the airport pairs crossing the area of the product of their weights
// (size squared, as the pairs are drawn) over this, within
// fallbackMinPerHour and fallbackMaxPerHour: about 14 an hour over Prague
// with the shipped airports, a few over the edges of the list, none where
// no pair crosses.
const (
	fallbackPairsPerOverflight = 400.0
	fallbackMinPerHour         = 1.0
	fallbackMaxPerHour         = 40.0
)

var fallbackCache sync.Map // fallbackKey → float64

type fallbackKey struct {
	lat, lon, radius float64
	airports         int
	exclude          string
	first            string
}

// FallbackOverflightsPerHour is the generic fallback's peak volume of an
// area with no entry in the flows table (density 1): from the config's
// airports outside it whose great circle crosses it (sizes and distances);
// 0 when none does.
func FallbackOverflightsPerHour(cfg ScheduleConfig, centre airport.LatLon, radiusNM float64, exclude []string) float64 {
	if radiusNM <= 0 || len(cfg.Airports) < 2 {
		return 0
	}
	key := fallbackKey{math.Round(centre.Lat*100) / 100, math.Round(centre.Lon*100) / 100, radiusNM, len(cfg.Airports), strings.Join(exclude, ","), cfg.Airports[0].ICAO}
	if v, ok := fallbackCache.Load(key); ok {
		return v.(float64)
	}
	var outside []ScheduleAirport
	for _, a := range cfg.Airports {
		if contains(exclude, a.ICAO) || calc.HaversineNM(centre.Lat, centre.Lon, a.Position.Lat, a.Position.Lon) <= radiusNM {
			continue
		}
		outside = append(outside, a)
	}
	sum := 0.0
	for i, a := range outside {
		for _, b := range outside[i+1:] {
			if crossesArea(a.Position, b.Position, centre, radiusNM, 200) {
				sum += float64(a.Size * a.Size * b.Size * b.Size)
			}
		}
	}
	v := 0.0
	if sum > 0 {
		v = math.Min(math.Max(sum/fallbackPairsPerOverflight, fallbackMinPerHour), fallbackMaxPerHour)
	}
	fallbackCache.Store(key, v)
	return v
}

// crossesArea reports whether the great circle a → b, at least minNM long,
// crosses the circle around c for half its radius or more (Overflights'
// rule), from the cross-track distance: a quick test without sampling.
func crossesArea(a, b, c airport.LatLon, radiusNM, minNM float64) bool {
	dist := calc.HaversineNM(a.Lat, a.Lon, b.Lat, b.Lon)
	if dist < minNM {
		return false
	}
	xt := math.Abs(calc.CrossTrackMeters(a.Lat, a.Lon, b.Lat, b.Lon, c.Lat, c.Lon)) / 1852
	if xt > radiusNM {
		return false
	}
	half := math.Sqrt(radiusNM*radiusNM - xt*xt) // half the chord inside
	if 2*half < radiusNM/2 {
		return false
	}
	along := calc.AlongTrackMeters(a.Lat, a.Lon, b.Lat, b.Lon, c.Lat, c.Lon) / 1852
	return along-half >= 0 && along+half <= dist
}

// overflightsByFlows is Overflights with o.Flows: by the area's streams,
// else the generic fallback.
func overflightsByFlows(cfg ScheduleConfig, o OverflightOptions, from, to time.Time) []Flight {
	if o.Density <= 0 {
		o.Density = 1
	}
	if o.RadiusNM <= 0 {
		return nil
	}
	rng := rand.New(rand.NewPCG(o.Seed, 0x0f1e^uint64(from.Truncate(time.Hour).Unix())))
	g := scheduler{cfg: cfg, rng: rng, used: map[string]bool{}, airports: map[string]ScheduleAirport{}}
	var outside []ScheduleAirport
	var weights []float64
	for _, a := range cfg.Airports {
		g.airports[a.ICAO] = a
		if contains(o.Exclude, a.ICAO) || calc.HaversineNM(o.Centre.Lat, o.Centre.Lon, a.Position.Lat, a.Position.Lon) <= o.RadiusNM {
			continue
		}
		outside = append(outside, a)
		weights = append(weights, float64(a.Size*a.Size))
	}
	if len(outside) < 2 {
		return nil
	}
	area, byStream := OverflightAreaAt(o.Centre)
	var out []Flight
	for h := from.Truncate(time.Hour); h.Before(to); h = h.Add(time.Hour) {
		n := g.count(OverflightRate(cfg, o, h))
		for range n {
			enter := h.Add(time.Duration(rng.Float64() * float64(time.Hour))).Truncate(time.Minute)
			if enter.Before(from) || !enter.Before(to) {
				continue
			}
			var f Flight
			var ok bool
			if byStream {
				f, ok = g.streamOverflight(area, outside, o, h, enter)
			} else {
				f, ok = g.overflight(outside, weights, o, enter)
			}
			if ok {
				out = append(out, f)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Enter.Before(out[j].Enter) })
	return out
}

// streamOverflight draws a stream by its weight at hour h, then a pair of
// it crossing the area and an airline serving both.
func (g *scheduler) streamOverflight(area OverflightArea, outside []ScheduleAirport, o OverflightOptions, h, enter time.Time) (Flight, bool) {
	ws := make([]float64, len(area.Streams))
	for i, s := range area.Streams {
		ws[i] = s.Weight * s.share(h.UTC().Hour())
	}
	for tries := 0; tries < 6; tries++ {
		si := pick(g.rng, ws)
		if si < 0 {
			return Flight{}, false
		}
		if f, ok := g.pairOf(area.Streams[si], outside, o, enter); ok {
			return f, true
		}
	}
	return Flight{}, false
}

// inRegion reports whether icao is in one of the regions (ICAO prefixes,
// "*" any).
func inRegion(icao string, regions []string) bool {
	for _, r := range regions {
		if r == "*" || strings.HasPrefix(icao, strings.ToUpper(r)) {
			return true
		}
	}
	return false
}

// pairOf draws a city pair of stream s (its origin in From, destination in
// To, by size) whose route crosses the area, and its flight.
func (g *scheduler) pairOf(s OverflightStream, outside []ScheduleAirport, o OverflightOptions, enter time.Time) (Flight, bool) {
	var froms, tos []ScheduleAirport
	var wf, wt []float64
	for _, a := range outside {
		w := float64(a.Size * a.Size)
		if inRegion(a.ICAO, s.From) {
			froms, wf = append(froms, a), append(wf, w)
		}
		if inRegion(a.ICAO, s.To) {
			tos, wt = append(tos, a), append(wt, w)
		}
	}
	minNM := s.MinNM
	if minNM <= 0 {
		minNM = 200
	}
	for tries := 0; tries < 30; tries++ {
		i, j := pick(g.rng, wf), pick(g.rng, wt)
		if i < 0 || j < 0 {
			return Flight{}, false
		}
		a, b := froms[i], tos[j]
		if a.ICAO == b.ICAO || !crossesArea(a.Position, b.Position, o.Centre, o.RadiusNM, minNM) {
			continue
		}
		if f, ok := g.crossingFlight(a, b, o, enter); ok {
			return f, true
		}
	}
	return Flight{}, false
}

// crossingFlight is the flight a → b crossing the area, entering it at
// enter: an airline serving both ends (a base at either end more often), a
// type of its fleet for the distance; false when none does or the route
// misses the area.
func (g *scheduler) crossingFlight(a, b ScheduleAirport, o OverflightOptions, enter time.Time) (Flight, bool) {
	dist := calc.HaversineNM(a.Position.Lat, a.Position.Lon, b.Position.Lat, b.Position.Lon)
	in, outAt, ok := crossing(a.Position, b.Position, dist, o.Centre, o.RadiusNM)
	if !ok || outAt-in < o.RadiusNM/2 {
		return Flight{}, false
	}
	var ws []float64
	for _, al := range g.cfg.Airlines {
		serves := func(x ScheduleAirport) bool {
			return contains(al.Bases, x.ICAO) || contains(al.Regions, "*") || contains(al.Regions, x.ICAO[:2])
		}
		w := al.Weight
		switch {
		case !serves(a) || !serves(b):
			w = 0
		case contains(al.Bases, a.ICAO) || contains(al.Bases, b.ICAO):
			w *= 4
		}
		ws = append(ws, w)
	}
	for range 3 {
		ai := pick(g.rng, ws)
		if ai < 0 {
			return Flight{}, false
		}
		al := g.cfg.Airlines[ai]
		typ, ok := g.fleetType(al, dist, math.Min(a.RunwayM, b.RunwayM))
		if !ok {
			ws[ai] = 0 // none of its fleet flies that far: another airline
			continue
		}
		kts := cruiseKts(typ)
		climb := 10 * time.Minute
		std := enter.Add(-climb - time.Duration(in/kts*float64(time.Hour))).Truncate(time.Minute)
		return Flight{Callsign: g.callsign(al.ICAO), Airline: al.ICAO, Type: typ, Origin: a.ICAO, Destination: b.ICAO,
			STD: std, STA: std.Add(blockTime(typ, dist)), DistanceNM: math.Round(dist),
			Enter: enter, Exit: std.Add(climb + time.Duration(outAt/kts*float64(time.Hour)))}, true
	}
	return Flight{}, false
}

// String names the area for the log: "LKAA (Praha FIR)".
func (a OverflightArea) String() string {
	if a.Name == "" {
		return a.Area
	}
	return fmt.Sprintf("%s (%s)", a.Area, a.Name)
}
