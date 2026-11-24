package world

import (
	"sync"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// An arrival is not parked beside a runway entry (#1694): on a stand there
// its body comes within the wingtip clearance of every departure turning
// onto the runway past it (TFFJ 7W, 46 m off the centreline of 10 by its
// threshold: a DHC-6 lining up passed one parked there 8 m nose to body).
// Such a stand is given only where no other fits, and logged.

// entryPaths are the runway entry paths of a graph, by graph: each a
// sampled line from the last taxiway node onto the runway (worked out once).
var entryPaths sync.Map // *airport.Graph -> [][]airport.LatLon

// runwayEntryPaths are g's runway entry paths, onto every runway end.
func runwayEntryPaths(g *airport.Graph) [][]airport.LatLon {
	if v, ok := entryPaths.Load(g); ok {
		return v.([][]airport.LatLon)
	}
	var out [][]airport.LatLon
	for _, r := range g.Layout.Runways {
		for _, end := range []string{r.Primary.Name, r.Secondary.Name} {
			entries, err := g.RunwayEntries(end)
			if err != nil {
				continue
			}
			for _, e := range entries {
				var pts []airport.LatLon
				for _, n := range e.Path {
					if n >= 0 && int(n) < len(g.Nodes) {
						pts = append(pts, g.Nodes[n].Position)
					}
				}
				if len(pts) > 1 {
					out = append(out, sampledWay(pts))
				}
			}
		}
	}
	v, _ := entryPaths.LoadOrStore(g, out)
	return v.([][]airport.LatLon)
}

// besideEntry is whether an aircraft of model parked on stand i of g comes
// within the wingtip clearance of a runway entry path: its half span and
// that of one of its size passing, and airport.DefaultWingtipMargin
// (meetsPlace, as the apron's ways are kept clear).
func (cc *controlCenter) besideEntry(g *airport.Graph, i int, model string) bool {
	if g == nil || i < 0 || i >= len(g.Layout.Parking) {
		return false
	}
	place := cc.placeOnStand(g, i, model)
	for _, pts := range runwayEntryPaths(g) {
		if meetsPlace(apronWay{pts, place.HalfSpan}, place) {
			return true
		}
	}
	return false
}

// entryStands are g's stands that fit half span half and that an
// aircraft of model parked on would stand beside a runway entry
// (besideEntry).
func (cc *controlCenter) entryStands(g *airport.Graph, half float64, model string) []int {
	var out []int
	for _, i := range g.Layout.SuitableStands(half) {
		if cc.besideEntry(g, i, model) {
			out = append(out, i)
		}
	}
	return out
}
