package nav

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// A route as filed text ("DCT VLM UL86 KEPAD DCT OKG"): fixes, and between
// two fixes the airway flown from one to the other. Speed and level groups
// (N0450F350, "VLM/N0450F350"), DCT and tokens the graph does not know
// (a SID or STAR name, an airport) are passed over.

// speedLevel is a speed and level group: N0450F350, M082F370, K0830S1130.
var speedLevel = regexp.MustCompile(`^[NKM]\d{3,4}[FASM]\d{3,4}$`)

// ExpandRoute is text's route over g: every fix, with each airway walked
// from one fix to the next. A fix ident with several fixes is the one
// nearest the point before (near, at the start). It returns the steps and
// the tokens passed over as not known; an error when an airway does not
// join its two fixes, or no fix is known at all.
func (g *AirwayGraph) ExpandRoute(text string, near airport.LatLon) ([]RouteStep, []string, error) {
	g.ensureLinked()
	var tokens []string
	for _, t := range strings.Fields(strings.ToUpper(text)) {
		t, _, _ = strings.Cut(t, "/")
		if t == "" || t == Direct || speedLevel.MatchString(t) {
			continue
		}
		tokens = append(tokens, t)
	}
	var steps []RouteStep
	var skipped []string
	at := near
	var last *Fix
	airway := ""
	for _, t := range tokens {
		if last != nil && airway == "" && g.hasAirway(t) {
			airway = t
			continue
		}
		f, ok := g.nearestFix(t, at)
		if !ok {
			if airway != "" {
				return nil, skipped, fmt.Errorf("nav: airway %s ends at %s, not a known fix", airway, t)
			}
			skipped = append(skipped, t)
			continue
		}
		if last != nil && airway != "" {
			walk, err := g.alongAirway(last.Key(), f.Key(), airway)
			if err != nil {
				return nil, skipped, err
			}
			steps = append(steps, walk...)
		} else {
			d := calc.HaversineNM(at.Lat, at.Lon, f.Position.Lat, f.Position.Lon)
			if last == nil {
				d = 0
			}
			steps = append(steps, RouteStep{Airway: Direct, Fix: f.Key(), Position: f.Position, DistanceNM: d})
		}
		airway, at, last = "", f.Position, &f
	}
	if airway != "" {
		skipped = append(skipped, airway) // an airway with no fix after it
	}
	if len(steps) == 0 {
		return nil, skipped, fmt.Errorf("nav: no known fix in %q", text)
	}
	return steps, skipped, nil
}

// ensureLinked builds the index and adjacency of a graph read from JSON.
func (g *AirwayGraph) ensureLinked() {
	if g.adj == nil {
		g.link()
	}
}

func (g *AirwayGraph) hasAirway(name string) bool {
	for _, a := range g.Airways {
		if a.Name == name {
			return true
		}
	}
	return false
}

// nearestFix is the fix ident nearest p.
func (g *AirwayGraph) nearestFix(ident string, p airport.LatLon) (Fix, bool) {
	best, bestNM := Fix{}, math.Inf(1)
	for _, f := range g.Find(ident) {
		if d := calc.HaversineNM(p.Lat, p.Lon, f.Position.Lat, f.Position.Lon); d < bestNM {
			best, bestNM = f, d
		}
	}
	return best, !math.IsInf(bestNM, 1)
}

// alongAirway is the airway's fixes from from (left out) to to, by its
// segments only.
func (g *AirwayGraph) alongAirway(from, to FixKey, airway string) ([]RouteStep, error) {
	prev := map[FixKey]FixKey{from: from}
	queue := []FixKey{from}
	for len(queue) > 0 && !has(prev, to) {
		k := queue[0]
		queue = queue[1:]
		for _, e := range g.adj[k] {
			if e.Airway != airway || has(prev, e.To) {
				continue
			}
			prev[e.To] = k
			queue = append(queue, e.To)
		}
	}
	if !has(prev, to) {
		return nil, fmt.Errorf("%w: %s does not join %s and %s", ErrNoRoute, airway, from.Ident, to.Ident)
	}
	var keys []FixKey
	for k := to; k != from; k = prev[k] {
		keys = append(keys, k)
	}
	steps := make([]RouteStep, 0, len(keys))
	at, _ := g.Fix(from)
	for i := len(keys) - 1; i >= 0; i-- {
		f, _ := g.Fix(keys[i])
		steps = append(steps, RouteStep{Airway: airway, Fix: keys[i], Position: f.Position,
			DistanceNM: calc.HaversineNM(at.Position.Lat, at.Position.Lon, f.Position.Lat, f.Position.Lon)})
		at = f
	}
	return steps, nil
}

func has(m map[FixKey]FixKey, k FixKey) bool {
	_, ok := m[k]
	return ok
}
