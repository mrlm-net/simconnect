//go:build windows
// +build windows

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
	// PerHour is the mean number of VFR arrivals an hour at each airport;
	// 0 means VFRPerHour. Density scales it (0 means 1).
	PerHour, Density float64
	Seed             uint64
	// Visual reports whether the weather at an airport allows VFR flight
	// (visual conditions); nil: always.
	Visual func(icao string) bool
	// Lead is how long before its STA a flight appears (ManagerOptions.
	// VFRLead): it must be day from then until it is down. 0: 8 min.
	Lead time.Duration
}

// VFRPerHour is the default mean of VFR arrivals an hour at an airport.
const VFRPerHour = 1.0

// VFRTypes are the light aircraft a VFR flight is, by weight.
var VFRTypes = []struct {
	Type   string
	Weight float64
}{{"C172", 40}, {"P28A", 25}, {"C152", 15}, {"DA40", 12}, {"SR22", 8}}

// VFRFlights generates the light aircraft flying in to the focus airports
// through the circuit between from and to (#568): by day only (Daylight
// from Lead before the STA to a quarter of an hour after it) and in visual
// conditions (Visual), with the registration of the airport's country as
// call sign (VFRRegistration). Each has Rules "VFR", no origin and the
// airport as destination.
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
		mean := opts.PerHour * opts.Density
		for h := from.Truncate(time.Hour); h.Before(to); h = h.Add(time.Hour) {
			n := int(mean)
			if rng.Float64() < mean-float64(n) {
				n++
			}
			for i := 0; i < n; i++ {
				at := h.Add(time.Duration(rng.Float64() * float64(time.Hour))).Truncate(5 * time.Minute)
				if at.Before(from) || !at.Before(to) || !Daylight(pos, at.Add(-opts.Lead)) || !Daylight(pos, at.Add(15*time.Minute)) {
					continue
				}
				typ := VFRTypes[max(0, pick(rng, weights))].Type
				out = append(out, Flight{Callsign: VFRRegistration(icao, rng), Type: typ, Destination: icao,
					STA: at, STD: at.Add(-opts.Lead), Rules: "VFR"})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].STA.Before(out[j].STA) })
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
