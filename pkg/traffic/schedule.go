package traffic

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// Schedules (#367): the flights that should exist around the centre of
// the world — who departs and arrives where and when — from a table of
// airlines (fleets, bases, regions), airports (position, size, runway)
// and time-of-day waves. Deterministic for a seed.

// Flight is one scheduled flight.
type Flight struct {
	Callsign    string    `json:"callsign"` // "CSA123"
	Airline     string    `json:"airline"`  // ICAO
	Type        string    `json:"type"`     // ICAO type designator
	Origin      string    `json:"origin"`
	Destination string    `json:"destination"`
	STD         time.Time `json:"std"`
	STA         time.Time `json:"sta"`
	DistanceNM  float64   `json:"distanceNM"`
	// Enter and Exit: when an overflight crosses into and out of the area
	// (Overflights, #369); zero for other flights.
	Enter time.Time `json:"enter,omitempty"`
	Exit  time.Time `json:"exit,omitempty"`
	// Rules is "VFR" for a light aircraft flying in by day through the
	// circuit (VFRFlights); "" an airline's IFR flight.
	Rules string `json:"rules,omitempty"`
	// Operator is who flies a VFR flight: a GAOperator's name ("LKPR
	// flying school") or "private" (#565).
	Operator string `json:"operator,omitempty"`
	// TouchAndGos a VFR arrival makes before its full stop, each a
	// stop-and-go with StopAndGo (training circuits, by its operator).
	TouchAndGos int  `json:"touchAndGos,omitempty"`
	StopAndGo   bool `json:"stopAndGo,omitempty"`
	// Observed: a real aircraft seen by a feed (#841), flown from where it
	// was seen: spawned at once, not by STD or STA; nil a scheduled one.
	Observed *Sighting `json:"observed,omitempty"`
	// Path is a recorded or given way (#845), its points timed for a
	// replayed day: an arrival or overflight appears where the path has it
	// at its start and flies the rest (an arrival to where it meets a
	// STAR); a departure flies its SID, then the path.
	Path []PathPoint `json:"path,omitempty"`
	// Rotation names the airframe flying the flight (a registration, an
	// app's aircraft id): the legs of one rotation are one aircraft, an
	// arrival turning around at its stand into the rotation's next
	// departure from that airport, whatever the airline, type or turn
	// window (TrafficManager pairing). "" the schedule pairs it as before.
	Rotation string `json:"rotation,omitempty"`
}

// Airline is an airline the generator schedules.
type Airline struct {
	ICAO string `json:"icao"`
	Name string `json:"name,omitempty"`
	// Cargo: a freighter airline, on cargo stands first (#833).
	Cargo bool `json:"cargo,omitempty"`
	// Telephony is the radio call sign ("SPEEDBIRD").
	Telephony string `json:"telephony,omitempty"`
	// Fleet is the types it flies with their weights.
	Fleet map[string]float64 `json:"fleet"`
	// Bases are its home airports: most of its flights start or end there.
	Bases []string `json:"bases"`
	// Regions are the ICAO prefixes it serves ("LK", "ED"); "*" anywhere.
	Regions []string `json:"regions"`
	// Weight is its size against the other airlines.
	Weight float64 `json:"weight"`
}

// ScheduleAirport is an airport flights can go to or come from.
type ScheduleAirport struct {
	ICAO string `json:"icao"`
	// Name is the destination as ATC says it in a clearance ("Frankfurt",
	// "London Heathrow").
	Name     string         `json:"name,omitempty"`
	Position airport.LatLon `json:"position"`
	// Size: 3 hub, 2 major, 1 regional.
	Size int `json:"size"`
	// RunwayM is the longest runway.
	RunwayM float64 `json:"runwayM"`
	// Carriers are the airlines its STANDS name that the config does not know
	// (standCarriers, #1172): the scenery's own answer to who parks here.
	Carriers []Airline `json:"carriers,omitempty"`
}

// TypeLimits are what a type can fly: the range and the runway it needs.
type TypeLimits struct {
	MinNM, MaxNM float64
	RunwayM      float64
}

// ScheduleConfig is the generator's data; DefaultScheduleConfig is built
// in, LoadScheduleConfig reads a JSON file of the same shape (to edit).
type ScheduleConfig struct {
	Airlines []Airline         `json:"airlines"`
	Airports []ScheduleAirport `json:"airports"`
	// Waves are the movements by local hour (0–23) against the peak (1).
	Waves [24]float64 `json:"waves"`
	// Types are the range and runway of each type (others: 100–2500 NM,
	// 2000 m).
	Types map[string]TypeLimits `json:"types"`
}

// ScheduleOptions steer a Schedule.
type ScheduleOptions struct {
	// Focus are the airports to schedule traffic for (e.g. the traffic
	// picture's airports); flights there go to and come from the config's
	// airports.
	Focus []string
	// PeakPerHour is the movements (departures plus arrivals) in the peak
	// hour at a hub (size 3); majors get half, regionals a fifth. 0: 24.
	PeakPerHour float64
	// Density scales everything; 0 means 1.
	Density float64
	Seed    uint64
	// Layouts are the focus airports' layouts when loaded: runway lengths
	// and the airlines their stands name (home carriers) come from there.
	Layouts map[string]*airport.Layout
	// Used are call signs already flying or planned (the hours scheduled
	// before): none is given again (#98: numbers were unique only within one
	// call, and a repeat was dropped or two flights shared a call sign).
	Used map[string]bool
}

// Schedule generates the flights with a departure (at a focus airport) or
// an arrival (at one) between from and to, sorted by the time at the focus
// airport.
func Schedule(cfg ScheduleConfig, opts ScheduleOptions, from, to time.Time) []Flight {
	if opts.PeakPerHour <= 0 {
		opts.PeakPerHour = 24
	}
	if opts.Density <= 0 {
		opts.Density = 1
	}
	rng := rand.New(rand.NewPCG(opts.Seed, 0x5ced))
	g := scheduler{cfg: cfg, rng: rng, used: map[string]bool{}, airports: map[string]ScheduleAirport{}}
	for cs := range opts.Used {
		g.used[cs] = true
	}
	for _, a := range cfg.Airports {
		g.airports[a.ICAO] = a
	}
	var out []Flight
	for _, icao := range opts.Focus {
		focus, ok := g.focusAirport(icao, opts.Layouts[icao])
		if !ok {
			continue
		}
		shares := standShares(opts.Layouts[icao])
		// The stands' own carriers that the config does not know (#1172).
		known := map[string]bool{}
		for _, a := range g.cfg.Airlines {
			known[strings.ToUpper(a.ICAO)] = true
		}
		focus.Carriers = standCarriers(cfg, opts.Layouts[icao], known, icao)
		size := []float64{0, 0.2, 0.5, 1}[max(1, min(3, focus.Size))]
		// Local solar time from the longitude: waves follow the sun.
		offset := time.Duration(focus.Position.Lon / 15 * float64(time.Hour))
		for h := from.Truncate(time.Hour); h.Before(to); h = h.Add(time.Hour) {
			local := h.Add(offset).UTC().Hour()
			n := g.count(opts.PeakPerHour * size * opts.Density * cfg.Waves[local])
			for i := 0; i < n; i++ {
				// Timetables use whole five minutes.
				at := h.Add(time.Duration(rng.Float64() * float64(time.Hour))).Truncate(5 * time.Minute)
				if at.Before(from) || !at.Before(to) {
					continue
				}
				if f, ok := g.flight(focus, shares, at, rng.Float64() < 0.5); ok {
					out = append(out, f)
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return focusTime(out[i], opts.Focus).Before(focusTime(out[j], opts.Focus)) })
	return out
}

// focusTime is when a flight is at a focus airport: its departure from
// one, else its arrival.
func focusTime(f Flight, focus []string) time.Time {
	for _, icao := range focus {
		if f.Origin == icao {
			return f.STD
		}
	}
	return f.STA
}

type scheduler struct {
	cfg      ScheduleConfig
	rng      *rand.Rand
	used     map[string]bool // call signs
	airports map[string]ScheduleAirport
}

// count rounds mean up or down at random, keeping it on average.
func (g *scheduler) count(mean float64) int {
	n := int(mean)
	if g.rng.Float64() < mean-float64(n) {
		n++
	}
	return n
}

// focusAirport is a focus airport's data: from the config, completed from
// its layout (position, longest runway) when loaded.
func (g *scheduler) focusAirport(icao string, l *airport.Layout) (ScheduleAirport, bool) {
	a, ok := g.airports[icao]
	if l != nil {
		if !ok {
			a = ScheduleAirport{ICAO: icao, Position: airport.LatLon{Lat: l.Latitude, Lon: l.Longitude}, Size: 1}
		}
		for _, r := range l.Runways {
			a.RunwayM = math.Max(a.RunwayM, r.Length)
		}
		ok = true
	}
	return a, ok
}

// standShares is how much of an airport's parking each airline holds, as a
// fraction of the biggest holder's share (the biggest holder is 1).
//
// WHY A FRACTION AND NOT A FLAG. This used to answer `map[string]bool` — "is
// named on a stand" — and a flag earned the full home-carrier multiplier. At a
// hub that is most of the config: Munich's stands name 105 airlines, ten of the
// eighteen built-in carriers among them, so they all took the eightfold at once
// and it cancelled out. Measured over 480 flights at EDDM: easyJet 11%, KLM 10%,
// Lufthansa 9% — easyJet outflying Lufthansa at its own base — while Lufthansa
// Cityline, on 86 of the 180 stands, did not reach the top twelve.
//
// One stand is a visitor and eighty-six is a hub, and only a proportion can say
// so. Counted over STANDS, not parking: a fuel pump names nobody worth flying.
func standShares(l *airport.Layout) map[string]float64 {
	out := map[string]float64{}
	if l == nil {
		return out
	}
	// A SHARED STAND IS A SHARE OF ONE STAND. Munich's stand 0 names twenty-three
	// airlines and stand 100 names twenty-eight — a common pool, not twenty-eight
	// airlines each with a gate — while stand 102 names Lufthansa and Cityline
	// alone. Counting a whole stand per name made a member of the pool look as
	// established as the airline with the gate to itself, and the pools are large
	// enough to outnumber the hub carrier many times over. Each name on a stand
	// holds its fraction of it.
	held := map[string]float64{}
	most := 0.0
	for _, p := range l.Parking {
		if p.Size() == airport.StandNone {
			continue
		}
		codes := make([]string, 0, len(p.Airlines))
		for _, a := range p.Airlines {
			if code := strings.ToUpper(a); code != "" {
				codes = append(codes, code)
			}
		}
		for _, code := range codes {
			held[code] += 1 / float64(len(codes))
			most = math.Max(most, held[code])
		}
	}
	for code, n := range held {
		out[code] = n / math.Max(1e-9, most)
	}
	return out
}

// homeWeight is what an airline's presence on the stands multiplies its weight
// by: the biggest holder approaches the home carrier's eightfold, a single stand
// among many barely moves. `Bases` stays the explicit "this is its base" signal
// and keeps the full multiplier whatever the stands say.
func homeWeight(share float64) float64 { return 1 + 7*share }

// standCarriers are the airlines an airport's STANDS name that the config does
// not know (#1172).
//
// THE AIRPORT'S OWN ANSWER TO WHO FLIES THERE. `defaultAirlines` is eighteen
// carriers; Munich's stands name a hundred and five, and the biggest holder of
// them — CLH, Lufthansa Cityline, on eighty-six stands — could never be
// scheduled because it was not one of the eighteen. A stand assigned to an
// airline is the scenery saying that airline parks here, which is exactly the
// question the schedule is asking.
//
// WHAT THE STANDS CAN AND CANNOT SAY. They give the code, how many stands it
// holds, and how big they are. So: weight from the count, because an airline on
// eighty-six stands is a bigger presence than one on two; fleet from the widest
// stand it holds, because that is the largest thing it can park; the airport as
// its base and its region, because stands there are the only thing known about
// where it flies. No telephony — the radio then carries the code, which is what
// a normaliser with a telephony table turns back into a name.
func standCarriers(cfg ScheduleConfig, l *airport.Layout, known map[string]bool, icao string) []Airline {
	if l == nil {
		return nil
	}
	widest := map[string]float64{}
	count := map[string]int{}
	for _, p := range l.Parking {
		if p.Size() == airport.StandNone {
			continue
		}
		for _, a := range p.Airlines {
			code := strings.ToUpper(a)
			if code == "" || known[code] {
				continue
			}
			count[code]++
			widest[code] = math.Max(widest[code], p.Radius)
		}
	}
	region := icao[:min(2, len(icao))]
	out := make([]Airline, 0, len(count))
	for code := range count {
		out = append(out, Airline{
			ICAO: code, Fleet: fleetForStand(cfg, widest[code]),
			// NO BASE, deliberately: the stands say who PARKS here, not who owns
			// the place, and the config's `Bases` still says that. A base is the
			// one thing that still earns the full eightfold whatever the stands
			// hold, so handing it out per stand would hand it to all ninety-odd
			// of Munich's at once.
			Regions: []string{region},
			// A BASELINE, not a share. How much of the field is theirs is applied
			// once, by homeWeight, to these and to the config's airlines alike —
			// counting it here as well made presence quadratic for one list and
			// linear for the other, which is the scale mismatch this came from.
			// One is about the weight of a mid-sized config carrier.
			Weight: 1,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ICAO < out[j].ICAO })
	return out
}

// fleetForStand is what fits the widest stand an airline holds: the largest
// thing it can park is the largest thing it can fly in.
//
// NOT A LIST WRITTEN HERE. The types are the ones the CONFIG schedules — so a
// host equipped for its airlines is equipped for these — and which of them fit
// comes from the engine's own aircraft-type table, by wing span against the
// stand's radius, the same test `Layout.SuitableStands` makes. Nothing about an
// airline's real fleet is known from a stand, so the honest inference is "what
// this airport's other airlines fly, that fits".
//
// Weighted towards the big end of what fits: an airline given a 45 m stand is
// not there to park turboprops on it.
func fleetForStand(cfg ScheduleConfig, radiusM float64) map[string]float64 {
	if radiusM <= 0 {
		return nil
	}
	type fit struct {
		t    string
		span float64
	}
	var fits []fit
	for t := range cfg.Types {
		half := MotionProfileFor(t).SpanMeters / 2
		if half <= 0 || half > radiusM {
			continue
		}
		fits = append(fits, fit{t, half * 2})
	}
	if len(fits) == 0 {
		return nil
	}
	sort.Slice(fits, func(i, j int) bool { return fits[i].span > fits[j].span })
	// The four biggest that fit, heaviest weighted first.
	out := map[string]float64{}
	for i, f := range fits {
		if i >= 4 {
			break
		}
		out[f.t] = float64(4 - i)
	}
	return out
}

// flight draws one departure from (or arrival at) focus at time at.
func (g *scheduler) flight(focus ScheduleAirport, shares map[string]float64, at time.Time, arrival bool) (Flight, bool) {
	prefix := focus.ICAO[:min(2, len(focus.ICAO))]
	airlines := append(append([]Airline(nil), g.cfg.Airlines...), focus.Carriers...)
	var weights []float64
	// ONE RULE FOR PRESENCE, both lists. A configured base is the config saying the
	// airline owns the place and keeps the full eightfold; otherwise how much of the
	// parking the airline holds decides, so the field's own carrier leads its own
	// field and a visitor with one stand stays a visitor.
	//
	// A STAND ALSO SAYS IT FLIES HERE AT ALL, which is why presence is tested before
	// the region filter: the scenery assigning a gate to an airline is better evidence
	// than a region list written once for every airport.
	for _, a := range g.cfg.Airlines {
		w := a.Weight
		switch {
		case contains(a.Bases, focus.ICAO):
			w *= 8 // its base, as the config declares it
		case shares[a.ICAO] > 0:
			w *= homeWeight(shares[a.ICAO])
		case !contains(a.Regions, prefix) && !contains(a.Regions, "*"):
			w = 0
		}
		weights = append(weights, w)
	}
	for _, a := range focus.Carriers {
		weights = append(weights, a.Weight*homeWeight(shares[a.ICAO]))
	}
	for tries := 0; tries < 8; tries++ {
		ai := pick(g.rng, weights)
		if ai < 0 {
			return Flight{}, false
		}
		al := airlines[ai]
		other, ok := g.otherEnd(al, focus)
		if !ok {
			continue
		}
		dist := calc.HaversineMeters(focus.Position.Lat, focus.Position.Lon, other.Position.Lat, other.Position.Lon) / 1852
		typ, ok := g.fleetType(al, dist, math.Min(focus.RunwayM, other.RunwayM))
		if !ok {
			continue
		}
		f := Flight{Airline: al.ICAO, Type: typ, Callsign: g.callsign(al.ICAO), DistanceNM: math.Round(dist)}
		block := blockTime(typ, dist)
		if arrival {
			f.Origin, f.Destination, f.STA, f.STD = other.ICAO, focus.ICAO, at, at.Add(-block)
		} else {
			f.Origin, f.Destination, f.STD, f.STA = focus.ICAO, other.ICAO, at, at.Add(block)
		}
		return f, true
	}
	return Flight{}, false
}

// otherEnd is where a flight of al from or to focus goes: a base of the
// airline when focus is not one, else anywhere in its regions, bigger
// airports more often.
func (g *scheduler) otherEnd(al Airline, focus ScheduleAirport) (ScheduleAirport, bool) {
	if !contains(al.Bases, focus.ICAO) {
		var bases []ScheduleAirport
		for _, b := range al.Bases {
			if a, ok := g.airports[b]; ok {
				bases = append(bases, a)
			}
		}
		if len(bases) > 0 {
			return bases[g.rng.IntN(len(bases))], true
		}
	}
	var cands []ScheduleAirport
	var weights []float64
	for _, a := range g.cfg.Airports {
		if a.ICAO == focus.ICAO || (!contains(al.Regions, "*") && !contains(al.Regions, a.ICAO[:min(2, len(a.ICAO))])) {
			continue
		}
		cands = append(cands, a)
		weights = append(weights, float64(a.Size*a.Size))
	}
	i := pick(g.rng, weights)
	if i < 0 {
		return ScheduleAirport{}, false
	}
	return cands[i], true
}

// fleetType draws a type of al's fleet that flies dist and fits a runway
// of runwayM (0: unknown, any).
func (g *scheduler) fleetType(al Airline, dist, runwayM float64) (string, bool) {
	var types []string
	for t := range al.Fleet {
		types = append(types, t)
	}
	sort.Strings(types) // deterministic
	var weights []float64
	for _, t := range types {
		lim := g.limits(t)
		w := al.Fleet[t]
		if dist < lim.MinNM || dist > lim.MaxNM || (runwayM > 0 && runwayM < lim.RunwayM) {
			w = 0
		}
		weights = append(weights, w)
	}
	i := pick(g.rng, weights)
	if i < 0 {
		return "", false
	}
	return types[i], true
}

func (g *scheduler) limits(t string) TypeLimits {
	if l, ok := g.cfg.Types[t]; ok {
		return l
	}
	return TypeLimits{MinNM: 100, MaxNM: 2500, RunwayM: 2000}
}

// callsign is a flight number of airline not used yet in this schedule.
func (g *scheduler) callsign(airline string) string {
	for {
		c := fmt.Sprintf("%s%d", airline, 100+g.rng.IntN(1900))
		if !g.used[c] {
			g.used[c] = true
			return c
		}
	}
}

// blockTime is gate to gate: 20 minutes of taxi, climb and descent plus the
// distance at the type's cruise speed (450 kt jets, 300 kt turboprops).
func blockTime(typ string, distNM float64) time.Duration {
	return time.Duration((20.0/60 + distNM/cruiseKts(typ)) * float64(time.Hour)).Round(5 * time.Minute)
}

// cruiseKts is a type's cruise ground speed for the schedule.
func cruiseKts(typ string) float64 {
	switch typ {
	case "AT76", "AT75", "DH8D", "ATR":
		return 290
	case "B77W", "B789", "B788", "A359", "A333":
		return 480
	}
	return 450
}

func pick(rng *rand.Rand, weights []float64) int {
	total := 0.0
	for _, w := range weights {
		total += w
	}
	if total <= 0 {
		return -1
	}
	x := rng.Float64() * total
	for i, w := range weights {
		if x -= w; x < 0 {
			return i
		}
	}
	return len(weights) - 1
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}

// LoadScheduleConfig reads a ScheduleConfig from a JSON file.
func LoadScheduleConfig(path string) (ScheduleConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return ScheduleConfig{}, err
	}
	var c ScheduleConfig
	return c, json.Unmarshal(b, &c)
}

// SaveScheduleConfig writes c as JSON, to edit.
func SaveScheduleConfig(path string, c ScheduleConfig) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// OverflightOptions steer Overflights.
type OverflightOptions struct {
	// Centre and RadiusNM are the area (e.g. the traffic picture's).
	Centre   airport.LatLon
	RadiusNM float64
	// PerHour is the overflights in the peak hour (default 6), scaled by
	// the waves at the centre's local time and by Density (0: 1).
	PerHour, Density float64
	Seed             uint64
	// Exclude are airports whose own traffic is scheduled (the focus
	// airports): overflights neither start nor end there.
	Exclude []string
}

// Overflights generates flights between the config's airports outside the
// area whose route crosses it, entering it between from and to (#369).
// Each has Enter and Exit, when it crosses into and out of the area at its
// cruise speed. Deterministic for a seed.
func Overflights(cfg ScheduleConfig, o OverflightOptions, from, to time.Time) []Flight {
	if o.PerHour <= 0 {
		o.PerHour = 6
	}
	if o.Density <= 0 {
		o.Density = 1
	}
	if o.RadiusNM <= 0 {
		return nil
	}
	// Seeded by the hour asked too: the manager asks an hour at a time, and
	// one seed drew the same flights every hour, dropped as repeats of the
	// first (live, LKPR: three overflights at the start, then none for two
	// hours).
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
	offset := time.Duration(o.Centre.Lon / 15 * float64(time.Hour))
	var out []Flight
	for h := from.Truncate(time.Hour); h.Before(to); h = h.Add(time.Hour) {
		n := g.count(o.PerHour * o.Density * cfg.Waves[h.Add(offset).UTC().Hour()])
		for i := 0; i < n; i++ {
			enter := h.Add(time.Duration(rng.Float64() * float64(time.Hour))).Truncate(time.Minute)
			if enter.Before(from) || !enter.Before(to) {
				continue
			}
			if f, ok := g.overflight(outside, weights, o, enter); ok {
				out = append(out, f)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Enter.Before(out[j].Enter) })
	return out
}

// overflight draws one flight crossing the area, entering it at enter.
func (g *scheduler) overflight(outside []ScheduleAirport, weights []float64, o OverflightOptions, enter time.Time) (Flight, bool) {
	for tries := 0; tries < 30; tries++ {
		a, b := outside[pick(g.rng, weights)], outside[pick(g.rng, weights)]
		dist := calc.HaversineNM(a.Position.Lat, a.Position.Lon, b.Position.Lat, b.Position.Lon)
		if a.ICAO == b.ICAO || dist < 200 {
			continue
		}
		in, outAt, ok := crossing(a.Position, b.Position, dist, o.Centre, o.RadiusNM)
		if !ok || outAt-in < o.RadiusNM/2 {
			continue // misses the area, or only clips it
		}
		// An airline serving both ends (a base, or its regions): a base
		// at either end more often.
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
		ai := pick(g.rng, ws)
		if ai < 0 {
			continue
		}
		al := g.cfg.Airlines[ai]
		typ, ok := g.fleetType(al, dist, math.Min(a.RunwayM, b.RunwayM))
		if !ok {
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

// crossing finds where the route a → b (dist NM) enters and leaves the
// circle around c: the distances along it, sampled every 5 NM along the
// great circle the flight flies (#468: a straight line in latitude and
// longitude had Dublin–Seoul cross Prague; its great circle is over Norway).
func crossing(a, b airport.LatLon, dist float64, c airport.LatLon, radiusNM float64) (in, out float64, ok bool) {
	in = -1
	for d := 0.0; d <= dist; d += 5 {
		lat, lon := calc.IntermediatePoint(a.Lat, a.Lon, b.Lat, b.Lon, d/dist)
		p := airport.LatLon{Lat: lat, Lon: lon}
		if calc.HaversineNM(c.Lat, c.Lon, p.Lat, p.Lon) <= radiusNM {
			if in < 0 {
				in = d
			}
			out = d
		}
	}
	return in, out, in >= 0
}
