package airspace

import (
	"math"
	"sort"
)

// The terminal airspace around an airport (#1486): its control zone and
// the terminal areas stacked on it, by a generic rule on the records,
// with no name read: from the controlled record (class B to D) holding the
// airport from the surface, each next layer is the controlled record
// holding the airport whose floor is the layer below's ceiling, the
// lowest ceiling among them (a terminal area before the control area over
// it). The stack stops where no record goes on, or at a ceiling above
// topM (en-route airspace). LKPR: RUZYNE CTR to 3,500 ft, PRAGUE TMA II to
// 9,500 ft, PRAGUE TMA IX to 16,500 ft; LZKZ: KOSICE CTR, TMA 3, TMA 4
// to 13,500 ft. Crossing traffic keeps out of it: above each layer's
// ceiling inside its outline, or around it.

// stackStepM: a record's floor this near a layer's ceiling goes on from
// it (limits come rounded from feet and flight levels).
const stackStepM = 50.0

// surfaceM: a record whose floor is this near the ground starts at the
// surface.
const surfaceM = 100.0

// TerminalStack is the terminal airspace of the airport at lat/lon (field
// elevation groundM metres), bottom up: its layers' outlines (each with
// its record), none above topM metres MSL. Empty: no controlled record
// holds the airport from the surface.
func (s *Store) TerminalStack(lat, lon, groundM, topM float64) ([]*Shape, error) {
	at, err := s.At(lat, lon)
	if err != nil {
		return nil, err
	}
	var ctl []*Airspace
	for _, a := range at {
		if (a.Type == ClassB || a.Type == ClassC || a.Type == ClassD) && !a.NoLimits() {
			ctl = append(ctl, a)
		}
	}
	sort.Slice(ctl, func(i, j int) bool { return ctl[i].ID < ctl[j].ID })
	var out []*Shape
	floor, used := groundM, map[uint64]bool{}
	for first := true; ; first = false {
		var next *Airspace
		for _, a := range ctl {
			f, c := a.Floor(groundM), a.Ceiling(groundM)
			from := math.Abs(f-floor) <= stackStepM || first && f <= groundM+surfaceM
			if used[a.ID] || !from || c <= floor || c > topM {
				continue
			}
			if next == nil || c < next.Ceiling(groundM) {
				next = a
			}
		}
		if next == nil {
			return out, nil
		}
		sh, err := s.Shape(next)
		if err != nil {
			return out, nil // no outline: the stack ends there
		}
		used[next.ID] = true
		out = append(out, sh)
		floor = next.Ceiling(groundM)
	}
}

// InStack reports whether a point at altM metres MSL is in one of the
// stack's layers, from marginM below its floor to marginM above its
// ceiling (groundM for AGL limits).
func InStack(stack []*Shape, lat, lon, altM, groundM, marginM float64) bool {
	for _, sh := range stack {
		a := sh.Airspace
		if altM >= a.Floor(groundM)-marginM && altM <= a.Ceiling(groundM)+marginM && sh.Inside(lat, lon) {
			return true
		}
	}
	return false
}
