package world

import (
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// taxiCrossings are the runways route r crosses on its way, other than rwy
// (its own: the departure's or the one it landed on), as the taxi clearance
// names them: cross, those not in use crossed before the first in use,
// cleared with the taxi ("cross runway 12": the user, live LKPR, FLBUB
// stopped at 12, a runway nobody used, and asked); hold, the others, held
// short of ("hold short of runway 06": Doc 4444 12.3.4.7 e). One designator
// each, as said.
func (it *controlled) taxiCrossings(r *airport.Route, rwy string) (cross, hold []string) {
	if r == nil || it.graph == nil {
		return nil, nil
	}
	g := it.graph
	var holds []int
	for _, n := range r.Nodes {
		if hs := g.Nodes[n].HoldShort; hs != nil && !hs.ILS && hs.Runway >= 0 && hs.Runway < len(g.Layout.Runways) {
			holds = append(holds, hs.Runway)
		}
	}
	inUse := map[string]bool{}
	for _, arrival := range []bool{true, false} {
		for _, e := range it.cc.runwaysInUse(g, arrival) {
			inUse[e.Name] = true
		}
	}
	var seen []int
	for k := 1; k < len(holds); k++ {
		idx := holds[k]
		if holds[k-1] != idx || slices.Contains(seen, idx) {
			continue // a crossing: two holding points of one runway in a row
		}
		name := g.Layout.Runways[idx].Name()
		ends := strings.Split(name, "/")
		if rwy != "" && slices.Contains(ends, rwy) {
			continue
		}
		seen = append(seen, idx)
		active := slices.ContainsFunc(ends, func(e string) bool { return inUse[e] })
		if !active && len(hold) == 0 {
			cross = append(cross, oneDesignator(name))
		} else {
			hold = append(hold, oneDesignator(name))
		}
	}
	return cross, hold
}

// taxiCrossingsNow are the runways it is cleared to cross with its taxi
// clearance now (taxiCrossings).
func (it *controlled) taxiCrossingsNow() []string {
	var r *airport.Route
	if it.dep != nil {
		r = it.dep.Route()
	} else if it.arr != nil {
		if p := it.arr.Plan(); p != nil {
			r = p.TaxiRoute()
		}
	}
	it.mu.Lock()
	rwy := it.view.Runway
	it.mu.Unlock()
	cross, _ := it.taxiCrossings(r, rwy)
	return cross
}
