package traffic

import (
	"math/rand/v2"
	"sort"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// VFROptions steer VFRFlights.
type VFROptions struct {
	// Focus are the airports light aircraft fly in to through the circuit;
	// Layouts give their position (an airport without one is left out).
	Focus   []string
	Layouts map[string]*airport.Layout
	// PerHour is the mean number of VFR arrivals an hour at each airport,
	// and as many departures; 0 means VFRPerHour. Density scales it (0
	// means 1).
	PerHour, Density float64
	Seed             uint64
	// Visual reports whether the weather at an airport allows VFR flight
	// (visual conditions); nil: always.
	Visual func(icao string) bool
	// Lead is how long before its STA a flight appears (ManagerOptions.
	// VFRLead): it must be day from then until it is down. 0: 8 min.
	Lead time.Duration
}

// VFRPerHour is the default mean of VFR arrivals (and of departures) an
// hour at an airport.
const VFRPerHour = 1.0

// VFRTypes are the light aircraft a VFR flight is, by weight.
var VFRTypes = []struct {
	Type   string
	Weight float64
}{{"C172", 40}, {"P28A", 25}, {"C152", 15}, {"DA40", 12}, {"SR22", 8}}

// VFRLargeTypes are the VFR flights' types at a large airport (#619):
// mid-size aircraft flying between fields, not trainers.
var VFRLargeTypes = []struct {
	Type   string
	Weight float64
}{{"DA62", 25}, {"DA42", 20}, {"BE58", 15}, {"SR22", 20}, {"TBM9", 10}, {"PC12", 10}}

// VFRLargeShare scales the VFR flights at a large airport: most of its
// general aviation flies IFR (BusinessFlights).
const VFRLargeShare = 0.5

// VFRFlights generates the light aircraft flying in to the focus airports
// through the circuit, and out of them, between from and to (#568): by day
// only (Daylight from Lead before the STA to a quarter of an hour after it;
// a departure from DepartureLead, 10 min, before its STD to half an hour
// after it) and in visual conditions (Visual), with the registration of
// the airport's country as call sign (VFRRegistration), flown by one of
// the airport's operators (GAOperatorsAt, #565): a school's or a club's
// own aircraft, or a private owner's. Each has Rules
// "VFR"; an arrival no origin and the airport as destination, a
// departure the airport as origin and no destination (it leaves the
// circuit to an exit point, Circuit.Departure).
func VFRFlights(opts VFROptions, from, to time.Time) []Flight {
	if opts.PerHour <= 0 {
		opts.PerHour = VFRPerHour
	}
	if opts.Density <= 0 {
		opts.Density = 1
	}
	if opts.Lead <= 0 {
		opts.Lead = 8 * time.Minute
	}
	rng := rand.New(rand.NewPCG(opts.Seed, 0x7f12))
	weights := make([]float64, len(VFRTypes))
	for i, t := range VFRTypes {
		weights[i] = t.Weight
	}
	var out []Flight
	for _, icao := range opts.Focus {
		l := opts.Layouts[icao]
		if l == nil || opts.Visual != nil && !opts.Visual(icao) {
			continue
		}
		pos := airport.LatLon{Lat: l.Latitude, Lon: l.Longitude}
		ops := GAOperatorsAt(icao)
		mean := opts.PerHour * opts.Density
		// A large airport (#619): fewer VFR flights, mid-size aircraft
		// flying in from or out to another field, no schools or clubs, no
		// training circuits (its GA is mostly business: BusinessFlights).
		large := LargeAirport(l)
		types, tw := VFRTypes, weights
		if large {
			mean *= VFRLargeShare
			types, tw = VFRLargeTypes, make([]float64, len(VFRLargeTypes))
			for i, t := range VFRLargeTypes {
				tw[i] = t.Weight
			}
		}
		for h := from.Truncate(time.Hour); h.Before(to); h = h.Add(time.Hour) {
			n := int(mean)
			if rng.Float64() < mean-float64(n) {
				n++
			}
			busy := map[string][]Flight{}
			for i := 0; i < 2*n; i++ {
				at := h.Add(time.Duration(rng.Float64() * float64(time.Hour))).Truncate(5 * time.Minute)
				f := Flight{Type: types[max(0, pick(rng, tw))].Type, Rules: "VFR"}
				if i%2 == 0 { // an arrival
					f.Destination, f.STA, f.STD = icao, at, at.Add(-opts.Lead)
				} else {
					f.Origin, f.STD, f.STA = icao, at, at.Add(30*time.Minute)
				}
				if at.Before(from) || !at.Before(to) {
					continue
				}
				// Lit from its appearance until it is down (an arrival with its
				// circuits) or well away (a departure).
				first, last := f.STD.Add(-DepartureLead), f.STA
				if f.Origin == "" {
					first, last = f.STD, f.STA.Add(15*time.Minute)
				}
				if !Daylight(pos, first) || !Daylight(pos, last) {
					continue
				}
				if large {
					f.Operator, f.Callsign = "private", VFRRegistration(icao, rng)
				} else {
					gaFlight(&f, ops, h, rng, busy, icao)
				}
				if f.TouchAndGos > 0 && !Daylight(pos, f.STA.Add(time.Duration(f.TouchAndGos)*7*time.Minute+15*time.Minute)) {
					f.TouchAndGos, f.StopAndGo = 0, false // no circuits into the dusk: a full stop
				}
				out = append(out, f)
			}
		}
	}
	at := func(f Flight) time.Time {
		if f.Origin != "" {
			return f.STD
		}
		return f.STA
	}
	sort.Slice(out, func(i, j int) bool { return at(out[i]).Before(at(out[j])) })
	return out
}

// vfrRegistrations: an airport's country (its ICAO prefix) and the
// nationality mark of the light aircraft based there, with the number of
// letters after it. The marks are the ICAO nationality marks of these
// countries; elsewhere a random one of them.
var vfrRegistrations = []struct {
	prefix, mark string
	letters      int
}{
	{"LK", "OK", 3}, {"LZ", "OM", 3}, {"ED", "D", 4}, {"LO", "OE", 3}, {"EP", "SP", 3}, {"LH", "HA", 3},
	{"EG", "G", 4}, {"LF", "F", 4}, {"EH", "PH", 3}, {"EB", "OO", 3}, {"LS", "HB", 3},
}

// VFRRegistration is a light aircraft's registration (without the hyphen)
// for an airport: the nationality mark of its country and random letters,
// "OKABC" at LKPR.
func VFRRegistration(icao string, rng *rand.Rand) string {
	r := vfrRegistrations[rng.IntN(len(vfrRegistrations))]
	for _, c := range vfrRegistrations {
		if strings.HasPrefix(icao, c.prefix) {
			r = c
			break
		}
	}
	b := []byte(r.mark)
	for i := 0; i < r.letters; i++ {
		b = append(b, byte('A'+rng.IntN(26)))
	}
	return string(b)
}
