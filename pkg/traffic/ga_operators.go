package traffic

import (
	"math/rand/v2"
	"time"
)

// GAKind is the kind of general aviation operator a light aircraft flies
// for (#565).
type GAKind string

const (
	// GASchool is a flying school: trainers, training circuits.
	GASchool GAKind = "school"
	// GAClub is an aero club: its members' aircraft, now and then a circuit.
	GAClub GAKind = "club"
	// GAPrivate is a private owner: in, a full stop, out.
	GAPrivate GAKind = "private"
)

// GAOperator is a general aviation operator based at an airport, with its
// fleet of light aircraft (registrations and types).
type GAOperator struct {
	Kind  GAKind
	Name  string // "LKPR flying school"
	Fleet []GAAircraft
	// Weight is its share of the airport's VFR flights.
	Weight float64
}

// GAAircraft is one aircraft of an operator's fleet.
type GAAircraft struct {
	Registration, Type string
}

// gaTypes are the types each kind of operator flies, by weight; a private
// owner flies VFRTypes.
var gaTypes = map[GAKind][]gaType{
	GASchool: {{"C152", 40}, {"C172", 35}, {"P28A", 15}, {"DA40", 10}},
	GAClub:   {{"C172", 40}, {"P28A", 30}, {"DA40", 20}, {"SR22", 10}},
}

// GAOperatorsAt are the operators based at icao, the same every time (by
// the airport only): a flying school with five trainers and an aero club
// with three aircraft, each with half and a quarter of the VFR flights,
// and private owners (no fleet: a fresh registration each flight) with the
// last quarter.
func GAOperatorsAt(icao string) []GAOperator {
	h := uint64(14695981039346656037)
	for i := 0; i < len(icao); i++ {
		h = (h ^ uint64(icao[i])) * 1099511628211
	}
	rng := rand.New(rand.NewPCG(h, 0x6a))
	seen := map[string]bool{}
	fleet := func(kind GAKind, n int) []GAAircraft {
		ts := gaTypesNow.Load()[kind]
		w := make([]float64, len(ts))
		for i, t := range ts {
			w[i] = t.Weight
		}
		var out []GAAircraft
		for len(out) < n {
			reg := VFRRegistration(icao, rng)
			if seen[reg] {
				continue
			}
			seen[reg] = true
			out = append(out, GAAircraft{Registration: reg, Type: ts[max(0, pick(rng, w))].Type})
		}
		return out
	}
	return []GAOperator{
		{Kind: GASchool, Name: icao + " flying school", Fleet: fleet(GASchool, 5), Weight: 50},
		{Kind: GAClub, Name: icao + " aero club", Fleet: fleet(GAClub, 3), Weight: 25},
		{Kind: GAPrivate, Name: "private", Weight: 25},
	}
}

// gaFleetHours: a fleet aircraft flies only in every gaFleetHours-th hour
// (by its place in the fleet), so its flights, each over within two hours
// (in, circuits, parked RemoveParkedAfter; or out of the zone), never
// overlap one from an earlier hour, whichever hours the Source is asked
// for.
const gaFleetHours = 3

// gaBusy is how long a flight keeps its aircraft from another flight in
// the same hour: from its appearance to after it is gone.
func gaBusy(f Flight) (from, to time.Time) {
	if f.Origin != "" {
		return f.STD.Add(-DepartureLead), f.STA
	}
	return f.STD, f.STA.Add(time.Duration(f.TouchAndGos)*7*time.Minute + 35*time.Minute)
}

// DepartureLead is how long before its STD a VFR departure appears (on
// its stand).
const DepartureLead = 10 * time.Minute

// gaFlight makes f a flight of an operator at icao in hour h: its kind
// picked by weight, then a free aircraft of its fleet (a private owner's
// when none is), its circuits by kind. busy are the fleet's flights so far
// in this hour.
func gaFlight(f *Flight, ops []GAOperator, h time.Time, rng *rand.Rand, busy map[string][]Flight, icao string) {
	w := make([]float64, len(ops))
	for i, o := range ops {
		w[i] = o.Weight
	}
	op := ops[max(0, pick(rng, w))]
	kind, name := GAPrivate, "private"
	if len(op.Fleet) > 0 {
		hour := int(h.Unix() / 3600)
		for i, a := range op.Fleet {
			if (hour+i)%gaFleetHours != 0 {
				continue
			}
			cand := *f
			cand.Callsign, cand.Type = a.Registration, a.Type
			circuits(&cand, op.Kind, rng)
			from, to := gaBusy(cand)
			free := true
			for _, b := range busy[a.Registration] {
				bf, bt := gaBusy(b)
				if from.Before(bt) && bf.Before(to) {
					free = false
					break
				}
			}
			if free {
				*f = cand
				f.Operator = op.Name
				busy[a.Registration] = append(busy[a.Registration], *f)
				return
			}
		}
	}
	f.Operator, f.Callsign = name, VFRRegistration(icao, rng)
	circuits(f, kind, rng)
}

// circuits sets an arrival's touch-and-goes by its operator's kind: a
// school's two to four (one in three of them stop-and-goes), a club's one
// or two one time in three, a private owner's none.
func circuits(f *Flight, kind GAKind, rng *rand.Rand) {
	f.TouchAndGos, f.StopAndGo = 0, false
	if f.Origin != "" {
		return
	}
	switch kind {
	case GASchool:
		f.TouchAndGos = 2 + rng.IntN(3)
		f.StopAndGo = rng.IntN(3) == 0
	case GAClub:
		if rng.IntN(3) == 0 {
			f.TouchAndGos = 1 + rng.IntN(2)
		}
	}
}
