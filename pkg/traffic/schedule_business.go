//go:build windows
// +build windows

package traffic

import (
	"math"
	"math/rand/v2"
	"sort"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// Business aviation (#619): at a large airport general aviation is mostly
// business jets and turboprops flying IFR between airports, with a flight
// plan, SID and STAR, parked on the GA apron. Light singles with their
// training circuits stay at the smaller fields (VFRFlights).

// BusinessOptions steer BusinessFlights.
type BusinessOptions struct {
	// Focus are the airports; Layouts tell the large ones (LargeAirport).
	Focus   []string
	Layouts map[string]*airport.Layout
	// PerHour is the mean number of business arrivals an hour at a large
	// airport at the peak, and as many departures; 0 means
	// BusinessPerHour. Density scales it (0 means 1).
	PerHour, Density float64
	Seed             uint64
}

// BusinessPerHour is the default mean of business arrivals (and of
// departures) an hour at a large airport at the peak of the day.
const BusinessPerHour = 1.5

// BusinessTypes are the types of the business flights, by weight; their
// cruise speed (kt, published, a little below the maximum) and range (NM,
// published, with reserves left) give the block time and how far they go.
var BusinessTypes = []struct {
	Type          string
	Weight        float64
	CruiseKts     float64
	MaxNM, MinRwy float64
}{
	{"E55P", 14, 440, 1800, 1300}, {"C25C", 10, 430, 1900, 1350}, {"C56X", 10, 420, 1800, 1400},
	{"C68A", 10, 430, 2400, 1400}, {"PC12", 8, 270, 1500, 1000}, {"C25B", 6, 400, 1800, 1300},
	{"C680", 6, 440, 2800, 1400}, {"C700", 6, 460, 3100, 1900}, {"E550", 6, 450, 3500, 1750},
	{"PC24", 6, 420, 1800, 1250}, {"B350", 6, 300, 1600, 1300}, {"E545", 5, 450, 2900, 1700},
	{"E50P", 4, 380, 1000, 1300}, {"TBM9", 4, 310, 1500, 1000}, {"BE20", 4, 290, 1500, 1000},
	{"C750", 3, 500, 3000, 2100},
}

// businessMinNM: a business flight goes at least this far.
const businessMinNM = 100.0

// LargeAirport reports whether an airport is large for general aviation:
// a runway of LargeRunwayMeters or more and LargeGates gates or more (LKPR
// 3712 m and 27 gates; LKTB 2650 m and none).
func LargeAirport(l *airport.Layout) bool {
	if l == nil {
		return false
	}
	gates, longest := 0, 0.0
	for _, p := range l.Parking {
		if p.IsGate() {
			gates++
		}
	}
	for _, r := range l.Runways {
		longest = math.Max(longest, r.Length)
	}
	return longest >= LargeRunwayMeters && gates >= LargeGates
}

// The size of a large airport (LargeAirport).
const (
	LargeRunwayMeters = 3000.0
	LargeGates        = 10
)

// BusinessFlights generates the business flights in to and out of the large
// focus airports between from and to: arrivals and as many departures,
// following the day's waves, each to or from another airport of cfg within
// its type's range and runway, its call sign a registration of the
// country it is based in (the focus airport's or the other end's).
// Operator "business"; Rules "" (IFR).
func BusinessFlights(cfg ScheduleConfig, opts BusinessOptions, from, to time.Time) []Flight {
	if opts.PerHour <= 0 {
		opts.PerHour = BusinessPerHour
	}
	if opts.Density <= 0 {
		opts.Density = 1
	}
	rng := rand.New(rand.NewPCG(opts.Seed, 0xb12))
	g := scheduler{cfg: cfg, rng: rng, used: map[string]bool{}, airports: map[string]ScheduleAirport{}}
	for _, a := range cfg.Airports {
		g.airports[a.ICAO] = a
	}
	weights := make([]float64, len(BusinessTypes))
	for i, t := range BusinessTypes {
		weights[i] = t.Weight
	}
	var out []Flight
	for _, icao := range opts.Focus {
		l := opts.Layouts[icao]
		if !LargeAirport(l) {
			continue
		}
		focus, ok := g.focusAirport(icao, l)
		if !ok {
			continue
		}
		offset := time.Duration(focus.Position.Lon / 15 * float64(time.Hour))
		for h := from.Truncate(time.Hour); h.Before(to); h = h.Add(time.Hour) {
			local := h.Add(offset).UTC().Hour()
			n := g.count(opts.PerHour * opts.Density * cfg.Waves[local])
			for i := 0; i < 2*n; i++ {
				at := h.Add(time.Duration(rng.Float64() * float64(time.Hour))).Truncate(5 * time.Minute)
				if at.Before(from) || !at.Before(to) {
					continue
				}
				if f, ok := g.businessFlight(focus, at, i%2 == 0, weights); ok {
					out = append(out, f)
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return focusTime(out[i], opts.Focus).Before(focusTime(out[j], opts.Focus)) })
	return out
}

// businessFlight draws one business arrival at (or departure from) focus
// at time at: a type by weight, an airport in its range with its runway,
// a registration.
func (g *scheduler) businessFlight(focus ScheduleAirport, at time.Time, arrival bool, weights []float64) (Flight, bool) {
	for tries := 0; tries < 8; tries++ {
		bt := BusinessTypes[max(0, pick(g.rng, weights))]
		var ends []ScheduleAirport
		for _, a := range g.cfg.Airports {
			if a.ICAO == focus.ICAO || a.RunwayM > 0 && a.RunwayM < bt.MinRwy {
				continue
			}
			d := calc.HaversineMeters(focus.Position.Lat, focus.Position.Lon, a.Position.Lat, a.Position.Lon) / 1852
			if d >= businessMinNM && d <= bt.MaxNM {
				ends = append(ends, a)
			}
		}
		if len(ends) == 0 {
			continue
		}
		other := ends[g.rng.IntN(len(ends))]
		dist := calc.HaversineMeters(focus.Position.Lat, focus.Position.Lon, other.Position.Lat, other.Position.Lon) / 1852
		// Based at the focus airport or at the other end.
		home := focus.ICAO
		if g.rng.Float64() < 0.4 {
			home = other.ICAO
		}
		cs := VFRRegistration(home, g.rng)
		for g.used[cs] {
			cs = VFRRegistration(home, g.rng)
		}
		g.used[cs] = true
		block := time.Duration((20.0/60 + dist/bt.CruiseKts) * float64(time.Hour)).Round(5 * time.Minute)
		f := Flight{Callsign: cs, Type: bt.Type, Operator: "business", DistanceNM: math.Round(dist)}
		if arrival {
			f.Origin, f.Destination, f.STA, f.STD = other.ICAO, focus.ICAO, at, at.Add(-block)
		} else {
			f.Origin, f.Destination, f.STD, f.STA = focus.ICAO, other.ICAO, at, at.Add(block)
		}
		return f, true
	}
	return Flight{}, false
}
